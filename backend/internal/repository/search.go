package repository

import (
	"context"
	"siagars/backend/internal/domain"
	"siagars/backend/internal/service"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// hospitalMatch translates the database-expressible part of a search into a structured filter.
// Every condition mirrors service.Filter exactly (AND services, null bpjs never matches,
// bed availability must be fresh and non-simulation); the lowercase q comparison is added by the pipeline.
func hospitalMatch(s service.Search, now time.Time) bson.D {
	and := bson.A{bson.D{{Key: "source_active", Value: bson.D{{Key: "$ne", Value: false}}}}}
	switch s.Dataset {
	case "simulation":
		and = append(and, bson.D{{Key: "data_origin", Value: "simulation"}})
	case "all":
	default:
		and = append(and, bson.D{{Key: "data_origin", Value: bson.D{{Key: "$ne", Value: "simulation"}}}})
	}
	if s.Province != "" {
		and = append(and, bson.D{{Key: "province_code", Value: s.Province}})
	}
	if s.City != "" {
		and = append(and, bson.D{{Key: "city_code", Value: s.City}})
	}
	if s.BPJS != nil {
		and = append(and, bson.D{{Key: "bpjs", Value: *s.BPJS}})
	}
	for _, slug := range s.Services {
		and = append(and, bson.D{{Key: "services", Value: bson.D{{Key: "$elemMatch", Value: bson.D{{Key: "slug", Value: slug}, {Key: "status", Value: bson.D{{Key: "$in", Value: bson.A{"available", "limited"}}}}}}}}})
	}
	if s.HasBeds {
		// domain.Freshness is "fresh" when source_updated_at is not after now and younger than one hour.
		and = append(and, bson.D{{Key: "beds", Value: bson.D{{Key: "$elemMatch", Value: bson.D{
			{Key: "available", Value: bson.D{{Key: "$gt", Value: 0}}},
			{Key: "data_origin", Value: bson.D{{Key: "$ne", Value: "simulation"}}},
			{Key: "source_updated_at", Value: bson.D{{Key: "$gt", Value: now.Add(-time.Hour)}, {Key: "$lte", Value: now}}}}}}}})
	}
	return bson.D{{Key: "$and", Value: and}}
}

// searchPipeline is the shared prefix: match, then the q substring test over "name city province"
// (the same haystack service.Filter builds), which needs the region names joined in.
func searchPipeline(s service.Search, now time.Time) bson.A {
	p := bson.A{bson.D{{Key: "$match", Value: hospitalMatch(s, now)}}}
	if s.Q == "" {
		return p
	}
	region := func(local, as string) bson.D {
		return bson.D{{Key: "$lookup", Value: bson.D{{Key: "from", Value: "regions"}, {Key: "localField", Value: local}, {Key: "foreignField", Value: "_id"}, {Key: "as", Value: as}}}}
	}
	name := func(as string) bson.D {
		return bson.D{{Key: "$ifNull", Value: bson.A{bson.D{{Key: "$first", Value: "$" + as + ".name"}}, ""}}}
	}
	haystack := bson.D{{Key: "$toLower", Value: bson.D{{Key: "$concat", Value: bson.A{"$name", " ", name("_city"), " ", name("_province")}}}}}
	return append(p, region("city_code", "_city"), region("province_code", "_province"),
		bson.D{{Key: "$match", Value: bson.D{{Key: "$expr", Value: bson.D{{Key: "$gte", Value: bson.A{bson.D{{Key: "$indexOfCP", Value: bson.A{haystack, bson.D{{Key: "$literal", Value: strings.ToLower(s.Q)}}}}}, 0}}}}}}},
		bson.D{{Key: "$unset", Value: bson.A{"_city", "_province"}}})
}

// SearchHospitals runs the search in MongoDB. Name ordering (the default) is sorted, counted and
// paginated in the database, so only one page is transferred. Distance and match ordering depend on
// per-hospital computation, so the filtered set is loaded and ordered by service.Filter.
// Case-insensitive matching and name ordering use $toLower, which only folds ASCII letters.
func (m *Mongo) SearchHospitals(ctx context.Context, s service.Search, now time.Time) ([]domain.Hospital, int, error) {
	if s.Sort == "distance" || s.Sort == "match" {
		cursor, e := m.DB.Collection("hospitals").Aggregate(ctx, searchPipeline(s, now))
		if e != nil {
			return nil, 0, e
		}
		var docs []HospitalDoc
		if e = cursor.All(ctx, &docs); e != nil {
			return nil, 0, e
		}
		hs, e := m.joinNames(ctx, docs)
		if e != nil {
			return nil, 0, e
		}
		out, total := service.Filter(hs, s, now)
		return out, total, nil
	}
	var counted struct {
		N int `bson:"n"`
	}
	cursor, e := m.DB.Collection("hospitals").Aggregate(ctx, append(searchPipeline(s, now), bson.D{{Key: "$count", Value: "n"}}))
	if e != nil {
		return nil, 0, e
	}
	if cursor.Next(ctx) {
		if e = cursor.Decode(&counted); e != nil {
			return nil, 0, e
		}
	}
	if e = cursor.Err(); e != nil {
		return nil, 0, e
	}
	skip := int64(s.Page-1) * int64(s.Limit)
	if skip >= int64(counted.N) {
		return []domain.Hospital{}, counted.N, nil
	}
	page := append(searchPipeline(s, now),
		bson.D{{Key: "$addFields", Value: bson.D{{Key: "_name_key", Value: bson.D{{Key: "$toLower", Value: "$name"}}}}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "_name_key", Value: 1}, {Key: "_id", Value: 1}}}},
		bson.D{{Key: "$skip", Value: skip}}, bson.D{{Key: "$limit", Value: int64(s.Limit)}},
		bson.D{{Key: "$unset", Value: "_name_key"}})
	cursor, e = m.DB.Collection("hospitals").Aggregate(ctx, page, options.Aggregate().SetAllowDiskUse(true))
	if e != nil {
		return nil, 0, e
	}
	var docs []HospitalDoc
	if e = cursor.All(ctx, &docs); e != nil {
		return nil, 0, e
	}
	hs, e := m.joinNames(ctx, docs)
	if e != nil {
		return nil, 0, e
	}
	for i := range hs {
		service.Decorate(&hs[i], s, now)
	}
	return hs, counted.N, nil
}

// Hospital returns one visible (not source-inactive) hospital with joined names.
func (m *Mongo) Hospital(ctx context.Context, id string) (domain.Hospital, error) {
	var d HospitalDoc
	if e := m.DB.Collection("hospitals").FindOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "source_active", Value: bson.D{{Key: "$ne", Value: false}}}}).Decode(&d); e != nil {
		return domain.Hospital{}, one(e)
	}
	hs, e := m.joinNames(ctx, []HospitalDoc{d})
	if e != nil {
		return domain.Hospital{}, e
	}
	return hs[0], nil
}

// joinNames loads only the regions the given documents reference, plus the service masters.
func (m *Mongo) joinNames(ctx context.Context, docs []HospitalDoc) ([]domain.Hospital, error) {
	codes := bson.A{}
	for _, d := range docs {
		codes = append(codes, d.CityCode, d.ProvinceCode)
	}
	regions, e := all[struct {
		Code string `bson:"_id"`
		Name string `bson:"name"`
	}](ctx, m.DB.Collection("regions"), bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: codes}}}})
	if e != nil {
		return nil, e
	}
	names := map[string]string{}
	for _, r := range regions {
		names[r.Code] = r.Name
	}
	masters, e := m.Services(ctx)
	if e != nil {
		return nil, e
	}
	serviceNames := map[string]string{}
	for _, s := range masters {
		serviceNames[s.Slug] = s.Name
	}
	return hospitalDTOs(docs, names, serviceNames), nil
}

// HospitalByExternalID returns the one visible hospital whose upstream code (for SATUSEHAT, kode_satusehat)
// equals external. A code shared by several namespaces is ambiguous and reported as not found.
func (m *Mongo) HospitalByExternalID(ctx context.Context, external string) (domain.Hospital, error) {
	docs, e := all[HospitalDoc](ctx, m.DB.Collection("hospitals"), bson.D{{Key: "external_id", Value: external}, {Key: "source_active", Value: bson.D{{Key: "$ne", Value: false}}}}, options.Find().SetLimit(2))
	if e != nil {
		return domain.Hospital{}, e
	}
	if len(docs) != 1 {
		return domain.Hospital{}, ErrNotFound
	}
	hs, e := m.joinNames(ctx, docs)
	if e != nil {
		return domain.Hospital{}, e
	}
	return hs[0], nil
}

// Regions lists every province and city exactly as stored, so clients never guess codes.
func (m *Mongo) Regions(ctx context.Context) ([]domain.Region, error) {
	docs, e := all[struct {
		Code   string  `bson:"_id"`
		Name   string  `bson:"name"`
		Kind   string  `bson:"kind"`
		Parent *string `bson:"parent_code"`
	}](ctx, m.DB.Collection("regions"), bson.D{}, options.Find().SetSort(bson.D{{Key: "kind", Value: -1}, {Key: "name", Value: 1}, {Key: "_id", Value: 1}}))
	if e != nil {
		return nil, e
	}
	out := make([]domain.Region, 0, len(docs))
	for _, d := range docs {
		out = append(out, domain.Region{Code: d.Code, Name: d.Name, Kind: d.Kind, ParentCode: d.Parent})
	}
	return out, nil
}

// SuggestHospitals returns up to limit hospitals whose name contains q (ASCII case-insensitive),
// earliest match first. q is passed as a $literal so it can never be read as an operator or field path.
func (m *Mongo) SuggestHospitals(ctx context.Context, q, dataset string, limit int) ([]domain.Suggestion, error) {
	match := hospitalMatch(service.Search{Dataset: dataset}, time.Time{})
	pos := bson.D{{Key: "$indexOfCP", Value: bson.A{bson.D{{Key: "$toLower", Value: "$name"}}, bson.D{{Key: "$literal", Value: strings.ToLower(q)}}}}}
	cursor, e := m.DB.Collection("hospitals").Aggregate(ctx, bson.A{
		bson.D{{Key: "$match", Value: match}},
		bson.D{{Key: "$addFields", Value: bson.D{{Key: "_pos", Value: pos}, {Key: "_name_key", Value: bson.D{{Key: "$toLower", Value: "$name"}}}}}},
		bson.D{{Key: "$match", Value: bson.D{{Key: "_pos", Value: bson.D{{Key: "$gte", Value: 0}}}}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "_pos", Value: 1}, {Key: "_name_key", Value: 1}, {Key: "_id", Value: 1}}}},
		bson.D{{Key: "$limit", Value: int64(limit)}},
	})
	if e != nil {
		return nil, e
	}
	var docs []HospitalDoc
	if e = cursor.All(ctx, &docs); e != nil {
		return nil, e
	}
	hs, e := m.joinNames(ctx, docs)
	if e != nil {
		return nil, e
	}
	out := make([]domain.Suggestion, 0, len(hs))
	for _, h := range hs {
		out = append(out, domain.Suggestion{ID: h.ID, ExternalID: h.ExternalID, Name: h.Name, CityName: h.CityName, DataOrigin: h.DataOrigin})
	}
	return out, nil
}

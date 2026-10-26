package repository

import (
	"context"
	"siagars/backend/internal/domain"
	"siagars/backend/internal/service"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// bboxCondition is the structured coordinate range of a box; documents without coordinates never match.
func bboxCondition(b service.BBox) bson.D {
	return bson.D{
		{Key: "latitude", Value: bson.D{{Key: "$gte", Value: b.MinLat}, {Key: "$lte", Value: b.MaxLat}}},
		{Key: "longitude", Value: bson.D{{Key: "$gte", Value: b.MinLng}, {Key: "$lte", Value: b.MaxLng}}},
	}
}

// inArea returns at most limit visible hospitals matching match, and whether more existed. Order is
// by _id so a capped result is deterministic; ranking happens afterwards in Go.
func (m *Mongo) inArea(ctx context.Context, match bson.D, limit int, projection bson.D) ([]HospitalDoc, bool, error) {
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(limit + 1))
	if projection != nil {
		opts.SetProjection(projection)
	}
	docs, e := all[HospitalDoc](ctx, m.DB.Collection("hospitals"), match, opts)
	if e != nil {
		return nil, false, e
	}
	if len(docs) > limit {
		return docs[:limit], true, nil
	}
	return docs, false, nil
}

// RecommendCandidates loads the hospitals a recommendation may score: dataset and region exactly like
// search, plus a coarse latitude/longitude box around the requested radius (exact Haversine runs in
// domain.Recommend). BPJS and service status are not filtered here so the exclusions can be counted.
func (m *Mongo) RecommendCandidates(ctx context.Context, q service.RecommendQuery) ([]domain.Hospital, bool, error) {
	match := hospitalMatch(service.Search{Dataset: q.Dataset, Province: q.Province, City: q.City}, time.Time{})
	if q.Area != nil {
		and, _ := match[0].Value.(bson.A)
		match = bson.D{{Key: "$and", Value: append(and, bboxCondition(*q.Area))}}
	}
	docs, truncated, e := m.inArea(ctx, match, service.MaxCandidates, nil)
	if e != nil {
		return nil, false, e
	}
	hs, e := m.joinNames(ctx, docs)
	return hs, truncated, e
}

// MarkerHospitals loads the hospitals inside a map box with only the fields a marker needs.
func (m *Mongo) MarkerHospitals(ctx context.Context, q service.MarkersQuery) ([]domain.Hospital, bool, error) {
	match := hospitalMatch(service.Search{Dataset: q.Dataset, BPJS: q.BPJS}, time.Time{})
	and, _ := match[0].Value.(bson.A)
	match = bson.D{{Key: "$and", Value: append(and, bboxCondition(q.BBox))}}
	keep := bson.D{}
	for _, f := range []string{"name", "latitude", "longitude", "phone", "bpjs", "bpjs_data_origin", "data_origin", "source_updated_at", "operational", "city_code", "province_code", "services", "beds"} {
		keep = append(keep, bson.E{Key: f, Value: 1})
	}
	docs, truncated, e := m.inArea(ctx, match, service.MaxMarkers, keep)
	if e != nil {
		return nil, false, e
	}
	return hospitalDTOs(docs, map[string]string{}, map[string]string{}), truncated, nil
}

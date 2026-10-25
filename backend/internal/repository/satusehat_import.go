package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"siagars/backend/internal/domain"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// NewUUID returns a random RFC 4122 version 4 UUID.
func NewUUID() (string, error) {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", e
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	s := hex.EncodeToString(b[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:], nil
}

// sourceHash fingerprints the upstream-owned fields so unchanged records only refresh sync timestamps.
func sourceHash(h domain.HospitalImport) string {
	h.SyncedAt = time.Time{}
	b, _ := json.Marshal(h)
	return Hash(string(b))
}
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// failedIndexes returns the model indexes rejected by an unordered bulk write; any other error is returned.
func failedIndexes(e error) (map[int]bool, error) {
	failed := map[int]bool{}
	if e == nil {
		return failed, nil
	}
	var bulk mongo.BulkWriteException
	if !errors.As(e, &bulk) || bulk.WriteConcernError != nil {
		return nil, e
	}
	for _, w := range bulk.WriteErrors {
		failed[w.Index] = true
	}
	return failed, nil
}

// UpsertSATUSEHATHospitals writes one normalized batch. Regions are inserted only when missing and must
// agree on kind and parent; hospitals are keyed by (source_namespace, external_id). Only upstream-owned
// fields are $set; the local UUID, BPJS, services, beds and verification fields are set on insert only.
// Writes are unordered so one rejected record does not block the rest; rejections are counted as Failed.
func (m *Mongo) UpsertSATUSEHATHospitals(ctx context.Context, items []domain.HospitalImport) (domain.ImportResult, error) {
	var result domain.ImportResult
	if len(items) == 0 {
		return result, nil
	}
	type region struct{ code, parent, kind, name string }
	regions, seen := []region{}, map[string]bool{}
	for _, h := range items {
		for _, r := range []region{{h.ProvinceCode, "", "province", h.ProvinceName}, {h.CityCode, h.ProvinceCode, "city", h.CityName}} {
			if !seen[r.code] {
				seen[r.code] = true
				regions = append(regions, r)
			}
		}
	}
	models := make([]mongo.WriteModel, 0, len(regions))
	for _, r := range regions {
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.D{{Key: "_id", Value: r.code}, {Key: "kind", Value: r.kind}, {Key: "parent_code", Value: nullable(r.parent)}}).
			SetUpdate(bson.D{{Key: "$setOnInsert", Value: bson.D{{Key: "name", Value: r.name}}}}).SetUpsert(true))
	}
	_, e := m.DB.Collection("regions").BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	failed, e := failedIndexes(e)
	if e != nil {
		return result, e
	}
	// A code that already exists with another kind or parent makes the upsert hit the _id index.
	badRegion := map[string]bool{}
	for i := range failed {
		badRegion[regions[i].code] = true
	}
	ids := make([]string, 0, len(items))
	for _, h := range items {
		ids = append(ids, h.ExternalID)
	}
	hospitals := m.DB.Collection("hospitals")
	existing, e := all[struct {
		ExternalID string `bson:"external_id"`
		Hash       string `bson:"source_hash"`
	}](ctx, hospitals, bson.D{{Key: "source_namespace", Value: items[0].SourceNamespace}, {Key: "external_id", Value: bson.D{{Key: "$in", Value: ids}}}}, options.Find().SetProjection(bson.D{{Key: "external_id", Value: 1}, {Key: "source_hash", Value: 1}}))
	if e != nil {
		return result, e
	}
	hashes := map[string]string{}
	for _, x := range existing {
		hashes[x.ExternalID] = x.Hash
	}
	const (
		created = iota
		updated
		unchanged
	)
	kinds := []int{}
	models = models[:0]
	for _, h := range items {
		if badRegion[h.ProvinceCode] || badRegion[h.CityCode] {
			result.Failed++
			continue
		}
		filter := bson.D{{Key: "source_namespace", Value: h.SourceNamespace}, {Key: "external_id", Value: h.ExternalID}}
		at := h.SyncedAt
		hash := sourceHash(h)
		old, found := hashes[h.ExternalID]
		if found && old == hash {
			models = append(models, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(bson.D{{Key: "$set", Value: bson.D{{Key: "synced_at", Value: at}, {Key: "last_seen_at", Value: at}}}}))
			kinds = append(kinds, unchanged)
			continue
		}
		id, e := NewUUID()
		if e != nil {
			return result, e
		}
		set := bson.D{
			{Key: "name", Value: h.Name}, {Key: "address", Value: h.Address}, {Key: "province_code", Value: h.ProvinceCode}, {Key: "city_code", Value: h.CityCode},
			{Key: "latitude", Value: h.Latitude}, {Key: "longitude", Value: h.Longitude}, {Key: "phone", Value: h.Phone},
			{Key: "data_origin", Value: "satusehat_sandbox"}, {Key: "source_active", Value: h.SourceActive}, {Key: "source_status", Value: nullable(h.SourceStatus)},
			{Key: "facility_code", Value: nullable(h.FacilityCode)}, {Key: "facility_type_code", Value: nullable(h.FacilityTypeCode)},
			{Key: "facility_subtype", Value: nullable(h.FacilitySubtype)}, {Key: "facility_class", Value: nullable(h.FacilityClass)},
			{Key: "facility_subtype_name", Value: nullable(h.FacilitySubtypeName)}, {Key: "facility_class_name", Value: nullable(h.FacilityClassName)}, {Key: "operational", Value: h.Operational},
			{Key: "source_hash", Value: hash}, {Key: "synced_at", Value: at}, {Key: "last_seen_at", Value: at},
		}
		// MSI has no per-record update timestamp and no verification evidence, so these stay null.
		insert := bson.D{
			{Key: "_id", Value: id}, {Key: "bpjs", Value: nil}, {Key: "services", Value: bson.A{}}, {Key: "beds", Value: bson.A{}},
			{Key: "source_url", Value: nil}, {Key: "source_updated_at", Value: nil}, {Key: "verified_at", Value: nil},
		}
		models = append(models, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(bson.D{{Key: "$set", Value: set}, {Key: "$setOnInsert", Value: insert}}).SetUpsert(true))
		if found {
			kinds = append(kinds, updated)
		} else {
			kinds = append(kinds, created)
		}
	}
	if len(models) == 0 {
		return result, nil
	}
	_, e = hospitals.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	if failed, e = failedIndexes(e); e != nil {
		return result, e
	}
	for i, kind := range kinds {
		switch {
		case failed[i]:
			result.Failed++
		case kind == created:
			result.Created++
		case kind == updated:
			result.Updated++
		default:
			result.Unchanged++
		}
	}
	return result, nil
}

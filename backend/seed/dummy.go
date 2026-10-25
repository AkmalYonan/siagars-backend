package seed

import (
	"context"
	"crypto/sha256"
	"errors"
	"siagars/backend/internal/repository"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// DummyFacilities builds synthetic, clearly labeled facility data for one real hospital so a client can
// exercise ICU/NICU/PICU/rawat-inap/BPJS UI before any sourced data exists. It is deterministic in
// (externalID, class): the same inputs always give the same output, so re-running never churns data.
// Every item carries data_origin=simulation with no source URL and no verification, and the values are
// not facts about the hospital. classCode is the MSI class letter (A..D) or "" when unknown.
func DummyFacilities(externalID, classCode string, at time.Time) (bpjs bool, services []repository.ServiceDoc, beds []repository.BedDoc) {
	h := sha256.Sum256([]byte("siagars-dummy-v1:" + externalID))
	pick := func(i, n int) int { return int(h[i%len(h)]) % n }
	tier := map[string]int{"A": 3, "B": 2, "C": 1}[classCode] // D and unknown are tier 0

	status := func(i int, minTier int) string {
		// Capability rises with class: below minTier the service is mostly absent or unknown.
		roll := pick(i, 100)
		switch {
		case tier >= minTier+1 && roll < 80:
			return "available"
		case tier >= minTier && roll < 55:
			return "available"
		case tier >= minTier && roll < 85:
			return "limited"
		case roll < 20:
			return "unavailable"
		case roll < 45:
			return "unknown"
		default:
			return "unavailable"
		}
	}
	statuses := map[string]string{
		"icu": status(0, 2), "nicu": status(1, 2), "picu": status(2, 3),
		"igd-24h": status(3, 1), "ambulance": status(4, 1),
		"rawat-inap": "available",
	}
	if tier == 0 && pick(5, 100) < 40 {
		statuses["rawat-inap"] = "limited"
	}
	for _, slug := range []string{"icu", "nicu", "picu", "igd-24h", "ambulance", "rawat-inap"} {
		services = append(services, repository.ServiceDoc{Slug: slug, Status: statuses[slug], ProvenanceDoc: repository.Simulation(at), Version: 1})
	}

	capacity := map[string][2]int{ // [base, spread] of total beds by tier
		"general": {[]int{20, 60, 120, 250}[tier], []int{10, 40, 80, 150}[tier]},
		"icu":     {[]int{4, 8, 12, 24}[tier], []int{4, 6, 10, 16}[tier]},
		"nicu":    {[]int{2, 4, 8, 12}[tier], []int{2, 4, 6, 10}[tier]},
		"picu":    {[]int{2, 4, 6, 10}[tier], []int{2, 3, 5, 8}[tier]},
	}
	// About one hospital in five has no bed figures at all, so the null path stays exercised.
	if pick(6, 100) >= 20 {
		for i, category := range []string{"general", "icu", "nicu", "picu"} {
			if category != "general" && (statuses[category] == "unavailable" || statuses[category] == "unknown") {
				continue
			}
			total := capacity[category][0] + pick(7+i, capacity[category][1]+1)
			available := pick(11+i, total+1)
			beds = append(beds, repository.BedDoc{Category: category, Total: &total, Available: &available, ProvenanceDoc: repository.Simulation(at), Version: 1})
		}
	}
	// Class A/B hospitals are more often BPJS partners; the result is synthetic either way.
	bpjs = pick(15, 100) < []int{35, 55, 70, 85}[tier]
	return bpjs, services, beds
}

// DummyOutcome counts what ApplyDummyFacilities did.
type DummyOutcome struct {
	Hospitals, ServicesAdded, BedsAdded, BPJSSet int
}

// ApplyDummyFacilities attaches DummyFacilities to every imported (non-simulation) hospital. It only
// adds: an existing service or bed item (curated, operator-updated or earlier dummy) is never replaced
// and BPJS is set only while still null, so it is safe to repeat. New items are origin simulation,
// version 1, never verified. BPJS gets bpjs_data_origin=simulation and no source URL or date.
func ApplyDummyFacilities(ctx context.Context, db *mongo.Database, at time.Time) (DummyOutcome, error) {
	at = at.UTC().Truncate(time.Millisecond)
	var out DummyOutcome
	masters := map[string]bool{}
	cur, e := db.Collection("services").Find(ctx, bson.D{})
	if e != nil {
		return out, e
	}
	var rows []struct {
		ID string `bson:"_id"`
	}
	if e = cur.All(ctx, &rows); e != nil {
		return out, e
	}
	for _, r := range rows {
		masters[r.ID] = true
	}
	if !masters["rawat-inap"] {
		return out, errors.New("master layanan rawat-inap belum ada; jalankan migrate dan seed dahulu")
	}
	hospitals := db.Collection("hospitals")
	cursor, e := hospitals.Find(ctx, bson.D{{Key: "external_id", Value: bson.D{{Key: "$type", Value: "string"}}}, {Key: "data_origin", Value: bson.D{{Key: "$ne", Value: "simulation"}}}},
		options.Find().SetProjection(bson.D{{Key: "_id", Value: 1}, {Key: "external_id", Value: 1}, {Key: "facility_class", Value: 1}}))
	if e != nil {
		return out, e
	}
	var docs []struct {
		ID         string  `bson:"_id"`
		ExternalID string  `bson:"external_id"`
		Class      *string `bson:"facility_class"`
	}
	if e = cursor.All(ctx, &docs); e != nil {
		return out, e
	}
	var models []mongo.WriteModel
	for _, d := range docs {
		class := ""
		if d.Class != nil {
			class = *d.Class
		}
		bpjs, services, beds := DummyFacilities(d.ExternalID, class, at)
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.D{{Key: "_id", Value: d.ID}, {Key: "bpjs", Value: nil}}).
			SetUpdate(bson.D{{Key: "$set", Value: bson.D{{Key: "bpjs", Value: bpjs}, {Key: "bpjs_data_origin", Value: "simulation"}}}}))
		for _, s := range services {
			models = append(models, pushMissing(d.ID, "services", "slug", s.Slug, s))
		}
		for _, b := range beds {
			models = append(models, pushMissing(d.ID, "beds", "category", b.Category, b))
		}
	}
	const batch = 1000
	for from := 0; from < len(models); from += batch {
		to := min(from+batch, len(models))
		if _, e := hospitals.BulkWrite(ctx, models[from:to], options.BulkWrite().SetOrdered(false)); e != nil {
			return out, e
		}
	}
	// Counts are derived from the final state instead of per-write results.
	out.Hospitals = len(docs)
	var e2 error
	if out.ServicesAdded, out.BedsAdded, out.BPJSSet, e2 = countDummy(ctx, hospitals, at); e2 != nil {
		return out, e2
	}
	return out, nil
}

// countDummy counts dummy items stamped with this run's timestamp.
func countDummy(ctx context.Context, c *mongo.Collection, at time.Time) (services, beds, bpjs int, err error) {
	count := func(field string) (int, error) {
		cur, e := c.Aggregate(ctx, bson.A{
			bson.D{{Key: "$unwind", Value: "$" + field}},
			bson.D{{Key: "$match", Value: bson.D{{Key: field + ".data_origin", Value: "simulation"}, {Key: field + ".synced_at", Value: at}, {Key: "data_origin", Value: bson.D{{Key: "$ne", Value: "simulation"}}}}}},
			bson.D{{Key: "$count", Value: "n"}}})
		if e != nil {
			return 0, e
		}
		var r struct {
			N int `bson:"n"`
		}
		if cur.Next(ctx) {
			if e = cur.Decode(&r); e != nil {
				return 0, e
			}
		}
		return r.N, cur.Err()
	}
	if services, err = count("services"); err != nil {
		return
	}
	if beds, err = count("beds"); err != nil {
		return
	}
	n, err := c.CountDocuments(ctx, bson.D{{Key: "bpjs_data_origin", Value: "simulation"}, {Key: "data_origin", Value: bson.D{{Key: "$ne", Value: "simulation"}}}})
	return services, beds, int(n), err
}

package integration

import (
	"context"
	"siagars/backend/internal/repository"
	"siagars/backend/seed"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// TestCurateApply runs only in the temporary database created by database(); it never touches siagars.
func TestCurateApply(t *testing.T) {
	p := database(t)
	ctx := context.Background()
	if e := seed.Apply(ctx, p); e != nil {
		t.Fatal(e)
	}
	at := time.Now().UTC().Truncate(time.Millisecond)
	ext, ns, name := "PLACEHOLDER-RS-0001", seed.DefaultCurationNamespace, "RS Contoh Fiktif Satu"
	msi := repository.HospitalDoc{ID: "40000000-0000-4000-8000-000000000001", ExternalID: &ext, SourceNamespace: &ns, Name: name, CityCode: "3171", ProvinceCode: "31",
		ProvenanceDoc: repository.ProvenanceDoc{DataOrigin: "satusehat_sandbox", SyncedAt: &at}, Services: []repository.ServiceDoc{}, Beds: []repository.BedDoc{}}
	// An operator already updated ICU; curation must not overwrite it.
	opAt := at.Add(-time.Hour)
	msi.Services = append(msi.Services, repository.ServiceDoc{Slug: "icu", Status: "limited", ProvenanceDoc: repository.Simulation(opAt), Version: 3})
	hospitals := p.DB.Collection("hospitals")
	if _, e := hospitals.InsertOne(ctx, msi); e != nil {
		t.Fatal(e)
	}
	src, srcAt := "https://sumber-fiktif.invalid/profil", time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)
	zero, nine := 0, 9
	yes := true
	rec := func(bpjs *bool) []seed.CuratedHospital {
		return []seed.CuratedHospital{{ExternalID: ext, SourceNamespace: ns, Name: name, BPJS: bpjs, SourceURL: src, SourceUpdatedAt: srcAt,
			Services: []seed.CuratedService{{Slug: "icu", Status: "available", SourceURL: src, SourceUpdatedAt: srcAt}, {Slug: "nicu", Status: "unknown", SourceURL: src, SourceUpdatedAt: srcAt}},
			Beds:     []seed.CuratedBed{{Category: "icu", Total: &nine, Available: &zero, SourceURL: src, SourceUpdatedAt: srcAt}, {Category: "nicu", Total: &nine, SourceURL: src, SourceUpdatedAt: srcAt}}}}
	}
	out, e := seed.ApplyCuration(ctx, p.DB, rec(&yes), at)
	if e != nil || len(out) != 1 || !out[0].Found || len(out[0].ServicesAdded) != 1 || len(out[0].ServicesKept) != 1 || len(out[0].BedsAdded) != 2 || !out[0].BPJSSet {
		t.Fatalf("%+v %v", out, e)
	}
	var got repository.HospitalDoc
	if e = hospitals.FindOne(ctx, bson.D{{Key: "_id", Value: msi.ID}}).Decode(&got); e != nil {
		t.Fatal(e)
	}
	if got.DataOrigin != "satusehat_sandbox" || got.VerifiedAt != nil || got.BPJS == nil || !*got.BPJS {
		t.Fatalf("hospital-level fields: %+v", got.ProvenanceDoc)
	}
	// BPJS carries its own dated source, separate from the hospital's provenance.
	if got.BPJSSourceURL == nil || *got.BPJSSourceURL != src || got.BPJSSourceUpdatedAt == nil || !got.BPJSSourceUpdatedAt.Equal(srcAt) {
		t.Fatalf("bpjs provenance: %v %v", got.BPJSSourceURL, got.BPJSSourceUpdatedAt)
	}
	if icu := got.Services[0]; icu.Status != "limited" || icu.Version != 3 || icu.DataOrigin != "simulation" {
		t.Fatalf("operator update overwritten: %+v", icu)
	}
	nicu := got.Services[1]
	if nicu.DataOrigin != "curated" || nicu.VerifiedAt != nil || nicu.Version != 1 || nicu.SourceURL == nil || *nicu.SourceURL != src || !nicu.SourceUpdatedAt.Equal(srcAt) {
		t.Fatalf("curated service: %+v", nicu)
	}
	icuBed, nicuBed := got.Beds[0], got.Beds[1]
	if icuBed.Available == nil || *icuBed.Available != 0 || nicuBed.Available != nil || nicuBed.Total == nil || nicuBed.VerifiedAt != nil || icuBed.DataOrigin != "curated" {
		t.Fatalf("beds must keep 0 vs null and stay unverified: %+v %+v", icuBed, nicuBed)
	}
	// Second run changes nothing, and a different bpjs value does not replace the stored one.
	no := false
	out, e = seed.ApplyCuration(ctx, p.DB, rec(&no), at.Add(time.Minute))
	if e != nil || len(out[0].ServicesAdded) != 0 || len(out[0].BedsAdded) != 0 || out[0].BPJSSet || !out[0].BPJSKept {
		t.Fatalf("not idempotent: %+v %v", out, e)
	}
	var again repository.HospitalDoc
	hospitals.FindOne(ctx, bson.D{{Key: "_id", Value: msi.ID}}).Decode(&again)
	if !*again.BPJS || len(again.Services) != 2 || len(again.Beds) != 2 {
		t.Fatalf("second run modified data: %+v", again)
	}
	// Unknown hospital, mismatched name and unknown service slug write nothing.
	missing := rec(nil)
	missing[0].ExternalID = "PLACEHOLDER-NOPE"
	wrong := rec(nil)
	wrong[0].Name = "Nama Lain"
	badSlug := rec(nil)
	badSlug[0].Services = []seed.CuratedService{{Slug: "tidak-ada", Status: "available", SourceURL: src, SourceUpdatedAt: srcAt}}
	out, e = seed.ApplyCuration(ctx, p.DB, append(append(missing, wrong...), badSlug...), at)
	if e != nil || out[0].Found || !out[1].NameMismatch || len(out[2].Missing) != 1 {
		t.Fatalf("%+v %v", out, e)
	}
	if n, _ := hospitals.CountDocuments(ctx, bson.D{{Key: "external_id", Value: "PLACEHOLDER-NOPE"}}); n != 0 {
		t.Fatal("curation must never create hospitals")
	}
}

// TestCurateAudit checks BE-22: one audit_logs entry per execution with counts only.
func TestCurateAudit(t *testing.T) {
	p := database(t)
	ctx := context.Background()
	counts := seed.AuditCounts([]seed.CurateOutcome{{ExternalID: "a", Found: true, BPJSSet: true, ServicesAdded: []string{"icu"}}, {ExternalID: "b"}})
	if e := p.AuditJob(ctx, "system:curate", "curate.apply", "batch-1.json", "curate", counts); e != nil {
		t.Fatal(e)
	}
	var row struct {
		ActorID string `bson:"actor_id"`
		Action  string `bson:"action"`
		Entity  string `bson:"entity_id"`
		Changes bson.M `bson:"changes"`
	}
	if e := p.DB.Collection("audit_logs").FindOne(ctx, bson.D{{Key: "action", Value: "curate.apply"}}).Decode(&row); e != nil {
		t.Fatal(e)
	}
	if row.ActorID != "system:curate" || row.Entity != "batch-1.json" || row.Changes["updated"] != int32(1) || row.Changes["failed"] != int32(1) || row.Changes["services"] != int32(1) {
		t.Fatalf("%+v", row)
	}
}

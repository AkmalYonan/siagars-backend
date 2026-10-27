package integration

import (
	"context"
	"siagars/backend/internal/repository"
	"siagars/backend/seed"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// TestApplyDummyFacilities runs only in the temporary database created by database().
func TestApplyDummyFacilities(t *testing.T) {
	p := database(t)
	ctx := context.Background()
	if e := seed.Apply(ctx, p); e != nil {
		t.Fatal(e)
	}
	at := time.Now().UTC().Truncate(time.Millisecond)
	ns := seed.DefaultCurationNamespace
	mk := func(id, ext string) repository.HospitalDoc {
		x := ext
		return repository.HospitalDoc{ID: id, ExternalID: &x, SourceNamespace: &ns, Name: "RS " + ext, CityCode: "3171", ProvinceCode: "31",
			ProvenanceDoc: repository.ProvenanceDoc{DataOrigin: "satusehat_sandbox", SyncedAt: &at}, Services: []repository.ServiceDoc{}, Beds: []repository.BedDoc{}}
	}
	real1, real2 := mk("40000000-0000-4000-8000-000000000011", "DUMMY-1"), mk("40000000-0000-4000-8000-000000000012", "DUMMY-2")
	// real2 already has operator-managed ICU and a curated BPJS value; neither may change.
	real2.Services = append(real2.Services, repository.ServiceDoc{Slug: "icu", Status: "limited", ProvenanceDoc: repository.Simulation(at.Add(-time.Hour)), Version: 4})
	yes, orig := true, "curated"
	real2.BPJS, real2.BPJSDataOrigin = &yes, &orig
	hospitals := p.DB.Collection("hospitals")
	if _, e := hospitals.InsertMany(ctx, []any{real1, real2}); e != nil {
		t.Fatal(e)
	}
	var seedBefore repository.HospitalDoc
	if e := hospitals.FindOne(ctx, bson.D{{Key: "_id", Value: "10000000-0000-4000-8000-000000000001"}}).Decode(&seedBefore); e != nil {
		t.Fatal(e)
	}

	out, e := seed.ApplyDummyFacilities(ctx, p.DB, at)
	if e != nil || out.Hospitals != 2 {
		t.Fatalf("%+v %v", out, e)
	}
	var g1, g2 repository.HospitalDoc
	hospitals.FindOne(ctx, bson.D{{Key: "_id", Value: real1.ID}}).Decode(&g1)
	hospitals.FindOne(ctx, bson.D{{Key: "_id", Value: real2.ID}}).Decode(&g2)
	if len(g1.Services) != 6 || g1.BPJS == nil || g1.BPJSDataOrigin == nil || *g1.BPJSDataOrigin != "simulation" {
		t.Fatalf("real1: %d layanan bpjs=%v", len(g1.Services), g1.BPJS)
	}
	if g1.DataOrigin != "satusehat_sandbox" {
		t.Fatalf("data_origin RS harus tetap sandbox: %s", g1.DataOrigin)
	}
	if len(g2.Services) != 6 || g2.Services[0].Slug != "icu" || g2.Services[0].Status != "limited" || g2.Services[0].Version != 4 {
		t.Fatalf("ICU operator tertimpa: %+v", g2.Services)
	}
	if g2.BPJS == nil || !*g2.BPJS || *g2.BPJSDataOrigin != "curated" {
		t.Fatalf("BPJS kurasi tertimpa: %v %v", g2.BPJS, g2.BPJSDataOrigin)
	}
	// Simulation hospitals are left alone.
	var seedAfter repository.HospitalDoc
	hospitals.FindOne(ctx, bson.D{{Key: "_id", Value: seedBefore.ID}}).Decode(&seedAfter)
	if len(seedAfter.Services) != len(seedBefore.Services) || len(seedAfter.Beds) != len(seedBefore.Beds) {
		t.Fatal("RS simulasi seed berubah")
	}

	// Idempotent: a second run (even later) changes nothing.
	n1 := len(g1.Beds)
	if _, e = seed.ApplyDummyFacilities(ctx, p.DB, at.Add(time.Hour)); e != nil {
		t.Fatal(e)
	}
	hospitals.FindOne(ctx, bson.D{{Key: "_id", Value: real1.ID}}).Decode(&g1)
	if len(g1.Services) != 6 || len(g1.Beds) != n1 {
		t.Fatalf("tidak idempoten: %d layanan %d bed", len(g1.Services), len(g1.Beds))
	}

	// Curation replaces untouched simulation items but keeps operator-updated ones.
	src, srcAt := "https://sumber-fiktif.invalid/x", time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)
	no := false
	_, e = seed.ApplyCuration(ctx, p.DB, []seed.CuratedHospital{
		{ExternalID: "DUMMY-1", SourceNamespace: ns, Name: "RS DUMMY-1", BPJS: &no, SourceURL: src, SourceUpdatedAt: srcAt,
			Services: []seed.CuratedService{{Slug: "icu", Status: "unavailable", SourceURL: src, SourceUpdatedAt: srcAt}}},
		{ExternalID: "DUMMY-2", SourceNamespace: ns, Name: "RS DUMMY-2", SourceURL: src, SourceUpdatedAt: srcAt,
			Services: []seed.CuratedService{{Slug: "icu", Status: "available", SourceURL: src, SourceUpdatedAt: srcAt}}},
	}, at.Add(2*time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	hospitals.FindOne(ctx, bson.D{{Key: "_id", Value: real1.ID}}).Decode(&g1)
	hospitals.FindOne(ctx, bson.D{{Key: "_id", Value: real2.ID}}).Decode(&g2)
	var icu1 *repository.ServiceDoc
	count := 0
	for i := range g1.Services {
		if g1.Services[i].Slug == "icu" {
			icu1 = &g1.Services[i]
			count++
		}
	}
	if count != 1 || icu1.DataOrigin != "curated" || icu1.Status != "unavailable" {
		t.Fatalf("kurasi tidak mengganti simulasi: %+v (n=%d)", icu1, count)
	}
	if g1.BPJS == nil || *g1.BPJS || *g1.BPJSDataOrigin != "curated" || g1.BPJSSourceURL == nil {
		t.Fatalf("BPJS kurasi: %v %v", g1.BPJS, g1.BPJSDataOrigin)
	}
	if g2.Services[0].Status != "limited" || g2.Services[0].Version != 4 {
		t.Fatalf("ICU operator tertimpa kurasi: %+v", g2.Services[0])
	}
}

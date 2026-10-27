package integration

import (
	"context"
	"siagars/backend/internal/repository"
	"siagars/backend/seed"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// TestRefreshSimulationOnlyTouchesSimulation is BE-05: after a run every simulation item is stamped with
// the run time, curated items keep their source date, nothing gets verified_at, and one audit entry is
// written as system:cron.
func TestRefreshSimulationOnlyTouchesSimulation(t *testing.T) {
	p := database(t)
	ctx := context.Background()
	if e := seed.Apply(ctx, p); e != nil {
		t.Fatal(e)
	}
	old := time.Now().UTC().Add(-72 * time.Hour).Truncate(time.Millisecond)
	src := "https://rs.example.go.id/layanan"
	curatedItem := repository.ProvenanceDoc{DataOrigin: "curated", SourceURL: &src, SourceUpdatedAt: &old, SyncedAt: &old}
	ext, ns := "REFRESH-1", seed.DefaultCurationNamespace
	mixed := repository.HospitalDoc{ID: "40000000-0000-4000-8000-000000000021", ExternalID: &ext, SourceNamespace: &ns, Name: "RS Campuran", CityCode: "3171", ProvinceCode: "31",
		ProvenanceDoc: repository.ProvenanceDoc{DataOrigin: "satusehat_sandbox", SyncedAt: &old},
		Services:      []repository.ServiceDoc{{Slug: "nicu", Status: "available", ProvenanceDoc: curatedItem, Version: 2}, {Slug: "icu", Status: "limited", ProvenanceDoc: repository.Simulation(old), Version: 1}},
		Beds:          []repository.BedDoc{{Category: "nicu", Total: ptr(4), Available: ptr(1), ProvenanceDoc: curatedItem, Version: 3}}}
	hospitals := p.DB.Collection("hospitals")
	if _, e := hospitals.InsertOne(ctx, mixed); e != nil {
		t.Fatal(e)
	}
	at := time.Now().UTC().Truncate(time.Millisecond)
	out, e := seed.RefreshSimulation(ctx, p, "system:cron", "rid-1", at)
	if e != nil || out.SimulationHospitals != 30 || out.ImportedHospitals != 1 || out.Services == 0 || out.Beds == 0 {
		t.Fatalf("%+v %v", out, e)
	}
	var got repository.HospitalDoc
	if e = hospitals.FindOne(ctx, bson.D{{Key: "_id", Value: mixed.ID}}).Decode(&got); e != nil {
		t.Fatal(e)
	}
	for _, s := range got.Services {
		switch {
		case s.DataOrigin == "curated" && (!s.SourceUpdatedAt.Equal(old) || s.Version != 2 || s.Status != "available"):
			t.Fatalf("layanan kurasi tersentuh: %+v", s)
		case s.DataOrigin == "simulation" && !s.SourceUpdatedAt.Equal(at):
			t.Fatalf("layanan simulasi tidak distempel: %+v", s)
		case s.VerifiedAt != nil:
			t.Fatalf("verified_at diisi: %+v", s)
		}
	}
	for _, b := range got.Beds {
		if b.DataOrigin == "curated" && !b.SourceUpdatedAt.Equal(old) {
			t.Fatalf("bed kurasi tersentuh: %+v", b)
		}
	}
	if !got.SyncedAt.Equal(old) || got.DataOrigin != "satusehat_sandbox" {
		t.Fatalf("provenance RS impor berubah: %+v", got.ProvenanceDoc)
	}
	var sim repository.HospitalDoc
	if e = hospitals.FindOne(ctx, bson.D{{Key: "_id", Value: seed.HospitalID(1)}}).Decode(&sim); e != nil || !sim.SourceUpdatedAt.Equal(at) || !sim.Services[0].SourceUpdatedAt.Equal(at) {
		t.Fatalf("RS simulasi tidak distempel: %v %+v", e, sim.ProvenanceDoc)
	}
	if n, _ := hospitals.CountDocuments(ctx, bson.D{{Key: "$or", Value: bson.A{bson.D{{Key: "verified_at", Value: bson.D{{Key: "$ne", Value: nil}}}}, bson.D{{Key: "services.verified_at", Value: bson.D{{Key: "$ne", Value: nil}}}}}}}); n != 0 {
		t.Fatalf("%d RS punya verified_at", n)
	}
	var audit bson.M
	if e = p.DB.Collection("audit_logs").FindOne(ctx, bson.D{{Key: "action", Value: "job.refresh_simulation"}}).Decode(&audit); e != nil || audit["actor_id"] != "system:cron" || audit["request_id"] != "rid-1" {
		t.Fatal(e, audit)
	}
}

func ptr(n int) *int { return &n }

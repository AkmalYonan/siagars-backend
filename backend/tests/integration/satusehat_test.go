package integration

import (
	"context"
	"fmt"
	"siagars/backend/internal/domain"
	"siagars/backend/internal/integrations/satusehat"
	"siagars/backend/seed"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Synthetic MSI-shaped records; no upstream call and no credential is involved.
func imported(id, name string, at time.Time) domain.HospitalImport {
	lat, lng := -6.25, 106.8
	address := "Jl. Sintetis " + id
	return domain.HospitalImport{ExternalID: id, SourceNamespace: satusehat.Namespace, Name: name, Address: &address, ProvinceCode: "31", ProvinceName: "DKI JAKARTA", CityCode: "3171", CityName: "KOTA ADM. JAKARTA PUSAT",
		Latitude: &lat, Longitude: &lng, SourceActive: true, SourceStatus: "valid", FacilityCode: "K" + id, FacilityTypeCode: "104", SyncedAt: at.UTC().Truncate(time.Millisecond)}
}
func TestSATUSEHATImport(t *testing.T) {
	p := database(t)
	ctx := context.Background()
	if e := seed.Apply(ctx, p); e != nil {
		t.Fatal(e)
	}
	if applied, e := p.Applied(ctx, "002_satusehat_msi.json"); e != nil || !applied {
		t.Fatal("migration 002", e)
	}
	hospitals := p.DB.Collection("hospitals")
	type stored struct {
		ID       string    `bson:"_id"`
		Name     string    `bson:"name"`
		Address  *string   `bson:"address"`
		BPJS     *bool     `bson:"bpjs"`
		SyncedAt time.Time `bson:"synced_at"`
		Services []bson.M  `bson:"services"`
		Beds     []bson.M  `bson:"beds"`
	}
	get := func(id string) stored {
		t.Helper()
		var s stored
		if e := hospitals.FindOne(ctx, bson.D{{Key: "source_namespace", Value: satusehat.Namespace}, {Key: "external_id", Value: id}}).Decode(&s); e != nil {
			t.Fatal(id, e)
		}
		return s
	}
	t0 := time.Now()
	r, e := p.UpsertSATUSEHATHospitals(ctx, []domain.HospitalImport{imported("1000000001", "RS Sandbox A", t0), imported("1000000002", "RS Sandbox B", t0)})
	if e != nil || r != (domain.ImportResult{Created: 2}) {
		t.Fatal("first import", r, e)
	}
	var city struct {
		Kind   string  `bson:"kind"`
		Parent *string `bson:"parent_code"`
		Name   string  `bson:"name"`
	}
	if e = p.DB.Collection("regions").FindOne(ctx, bson.D{{Key: "_id", Value: "3171"}}).Decode(&city); e != nil || city.Kind != "city" || city.Parent == nil || *city.Parent != "31" {
		t.Fatal("city region", city, e)
	}
	var province struct {
		Name string `bson:"name"`
	}
	if e = p.DB.Collection("regions").FindOne(ctx, bson.D{{Key: "_id", Value: "31"}, {Key: "kind", Value: "province"}}).Decode(&province); e != nil || province.Name != "DKI Jakarta" {
		t.Fatal("existing seed province must keep its name", province, e)
	}
	first := get("1000000001")
	if first.BPJS != nil || len(first.Services) != 0 || len(first.Beds) != 0 || len(first.ID) != 36 || first.ID[14] != '4' {
		t.Fatal("insert defaults", first)
	}
	// Public DTO shape for imported data.
	hs, e := p.Hospitals(ctx)
	if e != nil || len(hs) != 32 {
		t.Fatal("simulation and sandbox hospitals must both be listed", len(hs), e)
	}
	for _, h := range hs {
		if h.ID != first.ID {
			continue
		}
		h.Refresh(time.Now())
		if h.DataOrigin != "satusehat_sandbox" || h.SourceUpdatedAt != nil || h.VerifiedAt != nil || h.SyncedAt == nil || h.Freshness != "unknown" || h.BPJS != nil || len(h.Services) != 0 || len(h.Beds) != 0 || h.CityName == "" {
			t.Fatalf("public DTO %+v", h)
		}
	}
	// Second identical import: idempotent, keeps _id, only sync timestamps move.
	t1 := t0.Add(time.Minute)
	if r, e = p.UpsertSATUSEHATHospitals(ctx, []domain.HospitalImport{imported("1000000001", "RS Sandbox A", t1), imported("1000000002", "RS Sandbox B", t1)}); e != nil || r != (domain.ImportResult{Unchanged: 2}) {
		t.Fatal("second import", r, e)
	}
	again := get("1000000001")
	if again.ID != first.ID || !again.SyncedAt.After(first.SyncedAt) {
		t.Fatal("idempotency", again, first)
	}
	if n, _ := hospitals.CountDocuments(ctx, bson.D{{Key: "external_id", Value: "1000000001"}}); n != 1 {
		t.Fatal("duplicate", n)
	}
	if _, e = hospitals.InsertOne(ctx, bson.D{{Key: "_id", Value: "dup"}, {Key: "external_id", Value: "1000000001"}, {Key: "source_namespace", Value: satusehat.Namespace}, {Key: "name", Value: "x"}, {Key: "city_code", Value: "3171"}, {Key: "province_code", Value: "31"}, {Key: "data_origin", Value: "satusehat_sandbox"}, {Key: "services", Value: bson.A{}}, {Key: "beds", Value: bson.A{}}}); e == nil {
		t.Fatal("unique (source_namespace, external_id) missing")
	}
	// Curated local data on the imported hospital must survive upstream changes.
	now := time.Now().UTC().Truncate(time.Millisecond)
	if _, e = hospitals.UpdateOne(ctx, bson.D{{Key: "_id", Value: first.ID}}, bson.D{{Key: "$set", Value: bson.D{{Key: "bpjs", Value: true},
		{Key: "services", Value: bson.A{bson.D{{Key: "slug", Value: "icu"}, {Key: "status", Value: "available"}, {Key: "data_origin", Value: "curated"}, {Key: "source_updated_at", Value: now}, {Key: "version", Value: 3}}}},
		{Key: "beds", Value: bson.A{bson.D{{Key: "category", Value: "icu"}, {Key: "total", Value: 8}, {Key: "available", Value: 0}, {Key: "data_origin", Value: "curated"}, {Key: "version", Value: 2}}}}}}}); e != nil {
		t.Fatal(e)
	}
	changed := imported("1000000001", "RS Sandbox A Baru", t1.Add(time.Minute))
	address := "Jl. Baru 1"
	changed.Address = &address
	if r, e = p.UpsertSATUSEHATHospitals(ctx, []domain.HospitalImport{changed}); e != nil || r != (domain.ImportResult{Updated: 1}) {
		t.Fatal("update", r, e)
	}
	updated := get("1000000001")
	if updated.ID != first.ID || updated.Name != "RS Sandbox A Baru" || *updated.Address != "Jl. Baru 1" || updated.BPJS == nil || !*updated.BPJS || len(updated.Services) != 1 || len(updated.Beds) != 1 || fmt.Sprint(updated.Beds[0]["available"]) != "0" {
		t.Fatalf("curated fields overwritten: %+v", updated)
	}
	// An invalid record (validator rejects latitude) must not damage last-known-good data.
	broken := imported("1000000002", "RS Sandbox B Rusak", t1.Add(2*time.Minute))
	lat := 200.0
	broken.Latitude = &lat
	before := get("1000000002")
	if r, e = p.UpsertSATUSEHATHospitals(ctx, []domain.HospitalImport{broken, imported("1000000003", "RS Sandbox C", t1)}); e != nil || r.Failed != 1 || r.Created != 1 {
		t.Fatal("partial failure", r, e)
	}
	if after := get("1000000002"); after.Name != before.Name || !after.SyncedAt.Equal(before.SyncedAt) {
		t.Fatal("last-known-good changed", after)
	}
	// A region code that conflicts with an existing region of another kind is rejected, not overwritten.
	conflict := imported("1000000004", "RS Konflik", t1)
	conflict.ProvinceCode, conflict.CityCode = "36", "3171"
	if r, e = p.UpsertSATUSEHATHospitals(ctx, []domain.HospitalImport{conflict}); e != nil || r.Failed != 1 {
		t.Fatal("region conflict", r, e)
	}
	if n, _ := hospitals.CountDocuments(ctx, bson.D{{Key: "external_id", Value: "1000000004"}}); n != 0 {
		t.Fatal("hospital with inconsistent region written")
	}
	// Explicitly inactive upstream records disappear from public reads; simulation data stays.
	inactive := imported("1000000003", "RS Sandbox C", t1.Add(time.Minute))
	inactive.SourceActive = false
	if r, e = p.UpsertSATUSEHATHospitals(ctx, []domain.HospitalImport{inactive}); e != nil || r.Updated != 1 {
		t.Fatal("deactivate", r, e)
	}
	hs, e = p.Hospitals(ctx)
	if e != nil {
		t.Fatal(e)
	}
	simulation := 0
	for _, h := range hs {
		if h.Name == "RS Sandbox C" {
			t.Fatal("inactive hospital listed")
		}
		if h.DataOrigin == "simulation" {
			simulation++
		}
	}
	if simulation != 30 || len(hs) != 32 {
		t.Fatal("listing", simulation, len(hs))
	}
	// Seed referral and operator data are untouched by imports.
	if ref, _, e := p.Lookup(ctx, "SR-DEMO-001"); e != nil || ref.Version != 1 {
		t.Fatal("referral", e)
	}
}

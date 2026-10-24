package integration

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"siagars/backend/internal/config"
	httpapi "siagars/backend/internal/http"
	"siagars/backend/internal/repository"
	"siagars/backend/seed"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// TestCompareAgainstMongo serves GET /hospitals/compare from the real repository on a temporary
// database: a mix of a simulation seed hospital, a curated real one (null and zero beds) and an
// imported-style sandbox one with no services/beds.
func TestCompareAgainstMongo(t *testing.T) {
	p := database(t)
	ctx := context.Background()
	if e := seed.Apply(ctx, p); e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	zero, ten := 0, 10
	src := func(d time.Duration) *time.Time { v := now.Add(-d); return &v }
	curated := repository.HospitalDoc{ID: "40000000-0000-4000-8000-000000000001", Name: "RS Kurasi Uji", CityCode: "31.71", ProvinceCode: "31",
		ProvenanceDoc: repository.ProvenanceDoc{DataOrigin: "curated", SourceUpdatedAt: src(2 * time.Hour)}, Services: []repository.ServiceDoc{{Slug: "icu", Status: "limited", ProvenanceDoc: repository.ProvenanceDoc{DataOrigin: "curated", SourceUpdatedAt: src(time.Minute)}, Version: 1}},
		Beds: []repository.BedDoc{
			{Category: "icu", Total: &ten, Available: &zero, ProvenanceDoc: repository.ProvenanceDoc{DataOrigin: "curated", SourceUpdatedAt: src(time.Minute)}, Version: 1},
			{Category: "nicu", ProvenanceDoc: repository.ProvenanceDoc{DataOrigin: "curated"}, Version: 1}}}
	ext, ns := "ext-1", "test-ns"
	sandbox := repository.HospitalDoc{ID: "40000000-0000-4000-8000-000000000002", Name: "RS Sandbox Uji", CityCode: "31.71", ProvinceCode: "31", ExternalID: &ext, SourceNamespace: &ns,
		ProvenanceDoc: repository.ProvenanceDoc{DataOrigin: "satusehat_sandbox"}, Services: []repository.ServiceDoc{}, Beds: []repository.BedDoc{}}
	if _, e := p.DB.Collection("hospitals").InsertMany(ctx, []any{curated, sandbox}); e != nil {
		t.Fatal(e)
	}
	inactive := sandbox
	inactive.ID, inactive.Name = "40000000-0000-4000-8000-000000000003", "RS Nonaktif Uji"
	ext2 := "ext-2"
	inactive.ExternalID = &ext2
	if _, e := p.DB.Collection("hospitals").InsertOne(ctx, inactive); e != nil {
		t.Fatal(e)
	}
	if _, e := p.DB.Collection("hospitals").UpdateOne(ctx, bson.D{{Key: "_id", Value: inactive.ID}}, bson.D{{Key: "$set", Value: bson.D{{Key: "source_active", Value: false}}}}); e != nil {
		t.Fatal(e)
	}
	simID := "10000000-0000-4000-8000-000000000001"
	h := httpapi.New(p, config.Config{PublicOrigin: "http://localhost:3000"}, func(context.Context) error { return nil })
	get := func(ids ...string) (int, map[string]any) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/hospitals/compare?ids="+strings.Join(ids, ","), nil))
		var out map[string]any
		if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
			t.Fatal(e, w.Body.String())
		}
		return w.Code, out
	}
	code, out := get(sandbox.ID, simID, curated.ID)
	if code != 200 {
		t.Fatal(code, out)
	}
	data := out["data"].([]any)
	if len(data) != 3 || data[0].(map[string]any)["id"] != sandbox.ID || data[1].(map[string]any)["id"] != simID || data[2].(map[string]any)["id"] != curated.ID {
		t.Fatal("order", out)
	}
	for _, x := range data {
		m := x.(map[string]any)
		if m["city_name"] == "" || m["province_name"] == "" || m["services"] == nil || m["beds"] == nil {
			t.Errorf("incomplete detail DTO: %v", m)
		}
	}
	sb, sim, cur := data[0].(map[string]any), data[1].(map[string]any), data[2].(map[string]any)
	if sb["data_origin"] != "satusehat_sandbox" || sb["freshness"] != "unknown" || sb["bpjs"] != nil || len(sb["services"].([]any)) != 0 || len(sb["beds"].([]any)) != 0 || sb["verified_at"] != nil {
		t.Error("sandbox item", sb)
	}
	if sim["data_origin"] != "simulation" || len(sim["services"].([]any)) == 0 {
		t.Error("simulation item", sim["data_origin"])
	}
	beds := cur["beds"].([]any)
	icu, nicu := beds[0].(map[string]any), beds[1].(map[string]any)
	if icu["available"] != float64(0) || icu["total"] != float64(10) || icu["freshness"] != "fresh" || nicu["available"] != nil || nicu["total"] != nil || nicu["freshness"] != "unknown" {
		t.Error("null vs 0 per bed", icu, nicu)
	}
	if cur["freshness"] != "aging" || cur["services"].([]any)[0].(map[string]any)["freshness"] != "fresh" {
		t.Error("per-item provenance must be independent", cur["freshness"])
	}
	// Missing, source-inactive: 404 naming only those IDs; all-or-nothing.
	missing := "40000000-0000-4000-8000-000000000999"
	for _, bad := range [][]string{{curated.ID, missing}, {curated.ID, inactive.ID}} {
		code, out = get(bad...)
		msg, _ := out["error"].(map[string]any)["message"].(string)
		if code != 404 || out["data"] != nil || !strings.Contains(msg, bad[1]) || strings.Contains(msg, curated.ID) {
			t.Errorf("%v: %d %v", bad, code, out)
		}
	}
	if code, _ = get(curated.ID, curated.ID); code != 422 {
		t.Error("duplicate must be 422", code)
	}
	// The default list still hides simulation and never receives compare-only behavior.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/hospitals", nil))
	if w.Code != 200 || strings.Contains(w.Body.String(), simID) {
		t.Error("list contract changed", w.Code)
	}
}

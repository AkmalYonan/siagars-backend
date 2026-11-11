package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"siagars/backend/internal/config"
	"siagars/backend/internal/domain"
	"testing"
	"time"
)

// insightStore serves two small hospitals and counts how often the full list is read, so the 5-minute
// cache is observable.
type insightStore struct {
	fake
	reads *int
}

func (s insightStore) Hospitals(context.Context) ([]domain.Hospital, error) {
	*s.reads++
	now := time.Now().UTC()
	return []domain.Hospital{
		{ID: "1", CityCode: "3171", CityName: "Jakarta Selatan", ProvinceCode: "31", Provenance: domain.Provenance{DataOrigin: "curated", SourceUpdatedAt: &now}, Services: []domain.Service{{Slug: "nicu", Status: "available"}}},
		{ID: "2", CityCode: "3275", CityName: "Bekasi", ProvinceCode: "32", Provenance: domain.Provenance{DataOrigin: "simulation"}},
	}, nil
}
func (s insightStore) Services(context.Context) ([]domain.Service, error) {
	return []domain.Service{{Slug: "nicu", Name: "NICU"}}, nil
}
func (s insightStore) RegionExists(_ context.Context, code, _ string) (bool, error) {
	return code == "31" || code == "32", nil
}

func TestInsightsEndpoints(t *testing.T) {
	reads := 0
	h := New(insightStore{reads: &reads}, config.Config{PublicOrigin: "http://localhost:3000"}, nil)
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	for path, want := range map[string]int{
		"/api/v1/insights/summary": 200, "/api/v1/insights/coverage?province_code=31": 200, "/api/v1/insights/freshness?dataset=all": 200,
		"/api/v1/insights/service-gaps?service=nicu": 200, "/api/v1/insights/service-gaps": 422, "/api/v1/insights/service-gaps?service=nope": 422,
		"/api/v1/insights/summary?service=nicu": 422, "/api/v1/insights/summary?foo=1": 422, "/api/v1/insights/summary?dataset=x": 422,
		"/api/v1/insights/summary?province_code=99": 422, "/api/v1/insights/summary?province_code=": 422, "/api/v1/insights/summary?dataset=all&dataset=real": 422,
	} {
		w := get(path)
		if w.Code != want {
			t.Errorf("%s: %d %s", path, w.Code, w.Body.String())
		}
		if want != 200 && w.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s: galat harus no-store, dapat %q", path, w.Header().Get("Cache-Control"))
		}
	}
	w := get("/api/v1/insights/summary?dataset=all")
	var body struct {
		Data domain.InsightSummary
		Meta map[string]any
	}
	if w.Header().Get("Cache-Control") != "public, max-age=300" || json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Data.HospitalCount != 2 || body.Data.CuratedCount != 1 || body.Meta["generated_at"] == nil {
		t.Fatalf("%v %s", w.Header(), w.Body.String())
	}
	if src, _ := body.Meta["sources"].([]any); len(src) != 2 || src[0] != "curated" || src[1] != "simulation" {
		t.Fatalf("sources: %v", body.Meta["sources"])
	}
	// dataset=real (default) hides simulation; province filter narrows further.
	w = get("/api/v1/insights/summary")
	json.Unmarshal(w.Body.Bytes(), &body)
	if body.Data.HospitalCount != 1 {
		t.Fatalf("real: %s", w.Body.String())
	}
	// A repeated identical request is served from cache: no extra read of the hospital list.
	before := reads
	first := get("/api/v1/insights/summary?dataset=all").Body.String()
	if second := get("/api/v1/insights/summary?dataset=all").Body.String(); reads != before || first != second {
		t.Fatalf("cache tidak dipakai: reads %d→%d", before, reads)
	}
}

func TestInsightCacheExpiry(t *testing.T) {
	var c insightCache
	now := time.Now()
	c.put("k", []byte("v"), now)
	if _, ok := c.get("k", now.Add(insightsTTL-time.Second)); !ok {
		t.Fatal("harus masih ada")
	}
	if _, ok := c.get("k", now.Add(insightsTTL)); ok {
		t.Fatal("harus kedaluwarsa tepat di 5 menit")
	}
}

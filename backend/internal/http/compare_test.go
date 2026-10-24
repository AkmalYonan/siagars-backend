package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"siagars/backend/internal/config"
	"siagars/backend/internal/domain"
	"strings"
	"testing"
	"time"
)

type compareStore struct {
	fake
	hospitals []domain.Hospital
}

func (c compareStore) Hospitals(context.Context) ([]domain.Hospital, error) { return c.hospitals, nil }

const (
	cmpSim  = "10000000-0000-4000-8000-000000000001"
	cmpReal = "30000000-0000-4000-8000-000000000001"
	cmpBare = "30000000-0000-4000-8000-000000000002"
	cmpGone = "30000000-0000-4000-8000-000000000999"
)

func compareServer() (struct{}, func(string) *httptest.ResponseRecorder) {
	now := time.Now().UTC()
	zero := 0
	h := New(compareStore{hospitals: []domain.Hospital{
		{ID: cmpSim, Name: "RS Simulasi", Provenance: domain.Provenance{DataOrigin: "simulation", SourceUpdatedAt: &now},
			Services: []domain.Service{{Slug: "icu", Status: "available", Provenance: domain.Provenance{DataOrigin: "simulation", SourceUpdatedAt: &now}, Version: 1}},
			Beds:     []domain.Bed{{Category: "icu", Available: &zero, Provenance: domain.Provenance{DataOrigin: "simulation", SourceUpdatedAt: &now}, Version: 1}}},
		{ID: cmpReal, Name: "RS Nyata", Provenance: domain.Provenance{DataOrigin: "curated"}, Services: []domain.Service{}, Beds: []domain.Bed{}},
		{ID: cmpBare, Name: "RS Sandbox", Provenance: domain.Provenance{DataOrigin: "satusehat_sandbox"}, Services: []domain.Service{}, Beds: []domain.Bed{}},
	}}, config.Config{PublicOrigin: "http://localhost:3000"}, func(context.Context) error { return nil })
	return struct{}{}, func(query string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/hospitals/compare"+query, nil))
		return w
	}
}

func TestCompareHandler(t *testing.T) {
	_, get := compareServer()
	w := get("?ids=" + cmpReal + "," + cmpSim)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var body struct {
		Data []map[string]any `json:"data"`
		Meta map[string]any   `json:"meta"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &body); e != nil {
		t.Fatal(e)
	}
	if len(body.Data) != 2 || body.Data[0]["id"] != cmpReal || body.Data[1]["id"] != cmpSim || body.Meta["count"] != float64(2) || body.Meta["as_of"] == nil {
		t.Fatal(w.Body.String())
	}
	// Full detail DTO with per-item provenance; real data keeps null provenance, bed 0 stays 0.
	if body.Data[0]["freshness"] != "unknown" || body.Data[0]["verified_at"] != nil || body.Data[0]["data_origin"] != "curated" {
		t.Error("real item", body.Data[0])
	}
	bed := body.Data[1]["beds"].([]any)[0].(map[string]any)
	svc := body.Data[1]["services"].([]any)[0].(map[string]any)
	if bed["available"] != float64(0) || bed["total"] != nil || bed["data_origin"] != "simulation" || bed["freshness"] != "fresh" || svc["data_origin"] != "simulation" || svc["version"] != float64(1) {
		t.Error("simulation bed/service", bed, svc)
	}
	// Requested order is preserved, including three IDs in a different order.
	w = get("?ids=" + cmpBare + "," + cmpSim + "," + cmpReal)
	if w.Code != 200 || strings.Index(w.Body.String(), cmpBare) > strings.Index(w.Body.String(), cmpSim) || strings.Index(w.Body.String(), cmpSim) > strings.Index(w.Body.String(), cmpReal) {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestCompareHandlerErrors(t *testing.T) {
	_, get := compareServer()
	for name, c := range map[string]struct {
		query string
		want  int
	}{
		"hilang":            {"", 422},
		"satu":              {"?ids=" + cmpSim, 422},
		"empat":             {"?ids=" + cmpSim + "," + cmpReal + "," + cmpBare + "," + cmpGone, 422},
		"duplikat":          {"?ids=" + cmpSim + "," + cmpSim, 422},
		"bukan uuid":        {"?ids=" + cmpSim + ",abc", 422},
		"kapital":           {"?ids=" + cmpSim + "," + strings.ToUpper("abcdef00-0000-4000-8000-000000000001"), 422},
		"parameter lain":    {"?ids=" + cmpSim + "," + cmpReal + "&dataset=all", 422},
		"ids berulang":      {"?ids=" + cmpSim + "," + cmpReal + "&ids=" + cmpBare, 422},
		"salah satu hilang": {"?ids=" + cmpSim + "," + cmpGone, 404},
	} {
		w := get(c.query)
		if w.Code != c.want {
			t.Errorf("%s: %d %s", name, w.Code, w.Body.String())
			continue
		}
		var e struct {
			Error     struct{ Code, Message string }
			RequestID string `json:"request_id"`
			Data      any    `json:"data"`
		}
		if json.Unmarshal(w.Body.Bytes(), &e) != nil || e.Error.Code == "" || e.RequestID == "" || e.Data != nil {
			t.Errorf("%s: error body %s", name, w.Body.String())
		}
	}
	if w := get("?ids=" + cmpSim + "," + cmpGone); !strings.Contains(w.Body.String(), cmpGone) || strings.Contains(w.Body.String(), cmpSim) {
		t.Error("404 must name only the missing ID:", w.Body.String())
	}
	// A hospital literally called "compare" never shadows the detail route, and other methods get 405.
	h := New(compareStore{}, config.Config{}, func(context.Context) error { return nil })
	for method, want := range map[string]int{"POST": 405, "PATCH": 405} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, "/api/v1/hospitals/compare", nil))
		if w.Code != want || w.Header().Get("Allow") != "GET" {
			t.Errorf("%s: %d %v", method, w.Code, w.Header())
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/hospitals/"+cmpSim, nil))
	if w.Code != 404 {
		t.Error("detail route must still work:", w.Code)
	}
}

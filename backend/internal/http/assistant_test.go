package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"siagars/backend/internal/config"
	"siagars/backend/internal/domain"
	"strings"
	"testing"
)

type assistantTestStore struct {
	fake
	hospitals []domain.Hospital
	err       error
}

func (s assistantTestStore) Hospitals(context.Context) ([]domain.Hospital, error) {
	return s.hospitals, s.err
}

func (s assistantTestStore) Services(context.Context) ([]domain.Service, error) {
	return []domain.Service{{Slug: "nicu", Name: "NICU"}, {Slug: "igd-24h", Name: "IGD 24 Jam"}}, s.err
}

func (s assistantTestStore) Regions(context.Context) ([]domain.Region, error) {
	province := "32"
	return []domain.Region{
		{Code: province, Name: "Jawa Barat", Kind: "province"},
		{Code: "3275", Name: "Kota Bekasi", Kind: "city", ParentCode: &province},
		{Code: "3216", Name: "Kabupaten Bekasi", Kind: "city", ParentCode: &province},
	}, s.err
}

func (s assistantTestStore) RegionExists(context.Context, string, string) (bool, error) {
	return true, s.err
}

func assistantRequest(h http.Handler, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/api/v1/assistant/chat", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func assistantResponse(t *testing.T, w *httptest.ResponseRecorder) (assistantChatData, bool) {
	t.Helper()
	var result struct {
		Data assistantChatData
		Meta struct {
			Model     string
			Truncated bool `json:"candidates_truncated"`
		}
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Meta.Model != "rule-nlp-v1" ||
		w.Header().Get("Cache-Control") != "no-store" || result.Data.Recommendations == nil ||
		result.Data.Alternatives == nil || result.Data.EmergencyKeywords == nil {
		t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
	}
	return result.Data, result.Meta.Truncated
}

func assistantHospital(id, city string, bpjs *bool, bpjsOrigin, serviceStatus, serviceOrigin string) domain.Hospital {
	lat, lng := -6.2383, 106.9756
	return domain.Hospital{
		ID: id, Name: id, ProvinceCode: "32", CityCode: city, Latitude: &lat, Longitude: &lng,
		BPJS: bpjs, BPJSDataOrigin: &bpjsOrigin, Provenance: domain.Provenance{DataOrigin: "curated"},
		Services: []domain.Service{
			{Slug: "nicu", Status: serviceStatus, Provenance: domain.Provenance{DataOrigin: serviceOrigin}},
			{Slug: "igd-24h", Status: "available", Provenance: domain.Provenance{DataOrigin: "curated"}},
		},
	}
}

func TestAssistantMatchesAndAlternatives(t *testing.T) {
	yes, no := true, false
	candidates := []domain.Hospital{
		assistantHospital("reject", "3275", &no, "curated", "available", "curated"),
		assistantHospital("unknown", "3275", nil, "", "available", "curated"),
		assistantHospital("simulated-bpjs", "3275", &yes, "simulation", "available", "curated"),
		assistantHospital("legacy-bpjs", "3275", &yes, "", "available", "curated"),
		assistantHospital("limited", "3275", &yes, "curated", "limited", "curated"),
		assistantHospital("unknown-service", "3275", &yes, "curated", "unknown", "curated"),
		assistantHospital("simulated-service", "3275", &yes, "curated", "available", "simulation"),
		assistantHospital("other-city", "3216", &yes, "curated", "available", "curated"),
	}
	const body = `{"message":"Cari RS di Kota Bekasi yang punya NICU dan IGD serta terima BPJS","context":{"lat":0,"lng":0}}`
	store := assistantTestStore{hospitals: candidates}
	data, truncated := assistantResponse(t, assistantRequest(New(store, config.Config{}, nil), body))
	if truncated || len(data.Recommendations) != 0 || len(data.Alternatives) != 4 || data.ParsedQuery == nil ||
		data.ParsedQuery.CityCode == nil || *data.ParsedQuery.CityCode != "3275" ||
		data.ParsedQuery.BPJS != "wajib" || len(data.ParsedQuery.Services) != 2 {
		t.Fatalf("unexpected result: %+v", data)
	}
	for _, alt := range data.Alternatives {
		if alt.Recommendation.Hospital.ID == "reject" {
			if alt.ReasonCode != "bpjs_not_accepted" || alt.Recommendation.Hospital.BPJS == nil || *alt.Recommendation.Hospital.BPJS {
				t.Fatalf("unexpected rejection: %+v", alt)
			}
		} else if alt.ReasonCode != "bpjs_unknown" || alt.Recommendation.Hospital.BPJS != nil {
			t.Fatalf("unexpected unknown: %+v", alt)
		}
	}

	match := assistantHospital("match", "3275", &yes, "curated", "available", "curated")
	store.hospitals = append(candidates, match)
	h := New(store, config.Config{}, nil)
	data, _ = assistantResponse(t, assistantRequest(h, body))
	if len(data.Alternatives) != 0 || len(data.Recommendations) != 1 || data.Recommendations[0].Hospital.ID != "match" {
		t.Fatalf("unexpected primary match: %+v", data)
	}
	get := httptest.NewRecorder()
	h.ServeHTTP(get, httptest.NewRequest("GET", "/api/v1/recommendations?services=igd-24h,nicu&city_code=3275&province_code=32&bpjs=required", nil))
	var standard struct{ Data []domain.Recommendation }
	if get.Code != 200 || json.Unmarshal(get.Body.Bytes(), &standard) != nil {
		t.Fatalf("recommendations HTTP %d: %s", get.Code, get.Body.String())
	}
	for _, rec := range standard.Data {
		if rec.Hospital.ID == match.ID && rec.Score == data.Recommendations[0].Score {
			return
		}
	}
	t.Fatal("primary match score diverges from GET /recommendations")
}

func TestAssistantClarifiesAndHandlesEmergency(t *testing.T) {
	h := New(assistantTestStore{}, config.Config{}, nil)
	for _, tc := range []struct {
		body, reply string
		query       bool
	}{
		{`{"message":"NICU di Bekasi","context":{"lat":0,"lng":0}}`, "Kabupaten Bekasi", true},
		{`{"message":"Cari NICU di Kota Bekasi"}`, "wilayah yang lebih luas", true},
		{`{"message":"Cari RS di Kota Bekasi"}`, "Sebutkan layanan", true},
		{`{"message":"Cari NICU di Kota Tidak Dikenal","context":{"lat":0,"lng":0}}`, "Sebutkan kota", true},
		{`{"message":"Apa obat untuk demam?"}`, "hubungi tenaga kesehatan", false},
		{`{"message":"saya membaca komando"}`, "Saya bisa membantu", false},
		{`{"message":"Kota Bekasi","conversation_id":"10000000-0000-4000-8000-000000000001"}`, "Saya bisa membantu", false},
	} {
		data, _ := assistantResponse(t, assistantRequest(h, tc.body))
		if data.IsEmergency || (data.ParsedQuery != nil) != tc.query || !strings.Contains(data.Reply, tc.reply) {
			t.Errorf("%s: %+v", tc.body, data)
		}
	}
	offline := New(assistantTestStore{err: errors.New("offline")}, config.Config{}, nil)
	data, _ := assistantResponse(t, assistantRequest(offline, `{"message":"Cari RS, pasien PENDARAHAN-HEBAT dan tidak sadar"}`))
	if !data.IsEmergency || len(data.EmergencyKeywords) != 2 || data.EmergencyKeywords[1] != "perdarahan hebat" ||
		data.ParsedQuery != nil || !strings.Contains(data.Reply, "119") || !strings.Contains(data.Reply, "112") {
		t.Fatalf("unexpected emergency: %+v", data)
	}
	if w := assistantRequest(offline, `{"message":"Cari NICU di Kota Bekasi"}`); w.Code != 500 {
		t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
	}
}

func TestAssistantCoordinatesAndCandidateCap(t *testing.T) {
	yes := true
	var hs []domain.Hospital
	for i := 0; i < 510; i++ {
		hs = append(hs, assistantHospital(fmt.Sprintf("a-%03d", i), "3275", &yes, "curated", "unknown", "curated"))
	}
	match := assistantHospital("z-match", "3275", &yes, "curated", "available", "curated")
	hs = append(hs, match)
	h := New(assistantTestStore{hospitals: hs}, config.Config{}, nil)
	const body = `{"message":"NICU dekat saya","context":{"lat":-6.2383,"lng":106.9756}}`
	data, truncated := assistantResponse(t, assistantRequest(h, body))
	if truncated || len(data.Recommendations) != 1 || data.Recommendations[0].DistanceKM == nil ||
		*data.Recommendations[0].DistanceKM != 0 || data.ParsedQuery.CityCode != nil {
		t.Fatalf("unexpected coordinate result: %+v", data)
	}
	for i := 0; i < 510; i++ {
		m := match
		m.ID = fmt.Sprintf("match-%03d", i)
		hs = append(hs, m)
	}
	h = New(assistantTestStore{hospitals: hs}, config.Config{}, nil)
	data, truncated = assistantResponse(t, assistantRequest(h, body))
	if !truncated || len(data.Recommendations) != 10 || !strings.Contains(data.Reply, "batas jumlah kandidat") {
		t.Fatalf("unexpected capped result: %+v", data)
	}
}

func TestAssistantValidationAndRateLimit(t *testing.T) {
	h := New(assistantTestStore{}, config.Config{}, nil)
	for _, tc := range []struct {
		body string
		code int
	}{
		{`{"message":" "}`, 422},
		{`{"message":"` + strings.Repeat("é", 1001) + `"}`, 422},
		{`{"message":"NICU","conversation_id":"10000000-0000-1000-8000-000000000001"}`, 422},
		{`{"message":"NICU","context":{"lat":1}}`, 422},
		{`{"message":"NICU","context":{"lat":0,"lng":181}}`, 422},
		{`{"message":"NICU","context":{"latitude":0}}`, 400},
		{`{"message":"NICU","extra":1}`, 400},
	} {
		if w := assistantRequest(h, tc.body); w.Code != tc.code {
			t.Errorf("HTTP %d, want %d: %s", w.Code, tc.code, w.Body.String())
		}
	}
	h = New(assistantTestStore{}, config.Config{}, nil)
	for i := 0; i < 30; i++ {
		assistantResponse(t, assistantRequest(h, `{"message":"Halo"}`))
	}
	if w := assistantRequest(h, `{"message":"Halo"}`); w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
	}
}

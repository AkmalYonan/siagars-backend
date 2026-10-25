package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"siagars/backend/internal/config"
	"siagars/backend/internal/domain"
	"siagars/backend/internal/repository"
	"strings"
	"testing"
	"time"
)

const (
	extCode  = "1000000001"
	extOther = "1000000002"
)

func extPtr(s string) *string { return &s }

// directoryStore adds the optional Directory capability and an operator scope to the compare fixtures.
type directoryStore struct {
	compareStore
	scope []string
	token string
}

func (d directoryStore) Regions(context.Context) ([]domain.Region, error) {
	return []domain.Region{
		{Code: "31", Name: "DKI Jakarta", Kind: "province"}, {Code: "32", Name: "Jawa Barat", Kind: "province"},
		{Code: "3171", Name: "Kota Jakarta Pusat", Kind: "city", ParentCode: extPtr("31")},
		{Code: "31.71", Name: "Jakarta Selatan", Kind: "city", ParentCode: extPtr("31")},
		{Code: "99.99", Name: "Yatim", Kind: "city", ParentCode: extPtr("00")},
	}, nil
}
func (d directoryStore) SuggestHospitals(_ context.Context, q, dataset string, limit int) ([]domain.Suggestion, error) {
	return []domain.Suggestion{{ID: cmpSim, Name: "RS " + q + " " + dataset, CityName: "Kota", DataOrigin: "simulation"}}, nil
}
func (d directoryStore) Session(_ context.Context, token string) (domain.Operator, string, error) {
	if token != d.token {
		return domain.Operator{}, "", repository.ErrNotFound
	}
	return domain.Operator{ID: "operator", Email: "operator@example.invalid", HospitalIDs: d.scope}, repository.Hash(strings.Repeat("c", 64)), nil
}

func facilityServer(scope ...string) http.Handler {
	now := time.Now().UTC()
	total, avail := 40, 12
	hospitals := []domain.Hospital{
		{ID: cmpSim, Name: "RS Simulasi", Provenance: domain.Provenance{DataOrigin: "simulation", SourceUpdatedAt: &now},
			Services: []domain.Service{{Slug: "icu", Status: "available", Provenance: domain.Provenance{DataOrigin: "simulation", SourceUpdatedAt: &now}, Version: 1}, {Slug: "rawat-inap", Status: "limited", Provenance: domain.Provenance{DataOrigin: "simulation", SourceUpdatedAt: &now}, Version: 1}},
			Beds:     []domain.Bed{{Category: "general", Total: &total, Available: &avail, Provenance: domain.Provenance{DataOrigin: "simulation", SourceUpdatedAt: &now}, Version: 1}}},
		{ID: cmpReal, ExternalID: extPtr(extCode), Name: "RS Sandbox A", Provenance: domain.Provenance{DataOrigin: "satusehat_sandbox"}, Services: []domain.Service{}, Beds: []domain.Bed{}},
		{ID: cmpBare, ExternalID: extPtr(extOther), Name: "RS Sandbox B", Provenance: domain.Provenance{DataOrigin: "satusehat_sandbox"}, Services: []domain.Service{}, Beds: []domain.Bed{}},
	}
	return New(directoryStore{compareStore: compareStore{hospitals: hospitals}, scope: scope, token: strings.Repeat("a", 64)},
		config.Config{PublicOrigin: "http://localhost:3000"}, func(context.Context) error { return nil })
}

func fetch(h http.Handler, path string, cookie bool) (*httptest.ResponseRecorder, map[string]any) {
	r := httptest.NewRequest("GET", "/api/v1"+path, nil)
	if cookie {
		r.AddCookie(&http.Cookie{Name: "siagars_session", Value: strings.Repeat("a", 64)})
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w, body
}

func TestValidExternalCode(t *testing.T) {
	for code, want := range map[string]bool{
		"1000000001": true, "abc": true, "RS_01-x": true, strings.Repeat("a", 64): true,
		"": false, strings.Repeat("a", 65): false, "bad id": false, "$ne": false, "a/b": false, "x.y": false, "a,b": false,
		// UUID-shaped tokens must be valid lowercase UUIDs, so an uppercase UUID is never a "code".
		"10000000-0000-4000-8000-000000000001": true,
		"ABCDEF00-0000-4000-8000-000000000001": false,
	} {
		if got := validHospitalRef(code); got != want {
			t.Errorf("%q: got %v want %v", code, got, want)
		}
	}
}

func TestDetailAndCompareAcceptExternalCode(t *testing.T) {
	h := facilityServer()
	w, body := fetch(h, "/hospitals/"+extCode, false)
	if w.Code != 200 || obj(body["data"])["id"] != cmpReal || obj(body["data"])["external_id"] != extCode {
		t.Fatal(w.Code, w.Body.String())
	}
	if w, _ := fetch(h, "/hospitals/unknown-code", false); w.Code != 404 {
		t.Fatal(w.Code)
	}
	for _, bad := range []string{"%24ne", "ABCDEF00-0000-4000-8000-000000000001", "bad%20id"} {
		if w, _ := fetch(h, "/hospitals/"+bad, false); w.Code != 404 {
			t.Errorf("%s: %d", bad, w.Code)
		}
	}
	// A UUID and an external code, in request order; the response is always the full detail DTO.
	w, body = fetch(h, "/hospitals/compare?ids="+extCode+","+cmpSim+","+extOther, false)
	data, _ := body["data"].([]any)
	if w.Code != 200 || len(data) != 3 || obj(data[0])["id"] != cmpReal || obj(data[1])["id"] != cmpSim || obj(data[2])["id"] != cmpBare {
		t.Fatal(w.Code, w.Body.String())
	}
	for name, ids := range map[string]string{
		"kode dan UUID sama": extCode + "," + cmpReal, "kode duplikat": extCode + "," + extCode, "kode berbahaya": extCode + ",%24where",
		"UUID kapital": extCode + ",ABCDEF00-0000-4000-8000-000000000001",
	} {
		if w, body := fetch(h, "/hospitals/compare?ids="+ids, false); w.Code != 422 || obj(body["error"])["code"] != "validation_error" {
			t.Errorf("%s: %d %s", name, w.Code, w.Body.String())
		}
	}
	w, body = fetch(h, "/hospitals/compare?ids="+extCode+",9999999999", false)
	if w.Code != 404 || !strings.Contains(obj(body["error"])["message"].(string), "9999999999") || strings.Contains(obj(body["error"])["message"].(string), extCode+",") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestFacilitySummaryKeepsUnknownAsNull(t *testing.T) {
	h := facilityServer()
	_, body := fetch(h, "/hospitals/"+extCode, false)
	sum := obj(obj(body["data"])["facility_summary"])
	inp := obj(sum["inpatient_beds"])
	if sum["icu"] != "unknown" || sum["nicu"] != "unknown" || sum["picu"] != "unknown" || sum["rawat_inap"] != "unknown" || sum["bpjs"] != nil || inp["total"] != nil || inp["available"] != nil || inp["freshness"] != "unknown" || sum["last_source_updated_at"] != nil {
		t.Fatalf("data tidak ada harus unknown/null: %v", sum)
	}
	_, body = fetch(h, "/hospitals/"+cmpSim, false)
	sum = obj(obj(body["data"])["facility_summary"])
	inp = obj(sum["inpatient_beds"])
	if sum["icu"] != "available" || sum["rawat_inap"] != "limited" || sum["nicu"] != "unknown" || inp["total"] != float64(40) || inp["available"] != float64(12) || inp["freshness"] != "fresh" || sum["last_source_updated_at"] == nil {
		t.Fatalf("ringkasan salah: %v", sum)
	}
}

func obj(v any) map[string]any { m, _ := v.(map[string]any); return m }

func TestRegionsNestsCitiesAndKeepsCodesVerbatim(t *testing.T) {
	h := facilityServer()
	w, body := fetch(h, "/regions", false)
	data, _ := body["data"].([]any)
	if w.Code != 200 || len(data) != 2 {
		t.Fatal(w.Code, w.Body.String())
	}
	jakarta := obj(data[0])
	cities, _ := jakarta["cities"].([]any)
	if jakarta["code"] != "31" || len(cities) != 2 || obj(cities[0])["code"] != "3171" || obj(cities[1])["code"] != "31.71" || obj(data[1])["cities"] == nil {
		t.Fatalf("kota harus bersarang dan kode apa adanya: %s", w.Body.String())
	}
	if w, _ := fetch(h, "/regions?x=1", false); w.Code != 422 {
		t.Fatal(w.Code)
	}
}

func TestSuggestValidation(t *testing.T) {
	h := facilityServer()
	if w, body := fetch(h, "/hospitals/suggest?q=siaga&limit=5&dataset=all", false); w.Code != 200 || obj(body["meta"])["dataset"] != "all" {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, q := range []string{"", "?q=", "?q=%20", "?q=a&q=b", "?q=a&limit=0", "?q=a&limit=21", "?q=a&limit=x", "?q=a&dataset=fake", "?q=a&sort=name", "?q=" + strings.Repeat("a", 101)} {
		if w, _ := fetch(h, "/hospitals/suggest"+q, false); w.Code != 422 {
			t.Errorf("%q: %d", q, w.Code)
		}
	}
	// A leading $ is plain text, not an operator or field path.
	if w, _ := fetch(h, "/hospitals/suggest?q=%24where", false); w.Code != 200 {
		t.Fatal(w.Code)
	}
	r := httptest.NewRequest("POST", "/api/v1/hospitals/suggest?q=a", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 405 {
		t.Fatal(w.Code)
	}
}

func TestAdminHospitalsAreScopedToTheSession(t *testing.T) {
	h := facilityServer(cmpSim, cmpBare)
	if w, _ := fetch(h, "/admin/hospitals", false); w.Code != 401 {
		t.Fatal("tanpa sesi:", w.Code)
	}
	w, body := fetch(h, "/admin/hospitals", true)
	data, _ := body["data"].([]any)
	if w.Code != 200 || len(data) != 2 || obj(data[0])["id"] != cmpSim || obj(data[1])["id"] != cmpBare || obj(body["meta"])["count"] != float64(2) {
		t.Fatal(w.Code, w.Body.String())
	}
	if w, _ := fetch(h, "/admin/hospitals?hospital_id="+cmpReal, true); w.Code != 422 {
		t.Fatal("parameter tidak didukung:", w.Code)
	}
	if w, body := fetch(h, "/admin/hospitals/"+cmpSim, true); w.Code != 200 || obj(body["data"])["id"] != cmpSim {
		t.Fatal(w.Code, w.Body.String())
	}
	// Out of scope: 403 even though the hospital exists, and it never leaks whether another RS exists.
	if w, _ := fetch(h, "/admin/hospitals/"+cmpReal, true); w.Code != 403 {
		t.Fatal("di luar scope:", w.Code)
	}
	if w, _ := fetch(h, "/admin/hospitals/30000000-0000-4000-8000-000000000999", true); w.Code != 403 {
		t.Fatal("ID tak dikenal di luar scope juga 403:", w.Code)
	}
	if w, _ := fetch(h, "/admin/hospitals/"+extCode, true); w.Code != 404 {
		t.Fatal("kode eksternal bukan ID admin:", w.Code)
	}
	if w, _ := fetch(h, "/admin/hospitals/"+cmpSim, false); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if empty := facilityServer(); true {
		if w, body := fetch(empty, "/admin/hospitals", true); w.Code != 200 || len(body["data"].([]any)) != 0 {
			t.Fatal("scope kosong:", w.Code, w.Body.String())
		}
		if w, _ := fetch(empty, "/admin/hospitals/"+cmpSim, true); w.Code != 403 {
			t.Fatal("scope kosong:", w.Code)
		}
	}
}

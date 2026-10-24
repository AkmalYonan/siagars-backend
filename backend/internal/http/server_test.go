package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"siagars/backend/internal/config"
	"siagars/backend/internal/domain"
	"siagars/backend/internal/repository"
	"strings"
	"testing"
	"time"
)

type fake struct {
	Store
	sessionError error
	csrf         string
}

func (f fake) Session(context.Context, string) (domain.Operator, string, error) {
	return domain.Operator{ID: "operator", HospitalIDs: []string{}}, repository.Hash(f.csrf), f.sessionError
}
func (f fake) Services(context.Context) ([]domain.Service, error)   { return []domain.Service{}, nil }
func (f fake) Hospitals(context.Context) ([]domain.Hospital, error) { return []domain.Hospital{}, nil }
func (f fake) Referrals(context.Context, string) ([]domain.Referral, error) {
	return []domain.Referral{}, nil
}
func (f fake) Lookup(context.Context, string) (domain.Referral, []domain.History, error) {
	return domain.Referral{Status: "searching"}, []domain.History{}, nil
}
func TestHTTPBoundaries(t *testing.T) {
	cfg := config.Config{PublicOrigin: "http://localhost:3000", SessionTTL: 8 * time.Hour}
	h := New(fake{csrf: strings.Repeat("a", 64)}, cfg, func(context.Context) error { return errors.New("offline") })
	for _, c := range []struct {
		method, path, body, origin, csrf, cookie string
		want                                     int
	}{
		{"GET", "/api/v1/health", "", "", "", "", 200}, {"GET", "/api/v1/ready", "", "", "", "", 503}, {"POST", "/api/v1/health", "", "", "", "", 405}, {"GET", "/missing", "", "", "", "", 404},
		{"GET", "/api/v1/hospitals?unknown=yes", "", "", "", "", 422}, {"GET", "/api/v1/hospitals?dataset=fake", "", "", "", "", 422}, {"GET", "/api/v1/hospitals?dataset=simulation", "", "", "", "", 200}, {"GET", "/api/v1/auth/me", "", "", "", "", 401},
		{"GET", "/api/v1/admin/referrals", "", "", "", strings.Repeat("b", 64), 200},
		{"POST", "/api/v1/auth/login", "{}", "https://evil.test", "", "", 403},
		{"POST", "/api/v1/referrals/lookup", "{", "", "", "", 400},
		{"POST", "/api/v1/referrals/lookup", "{\"extra\":1}", "", "", "", 400},
		{"POST", "/api/v1/referrals/lookup", "{}", "", "", "", 404},
		{"POST", "/api/v1/auth/logout", "{}", "http://localhost:3000", "bad", strings.Repeat("b", 64), 403},
		{"PATCH", "/api/v1/admin/referrals/20000000-0000-4000-8000-000000000001/status", "{\"status\":\"wrong\",\"version\":1}", "http://localhost:3000", strings.Repeat("a", 64), strings.Repeat("b", 64), 422},
	} {
		r := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", c.origin)
		r.Header.Set("X-CSRF-Token", c.csrf)
		if c.cookie != "" {
			r.AddCookie(&http.Cookie{Name: "siagars_session", Value: c.cookie})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != c.want {
			t.Errorf("%s %s: %d body=%s", c.method, c.path, w.Code, w.Body.String())
		}
		if w.Header().Get("X-Request-ID") == "" || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("headers")
		}
	}
}
func TestExpiredSession(t *testing.T) {
	s := &Server{repo: fake{sessionError: repository.ErrNotFound}}
	r := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	r.AddCookie(&http.Cookie{Name: "siagars_session", Value: strings.Repeat("b", 64)})
	w := httptest.NewRecorder()
	s.me(w, r)
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
}
func TestLookupDTO(t *testing.T) {
	s := &Server{repo: fake{}}
	r := httptest.NewRequest("POST", "/api/v1/referrals/lookup", strings.NewReader("{\"tracking_code\":\"SR-DEMO-001\"}"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.lookup(w, r)
	body := w.Body.String()
	for _, private := range []string{"owner_hospital_id", "tracking_token_hash", "actor_id", "version", "internal_note"} {
		if strings.Contains(body, private) {
			t.Fatal(body)
		}
	}
	if !strings.Contains(body, "public_status") {
		t.Fatal(body)
	}
}
func TestLimiterWindowAndCap(t *testing.T) {
	var l limiter
	now := time.Now()
	for i := 0; i < 5; i++ {
		if !l.allow("key", 5, time.Minute, now) {
			t.Fatal(i)
		}
	}
	if l.allow("key", 5, time.Minute, now) {
		t.Fatal("limit")
	}
	if !l.allow("key", 5, time.Minute, now.Add(time.Minute)) {
		t.Fatal("expiry")
	}
}
func TestBodyLimit(t *testing.T) {
	s := &Server{}
	h := s.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { var b map[string]any; decode(w, r, &b) }))
	r := httptest.NewRequest("POST", "/", strings.NewReader("{\"x\":\""+strings.Repeat("a", 70<<10)+"\"}"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 413 {
		t.Fatal(w.Code)
	}
}
func TestDocs(t *testing.T) {
	h := New(fake{}, config.Config{PublicOrigin: "http://localhost:3000"}, func(context.Context) error { return nil })
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	page := get("/docs")
	if page.Code != 200 || !strings.HasPrefix(page.Header().Get("Content-Type"), "text/html") || !strings.Contains(page.Body.String(), "swagger-ui-bundle.js") || !strings.Contains(page.Header().Get("Content-Security-Policy"), "script-src 'self' https://cdn.jsdelivr.net") {
		t.Fatal(page.Code, page.Header())
	}
	if strings.Contains(page.Body.String(), "<script>") {
		t.Fatal("inline script would violate the CSP")
	}
	spec := get("/docs/openapi.json")
	var doc struct {
		OpenAPI string                    `json:"openapi"`
		Servers []map[string]string       `json:"servers"`
		Paths   map[string]map[string]any `json:"paths"`
	}
	if spec.Code != 200 || json.Unmarshal(spec.Body.Bytes(), &doc) != nil || doc.OpenAPI != "3.0.3" || len(doc.Servers) != 1 || doc.Servers[0]["url"] != "/api/v1" || len(doc.Paths) != 15 {
		t.Fatal(spec.Code, doc.OpenAPI, doc.Servers, len(doc.Paths))
	}
	if w := get("/docs/swagger-init.js"); w.Code != 200 || !strings.Contains(w.Body.String(), "/docs/openapi.json") {
		t.Fatal(w.Code)
	}
	if w := get("/docs/"); w.Code != 301 || w.Header().Get("Location") != "/docs" {
		t.Fatal(w.Code)
	}
	if w := get("/docs/unknown"); w.Code != 404 {
		t.Fatal(w.Code)
	}
}

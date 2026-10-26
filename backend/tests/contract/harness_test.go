package contract

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"siagars/backend/internal/config"
	"siagars/backend/internal/domain"
	httpapi "siagars/backend/internal/http"
	"siagars/backend/internal/repository"
)

const (
	origin   = "http://localhost:3000"
	password = "contract-test-password"
	csrfRaw  = "c" // only used to derive session-bound tokens below
)

var (
	h1 = "10000000-0000-4000-8000-000000000001"
	h2 = "10000000-0000-4000-8000-000000000002"
	// refA belongs to hospital h1 (operator A), refB to h2 (operator B).
	refA = "20000000-0000-4000-8000-000000000001"
	refB = "20000000-0000-4000-8000-000000000002"
	// Sessions are 64 chars to pass the cookie shape check; the CSRF value is what the fake stores a hash of.
	tokenA = strings.Repeat("a", 64)
	tokenB = strings.Repeat("b", 64)
	csrf   = strings.Repeat(csrfRaw, 64)
)

type referral struct {
	domain.Referral
	code    string
	history []domain.History
}

// store is an in-memory httpapi.Store. It enforces the same rules the Mongo repository promises
// (operator scope, optimistic version, state machine) so handler error mapping can be asserted without Atlas.
type store struct {
	httpapi.Store
	mu        sync.Mutex
	hospitals []domain.Hospital
	services  []domain.Service
	regions   map[string]string
	hash      string
	sessions  map[string]session
	referrals []*referral
	err       error // when set, Hospitals fails with it

	audit   []string // request ids recorded by mutations
	revoked []string
	created []string
}
type session struct {
	op   domain.Operator
	csrf string
}

func (s *store) Hospitals(context.Context) ([]domain.Hospital, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	out := make([]domain.Hospital, len(s.hospitals))
	for i, h := range s.hospitals {
		h.Services = append([]domain.Service{}, h.Services...)
		h.Beds = append([]domain.Bed{}, h.Beds...)
		out[i] = h
	}
	return out, nil
}
func (s *store) Services(context.Context) ([]domain.Service, error) { return s.services, nil }
func (s *store) RegionExists(_ context.Context, code, kind string) (bool, error) {
	return s.regions[code] == kind, nil
}
func (s *store) Credentials(_ context.Context, email string) (string, string, error) {
	if email != "operator-a@example.test" {
		return "", "", repository.ErrNotFound
	}
	return "30000000-0000-4000-8000-00000000000a", s.hash, nil
}
func (s *store) CreateSession(_ context.Context, id, token, csrfValue string, _ time.Time, old string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, old)
	s.created = append(s.created, token)
	s.sessions[token] = session{domain.Operator{ID: id, Email: "operator-a@example.test", HospitalIDs: []string{h1}}, repository.Hash(csrfValue)}
	return nil
}
func (s *store) Session(_ context.Context, token string) (domain.Operator, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, ok := s.sessions[token]
	if !ok {
		return domain.Operator{}, "", repository.ErrNotFound
	}
	return x.op, x.csrf, nil
}
func (s *store) Revoke(_ context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revoked = append(s.revoked, token)
	delete(s.sessions, token)
	return nil
}
func (s *store) scoped(operator, hospital string) bool {
	for _, x := range s.sessions {
		if x.op.ID == operator {
			for _, id := range x.op.HospitalIDs {
				if id == hospital {
					return true
				}
			}
		}
	}
	return false
}
func (s *store) Referrals(_ context.Context, operator string) ([]domain.AdminReferral, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []domain.AdminReferral{}
	for _, r := range s.referrals {
		if s.scoped(operator, r.OwnerHospitalID) {
			name := ""
			for _, h := range s.hospitals {
				if h.ID == r.OwnerHospitalID {
					name = h.Name
				}
			}
			out = append(out, domain.AdminReferral{Referral: r.Referral, HospitalName: name})
		}
	}
	return out, nil
}
func (s *store) Lookup(_ context.Context, code string) (domain.Referral, []domain.History, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.referrals {
		if r.code == code {
			return r.Referral, append([]domain.History{}, r.history...), nil
		}
	}
	return domain.Referral{}, nil, repository.ErrNotFound
}
func (s *store) UpdateReferral(_ context.Context, operator, id, status, note string, version int, request string) (domain.Referral, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.referrals {
		if r.ID != id {
			continue
		}
		switch {
		case !s.scoped(operator, r.OwnerHospitalID):
			return domain.Referral{}, repository.ErrForbidden
		case r.Version != version || !domain.CanTransition(r.Status, status, note):
			return domain.Referral{}, repository.ErrConflict
		}
		from := r.Status
		r.Status, r.PublicNote, r.UpdatedAt = status, note, time.Now().UTC()
		r.Version++
		r.history = append(r.history, domain.History{FromStatus: &from, ToStatus: status, PublicNote: note, CreatedAt: r.UpdatedAt})
		s.audit = append(s.audit, request)
		return r.Referral, nil
	}
	return domain.Referral{}, repository.ErrNotFound
}
func (s *store) UpdateFacility(_ context.Context, operator, hospital, key string, version int, status *string, total, available *int, request string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.hospitals {
		h := &s.hospitals[i]
		if h.ID != hospital {
			continue
		}
		if !s.scoped(operator, hospital) {
			return repository.ErrForbidden
		}
		now := time.Now().UTC()
		if status != nil {
			for j := range h.Services {
				if x := &h.Services[j]; x.Slug == key {
					if x.Version != version {
						return repository.ErrConflict
					}
					x.Status, x.Version, x.DataOrigin, x.SourceUpdatedAt = *status, x.Version+1, "simulation", &now
					s.audit = append(s.audit, request)
					return nil
				}
			}
			return repository.ErrNotFound
		}
		for j := range h.Beds {
			if x := &h.Beds[j]; x.Category == key {
				if x.Version != version {
					return repository.ErrConflict
				}
				x.Total, x.Available, x.Version, x.DataOrigin, x.SourceUpdatedAt = total, available, x.Version+1, "simulation", &now
				s.audit = append(s.audit, request)
				return nil
			}
		}
		return repository.ErrNotFound
	}
	return repository.ErrNotFound
}

// rig bundles a handler with its backing store and the spec used to check responses.
type rig struct {
	t       *testing.T
	h       http.Handler
	store   *store
	spec    spec
	ready   error
	cfg     config.Config
	addrSeq int
}

func newRig(t *testing.T, hospitals []domain.Hospital) *rig {
	t.Helper()
	hash, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if e != nil {
		t.Fatal(e)
	}
	st := &store{
		hospitals: hospitals,
		services:  []domain.Service{{Slug: "icu", Name: "ICU", Category: "critical_care"}, {Slug: "nicu", Name: "NICU", Category: "critical_care"}, {Slug: "igd", Name: "IGD", Category: "emergency"}},
		regions:   map[string]string{"31": "province", "32": "province", "3171": "city", "3273": "city"},
		hash:      string(hash),
		sessions:  map[string]session{},
	}
	st.sessions[tokenA] = session{domain.Operator{ID: "30000000-0000-4000-8000-00000000000a", Email: "operator-a@example.test", HospitalIDs: []string{h1}}, repository.Hash(csrf)}
	st.sessions[tokenB] = session{domain.Operator{ID: "30000000-0000-4000-8000-00000000000b", Email: "operator-b@example.test", HospitalIDs: []string{h2}}, repository.Hash(csrf)}
	up := time.Date(2026, 10, 23, 7, 0, 0, 0, time.UTC)
	dest := h1
	st.referrals = []*referral{
		{Referral: domain.Referral{ID: refA, OwnerHospitalID: h1, DestinationHospitalID: &dest, Status: "submitted", PublicNote: "Skenario publik sintetis", NextStep: "Menunggu verifikasi", Version: 1, IsDemo: true, UpdatedAt: up}, code: "SR-DEMO-001",
			history: []domain.History{{ToStatus: "submitted", PublicNote: "Skenario publik sintetis", CreatedAt: up}}},
		{Referral: domain.Referral{ID: refB, OwnerHospitalID: h2, Status: "confirmed", PublicNote: "ok", NextStep: "selesai", Version: 3, IsDemo: true, UpdatedAt: up}, code: "SR-DEMO-002"},
	}
	r := &rig{t: t, store: st, spec: loadSpec(t), cfg: config.Config{PublicOrigin: origin, SessionTTL: 8 * time.Hour}}
	r.reload()
	return r
}

// reload rebuilds the handler (fresh rate limiters) after r.cfg was modified.
func (r *rig) reload() {
	r.h = httpapi.New(r.store, r.cfg, func(context.Context) error { return r.ready })
}

type call struct {
	method, path, body string
	origin, csrf       string
	cookie             string
	contentType        string
	remote             string
	header             map[string]string
}

// do sends one request and fails the test unless the response is documented in the OpenAPI contract:
// declared status, schema-valid JSON body, X-Request-ID, no-store and (for errors) matching request_id.
func (r *rig) do(c call) (*httptest.ResponseRecorder, map[string]any) {
	r.t.Helper()
	w, body := r.raw(c)
	r.conform(c, w, body)
	return w, body
}
func (r *rig) raw(c call) (*httptest.ResponseRecorder, map[string]any) {
	r.t.Helper()
	req := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
	switch ct := c.contentType; {
	case ct == "-":
	case ct == "":
		req.Header.Set("Content-Type", "application/json")
	default:
		req.Header.Set("Content-Type", ct)
	}
	if c.origin != "" {
		req.Header.Set("Origin", c.origin)
	}
	if c.csrf != "" {
		req.Header.Set("X-CSRF-Token", c.csrf)
	}
	if c.cookie != "" {
		req.AddCookie(&http.Cookie{Name: "siagars_session", Value: c.cookie})
	}
	if c.remote != "" {
		req.RemoteAddr = c.remote
	}
	for k, v := range c.header {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.h.ServeHTTP(w, req)
	var body map[string]any
	if w.Body.Len() > 0 {
		if e := decodeJSON(w.Body.Bytes(), &body); e != nil {
			r.t.Fatalf("%s %s: body bukan JSON: %v: %s", c.method, c.path, e, w.Body.String())
		}
	}
	return w, body
}

// template maps a concrete request path back to its OpenAPI path template ("" if the path is not in the contract).
// Literal templates win over parameterised ones, as in net/http (e.g. /hospitals/compare over /hospitals/{id}).
func (r *rig) template(path string) string {
	parts := strings.Split(strings.TrimPrefix(strings.SplitN(path, "?", 2)[0], "/api/v1"), "/")
	best, bestParams := "", 1<<30
next:
	for tmpl := range r.spec.paths() {
		want := strings.Split(tmpl, "/")
		if len(want) != len(parts) {
			continue
		}
		params := 0
		for i := range want {
			if strings.HasPrefix(want[i], "{") {
				params++
			} else if want[i] != parts[i] {
				continue next
			}
		}
		if params < bestParams {
			best, bestParams = tmpl, params
		}
	}
	return best
}
func (r *rig) conform(c call, w *httptest.ResponseRecorder, body map[string]any) {
	r.t.Helper()
	tmpl := r.template(c.path)
	if w.Header().Get("X-Request-ID") == "" || w.Header().Get("Cache-Control") != "no-store" {
		r.t.Errorf("%s %s: header X-Request-ID/Cache-Control no-store hilang: %v", c.method, c.path, w.Header())
	}
	if tmpl == "" {
		return // unknown routes are outside the contract
	}
	method := strings.ToLower(c.method)
	lookup := tmpl
	if _, declared := r.spec.paths()[tmpl][method]; !declared {
		if w.Code != 405 {
			r.t.Errorf("%s %s: metode tidak ada di OpenAPI tetapi status %d, harus 405", c.method, c.path, w.Code)
		}
		for m := range r.spec.paths()[tmpl] {
			method = m
		}
	}
	schema, ok := r.spec.responseSchema(method, lookup, w.Code)
	if !ok {
		r.t.Errorf("%s %s: status %d tidak terdokumentasi di OpenAPI", c.method, c.path, w.Code)
		return
	}
	for _, e := range r.spec.validate(schema, any(body)) {
		r.t.Errorf("%s %s [%d]: %s\n%s", c.method, c.path, w.Code, e, w.Body.String())
	}
	if w.Code >= 400 {
		if got, _ := body["request_id"].(string); got == "" || got != w.Header().Get("X-Request-ID") {
			r.t.Errorf("%s %s: request_id body %q != header %q", c.method, c.path, got, w.Header().Get("X-Request-ID"))
		}
	}
}

// ip returns a fresh client address so rate limit buckets never leak between cases.
func (r *rig) ip() string {
	r.addrSeq++
	return fmt.Sprintf("198.51.100.%d:1234", r.addrSeq%250+1)
}
func mutate(method, path, body, cookie string) call {
	return call{method: method, path: path, body: body, origin: origin, csrf: csrf, cookie: cookie}
}
func errorCode(t *testing.T, body map[string]any) string {
	t.Helper()
	e := obj(body["error"])
	c, _ := e["code"].(string)
	if c == "" {
		t.Fatalf("bukan body error: %v", body)
	}
	return c
}
func dataList(t *testing.T, body map[string]any) []any {
	t.Helper()
	l, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("data bukan array: %v", body["data"])
	}
	return l
}
func ids(t *testing.T, body map[string]any) []string {
	t.Helper()
	out := []string{}
	for _, x := range dataList(t, body) {
		out = append(out, obj(x)["id"].(string))
	}
	return out
}
func equalStrings(a, b []string) bool { return strings.Join(a, ",") == strings.Join(b, ",") }
func mustJSON(v any) string {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return string(b)
}

var errBoom = errors.New("mongo: auth failed for mongodb+srv://secret-user:secret-pw@cluster0.example.test")

// num reads a JSON number decoded with UseNumber.
func num(v any) float64 {
	n, ok := v.(json.Number)
	if !ok {
		return math.NaN()
	}
	return float(n)
}

// logs captures everything the server writes through slog so tests can assert secrets never reach it.
var logs struct {
	sync.Mutex
	bytes.Buffer
}

type logWriter struct{}

func (logWriter) Write(b []byte) (int, error) {
	logs.Lock()
	defer logs.Unlock()
	return logs.Buffer.Write(b)
}
func logged() string { logs.Lock(); defer logs.Unlock(); return logs.String() }

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(logWriter{}, nil)))
	os.Exit(m.Run())
}

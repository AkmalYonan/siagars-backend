package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"siagars/backend/docs"
	"siagars/backend/internal/config"
	"siagars/backend/internal/domain"
	"siagars/backend/internal/repository"
	"siagars/backend/internal/service"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type Store interface {
	Hospitals(context.Context) ([]domain.Hospital, error)
	Services(context.Context) ([]domain.Service, error)
	RegionExists(context.Context, string, string) (bool, error)
	Credentials(context.Context, string) (string, string, error)
	CreateSession(context.Context, string, string, string, time.Time, string) error
	Session(context.Context, string) (domain.Operator, string, error)
	Revoke(context.Context, string) error
	Referrals(context.Context, string) ([]domain.Referral, error)
	Lookup(context.Context, string) (domain.Referral, []domain.History, error)
	UpdateReferral(context.Context, string, string, string, string, int, string) (domain.Referral, error)
	UpdateFacility(context.Context, string, string, string, int, *string, *int, *int, string) error
}

// Searcher and HospitalReader are optional Store capabilities: when the store implements them, search
// and detail reads run in the database instead of loading every hospital. Other stores fall back to Hospitals.
type Searcher interface {
	SearchHospitals(context.Context, service.Search, time.Time) ([]domain.Hospital, int, error)
}
type HospitalReader interface {
	Hospital(context.Context, string) (domain.Hospital, error)
}

// ExternalReader resolves a hospital by its upstream code (SATUSEHAT kode_satusehat).
type ExternalReader interface {
	HospitalByExternalID(context.Context, string) (domain.Hospital, error)
}

// Directory serves the region master and name suggestions; stores without it answer 501.
type Directory interface {
	Regions(context.Context) ([]domain.Region, error)
	SuggestHospitals(ctx context.Context, q, dataset string, limit int) ([]domain.Suggestion, error)
}
type Server struct {
	repo  Store
	cfg   config.Config
	ready func(context.Context) error
	lim   limiter
	dummy []byte
}
type contextKey int

const requestKey contextKey = 0

func New(repo Store, cfg config.Config, ready func(context.Context) error) http.Handler {
	dummy, _ := bcrypt.GenerateFromPassword([]byte(random()), 12)
	s := &Server{repo: repo, cfg: cfg, ready: ready, dummy: dummy}
	mux := http.NewServeMux()
	routes := []struct {
		method, path string
		handler      http.HandlerFunc
	}{
		{"GET", "/health", s.health}, {"GET", "/ready", s.readiness}, {"GET", "/services", s.services}, {"GET", "/hospitals", s.hospitals}, {"GET", "/hospitals/compare", s.compare}, {"GET", "/hospitals/suggest", s.suggest}, {"GET", "/regions", s.regions}, {"GET", "/hospitals/{id}", s.hospital},
		{"POST", "/referrals/lookup", s.lookup}, {"POST", "/bpjs/check", s.bpjsCheck}, {"GET", "/admin/bpjs-checks", s.bpjsHistory}, {"POST", "/admin/bpjs-checks/{id}/reveal", s.bpjsReveal}, {"POST", "/auth/login", s.login}, {"GET", "/auth/me", s.me}, {"GET", "/auth/csrf", s.csrf}, {"POST", "/auth/logout", s.logout},
		{"GET", "/admin/referrals", s.referrals}, {"GET", "/admin/hospitals", s.adminHospitals}, {"GET", "/admin/hospitals/{id}", s.adminHospital}, {"PATCH", "/admin/referrals/{id}/status", s.updateReferral}, {"PATCH", "/admin/hospitals/{id}/services/{slug}", s.updateService}, {"PATCH", "/admin/hospitals/{id}/beds/{category}", s.updateBed},
	}
	for _, route := range routes {
		path := "/api/v1" + route.path
		mux.HandleFunc(route.method+" "+path, route.handler)
		if route.path == "/hospitals/compare" || route.path == "/hospitals/suggest" {
			// An all-methods fallback here would conflict with GET /hospitals/{id}; other methods on this
			// path already reach the {id} fallback, which answers 405 with Allow: GET.
			continue
		}
		method := route.method
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Allow", method)
			fail(w, r, 405, "method_not_allowed", "Metode tidak didukung")
		})
	}
	registerDocs(mux)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { fail(w, r, 404, "not_found", "Resource tidak ditemukan") })
	return s.middleware(mux)
}
func random() string {
	var b [32]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b[:])
}
func rid(r *http.Request) string { v, _ := r.Context().Value(requestKey).(string); return v }
func send(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}
func ok(w http.ResponseWriter, data any) { send(w, 200, map[string]any{"data": data}) }
func fail(w http.ResponseWriter, r *http.Request, code int, key, message string) {
	send(w, code, map[string]any{"error": map[string]any{"code": key, "message": message}, "request_id": rid(r)})
}
func (s *Server) dbError(w http.ResponseWriter, r *http.Request, e error) {
	switch {
	case errors.Is(e, repository.ErrNotFound):
		fail(w, r, 404, "not_found", "Resource tidak ditemukan")
	case errors.Is(e, repository.ErrForbidden):
		fail(w, r, 403, "forbidden", "Akses rumah sakit ditolak")
	case errors.Is(e, repository.ErrConflict):
		fail(w, r, 409, "conflict", "Versi atau transisi bentrok; muat ulang data")
	default:
		slog.Error("request failed", "request_id", rid(r), "route", r.Pattern)
		fail(w, r, 500, "internal_error", "Terjadi kesalahan internal")
	}
}
func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	typ, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || typ != "application/json" {
		fail(w, r, 422, "validation_error", "Content-Type harus application/json")
		return false
	}
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e = d.Decode(target); e != nil {
		var max *http.MaxBytesError
		if errors.As(e, &max) {
			fail(w, r, 413, "body_too_large", "Body maksimal 64 KB")
		} else {
			fail(w, r, 400, "invalid_json", "JSON tidak valid atau field tidak didukung")
		}
		return false
	}
	if e = d.Decode(new(any)); e != io.EOF {
		fail(w, r, 400, "invalid_json", "Body harus berisi satu objek JSON")
		return false
	}
	return true
}
func validUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

type recorder struct {
	http.ResponseWriter
	status int
}

func (w *recorder) WriteHeader(n int) {
	if w.status == 0 {
		w.status = n
		w.ResponseWriter.WriteHeader(n)
	}
}
func (w *recorder) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		id := random()
		r = r.WithContext(context.WithValue(r.Context(), requestKey, id))
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		rw := &recorder{ResponseWriter: w}
		defer func() {
			if recover() != nil {
				if rw.status == 0 {
					fail(rw, r, 500, "internal_error", "Terjadi kesalahan internal")
				}
				slog.Error("panic recovered", "request_id", id)
			}
			slog.Info("http", "request_id", id, "method", r.Method, "route", r.Pattern, "status", rw.status, "duration_ms", time.Since(started).Milliseconds())
		}()
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		r.Body = http.MaxBytesReader(rw, r.Body, 64<<10)
		if r.Method == "HEAD" {
			fail(rw, r, 405, "method_not_allowed", "Metode tidak didukung")
			return
		}
		next.ServeHTTP(rw, r)
	})
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ok(w, map[string]string{"status": "ok"})
}
func (s *Server) readiness(w http.ResponseWriter, r *http.Request) {
	ctx, c := context.WithTimeout(r.Context(), 2*time.Second)
	defer c()
	if s.ready(ctx) != nil {
		fail(w, r, 503, "not_ready", "Database belum siap")
		return
	}
	ok(w, map[string]string{"status": "ready"})
}
func (s *Server) services(w http.ResponseWriter, r *http.Request) {
	ss, e := s.repo.Services(r.Context())
	if e != nil {
		s.dbError(w, r, e)
		return
	}
	type master struct {
		Slug     string `json:"slug"`
		Name     string `json:"name"`
		Category string `json:"category"`
	}
	out := []master{}
	for _, x := range ss {
		out = append(out, master{x.Slug, x.Name, x.Category})
	}
	ok(w, out)
}
func (s *Server) hospitals(w http.ResponseWriter, r *http.Request) {
	q, e := service.ParseSearch(r.URL.Query())
	if e != nil {
		fail(w, r, 422, "validation_error", e.Error())
		return
	}
	for _, x := range []struct{ code, kind string }{{q.City, "city"}, {q.Province, "province"}} {
		if x.code != "" {
			exists, e := s.repo.RegionExists(r.Context(), x.code, x.kind)
			if e != nil {
				s.dbError(w, r, e)
				return
			}
			if !exists {
				fail(w, r, 422, "validation_error", "Kode wilayah tidak dikenal")
				return
			}
		}
	}
	masters, e := s.repo.Services(r.Context())
	if e != nil {
		s.dbError(w, r, e)
		return
	}
	for _, slug := range q.Services {
		found := false
		for _, x := range masters {
			if x.Slug == slug {
				found = true
			}
		}
		if !found {
			fail(w, r, 422, "validation_error", "Slug layanan tidak dikenal")
			return
		}
	}
	now := time.Now().UTC()
	var out []domain.Hospital
	var total int
	if searcher, yes := s.repo.(Searcher); yes {
		out, total, e = searcher.SearchHospitals(r.Context(), q, now)
	} else {
		var hs []domain.Hospital
		if hs, e = s.repo.Hospitals(r.Context()); e == nil {
			out, total = service.Filter(hs, q, now)
		}
	}
	if e != nil {
		s.dbError(w, r, e)
		return
	}
	send(w, 200, map[string]any{"data": out, "meta": map[string]any{"page": q.Page, "limit": q.Limit, "total": total, "total_pages": (total + q.Limit - 1) / q.Limit, "as_of": now, "data_scope": "pilot", "dataset": q.Dataset}})
}

// hospitalByID returns a visible hospital with refreshed freshness; repository.ErrNotFound if absent.
func (s *Server) hospitalByID(ctx context.Context, id string) (domain.Hospital, error) {
	if reader, yes := s.repo.(HospitalReader); yes {
		h, e := reader.Hospital(ctx, id)
		h.Refresh(time.Now().UTC())
		return h, e
	}
	hs, e := s.repo.Hospitals(ctx)
	if e != nil {
		return domain.Hospital{}, e
	}
	for _, h := range hs {
		if h.ID == id {
			h.Refresh(time.Now().UTC())
			return h, nil
		}
	}
	return domain.Hospital{}, repository.ErrNotFound
}

// validExternalCode matches upstream facility codes such as SATUSEHAT kode_satusehat. A 36-character
// dashed token is UUID-shaped and must then be a valid lowercase UUID, so uppercase UUIDs stay rejected.
func validExternalCode(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '-' || c == '_') {
			return false
		}
	}
	return len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-'
}

// validHospitalRef accepts a hospital UUID or an upstream facility code.
func validHospitalRef(s string) bool { return validUUID(s) || validExternalCode(s) }

// hospitalByRef resolves a UUID or an upstream facility code to a visible hospital.
func (s *Server) hospitalByRef(ctx context.Context, ref string) (domain.Hospital, error) {
	if validUUID(ref) {
		return s.hospitalByID(ctx, ref)
	}
	if reader, yes := s.repo.(ExternalReader); yes {
		h, e := reader.HospitalByExternalID(ctx, ref)
		h.Refresh(time.Now().UTC())
		return h, e
	}
	hs, e := s.repo.Hospitals(ctx)
	if e != nil {
		return domain.Hospital{}, e
	}
	for _, h := range hs {
		if h.ExternalID != nil && *h.ExternalID == ref {
			h.Refresh(time.Now().UTC())
			return h, nil
		}
	}
	return domain.Hospital{}, repository.ErrNotFound
}
func (s *Server) findHospital(w http.ResponseWriter, r *http.Request, id string) (domain.Hospital, bool) {
	if !validUUID(id) {
		fail(w, r, 404, "not_found", "Rumah sakit tidak ditemukan")
		return domain.Hospital{}, false
	}
	h, e := s.hospitalByID(r.Context(), id)
	if errors.Is(e, repository.ErrNotFound) {
		fail(w, r, 404, "not_found", "Rumah sakit tidak ditemukan")
		return domain.Hospital{}, false
	}
	if e != nil {
		s.dbError(w, r, e)
		return domain.Hospital{}, false
	}
	return h, true
}
func (s *Server) hospital(w http.ResponseWriter, r *http.Request) {
	ref := r.PathValue("id")
	if !validHospitalRef(ref) {
		fail(w, r, 404, "not_found", "Rumah sakit tidak ditemukan")
		return
	}
	h, e := s.hospitalByRef(r.Context(), ref)
	switch {
	case errors.Is(e, repository.ErrNotFound):
		fail(w, r, 404, "not_found", "Rumah sakit tidak ditemukan")
	case e != nil:
		s.dbError(w, r, e)
	default:
		send(w, 200, map[string]any{"data": h, "meta": map[string]any{"as_of": time.Now().UTC()}})
	}
}

// compare returns the full detail DTO of two or three hospitals in the order of ids. It is all or
// nothing: if any ID is unknown the response is 404 naming the missing IDs, so a partial result is
// never mistaken for a complete comparison.
func (s *Server) compare(w http.ResponseWriter, r *http.Request) {
	ids, e := service.ParseCompare(r.URL.Query(), validHospitalRef)
	if e != nil {
		fail(w, r, 422, "validation_error", e.Error())
		return
	}
	out := make([]domain.Hospital, 0, len(ids))
	var missing []string
	resolved := map[string]bool{}
	for _, id := range ids {
		h, e := s.hospitalByRef(r.Context(), id)
		switch {
		case errors.Is(e, repository.ErrNotFound):
			missing = append(missing, id)
		case e != nil:
			s.dbError(w, r, e)
			return
		case resolved[h.ID]:
			// A UUID and a facility code naming the same hospital would compare it with itself.
			fail(w, r, 422, "validation_error", "ids tidak boleh memuat ID duplikat")
			return
		default:
			resolved[h.ID] = true
			out = append(out, h)
		}
	}
	if len(missing) > 0 {
		fail(w, r, 404, "not_found", "Rumah sakit tidak ditemukan: "+strings.Join(missing, ", "))
		return
	}
	send(w, 200, map[string]any{"data": out, "meta": map[string]any{"as_of": time.Now().UTC(), "count": len(out)}})
}
func (s *Server) regions(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Query()) > 0 {
		fail(w, r, 422, "validation_error", "Parameter tidak didukung")
		return
	}
	dir, yes := s.repo.(Directory)
	if !yes {
		fail(w, r, 501, "not_implemented", "Fitur belum tersedia")
		return
	}
	regions, e := dir.Regions(r.Context())
	if e != nil {
		s.dbError(w, r, e)
		return
	}
	type city struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	type province struct {
		Code   string `json:"code"`
		Name   string `json:"name"`
		Cities []city `json:"cities"`
	}
	out, index := []*province{}, map[string]*province{}
	for _, x := range regions {
		if x.Kind == "province" {
			p := &province{Code: x.Code, Name: x.Name, Cities: []city{}}
			index[x.Code] = p
			out = append(out, p)
		}
	}
	for _, x := range regions {
		if x.Kind != "city" || x.ParentCode == nil {
			continue
		}
		if p := index[*x.ParentCode]; p != nil {
			p.Cities = append(p.Cities, city{x.Code, x.Name})
		}
	}
	send(w, 200, map[string]any{"data": out, "meta": map[string]any{"count": len(out)}})
}
func (s *Server) suggest(w http.ResponseWriter, r *http.Request) {
	q, e := service.ParseSuggest(r.URL.Query())
	if e != nil {
		fail(w, r, 422, "validation_error", e.Error())
		return
	}
	dir, yes := s.repo.(Directory)
	if !yes {
		fail(w, r, 501, "not_implemented", "Fitur belum tersedia")
		return
	}
	out, e := dir.SuggestHospitals(r.Context(), q.Q, q.Dataset, q.Limit)
	if e != nil {
		s.dbError(w, r, e)
		return
	}
	send(w, 200, map[string]any{"data": out, "meta": map[string]any{"count": len(out), "dataset": q.Dataset}})
}

// adminHospitals lists the hospitals assigned to the signed-in operator. The scope comes from the
// server-side session, never from the request.
func (s *Server) adminHospitals(w http.ResponseWriter, r *http.Request) {
	op, yes := s.auth(w, r, false)
	if !yes {
		return
	}
	if len(r.URL.Query()) > 0 {
		fail(w, r, 422, "validation_error", "Parameter tidak didukung")
		return
	}
	out := []domain.Hospital{}
	for _, id := range op.HospitalIDs {
		h, e := s.hospitalByID(r.Context(), id)
		if errors.Is(e, repository.ErrNotFound) {
			continue
		}
		if e != nil {
			s.dbError(w, r, e)
			return
		}
		out = append(out, h)
	}
	send(w, 200, map[string]any{"data": out, "meta": map[string]any{"count": len(out), "as_of": time.Now().UTC()}})
}

// adminHospital returns one hospital with service/bed versions for optimistic-locking updates.
// A hospital outside the operator's scope is 403, checked before any lookup.
func (s *Server) adminHospital(w http.ResponseWriter, r *http.Request) {
	op, yes := s.auth(w, r, false)
	if !yes {
		return
	}
	id := r.PathValue("id")
	if !validUUID(id) {
		fail(w, r, 404, "not_found", "Resource tidak ditemukan")
		return
	}
	if !slices.Contains(op.HospitalIDs, id) {
		fail(w, r, 403, "forbidden", "Akses rumah sakit ditolak")
		return
	}
	if h, found := s.findHospital(w, r, id); found {
		send(w, 200, map[string]any{"data": h, "meta": map[string]any{"as_of": time.Now().UTC()}})
	}
}
func (s *Server) origin(w http.ResponseWriter, r *http.Request) bool {
	if !s.cfg.OriginAllowed(r.Header.Get("Origin")) {
		fail(w, r, 403, "invalid_origin", "Origin tidak diizinkan")
		return false
	}
	return true
}
func sessionToken(r *http.Request) string {
	c, e := r.Cookie("siagars_session")
	if e != nil || len(c.Value) != 64 {
		return ""
	}
	return c.Value
}
func csrfToken(token string) string { return repository.Hash("siagars-csrf-v1:" + token) }
func (s *Server) auth(w http.ResponseWriter, r *http.Request, mutation bool) (domain.Operator, bool) {
	token := sessionToken(r)
	if token == "" {
		fail(w, r, 401, "unauthorized", "Session tidak valid")
		return domain.Operator{}, false
	}
	op, hash, e := s.repo.Session(r.Context(), token)
	if e != nil {
		if errors.Is(e, repository.ErrNotFound) {
			fail(w, r, 401, "unauthorized", "Session tidak valid")
		} else {
			s.dbError(w, r, e)
		}
		return op, false
	}
	if mutation {
		if !s.origin(w, r) {
			return op, false
		}
		value := r.Header.Get("X-CSRF-Token")
		if len(value) != 64 || subtle.ConstantTimeCompare([]byte(repository.Hash(value)), []byte(hash)) != 1 {
			fail(w, r, 403, "invalid_csrf", "CSRF token tidak valid")
			return op, false
		}
	}
	return op, true
}
func (s *Server) cookie(w http.ResponseWriter, token string, expire time.Time, max int) {
	http.SetCookie(w, &http.Cookie{Name: "siagars_session", Value: token, Path: "/api/v1", HttpOnly: true, Secure: s.cfg.SecureCookie, SameSite: http.SameSiteLaxMode, Expires: expire, MaxAge: max})
}
func ip(r *http.Request) string {
	host, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil {
		return r.RemoteAddr
	}
	return host
}

// clientIP is the rate-limit identity. X-Forwarded-For is honored only when the direct peer is a
// configured trusted proxy: the list is read right to left and the first address that is not itself
// a trusted proxy is the client. Anything unparseable falls back to the peer address.
func (s *Server) clientIP(r *http.Request) string {
	peer := ip(r)
	addr, e := netip.ParseAddr(peer)
	if e != nil || !s.trusted(addr) {
		return peer
	}
	var hops []string
	for _, v := range r.Header.Values("X-Forwarded-For") {
		hops = append(hops, strings.Split(v, ",")...)
	}
	for i := len(hops) - 1; i >= 0; i-- {
		hop, e := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if e != nil {
			return peer
		}
		if !s.trusted(hop) {
			return hop.Unmap().String()
		}
	}
	return peer
}
func (s *Server) trusted(a netip.Addr) bool {
	a = a.Unmap()
	for _, p := range s.cfg.TrustedProxies {
		if p.Contains(a) {
			return true
		}
	}
	return false
}
func (s *Server) rate(w http.ResponseWriter, r *http.Request, key string, max int, window time.Duration) bool {
	if !s.lim.allow(key, max, window, time.Now()) {
		w.Header().Set("Retry-After", "60")
		fail(w, r, 429, "rate_limited", "Terlalu banyak percobaan; coba lagi nanti")
		return false
	}
	return true
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !s.origin(w, r) {
		return
	}
	if !s.rate(w, r, "login-ip:"+s.clientIP(r), 100, 15*time.Minute) {
		return
	}
	var b struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &b) {
		return
	}
	b.Email = strings.ToLower(strings.TrimSpace(b.Email))
	if len(b.Email) > 254 || !strings.Contains(b.Email, "@") || len(b.Password) < 1 || len(b.Password) > 72 {
		fail(w, r, 422, "validation_error", "Email/password tidak valid")
		return
	}
	key := "login:" + repository.Hash(s.clientIP(r)+"|"+b.Email)
	if !s.rate(w, r, key, 5, 15*time.Minute) {
		return
	}
	id, hash, e := s.repo.Credentials(r.Context(), b.Email)
	if e != nil && !errors.Is(e, repository.ErrNotFound) {
		s.dbError(w, r, e)
		return
	}
	compare := []byte(hash)
	if e != nil {
		compare = s.dummy
	}
	passwordError := bcrypt.CompareHashAndPassword(compare, []byte(b.Password))
	if e != nil || passwordError != nil {
		fail(w, r, 401, "invalid_credentials", "Email atau password tidak valid")
		return
	}
	token := random()
	csrf := csrfToken(token)
	expiry := time.Now().UTC().Add(s.cfg.SessionTTL)
	if e = s.repo.CreateSession(r.Context(), id, token, csrf, expiry, sessionToken(r)); e != nil {
		s.dbError(w, r, e)
		return
	}
	s.lim.clear(key)
	s.cookie(w, token, expiry, int(s.cfg.SessionTTL.Seconds()))
	ok(w, map[string]any{"csrf_token": csrf, "expires_at": expiry})
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	if op, yes := s.auth(w, r, false); yes {
		ok(w, op)
	}
}
func (s *Server) csrf(w http.ResponseWriter, r *http.Request) {
	if _, yes := s.auth(w, r, false); yes {
		ok(w, map[string]string{"csrf_token": csrfToken(sessionToken(r))})
	}
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if _, yes := s.auth(w, r, true); !yes {
		return
	}
	if e := s.repo.Revoke(r.Context(), sessionToken(r)); e != nil {
		s.dbError(w, r, e)
		return
	}
	s.cookie(w, "", time.Unix(1, 0), -1)
	ok(w, map[string]bool{"logged_out": true})
}
func (s *Server) lookup(w http.ResponseWriter, r *http.Request) {
	if !s.rate(w, r, "lookup:"+s.clientIP(r), 10, time.Minute) {
		return
	}
	var b struct {
		TrackingCode string `json:"tracking_code"`
	}
	if !decode(w, r, &b) {
		return
	}
	if len(b.TrackingCode) == 0 || len(b.TrackingCode) > 256 {
		fail(w, r, 404, "not_found", "Resource tidak ditemukan")
		return
	}
	ref, h, e := s.repo.Lookup(r.Context(), b.TrackingCode)
	if e != nil {
		s.dbError(w, r, e)
		return
	}
	var destination any
	if ref.DestinationHospitalID != nil {
		hospital, e := s.hospitalByID(r.Context(), *ref.DestinationHospitalID)
		if e != nil && !errors.Is(e, repository.ErrNotFound) {
			s.dbError(w, r, e)
			return
		}
		if e == nil {
			destination = hospital
		}
	}
	ok(w, map[string]any{"public_status": ref.Status, "public_note": ref.PublicNote, "next_step": ref.NextStep, "destination": destination, "updated_at": ref.UpdatedAt, "history": h})
}
func (s *Server) referrals(w http.ResponseWriter, r *http.Request) {
	op, yes := s.auth(w, r, false)
	if !yes {
		return
	}
	refs, e := s.repo.Referrals(r.Context(), op.ID)
	if e != nil {
		s.dbError(w, r, e)
		return
	}
	ok(w, refs)
}
func (s *Server) updateReferral(w http.ResponseWriter, r *http.Request) {
	op, yes := s.auth(w, r, true)
	if !yes {
		return
	}
	id := r.PathValue("id")
	if !validUUID(id) {
		fail(w, r, 404, "not_found", "Resource tidak ditemukan")
		return
	}
	var b struct {
		Status     string `json:"status"`
		PublicNote string `json:"public_note"`
		Version    int    `json:"version"`
	}
	if !decode(w, r, &b) {
		return
	}
	if !domain.ValidStatus(b.Status) || b.Version < 1 || utf8.RuneCountInString(b.PublicNote) > 500 {
		fail(w, r, 422, "validation_error", "Status/version/catatan tidak valid")
		return
	}
	ref, e := s.repo.UpdateReferral(r.Context(), op.ID, id, b.Status, strings.TrimSpace(b.PublicNote), b.Version, rid(r))
	if e != nil {
		s.dbError(w, r, e)
		return
	}
	ok(w, ref)
}
func (s *Server) updateService(w http.ResponseWriter, r *http.Request) {
	op, yes := s.auth(w, r, true)
	if !yes {
		return
	}
	id := r.PathValue("id")
	if !validUUID(id) {
		fail(w, r, 404, "not_found", "Resource tidak ditemukan")
		return
	}
	var b struct {
		Status  string `json:"status"`
		Version int    `json:"version"`
	}
	if !decode(w, r, &b) {
		return
	}
	if !domain.ValidServiceStatus(b.Status) || b.Version < 1 {
		fail(w, r, 422, "validation_error", "Status/version tidak valid")
		return
	}
	if e := s.repo.UpdateFacility(r.Context(), op.ID, id, r.PathValue("slug"), b.Version, &b.Status, nil, nil, rid(r)); e != nil {
		s.dbError(w, r, e)
		return
	}
	if h, found := s.findHospital(w, r, id); found {
		for _, x := range h.Services {
			if x.Slug == r.PathValue("slug") {
				ok(w, x)
				return
			}
		}
	}
}
func (s *Server) updateBed(w http.ResponseWriter, r *http.Request) {
	op, yes := s.auth(w, r, true)
	if !yes {
		return
	}
	id := r.PathValue("id")
	if !validUUID(id) {
		fail(w, r, 404, "not_found", "Resource tidak ditemukan")
		return
	}
	var b struct {
		Total     json.RawMessage `json:"total"`
		Available json.RawMessage `json:"available"`
		Version   int             `json:"version"`
	}
	if !decode(w, r, &b) {
		return
	}
	var total, available *int
	if len(b.Total) == 0 || len(b.Available) == 0 || json.Unmarshal(b.Total, &total) != nil || json.Unmarshal(b.Available, &available) != nil || b.Version < 1 || !domain.ValidBeds(total, available) {
		fail(w, r, 422, "validation_error", "total/available wajib, boleh null, dan 0 <= available <= total jika diketahui")
		return
	}
	category := r.PathValue("category")
	if category != "icu" && category != "nicu" && category != "picu" && category != "general" {
		fail(w, r, 422, "validation_error", "Kategori bed tidak valid")
		return
	}
	if e := s.repo.UpdateFacility(r.Context(), op.ID, id, category, b.Version, nil, total, available, rid(r)); e != nil {
		s.dbError(w, r, e)
		return
	}
	if h, found := s.findHospital(w, r, id); found {
		for _, x := range h.Beds {
			if x.Category == category {
				ok(w, x)
				return
			}
		}
	}
}

type bucket struct {
	count int
	until time.Time
}
type limiter struct {
	mu      sync.Mutex
	entries map[string]bucket
}

func (l *limiter) allow(key string, max int, window time.Duration, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.entries == nil {
		l.entries = map[string]bucket{}
	}
	if len(l.entries) >= 10000 {
		for k, v := range l.entries {
			if !now.Before(v.until) {
				delete(l.entries, k)
			}
		}
		if _, ok := l.entries[key]; !ok && len(l.entries) >= 10000 {
			return false
		}
	}
	b := l.entries[key]
	if !now.Before(b.until) {
		b = bucket{until: now.Add(window)}
	}
	if b.count >= max {
		return false
	}
	b.count++
	l.entries[key] = b
	return true
}
func (l *limiter) clear(key string) { l.mu.Lock(); defer l.mu.Unlock(); delete(l.entries, key) }

// The Scalar bundle loads from a pinned, integrity-checked CDN URL; everything else is same-origin.
// 'unsafe-eval' is required by the bundled Vue runtime; the page has no other script.
const docsCSP = "default-src 'none'; script-src https://cdn.jsdelivr.net 'unsafe-eval'; style-src https://cdn.jsdelivr.net 'unsafe-inline'; img-src 'self' data: https://cdn.jsdelivr.net; font-src 'self' data: https://cdn.jsdelivr.net; connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

// registerDocs serves the Scalar API reference at /docs and the embedded OpenAPI contract at /docs/openapi.json.
// Both live outside /api/v1, so the API contract itself is unchanged.
func registerDocs(mux *http.ServeMux) {
	spec := docs.OpenAPI
	// Point "Try it out" at whichever host serves the docs instead of the hard-coded development URL.
	var doc map[string]any
	if json.Unmarshal(docs.OpenAPI, &doc) == nil {
		doc["servers"] = []map[string]string{{"url": "/api/v1", "description": "Server ini"}}
		if b, e := json.Marshal(doc); e == nil {
			spec = b
		}
	}
	serve := func(contentType string, body []byte) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Content-Security-Policy", docsCSP)
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.WriteHeader(200)
			w.Write(body)
		}
	}
	mux.HandleFunc("GET /docs", serve("text/html; charset=utf-8", docs.ReferencePage))
	mux.HandleFunc("GET /docs/{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs", http.StatusMovedPermanently)
	})
	mux.HandleFunc("GET /docs/openapi.json", serve("application/json; charset=utf-8", spec))
}

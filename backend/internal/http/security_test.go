package httpapi

import (
	"context"
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/http/httptest"
	"siagars/backend/internal/config"
	"siagars/backend/internal/domain"
	"siagars/backend/internal/repository"
	"strings"
	"sync"
	"testing"
	"time"
)

// secStore is an in-memory operator/session store with several concurrent sessions, so cross-session
// behavior (CSRF binding, independent logout, rotation) can be tested without MongoDB.
type secStore struct {
	Store
	mu       sync.Mutex
	email    string
	hash     string
	active   bool
	scope    []string
	sessions map[string]*secSession
	lookups  int
}
type secSession struct {
	csrf    string
	expires time.Time
	revoked bool
}

func (s *secStore) Credentials(_ context.Context, email string) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lookups++
	if email != s.email || !s.active {
		return "", "", repository.ErrNotFound
	}
	return "operator", s.hash, nil
}
func (s *secStore) CreateSession(_ context.Context, _ string, token, csrf string, expires time.Time, old string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if x := s.sessions[old]; old != "" && x != nil {
		x.revoked = true
	}
	s.sessions[token] = &secSession{csrf: repository.Hash(csrf), expires: expires}
	return nil
}
func (s *secStore) Session(_ context.Context, token string) (domain.Operator, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x := s.sessions[token]
	if x == nil || x.revoked || !time.Now().Before(x.expires) || !s.active {
		return domain.Operator{}, "", repository.ErrNotFound
	}
	return domain.Operator{ID: "operator", Email: s.email, HospitalIDs: s.scope}, x.csrf, nil
}
func (s *secStore) Revoke(_ context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if x := s.sessions[token]; x != nil {
		x.revoked = true
	}
	return nil
}
func (s *secStore) Services(context.Context) ([]domain.Service, error) { return nil, nil }
func (s *secStore) Referrals(context.Context, string) ([]domain.Referral, error) {
	return []domain.Referral{}, nil
}

const (
	secOrigin   = "http://localhost:3000"
	secEmail    = "admin@example.invalid"
	secPassword = "correct-horse-battery"
)

type secRig struct {
	t     *testing.T
	store *secStore
	h     http.Handler
}

func newSecRig(t *testing.T) *secRig {
	hash, e := bcrypt.GenerateFromPassword([]byte(secPassword), bcrypt.MinCost)
	if e != nil {
		t.Fatal(e)
	}
	store := &secStore{email: secEmail, hash: string(hash), active: true, scope: []string{"hospital"}, sessions: map[string]*secSession{}}
	return &secRig{t, store, New(store, config.Config{PublicOrigin: secOrigin, SessionTTL: time.Hour}, func(context.Context) error { return nil })}
}

type secReq struct {
	method, path, body, origin, csrf, cookie, contentType string
	noOrigin                                              bool
}

func (r *secRig) do(c secReq) *httptest.ResponseRecorder {
	req := httptest.NewRequest(c.method, "/api/v1"+c.path, strings.NewReader(c.body))
	if c.contentType == "" {
		c.contentType = "application/json"
	}
	req.Header.Set("Content-Type", c.contentType)
	if !c.noOrigin {
		if c.origin == "" {
			c.origin = secOrigin
		}
		req.Header.Set("Origin", c.origin)
	}
	if c.csrf != "" {
		req.Header.Set("X-CSRF-Token", c.csrf)
	}
	if c.cookie != "" {
		req.AddCookie(&http.Cookie{Name: "siagars_session", Value: c.cookie})
	}
	w := httptest.NewRecorder()
	r.h.ServeHTTP(w, req)
	return w
}
func loginBody(email, password string) string {
	b, _ := json.Marshal(map[string]string{"email": email, "password": password})
	return string(b)
}

// login returns the session cookie and CSRF token of a successful login.
func (r *secRig) login() (cookie, csrf string) {
	r.t.Helper()
	w := r.do(secReq{method: "POST", path: "/auth/login", body: loginBody(secEmail, secPassword)})
	if w.Code != 200 || len(w.Result().Cookies()) != 1 {
		r.t.Fatal("login:", w.Code, w.Body.String())
	}
	var p struct {
		Data struct {
			CSRF string `json:"csrf_token"`
		}
	}
	_ = json.Unmarshal(w.Body.Bytes(), &p)
	return w.Result().Cookies()[0].Value, p.Data.CSRF
}
func errCode(w *httptest.ResponseRecorder) string {
	var p struct{ Error struct{ Code string } }
	_ = json.Unmarshal(w.Body.Bytes(), &p)
	return p.Error.Code
}

func TestOriginVariantsAreRejected(t *testing.T) {
	r := newSecRig(t)
	cookie, csrf := r.login()
	for _, origin := range []string{"null", "HTTP://LOCALHOST:3000", "http://localhost:3000/", "http://localhost:3001", "https://localhost:3000", "http://localhost:3000.evil.test", " http://localhost:3000"} {
		if w := r.do(secReq{method: "POST", path: "/auth/login", body: loginBody(secEmail, secPassword), origin: origin}); w.Code != 403 || errCode(w) != "invalid_origin" {
			t.Errorf("login Origin %q: %d %s", origin, w.Code, w.Body.String())
		}
		if w := r.do(secReq{method: "POST", path: "/auth/logout", origin: origin, cookie: cookie, csrf: csrf}); w.Code != 403 || errCode(w) != "invalid_origin" {
			t.Errorf("logout Origin %q: %d %s", origin, w.Code, w.Body.String())
		}
	}
	if w := r.do(secReq{method: "POST", path: "/auth/login", body: loginBody(secEmail, secPassword), noOrigin: true}); w.Code != 403 {
		t.Errorf("tanpa Origin: %d", w.Code)
	}
	// None of the rejected logouts may have revoked the session.
	if w := r.do(secReq{method: "GET", path: "/auth/me", cookie: cookie}); w.Code != 200 {
		t.Fatal("sesi tercabut oleh permintaan yang ditolak:", w.Code)
	}
	// A valid Origin with a bad CSRF token is rejected on its own, and vice versa.
	if w := r.do(secReq{method: "POST", path: "/auth/logout", cookie: cookie, csrf: strings.Repeat("0", 64)}); w.Code != 403 || errCode(w) != "invalid_csrf" {
		t.Errorf("CSRF salah: %d", w.Code)
	}
	if w := r.do(secReq{method: "POST", path: "/auth/logout", cookie: cookie}); w.Code != 403 || errCode(w) != "invalid_csrf" {
		t.Errorf("CSRF hilang: %d", w.Code)
	}
}

func TestCSRFTokenIsBoundToItsSession(t *testing.T) {
	r := newSecRig(t)
	cookieA, csrfA := r.login()
	cookieB, csrfB := r.login()
	if cookieA == cookieB || csrfA == csrfB {
		t.Fatal("token sesi dan CSRF harus unik per login")
	}
	if w := r.do(secReq{method: "POST", path: "/auth/logout", cookie: cookieB, csrf: csrfA}); w.Code != 403 || errCode(w) != "invalid_csrf" {
		t.Fatalf("CSRF sesi A dipakai di sesi B: %d", w.Code)
	}
	// CSRF alone, without the session cookie, never authenticates.
	if w := r.do(secReq{method: "POST", path: "/auth/logout", csrf: csrfA}); w.Code != 401 {
		t.Fatalf("tanpa cookie: %d", w.Code)
	}
	// Logging out one session leaves the other one usable.
	if w := r.do(secReq{method: "POST", path: "/auth/logout", cookie: cookieA, csrf: csrfA}); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := r.do(secReq{method: "GET", path: "/auth/me", cookie: cookieA}); w.Code != 401 {
		t.Fatal("sesi A harus tercabut:", w.Code)
	}
	if w := r.do(secReq{method: "GET", path: "/auth/me", cookie: cookieB}); w.Code != 200 {
		t.Fatal("sesi B harus tetap hidup:", w.Code)
	}
}

func TestLoginWithOldCookieRotatesIt(t *testing.T) {
	r := newSecRig(t)
	old, _ := r.login()
	w := r.do(secReq{method: "POST", path: "/auth/login", body: loginBody(secEmail, secPassword), cookie: old})
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	fresh := w.Result().Cookies()[0].Value
	if fresh == old {
		t.Fatal("cookie harus dirotasi")
	}
	if w := r.do(secReq{method: "GET", path: "/auth/me", cookie: old}); w.Code != 401 {
		t.Fatal("cookie lama harus tercabut:", w.Code)
	}
	if w := r.do(secReq{method: "GET", path: "/auth/me", cookie: fresh}); w.Code != 200 {
		t.Fatal(w.Code)
	}
}

func TestInactiveOperatorCannotLoginOrKeepASession(t *testing.T) {
	r := newSecRig(t)
	cookie, _ := r.login()
	r.store.active = false
	if w := r.do(secReq{method: "GET", path: "/auth/me", cookie: cookie}); w.Code != 401 {
		t.Fatal("sesi operator nonaktif harus ditolak:", w.Code)
	}
	w := r.do(secReq{method: "POST", path: "/auth/login", body: loginBody(secEmail, secPassword)})
	if w.Code != 401 || errCode(w) != "invalid_credentials" || len(w.Result().Cookies()) != 0 {
		t.Fatal("login operator nonaktif:", w.Code, w.Body.String())
	}
}

func TestUnknownAccountAndWrongPasswordAreIndistinguishable(t *testing.T) {
	r := newSecRig(t)
	strip := func(w *httptest.ResponseRecorder) string {
		var p map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &p)
		delete(p, "request_id")
		b, _ := json.Marshal(p)
		return w.Result().Status + string(b)
	}
	wrong := r.do(secReq{method: "POST", path: "/auth/login", body: loginBody(secEmail, "wrong-password")})
	unknown := r.do(secReq{method: "POST", path: "/auth/login", body: loginBody("nobody@example.invalid", "wrong-password")})
	if wrong.Code != 401 || strip(wrong) != strip(unknown) {
		t.Fatalf("respons berbeda:\n%s\n%s", strip(wrong), strip(unknown))
	}
	if len(wrong.Result().Cookies())+len(unknown.Result().Cookies()) != 0 {
		t.Fatal("login gagal tidak boleh menetapkan cookie")
	}
	// Unknown accounts still run a bcrypt comparison against a dummy hash (server.go login); that
	// timing equalization is by construction and is not asserted here.
}

func TestLoginRateLimitPerAccountAndResetOnSuccess(t *testing.T) {
	r := newSecRig(t)
	for i := 0; i < 4; i++ {
		if w := r.do(secReq{method: "POST", path: "/auth/login", body: loginBody(secEmail, "wrong")}); w.Code != 401 {
			t.Fatal(i, w.Code)
		}
	}
	// A success before the limit clears the per-account counter.
	if w := r.do(secReq{method: "POST", path: "/auth/login", body: loginBody(secEmail, secPassword)}); w.Code != 200 {
		t.Fatal(w.Code)
	}
	for i := 0; i < 5; i++ {
		if w := r.do(secReq{method: "POST", path: "/auth/login", body: loginBody(secEmail, "wrong")}); w.Code != 401 {
			t.Fatal("setelah reset:", i, w.Code)
		}
	}
	// The sixth attempt is blocked even with the correct password, and does not reach the store.
	before := r.store.lookups
	w := r.do(secReq{method: "POST", path: "/auth/login", body: loginBody(secEmail, secPassword)})
	if w.Code != 429 || w.Header().Get("Retry-After") == "" || errCode(w) != "rate_limited" || r.store.lookups != before {
		t.Fatal(w.Code, w.Header(), w.Body.String())
	}
	// Another account from the same address has its own counter.
	if w := r.do(secReq{method: "POST", path: "/auth/login", body: loginBody("other@example.invalid", "wrong")}); w.Code != 401 {
		t.Fatal("akun lain tidak boleh ikut terkunci:", w.Code)
	}
}

func TestLoginInputValidation(t *testing.T) {
	r := newSecRig(t)
	for name, c := range map[string]struct {
		secReq
		want int
	}{
		"bukan JSON":          {secReq{method: "POST", path: "/auth/login", body: loginBody(secEmail, secPassword), contentType: "text/plain"}, 422},
		"form":                {secReq{method: "POST", path: "/auth/login", body: "email=a&password=b", contentType: "application/x-www-form-urlencoded"}, 422},
		"JSON dengan charset": {secReq{method: "POST", path: "/auth/login", body: loginBody(secEmail, "wrong"), contentType: "application/json; charset=utf-8"}, 401},
		"password 73 byte":    {secReq{method: "POST", path: "/auth/login", body: loginBody(secEmail, strings.Repeat("a", 73))}, 422},
		"password kosong":     {secReq{method: "POST", path: "/auth/login", body: loginBody(secEmail, "")}, 422},
		"email tanpa @":       {secReq{method: "POST", path: "/auth/login", body: loginBody("admin", secPassword)}, 422},
		"email 255 byte":      {secReq{method: "POST", path: "/auth/login", body: loginBody(strings.Repeat("a", 250)+"@b.co", secPassword)}, 422},
		"field tak dikenal":   {secReq{method: "POST", path: "/auth/login", body: `{"email":"a@b.c","password":"x","admin":true}`}, 400},
		"dua objek":           {secReq{method: "POST", path: "/auth/login", body: loginBody(secEmail, "x") + loginBody(secEmail, "x")}, 400},
		"JSON rusak":          {secReq{method: "POST", path: "/auth/login", body: `{"email":`}, 400},
		"body 70 KB":          {secReq{method: "POST", path: "/auth/login", body: `{"email":"a@b.c","password":"` + strings.Repeat("x", 70<<10) + `"}`}, 413},
		"password bukan str":  {secReq{method: "POST", path: "/auth/login", body: `{"email":"a@b.c","password":123}`}, 400},
		"injeksi operator":    {secReq{method: "POST", path: "/auth/login", body: `{"email":{"$ne":""},"password":"x"}`}, 400},
	} {
		if w := r.do(c.secReq); w.Code != c.want {
			t.Errorf("%s: %d (mau %d) %s", name, w.Code, c.want, w.Body.String())
		}
	}
	// Uppercase and padded e-mail are normalized, so the same account is reached.
	if w := r.do(secReq{method: "POST", path: "/auth/login", body: loginBody("  ADMIN@Example.INVALID ", secPassword)}); w.Code != 200 {
		t.Errorf("normalisasi email: %d", w.Code)
	}
}

func TestSecurityHeadersAndNoCORS(t *testing.T) {
	r := newSecRig(t)
	cookie, csrf := r.login()
	for name, w := range map[string]*httptest.ResponseRecorder{
		"login 200":  r.do(secReq{method: "POST", path: "/auth/login", body: loginBody(secEmail, secPassword)}),
		"login 401":  r.do(secReq{method: "POST", path: "/auth/login", body: loginBody(secEmail, "wrong")}),
		"login 403":  r.do(secReq{method: "POST", path: "/auth/login", body: "{}", origin: "https://evil.test"}),
		"me 401":     r.do(secReq{method: "GET", path: "/auth/me"}),
		"me 200":     r.do(secReq{method: "GET", path: "/auth/me", cookie: cookie}),
		"csrf 200":   r.do(secReq{method: "GET", path: "/auth/csrf", cookie: cookie}),
		"logout 403": r.do(secReq{method: "POST", path: "/auth/logout", cookie: cookie, csrf: "x"}),
		"404":        r.do(secReq{method: "GET", path: "/nope"}),
	} {
		h := w.Header()
		if h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Cache-Control") != "no-store" || h.Get("X-Frame-Options") != "DENY" || h.Get("Referrer-Policy") != "no-referrer" || len(h.Get("X-Request-ID")) != 64 {
			t.Errorf("%s: header keamanan kurang: %v", name, h)
		}
		for k := range h {
			if strings.HasPrefix(k, "Access-Control-") {
				t.Errorf("%s: tidak boleh ada header CORS: %s", name, k)
			}
		}
	}
	_ = csrf
	// A preflight must not be answered as an allowed cross-origin request.
	req := httptest.NewRequest("OPTIONS", "/api/v1/auth/login", nil)
	req.Header.Set("Origin", "https://evil.test")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()
	r.h.ServeHTTP(w, req)
	if w.Code < 400 || w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("preflight: %d %v", w.Code, w.Header())
	}
	// The session cookie must not leak into a login failure, and is scoped and HttpOnly on success.
	ok := r.do(secReq{method: "POST", path: "/auth/login", body: loginBody(secEmail, secPassword)})
	c := ok.Result().Cookies()[0]
	if c.Name != "siagars_session" || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/api/v1" || c.Domain != "" || c.MaxAge <= 0 || len(c.Value) != 64 || c.Secure {
		t.Errorf("cookie dev (http) salah: %+v", c)
	}
	if strings.Contains(ok.Body.String(), c.Value) {
		t.Error("token sesi tidak boleh ada di body")
	}
}

func TestSessionCookieIsRejectedWhenMalformed(t *testing.T) {
	r := newSecRig(t)
	for _, cookie := range []string{"short", strings.Repeat("g", 64), strings.Repeat("a", 65), strings.Repeat("A", 64)} {
		if w := r.do(secReq{method: "GET", path: "/auth/me", cookie: cookie}); w.Code != 401 {
			t.Errorf("%q: %d", cookie, w.Code)
		}
	}
}

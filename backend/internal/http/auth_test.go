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
	"testing"
	"time"
)

type authStore struct {
	Store
	hash, token, csrf string
	expires           time.Time
	revoked           bool
}

func (s *authStore) Credentials(context.Context, string) (string, string, error) {
	return "operator", s.hash, nil
}
func (s *authStore) CreateSession(_ context.Context, _ string, token, csrf string, expires time.Time, _ string) error {
	s.token = token
	s.csrf = repository.Hash(csrf)
	s.expires = expires
	s.revoked = false
	return nil
}
func (s *authStore) Session(_ context.Context, token string) (domain.Operator, string, error) {
	if token != s.token || s.revoked || !time.Now().Before(s.expires) {
		return domain.Operator{}, "", repository.ErrNotFound
	}
	return domain.Operator{ID: "operator", Email: "operator@example.invalid", HospitalIDs: []string{"hospital"}}, s.csrf, nil
}
func (s *authStore) Revoke(context.Context, string) error { s.revoked = true; return nil }
func TestLoginCSRFLogoutAndExpiry(t *testing.T) {
	hash, e := bcrypt.GenerateFromPassword([]byte("unit-test-password"), bcrypt.MinCost)
	if e != nil {
		t.Fatal(e)
	}
	store := &authStore{hash: string(hash)}
	cfg := config.Config{PublicOrigin: "https://siagars.example", SecureCookie: true, SessionTTL: 8 * time.Hour}
	h := New(store, cfg, func(context.Context) error { return nil })
	request := func(method, path, body, token, csrf string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/v1"+path, strings.NewReader(body))
		r.Header.Set("Origin", cfg.PublicOrigin)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", csrf)
		if token != "" {
			r.AddCookie(&http.Cookie{Name: "siagars_session", Value: token})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	bad := request("POST", "/auth/login", "{\"email\":\"operator@example.invalid\",\"password\":\"wrong\"}", "", "")
	if bad.Code != 401 {
		t.Fatal(bad.Code)
	}
	login := request("POST", "/auth/login", "{\"email\":\"operator@example.invalid\",\"password\":\"unit-test-password\"}", "", "")
	if login.Code != 200 {
		t.Fatal(login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal(cookies)
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/api/v1" || len(cookie.Value) != 64 {
		t.Fatal(cookie)
	}
	var payload struct {
		Data struct {
			CSRF string `json:"csrf_token"`
		}
	}
	if e = json.Unmarshal(login.Body.Bytes(), &payload); e != nil {
		t.Fatal(e)
	}
	token := cookie.Value
	if w := request("GET", "/auth/me", "", token, ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := request("GET", "/auth/csrf", "", token, ""); !strings.Contains(w.Body.String(), payload.Data.CSRF) {
		t.Fatal(w.Body.String())
	}
	if w := request("POST", "/auth/logout", "", token, "invalid"); w.Code != 403 || store.revoked {
		t.Fatal(w.Code)
	}
	if w := request("POST", "/auth/logout", "", token, payload.Data.CSRF); w.Code != 200 || !store.revoked || w.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal(w.Code)
	}
	if w := request("GET", "/auth/me", "", token, ""); w.Code != 401 {
		t.Fatal("revoked accepted")
	}
	store.revoked = false
	store.expires = time.Now().Add(-time.Second)
	if w := request("GET", "/auth/me", "", token, ""); w.Code != 401 {
		t.Fatal("expired accepted")
	}
}

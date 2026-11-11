package httpapi

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"
)

type unavailableRateStore struct{}

func (unavailableRateStore) TakeRateLimit(context.Context, string, int, time.Duration, time.Time) (bool, error) {
	return false, errors.New("unavailable")
}

func (unavailableRateStore) ClearRateLimit(context.Context, string, time.Duration, time.Time) error {
	return errors.New("unavailable")
}

func TestRateLimitStoreUnavailable(t *testing.T) {
	server := &Server{sharedLimiter: unavailableRateStore{}}
	r := httptest.NewRequest("POST", "/", nil)
	for _, key := range []string{"login-ip:test", "login:test"} {
		response := httptest.NewRecorder()
		if server.rate(response, r, key, 1, time.Minute) || response.Code != 503 {
			t.Fatalf("login limiter error: status %d", response.Code)
		}
	}
	for _, key := range []string{"lookup:test", "bpjs:test"} {
		if !server.rate(httptest.NewRecorder(), r, key, 1, time.Minute) {
			t.Fatal("public limiter did not fall back")
		}
		response := httptest.NewRecorder()
		if server.rate(response, r, key, 1, time.Minute) || response.Code != 429 {
			t.Fatalf("fallback quota: status %d", response.Code)
		}
	}
}

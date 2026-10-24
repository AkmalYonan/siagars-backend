package satusehat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	maxTokenBody  = 64 << 10
	maxTokenTTL   = 24 * time.Hour
	tokenMargin   = 60 * time.Second
	maxTokenBytes = 8 << 10
)

type Credentials struct{ ClientID, ClientSecret string }

// TokenSource is what the MSI client needs from the OAuth layer.
type TokenSource interface {
	AccessToken(context.Context) (string, error)
	Invalidate(token string)
}

// TokenProvider caches one client_credentials token in memory and shares a single
// in-flight request between concurrent callers. Tokens are never persisted or logged.
type TokenProvider struct {
	endpoint string
	creds    Credentials
	http     *http.Client
	now      func() time.Time
	group    singleflight.Group
	mu       sync.Mutex
	token    string
	expiry   time.Time
}

func NewTokenProvider(baseURL string, creds Credentials, client *http.Client) *TokenProvider {
	return &TokenProvider{endpoint: strings.TrimRight(baseURL, "/") + "/oauth2/v1/accesstoken?grant_type=client_credentials", creds: creds, http: client, now: time.Now}
}
func (t *TokenProvider) cached() (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.token, t.token != "" && t.now().Before(t.expiry)
}

// AccessToken returns a cached token or fetches a new one. The upstream request is detached
// from the first caller's cancellation so one canceled caller cannot fail the others; it stays
// bounded by the HTTP client timeout.
func (t *TokenProvider) AccessToken(ctx context.Context) (string, error) {
	if token, ok := t.cached(); ok {
		return token, nil
	}
	result := t.group.DoChan("token", func() (any, error) {
		if token, ok := t.cached(); ok {
			return token, nil
		}
		return t.fetch(context.WithoutCancel(ctx))
	})
	select {
	case <-ctx.Done():
		return "", &Error{Op: "oauth", Category: CategoryCanceled, err: ctx.Err()}
	case r := <-result:
		if r.Err != nil {
			return "", r.Err
		}
		return r.Val.(string), nil
	}
}

// Invalidate drops the cached token only if it is still the one that was rejected,
// so a fresh token fetched by another caller is not discarded.
func (t *TokenProvider) Invalidate(token string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.token == token {
		t.token, t.expiry = "", time.Time{}
	}
}

// fetch performs exactly one OAuth request. It is not retried: repeated failed
// authentication attempts can be penalized upstream.
func (t *TokenProvider) fetch(ctx context.Context) (string, error) {
	form := url.Values{"client_id": {t.creds.ClientID}, "client_secret": {t.creds.ClientSecret}}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint, strings.NewReader(form.Encode()))
	if e != nil {
		return "", &Error{Op: "oauth", Category: CategoryBadRequest}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	started := t.now()
	res, e := t.http.Do(req)
	if e != nil {
		return "", transportError("oauth", e, 1)
	}
	defer res.Body.Close()
	body, e := readLimited(res.Body, maxTokenBody)
	if e != nil {
		return "", &Error{Op: "oauth", Category: CategoryInvalid, Status: res.StatusCode}
	}
	if res.StatusCode != http.StatusOK {
		category := statusCategory(res.StatusCode)
		if res.StatusCode == 400 {
			// The documented response for a wrong client_id/secret pair is a 4xx OperationOutcome.
			category = CategoryAuth
		}
		return "", &Error{Op: "oauth", Category: category, Status: res.StatusCode}
	}
	if !isJSON(res.Header.Get("Content-Type")) {
		return "", &Error{Op: "oauth", Category: CategoryInvalid, Status: res.StatusCode}
	}
	var payload tokenResponse
	if json.Unmarshal(body, &payload) != nil {
		return "", &Error{Op: "oauth", Category: CategoryInvalid, Status: res.StatusCode}
	}
	seconds, e := strconv.ParseInt(strings.TrimSpace(string(payload.ExpiresIn)), 10, 64)
	token := strings.TrimSpace(payload.AccessToken)
	if e != nil || seconds <= 0 || token == "" || len(token) > maxTokenBytes || strings.ContainsAny(token, " \r\n\t") {
		return "", &Error{Op: "oauth", Category: CategoryInvalid, Status: res.StatusCode}
	}
	ttl := min(time.Duration(seconds)*time.Second, maxTokenTTL)
	// Expiry is measured from when the request was sent, minus a safety margin
	// (a quarter of the lifetime for short-lived tokens).
	margin := tokenMargin
	if ttl <= 4*tokenMargin {
		margin = ttl / 4
	}
	t.mu.Lock()
	t.token, t.expiry = token, started.Add(ttl-margin)
	t.mu.Unlock()
	return token, nil
}
func readLimited(r io.Reader, limit int64) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(r, limit+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > limit {
		return nil, errors.New("response body too large")
	}
	return b, nil
}
func isJSON(contentType string) bool {
	typ, _, e := mime.ParseMediaType(contentType)
	return e == nil && (typ == "application/json" || strings.HasSuffix(typ, "+json"))
}
func transportError(op string, e error, attempts int) *Error {
	if errors.Is(e, context.Canceled) {
		return &Error{Op: op, Category: CategoryCanceled, Attempts: attempts, err: context.Canceled}
	}
	if errors.Is(e, context.DeadlineExceeded) {
		return &Error{Op: op, Category: CategoryTimeout, Attempts: attempts, err: context.DeadlineExceeded}
	}
	var timeout interface{ Timeout() bool }
	if errors.As(e, &timeout) && timeout.Timeout() {
		return &Error{Op: op, Category: CategoryTimeout, Attempts: attempts}
	}
	return &Error{Op: op, Category: CategoryNetwork, Attempts: attempts}
}

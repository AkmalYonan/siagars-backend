package satusehat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Placeholder values only; no real credential is used anywhere in tests.
const (
	testClientID = "test-client-id-placeholder"
	testSecret   = "test-secret-placeholder-value"
)

type upstream struct {
	tokens, msi atomic.Int32
	tokenFn     func(w http.ResponseWriter, r *http.Request, n int32)
	msiFn       func(w http.ResponseWriter, r *http.Request, n int32)
}

func (u *upstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/oauth2/v1/accesstoken":
		n := u.tokens.Add(1)
		if u.tokenFn != nil {
			u.tokenFn(w, r, n)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"access_token":"token-%d","token_type":"BearerToken","expires_in":"3599","issued_at":"1671109805593"}`, n)
	case "/masterdata/v1/mastersaranaindex/mastersarana":
		n := u.msi.Add(1)
		if u.msiFn != nil {
			u.msiFn(w, r, n)
			return
		}
		page(w, 1, 1, facility("1000000001", "RS Uji"))
	default:
		http.NotFound(w, r)
	}
}
func facility(id, name string) string {
	return fmt.Sprintf(`{"kode_satusehat":%q,"kode_sarana":"K-%s","nama":%q,"telp":" 021-000 ","alamat":"Jl. Uji","longitude":"106.8","latitude":"-6.2","operasional":true,"status_sarana":"valid","status_aktif":true,"provinsi":{"kode":"31","nama":"DKI JAKARTA"},"kabkota":{"kode":3171,"nama":"KOTA ADM. JAKARTA PUSAT"},"jenis_sarana":{"kode":"104","nama":"Rumah Sakit"},"subjenis":{"kode":"1041","nama":"RSU"},"kelas_sarana":{"kode":"B","nama":"Kelas B"}}`, id, id, name)
}
func page(w http.ResponseWriter, n, total int, items ...string) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status_code":200,"message":"Success","page":%d,"total_page":%d,"data":[%s]}`, n, total, strings.Join(items, ","))
}
func setup(t *testing.T, u *upstream, timeout time.Duration) (*TokenProvider, *MSIClient, *[]time.Duration) {
	t.Helper()
	server := httptest.NewServer(u)
	t.Cleanup(server.Close)
	client := &http.Client{Timeout: timeout}
	tokens := NewTokenProvider(server.URL, Credentials{testClientID, testSecret}, client)
	msi := NewMSIClient(server.URL, tokens, client)
	var mu sync.Mutex
	slept := &[]time.Duration{}
	msi.sleep = func(ctx context.Context, d time.Duration) error {
		mu.Lock()
		*slept = append(*slept, d)
		mu.Unlock()
		return ctx.Err()
	}
	return tokens, msi, slept
}
func category(t *testing.T, e error) Category {
	t.Helper()
	var se *Error
	if !errors.As(e, &se) {
		t.Fatalf("expected *Error, got %v", e)
	}
	for _, secret := range []string{testClientID, testSecret, "token-"} {
		if strings.Contains(e.Error(), secret) {
			t.Fatalf("error leaks sensitive value: %s", e.Error())
		}
	}
	return se.Category
}

func TestOAuthRequestShape(t *testing.T) {
	u := &upstream{}
	u.tokenFn = func(w http.ResponseWriter, r *http.Request, n int32) {
		if r.Method != http.MethodPost || r.URL.Query().Get("grant_type") != "client_credentials" || len(r.URL.Query()) != 1 {
			t.Errorf("method/query: %s %s", r.Method, r.URL.RawQuery)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("content type %q", r.Header.Get("Content-Type"))
		}
		if e := r.ParseForm(); e != nil || r.PostForm.Get("client_id") != testClientID || r.PostForm.Get("client_secret") != testSecret || len(r.PostForm) != 2 {
			t.Errorf("form fields wrong")
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte(`{"access_token":"token-a","expires_in":3599}`))
	}
	tokens, _, _ := setup(t, u, time.Second)
	if token, e := tokens.AccessToken(context.Background()); e != nil || token != "token-a" {
		t.Fatal(token, e)
	}
}
func TestTokenCacheAndExpiry(t *testing.T) {
	u := &upstream{}
	tokens, _, _ := setup(t, u, time.Second)
	clock := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	tokens.now = func() time.Time { return clock }
	first, e := tokens.AccessToken(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	clock = clock.Add(3599*time.Second - tokenMargin - time.Second)
	if again, _ := tokens.AccessToken(context.Background()); again != first || u.tokens.Load() != 1 {
		t.Fatal("token not cached before expiry")
	}
	clock = clock.Add(2 * time.Second)
	if renewed, _ := tokens.AccessToken(context.Background()); renewed == first || u.tokens.Load() != 2 {
		t.Fatal("expired token not refreshed")
	}
}
func TestTokenShortExpiryMargin(t *testing.T) {
	u := &upstream{tokenFn: func(w http.ResponseWriter, r *http.Request, n int32) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"access_token":"token-%d","expires_in":"40"}`, n)
	}}
	tokens, _, _ := setup(t, u, time.Second)
	clock := time.Now()
	tokens.now = func() time.Time { return clock }
	tokens.AccessToken(context.Background())
	clock = clock.Add(29 * time.Second)
	tokens.AccessToken(context.Background())
	clock = clock.Add(2 * time.Second)
	tokens.AccessToken(context.Background())
	if u.tokens.Load() != 2 {
		t.Fatal("short-lived token margin", u.tokens.Load())
	}
}
func TestTokenSingleFlight(t *testing.T) {
	u := &upstream{}
	release := make(chan struct{})
	u.tokenFn = func(w http.ResponseWriter, r *http.Request, n int32) {
		<-release
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"token-shared","expires_in":"3599"}`))
	}
	tokens, _, _ := setup(t, u, 5*time.Second)
	var wg sync.WaitGroup
	results := make(chan string, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, e := tokens.AccessToken(context.Background())
			if e != nil {
				t.Error(e)
			}
			results <- token
		}()
	}
	time.Sleep(100 * time.Millisecond)
	close(release)
	wg.Wait()
	close(results)
	for token := range results {
		if token != "token-shared" {
			t.Fatal(token)
		}
	}
	if u.tokens.Load() != 1 {
		t.Fatal("concurrent callers created", u.tokens.Load(), "tokens")
	}
}
func TestTokenRejectedResponses(t *testing.T) {
	for _, c := range []struct {
		name, contentType, body string
		status                  int
		want                    Category
	}{
		{"bad credentials", "application/json", `{"resourceType":"OperationOutcome"}`, 401, CategoryAuth},
		{"bad request", "application/json", `{"resourceType":"OperationOutcome"}`, 400, CategoryAuth},
		{"forbidden", "application/json", `{}`, 403, CategoryForbidden},
		{"gateway", "text/plain", "Gateway Timeout", 504, CategoryUpstream},
		{"html", "text/html", `<html></html>`, 200, CategoryInvalid},
		{"invalid json", "application/json", `{`, 200, CategoryInvalid},
		{"empty token", "application/json", `{"access_token":"","expires_in":"3599"}`, 200, CategoryInvalid},
		{"zero expiry", "application/json", `{"access_token":"abc","expires_in":"0"}`, 200, CategoryInvalid},
		{"bad expiry", "application/json", `{"access_token":"abc","expires_in":"soon"}`, 200, CategoryInvalid},
		{"oversized", "application/json", `{"access_token":"` + strings.Repeat("a", maxTokenBody) + `","expires_in":"1"}`, 200, CategoryInvalid},
	} {
		t.Run(c.name, func(t *testing.T) {
			u := &upstream{tokenFn: func(w http.ResponseWriter, r *http.Request, n int32) {
				w.Header().Set("Content-Type", c.contentType)
				w.WriteHeader(c.status)
				w.Write([]byte(c.body))
			}}
			tokens, _, _ := setup(t, u, time.Second)
			_, e := tokens.AccessToken(context.Background())
			if got := category(t, e); got != c.want {
				t.Fatal(got)
			}
			if u.tokens.Load() != 1 {
				t.Fatal("OAuth must not be retried")
			}
		})
	}
}
func TestMSIRequestAndBearer(t *testing.T) {
	u := &upstream{}
	u.msiFn = func(w http.ResponseWriter, r *http.Request, n int32) {
		q := r.URL.Query()
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer token-1" || r.Header.Get("Accept") != "application/json" {
			t.Errorf("headers/method wrong")
		}
		if q.Get("jenis_sarana") != "104" || q.Get("status_aktif") != "true" || q.Get("page") != "2" || q.Get("limit") != "200" || len(q) != 4 {
			t.Errorf("query %s", r.URL.RawQuery)
		}
		page(w, 2, 3, facility("1", "A"), facility("2", "B"))
	}
	_, msi, _ := setup(t, u, time.Second)
	b, e := msi.FetchPage(context.Background(), "", 2, 200)
	if e != nil || len(b.Records) != 2 || b.NextPage == nil || *b.NextPage != 3 || b.TotalPage != 3 {
		t.Fatal(b, e)
	}
	if string(b.Records[0].City.Code) != "3171" {
		t.Fatal("numeric region code not normalized", b.Records[0].City.Code)
	}
	if _, e = msi.FetchPage(context.Background(), "", 1, MaxPageLimit+1); category(t, e) != CategoryBadRequest {
		t.Fatal("limit bound")
	}
	for _, bad := range []string{"31", "31.71", "317a", "31710"} {
		if _, e = msi.FetchPage(context.Background(), bad, 1, 10); category(t, e) != CategoryBadRequest {
			t.Fatal("kabkota validation", bad)
		}
	}
}
func TestMSIKabKotaFilter(t *testing.T) {
	u := &upstream{msiFn: func(w http.ResponseWriter, r *http.Request, n int32) {
		if r.URL.Query().Get("kode_kabkota") != "3171" || len(r.URL.Query()) != 5 {
			t.Errorf("query %s", r.URL.RawQuery)
		}
		page(w, 1, 1)
	}}
	_, msi, _ := setup(t, u, time.Second)
	if _, e := msi.FetchPage(context.Background(), "3171", 1, 10); e != nil {
		t.Fatal(e)
	}
}
func TestMSIPagination(t *testing.T) {
	for _, c := range []struct {
		name     string
		body     string
		page     int
		next     int
		invalid  bool
		contains int
	}{
		{"last page", `{"status_code":200,"page":3,"total_page":3,"data":[` + facility("1", "A") + `]}`, 3, 0, false, 1},
		{"empty page", `{"status_code":200,"page":1,"total_page":5,"data":[]}`, 1, 0, false, 0},
		{"null data", `{"status_code":200,"page":1,"total_page":0,"data":null}`, 1, 0, false, 0},
		{"negative total", `{"status_code":200,"page":1,"total_page":-1,"data":[]}`, 1, 0, true, 0},
		{"absurd total", `{"status_code":200,"page":1,"total_page":99999999,"data":[]}`, 1, 0, true, 0},
		{"page mismatch", `{"status_code":200,"page":7,"total_page":9,"data":[]}`, 1, 0, true, 0},
		{"body status error", `{"status_code":400,"message":"limit cannot be more than 2000","data":null}`, 1, 0, true, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			u := &upstream{msiFn: func(w http.ResponseWriter, r *http.Request, n int32) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(c.body))
			}}
			_, msi, _ := setup(t, u, time.Second)
			b, e := msi.FetchPage(context.Background(), "", c.page, 10)
			if c.invalid {
				if e == nil {
					t.Fatal("expected error")
				}
				category(t, e)
				return
			}
			if e != nil || b.NextPage != nil || len(b.Records) != c.contains {
				t.Fatal(b, e)
			}
		})
	}
}
func TestMSI401RefreshesOnce(t *testing.T) {
	u := &upstream{}
	u.msiFn = func(w http.ResponseWriter, r *http.Request, n int32) {
		if r.Header.Get("Authorization") == "Bearer token-1" {
			w.WriteHeader(401)
			return
		}
		page(w, 1, 1)
	}
	_, msi, _ := setup(t, u, time.Second)
	if _, e := msi.FetchPage(context.Background(), "", 1, 10); e != nil {
		t.Fatal(e)
	}
	if u.tokens.Load() != 2 || u.msi.Load() != 2 {
		t.Fatal("refresh", u.tokens.Load(), u.msi.Load())
	}
	always := &upstream{msiFn: func(w http.ResponseWriter, r *http.Request, n int32) { w.WriteHeader(401) }}
	_, msi, _ = setup(t, always, time.Second)
	if _, e := msi.FetchPage(context.Background(), "", 1, 10); category(t, e) != CategoryAuth || always.msi.Load() != 2 || always.tokens.Load() != 2 {
		t.Fatal("401 retried more than once", always.msi.Load())
	}
}
func TestMSINoRetryForClientErrors(t *testing.T) {
	for status, want := range map[int]Category{403: CategoryForbidden, 400: CategoryBadRequest, 422: CategoryBadRequest} {
		u := &upstream{msiFn: func(w http.ResponseWriter, r *http.Request, n int32) { w.WriteHeader(status) }}
		_, msi, slept := setup(t, u, time.Second)
		if _, e := msi.FetchPage(context.Background(), "", 1, 10); category(t, e) != want || u.msi.Load() != 1 || len(*slept) != 0 {
			t.Fatal(status, "retried")
		}
	}
}
func TestMSI429RetryAfter(t *testing.T) {
	u := &upstream{msiFn: func(w http.ResponseWriter, r *http.Request, n int32) {
		if n == 1 {
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(429)
			return
		}
		page(w, 1, 1)
	}}
	_, msi, slept := setup(t, u, time.Second)
	if _, e := msi.FetchPage(context.Background(), "", 1, 10); e != nil || len(*slept) != 1 || (*slept)[0] != 3*time.Second {
		t.Fatal(e, *slept)
	}
	tooLong := &upstream{msiFn: func(w http.ResponseWriter, r *http.Request, n int32) {
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(429)
	}}
	_, msi, slept = setup(t, tooLong, time.Second)
	if _, e := msi.FetchPage(context.Background(), "", 1, 10); category(t, e) != CategoryRateLimited || len(*slept) != 0 || tooLong.msi.Load() != 1 {
		t.Fatal("Retry-After beyond cap must fail fast")
	}
	always := &upstream{msiFn: func(w http.ResponseWriter, r *http.Request, n int32) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(429)
	}}
	_, msi, _ = setup(t, always, time.Second)
	if _, e := msi.FetchPage(context.Background(), "", 1, 10); category(t, e) != CategoryRateLimited || always.msi.Load() != 3 {
		t.Fatal("429 retries unbounded", always.msi.Load())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, msi, slept = setup(t, &upstream{msiFn: func(w http.ResponseWriter, r *http.Request, n int32) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(429)
	}}, time.Second)
	if _, e := msi.FetchPage(ctx, "", 1, 10); category(t, e) != CategoryRateLimited || len(*slept) != 0 {
		t.Fatal("Retry-After past context deadline must fail fast")
	}
}
func TestMSI5xxAndTimeoutBounded(t *testing.T) {
	u := &upstream{msiFn: func(w http.ResponseWriter, r *http.Request, n int32) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(503)
		w.Write([]byte("Gateway Timeout"))
	}}
	_, msi, slept := setup(t, u, time.Second)
	if _, e := msi.FetchPage(context.Background(), "", 1, 10); category(t, e) != CategoryUpstream || u.msi.Load() != 3 || len(*slept) != 2 {
		t.Fatal("5xx", u.msi.Load(), *slept)
	}
	if (*slept)[0] < msi.backoff || (*slept)[1] < 2*msi.backoff || (*slept)[1] >= 4*msi.backoff {
		t.Fatal("backoff not exponential with jitter", *slept)
	}
	recovering := &upstream{msiFn: func(w http.ResponseWriter, r *http.Request, n int32) {
		if n < 3 {
			w.WriteHeader(502)
			return
		}
		page(w, 1, 1)
	}}
	_, msi, _ = setup(t, recovering, time.Second)
	if _, e := msi.FetchPage(context.Background(), "", 1, 10); e != nil {
		t.Fatal(e)
	}
	slow := &upstream{msiFn: func(w http.ResponseWriter, r *http.Request, n int32) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}}
	_, msi, _ = setup(t, slow, 50*time.Millisecond)
	if _, e := msi.FetchPage(context.Background(), "", 1, 10); category(t, e) != CategoryTimeout || slow.msi.Load() != 3 {
		t.Fatal("timeout", slow.msi.Load())
	}
}
func TestMSIRejectsInvalidBodies(t *testing.T) {
	for name, fn := range map[string]func(http.ResponseWriter){
		"invalid json": func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":[`))
		},
		"wrong type": func(w http.ResponseWriter) { w.Header().Set("Content-Type", "text/html"); w.Write([]byte(`{}`)) },
		"bad field": func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":[{"nama":{"x":1}}]}`))
		},
		"oversized": func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":[],"message":"` + strings.Repeat("x", maxMSIBody) + `"}`))
		},
	} {
		t.Run(name, func(t *testing.T) {
			u := &upstream{msiFn: func(w http.ResponseWriter, r *http.Request, n int32) { fn(w) }}
			_, msi, _ := setup(t, u, 5*time.Second)
			if _, e := msi.FetchPage(context.Background(), "", 1, 10); category(t, e) != CategoryInvalid || u.msi.Load() != 1 {
				t.Fatal("invalid body retried or accepted", u.msi.Load())
			}
		})
	}
}
func TestNullFieldsDoNotPanic(t *testing.T) {
	var r MSIResponse
	body := `{"status_code":200,"page":1,"total_page":1,"data":[{"kode_satusehat":null,"kode_sarana":null,"nama":null,"telp":null,"alamat":null,"longitude":null,"latitude":null,"operasional":null,"status_sarana":null,"status_aktif":null,"provinsi":null,"kabkota":null,"jenis_sarana":null,"subjenis":null,"kelas_sarana":null},{}]}`
	if e := json.Unmarshal([]byte(body), &r); e != nil {
		t.Fatal(e)
	}
	for _, f := range r.Data {
		if _, reason, ok := Map(f, Namespace, time.Now()); ok || reason != SkipType {
			t.Fatal(reason)
		}
	}
}
func valid() MSIFacility {
	var f MSIFacility
	if e := json.Unmarshal([]byte(facility("1000000001", "  RS Uji  ")), &f); e != nil {
		panic(e)
	}
	return f
}
func TestMapNormalizes(t *testing.T) {
	at := time.Date(2026, 10, 23, 1, 2, 3, 456789, time.FixedZone("WIB", 7*3600))
	h, _, ok := Map(valid(), Namespace, at)
	if !ok || h.ExternalID != "1000000001" || h.Name != "RS Uji" || h.SourceNamespace != Namespace || *h.Phone != "021-000" || h.ProvinceCode != "31" || h.CityCode != "3171" || h.CityName == "" {
		t.Fatal(h)
	}
	if *h.Latitude != -6.2 || *h.Longitude != 106.8 || !h.SourceActive || h.SourceStatus != "valid" || h.FacilityTypeCode != "104" || h.FacilitySubtype != "1041" || h.FacilityClass != "B" || h.FacilityCode != "K-1000000001" {
		t.Fatal(h)
	}
	if h.SyncedAt.Location() != time.UTC || h.SyncedAt.Nanosecond()%int(time.Millisecond) != 0 {
		t.Fatal("synced_at must be UTC millisecond", h.SyncedAt)
	}
	f := valid()
	f.Phone, f.Address = "   ", ""
	f.Active = Flag{false, true}
	if h, _, _ = Map(f, Namespace, at); h.Phone != nil || h.Address != nil || h.SourceActive {
		t.Fatal("empty strings must be nil and explicit inactive kept", h)
	}
	f.Active = Flag{}
	if h, _, _ = Map(f, Namespace, at); !h.SourceActive {
		t.Fatal("missing status_aktif must not deactivate")
	}
}
func TestCoordinatesPairwise(t *testing.T) {
	for _, c := range []struct {
		lat, lng Text
		ok       bool
	}{
		{"-6.2", "106.8", true}, {"", "106.8", false}, {"-6.2", "", false}, {"abc", "106.8", false},
		{"-91", "106.8", false}, {"-6.2", "181", false}, {"NaN", "106.8", false}, {"0", "0", false}, {"Inf", "1", false},
	} {
		lat, lng := coordinates(c.lat, c.lng)
		if (lat != nil) != c.ok || (lng != nil) != c.ok {
			t.Errorf("%q,%q", c.lat, c.lng)
		}
	}
}
func TestMapSkipsIncompleteRecords(t *testing.T) {
	for want, change := range map[string]func(*MSIFacility){
		SkipType:   func(f *MSIFacility) { f.FacilityType.Code = "103" },
		SkipID:     func(f *MSIFacility) { f.SATUSEHATCode = " " },
		SkipName:   func(f *MSIFacility) { f.Name = "" },
		SkipRegion: func(f *MSIFacility) { f.City = nil },
	} {
		f := valid()
		change(&f)
		if _, reason, ok := Map(f, Namespace, time.Now()); ok || reason != want {
			t.Fatal(want, reason)
		}
	}
	for _, change := range []func(*MSIFacility){
		func(f *MSIFacility) { f.Province = nil },
		func(f *MSIFacility) { f.Province.Code = "" },
		func(f *MSIFacility) { f.City.Code = "3271" },
		func(f *MSIFacility) { f.City.Code = "31.71" },
		func(f *MSIFacility) { f.City.Name = "" },
	} {
		f := valid()
		change(&f)
		if _, reason, ok := Map(f, Namespace, time.Now()); ok || reason != SkipRegion {
			t.Fatal("region", reason)
		}
	}
}
func TestRetryAfterParsing(t *testing.T) {
	now := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	if d, ok := parseRetryAfter("5", now); !ok || d != 5*time.Second {
		t.Fatal(d)
	}
	if d, ok := parseRetryAfter(now.Add(10*time.Second).Format(time.RFC1123), now); !ok || d != 10*time.Second {
		t.Fatal(d)
	}
	for _, v := range []string{"", "-1", "soon", "5s"} {
		if _, ok := parseRetryAfter(v, now); ok {
			t.Fatal(v)
		}
	}
}
func TestDisabledSource(t *testing.T) {
	if _, e := (DisabledSource{}).FetchPage(context.Background(), "", 1, 1); !errors.Is(e, ErrDisabled) {
		t.Fatal(e)
	}
}

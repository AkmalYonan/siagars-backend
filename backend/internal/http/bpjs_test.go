package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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

// bpjsFake adds the BPJS store to the in-memory operator store.
type bpjsFake struct {
	*secStore
	bm         sync.Mutex
	members    map[string]domain.BPJSMember
	records    []domain.BPJSRecord
	audits     []string
	failRecord bool
	failAudit  bool
}

func (f *bpjsFake) BPJSMember(_ context.Context, nik string) (domain.BPJSMember, error) {
	if m, ok := f.members[nik]; ok {
		return m, nil
	}
	return domain.BPJSMember{}, repository.ErrNotFound
}
func (f *bpjsFake) RecordBPJSCheck(_ context.Context, r domain.BPJSRecord) error {
	f.bm.Lock()
	defer f.bm.Unlock()
	if f.failRecord {
		return errors.New("db down")
	}
	f.records = append(f.records, r)
	return nil
}
func (f *bpjsFake) id(i int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1) }
func (f *bpjsFake) BPJSHistory(_ context.Context, page, limit int, result string) ([]domain.BPJSHistoryItem, int, error) {
	f.bm.Lock()
	defer f.bm.Unlock()
	var all []domain.BPJSHistoryItem
	for i := len(f.records) - 1; i >= 0; i-- {
		r := f.records[i]
		if result == "" || r.Result == result {
			all = append(all, domain.BPJSHistoryItem{ID: f.id(i), NIKMasked: r.NIKMasked, NameMasked: r.NameMasked, BirthYear: r.BirthYear, Result: r.Result, Status: r.Status, CheckedAt: time.Now().UTC(), RepeatCount: 1})
		}
	}
	from := min((page-1)*limit, len(all))
	return all[from:min(from+limit, len(all))], len(all), nil
}
func (f *bpjsFake) RevealBPJS(_ context.Context, id string) (domain.BPJSReveal, error) {
	f.bm.Lock()
	defer f.bm.Unlock()
	for i, r := range f.records {
		if f.id(i) == id {
			if r.Raw.NIK == "" {
				return domain.BPJSReveal{}, repository.ErrRawUnavailable
			}
			return domain.BPJSReveal{ID: id, NIK: r.Raw.NIK, Name: r.Raw.Name, BirthDate: r.Raw.BirthDate, CheckedAt: time.Now().UTC()}, nil
		}
	}
	return domain.BPJSReveal{}, repository.ErrNotFound
}
func (f *bpjsFake) AuditBPJS(_ context.Context, actor, action, id, _ string) error {
	f.bm.Lock()
	defer f.bm.Unlock()
	if f.failAudit {
		return errors.New("audit down")
	}
	f.audits = append(f.audits, action+":"+id)
	return nil
}

func newBPJSRig(t *testing.T) (*secRig, *bpjsFake) {
	r := newSecRig(t)
	cls, name := "3", "Pekerja Bukan Penerima Upah"
	fake := &bpjsFake{secStore: r.store, members: map[string]domain.BPJSMember{
		"9999010101900001": {NIK: "9999010101900001", Name: "Budi Santoso", BirthDate: "1990-01-01", Status: "active", CareClass: &cls, MembershipType: &domain.Coded{Code: "PBPU", Name: &name}},
		"9999020202850002": {NIK: "9999020202850002", Name: "Siti Aminah", BirthDate: "1985-02-02", Status: "inactive"},
	}}
	r.h = New(fake, config.Config{PublicOrigin: secOrigin, SessionTTL: time.Hour}, func(context.Context) error { return nil })
	return r, fake
}
func check(r *secRig, nik, name, birth string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"nik": nik, "name": name, "birth_date": birth})
	return r.do(secReq{method: "POST", path: "/bpjs/check", body: string(body), noOrigin: true})
}

func TestBPJSCheckFoundAndMasked(t *testing.T) {
	r, fake := newBPJSRig(t)
	w := check(r, "9999010101900001", "  budi   SANTOSO ", "1990-01-01")
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Body.String(), w.Header())
	}
	var out struct {
		Data struct {
			Result      string
			Participant map[string]any
			DataOrigin  string `json:"data_origin"`
		}
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	p := out.Data.Participant
	if out.Data.Result != "found" || out.Data.DataOrigin != "simulation" || p["nik_masked"] != "9999********0001" || p["name_masked"] != "BU** SA*****" || p["status"] != "active" || p["care_class"] != "3" {
		t.Fatalf("%s", w.Body.String())
	}
	for _, leak := range []string{"9999010101900001", "Budi", "SANTOSO", "1990"} {
		if strings.Contains(w.Body.String(), leak) {
			t.Errorf("respons memuat %q", leak)
		}
	}
	if len(fake.records) != 1 || fake.records[0].Result != "found" || fake.records[0].Raw.Name != "budi SANTOSO" {
		t.Fatalf("riwayat: %+v", fake.records)
	}
}

func TestBPJSNotFoundIsIndistinguishable(t *testing.T) {
	r, _ := newBPJSRig(t)
	strip := func(w *httptest.ResponseRecorder) string {
		var m map[string]map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &m)
		delete(m["data"], "checked_at")
		b, _ := json.Marshal(m)
		return string(b)
	}
	cases := []*httptest.ResponseRecorder{
		check(r, "9999999999999999", "Budi Santoso", "1990-01-01"),
		check(r, "9999010101900001", "Bukan Budi", "1990-01-01"),
		check(r, "9999010101900001", "Budi Santoso", "1990-01-02"),
	}
	want := `{"data":{"data_origin":"simulation","participant":null,"result":"not_found"}}`
	for i, w := range cases {
		if w.Code != 200 || strip(w) != want {
			t.Errorf("kasus %d: %d %s", i, w.Code, strip(w))
		}
	}
}

func TestBPJSValidationAndMethods(t *testing.T) {
	r, fake := newBPJSRig(t)
	for name, w := range map[string]*httptest.ResponseRecorder{
		"nik pendek":         check(r, "123", "Budi", "1990-01-01"),
		"tanggal masa depan": check(r, "9999010101900001", "Budi", "2999-01-01"),
		"nama berangka":      check(r, "9999010101900001", "B1", "1990-01-01"),
	} {
		if w.Code != 422 || errCode(w) != "validation_error" {
			t.Errorf("%s: %d %s", name, w.Code, w.Body.String())
		}
	}
	if w := r.do(secReq{method: "POST", path: "/bpjs/check", body: `{"nik":"9999010101900001","name":"Budi","birth_date":"1990-01-01","x":1}`, noOrigin: true}); w.Code != 400 || errCode(w) != "invalid_json" {
		t.Errorf("field asing: %d", w.Code)
	}
	if w := r.do(secReq{method: "POST", path: "/bpjs/check", body: `{}`, contentType: "text/plain", noOrigin: true}); w.Code != 422 {
		t.Errorf("content-type: %d", w.Code)
	}
	if w := r.do(secReq{method: "GET", path: "/bpjs/check", noOrigin: true}); w.Code != 405 {
		t.Errorf("GET: %d", w.Code)
	}
	if len(fake.records) != 0 {
		t.Fatal("input yang ditolak ikut tersimpan")
	}
}

func TestBPJSRateLimit(t *testing.T) {
	r, _ := newBPJSRig(t)
	for i := 0; i < 10; i++ {
		if w := check(r, "9999010101900001", "Budi Santoso", "1990-01-01"); w.Code != 200 {
			t.Fatalf("permintaan %d: %d", i+1, w.Code)
		}
	}
	w := check(r, "9999010101900001", "Budi Santoso", "1990-01-01")
	if w.Code != 429 || w.Header().Get("Retry-After") == "" || errCode(w) != "rate_limited" {
		t.Fatal(w.Code, w.Header())
	}
}

func TestBPJSHistoryFailureDoesNotBreakAnswerAndNothingIsLogged(t *testing.T) {
	r, fake := newBPJSRig(t)
	fake.failRecord = true
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(old)
	w := check(r, "9999010101900001", "Budi Santoso", "1990-01-01")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"found"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if !strings.Contains(logs.String(), "bpjs history write failed") {
		t.Fatal("kegagalan tidak dicatat:", logs.String())
	}
	check(r, "9999020202850002", "Siti Aminah", "1985-02-02")
	for _, leak := range []string{"9999010101900001", "Budi", "Santoso", "1990-01-01", "Siti", "Aminah", "9999020202850002"} {
		if strings.Contains(logs.String(), leak) {
			t.Errorf("log memuat input %q:\n%s", leak, logs.String())
		}
	}
}

func revealPath(id string) string  { return "/admin/bpjs-checks/" + id + "/reveal" }
func passwordBody(p string) string { return `{"password":"` + p + `"}` }

func TestBPJSAdminHistoryAndReveal(t *testing.T) {
	r, fake := newBPJSRig(t)
	check(r, "9999010101900001", "Budi Santoso", "1990-01-01")
	check(r, "9999999999999999", "Orang Lain", "1980-05-05")
	if w := r.do(secReq{method: "GET", path: "/admin/bpjs-checks"}); w.Code != 401 {
		t.Fatalf("tanpa sesi: %d", w.Code)
	}
	cookie, csrf := r.login()
	w := r.do(secReq{method: "GET", path: "/admin/bpjs-checks?limit=1", cookie: cookie})
	if w.Code != 200 || strings.Contains(w.Body.String(), "9999999999999999") || strings.Contains(w.Body.String(), "Orang") || !strings.Contains(w.Body.String(), `"total":2`) || !strings.Contains(w.Body.String(), `"total_pages":2`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := r.do(secReq{method: "GET", path: "/admin/bpjs-checks?result=found", cookie: cookie}); !strings.Contains(w.Body.String(), `"total":1`) {
		t.Fatal(w.Body.String())
	}
	for _, q := range []string{"?result=x", "?limit=51", "?limit=0", "?page=0", "?bogus=1", "?page=1&page=2"} {
		if w := r.do(secReq{method: "GET", path: "/admin/bpjs-checks" + q, cookie: cookie}); w.Code != 422 {
			t.Errorf("%s: %d", q, w.Code)
		}
	}
	id := fake.id(0)
	if w := r.do(secReq{method: "POST", path: revealPath(id), body: passwordBody(secPassword)}); w.Code != 401 {
		t.Errorf("tanpa sesi: %d", w.Code)
	}
	if w := r.do(secReq{method: "POST", path: revealPath(id), body: passwordBody(secPassword), cookie: cookie}); w.Code != 403 || errCode(w) != "invalid_csrf" {
		t.Errorf("tanpa csrf: %d", w.Code)
	}
	if w := r.do(secReq{method: "POST", path: revealPath(id), body: passwordBody(secPassword), cookie: cookie, csrf: csrf, origin: "https://evil.example"}); w.Code != 403 || errCode(w) != "invalid_origin" {
		t.Errorf("origin asing: %d", w.Code)
	}
	if len(fake.audits) != 0 {
		t.Fatalf("audit tercatat sebelum re-auth: %v", fake.audits)
	}
	w = r.do(secReq{method: "POST", path: revealPath(id), body: passwordBody("salah-salah-salah"), cookie: cookie, csrf: csrf})
	if w.Code != 401 || errCode(w) != "invalid_credentials" || strings.Contains(w.Body.String(), "9999") || len(fake.audits) != 1 || !strings.HasPrefix(fake.audits[0], "bpjs.reveal_denied:") {
		t.Fatal(w.Code, w.Body.String(), fake.audits)
	}
	w = r.do(secReq{method: "POST", path: revealPath(id), body: passwordBody(secPassword), cookie: cookie, csrf: csrf})
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Body.String(), `"nik":"9999010101900001"`) || !strings.Contains(w.Body.String(), `"name":"Budi Santoso"`) || fake.audits[len(fake.audits)-1] != "bpjs.reveal:"+id {
		t.Fatal(w.Code, w.Body.String(), fake.audits)
	}
	if w := r.do(secReq{method: "POST", path: revealPath(fake.id(99)), body: passwordBody(secPassword), cookie: cookie, csrf: csrf}); w.Code != 404 {
		t.Errorf("id tidak ada: %d", w.Code)
	}
	if w := r.do(secReq{method: "POST", path: revealPath("NOPE"), body: passwordBody(secPassword), cookie: cookie, csrf: csrf}); w.Code != 404 {
		t.Errorf("id jelek: %d", w.Code)
	}
	fake.records[1].Raw = domain.BPJSRaw{}
	if w := r.do(secReq{method: "POST", path: revealPath(fake.id(1)), body: passwordBody(secPassword), cookie: cookie, csrf: csrf}); w.Code != 409 || errCode(w) != "history_raw_unavailable" {
		t.Errorf("tanpa data asli: %d %s", w.Code, w.Body.String())
	}
	fake.failAudit = true
	if w := r.do(secReq{method: "POST", path: revealPath(id), body: passwordBody(secPassword), cookie: cookie, csrf: csrf}); w.Code != 500 || strings.Contains(w.Body.String(), "9999") {
		t.Errorf("audit gagal: %d %s", w.Code, w.Body.String())
	}
}

func TestBPJSRevealRateLimit(t *testing.T) {
	r, fake := newBPJSRig(t)
	check(r, "9999010101900001", "Budi Santoso", "1990-01-01")
	cookie, csrf := r.login()
	for i := 0; i < 5; i++ {
		if w := r.do(secReq{method: "POST", path: revealPath(fake.id(0)), body: passwordBody("salah-salah-salah"), cookie: cookie, csrf: csrf}); w.Code != 401 {
			t.Fatalf("percobaan %d: %d", i+1, w.Code)
		}
	}
	w := r.do(secReq{method: "POST", path: revealPath(fake.id(0)), body: passwordBody(secPassword), cookie: cookie, csrf: csrf})
	if w.Code != 429 || strings.Contains(w.Body.String(), "9999") {
		t.Fatal("rate limit reveal:", w.Code)
	}
}

func TestBPJSStoreWithoutSupportAnswers501(t *testing.T) {
	r := newSecRig(t)
	if w := check(r, "9999010101900001", "Budi", "1990-01-01"); w.Code != http.StatusNotImplemented {
		t.Fatal(w.Code)
	}
}

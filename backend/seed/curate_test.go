package seed

import (
	"os"
	"strings"
	"testing"
	"time"
)

var curateNow = time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)

func valid() CurationRecord {
	b := true
	t, a := 10, 0
	return CurationRecord{ExternalID: "X-1", BPJS: &b, SourceURL: "https://sumber.invalid/x", SourceUpdatedAt: "2026-10-20T08:00:00Z",
		Services: []CurationService{{Slug: "icu", Status: "available"}}, Beds: []CurationBed{{Category: "icu", Total: &t, Available: &a}}}
}

func errorsOf(issues []CurationIssue) string {
	var s []string
	for _, i := range issues {
		if !i.Warning {
			s = append(s, i.Field+": "+i.Message)
		}
	}
	return strings.Join(s, "|")
}

func TestValidateCurationAccepts(t *testing.T) {
	out, issues := ValidateCuration([]CurationRecord{valid()}, curateNow)
	if len(out) != 1 || errorsOf(issues) != "" {
		t.Fatalf("%v %v", out, issues)
	}
	c := out[0]
	if c.SourceNamespace != DefaultCurationNamespace || c.Beds[0].Available == nil || *c.Beds[0].Available != 0 {
		t.Fatalf("zero must stay zero: %+v", c)
	}
	if !HasPlaceholder(issues) {
		t.Fatal(".invalid host must be flagged placeholder")
	}
}

func TestValidateCurationRejects(t *testing.T) {
	neg, big := -1, 5
	cases := map[string]func(*CurationRecord){
		"source_url wajib":           func(r *CurationRecord) { r.SourceURL = "" },
		"https":                      func(r *CurationRecord) { r.SourceURL = "http://x.invalid/" },
		"source_updated_at wajib":    func(r *CurationRecord) { r.SourceUpdatedAt = "" },
		"RFC3339":                    func(r *CurationRecord) { r.SourceUpdatedAt = "20/10/2026" },
		"masa depan":                 func(r *CurationRecord) { r.SourceUpdatedAt = "2026-10-25T00:00:00Z" },
		"external_id":                func(r *CurationRecord) { r.ExternalID = " " },
		"verified_at":                func(r *CurationRecord) { r.VerifiedAt = "2026-10-20T00:00:00Z" },
		"data_origin":                func(r *CurationRecord) { r.DataOrigin = "satusehat_production" },
		"status harus":               func(r *CurationRecord) { r.Services[0].Status = "ready" },
		"duplikat":                   func(r *CurationRecord) { r.Services = append(r.Services, r.Services[0]) },
		"slug":                       func(r *CurationRecord) { r.Services[0].Slug = "ICU; drop" },
		"kategori":                   func(r *CurationRecord) { r.Beds[0].Category = "vip" },
		"tidak boleh negatif":        func(r *CurationRecord) { r.Beds[0].Available = &neg },
		"available harus <= total":   func(r *CurationRecord) { r.Beds[0].Available = &big; r.Beds[0].Total = new(int) },
		"keduanya kosong":            func(r *CurationRecord) { r.Beds[0].Total, r.Beds[0].Available = nil, nil },
		"tidak ada bpjs":             func(r *CurationRecord) { r.BPJS, r.Services, r.Beds = nil, nil, nil },
		"service source_updated_at":  func(r *CurationRecord) { r.Services[0].SourceUpdatedAt = "bukan-waktu" },
		"service source_url invalid": func(r *CurationRecord) { r.Services[0].SourceURL = "ftp://x" },
	}
	for name, mutate := range cases {
		r := valid()
		mutate(&r)
		out, issues := ValidateCuration([]CurationRecord{r}, curateNow)
		if len(out) != 0 || errorsOf(issues) == "" {
			t.Errorf("%s: expected rejection, got %v %v", name, out, issues)
		}
	}
	a, b := valid(), valid()
	if out, issues := ValidateCuration([]CurationRecord{a, b}, curateNow); len(out) != 1 || !strings.Contains(errorsOf(issues), "duplikat dalam file") {
		t.Fatalf("duplicate external_id: %v %v", out, issues)
	}
}

func TestCurationNullIsNotZero(t *testing.T) {
	recs, e := ParseCuration("a.json", []byte(`{"schema_version":1,"hospitals":[{"external_id":"X","bpjs":null,"source_url":"https://s.invalid/","source_updated_at":"2026-10-20T00:00:00Z","beds":[{"category":"icu","total":0,"available":null}]}]}`))
	if e != nil {
		t.Fatal(e)
	}
	out, issues := ValidateCuration(recs, curateNow)
	if len(out) != 1 {
		t.Fatal(issues)
	}
	if out[0].BPJS != nil || out[0].Beds[0].Total == nil || *out[0].Beds[0].Total != 0 || out[0].Beds[0].Available != nil {
		t.Fatalf("%+v", out[0])
	}
}

func TestParseCurationJSONStrict(t *testing.T) {
	for name, body := range map[string]string{
		"unknown field": `{"schema_version":1,"hospitals":[{"external_id":"X","address":"?"}]}`,
		"version":       `{"schema_version":2,"hospitals":[]}`,
		"trailing":      `{"schema_version":1,"hospitals":[]} {}`,
		"malformed":     `{`,
	} {
		if _, e := ParseCuration("a.json", []byte(body)); e == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if _, e := ParseCuration("a.txt", nil); e == nil {
		t.Error("extension accepted")
	}
}

func TestParseCurationCSV(t *testing.T) {
	head := "external_id,bpjs,source_url,source_updated_at,svc_icu,bed_icu_total,bed_icu_available\n"
	recs, e := ParseCuration("a.csv", []byte(head+"X,false,https://s.invalid/,2026-10-20T00:00:00Z,limited,0,\n,,,,,,\n"))
	if e != nil || len(recs) != 1 {
		t.Fatal(e, recs)
	}
	r := recs[0]
	if r.BPJS == nil || *r.BPJS || len(r.Services) != 1 || r.Beds[0].Total == nil || *r.Beds[0].Total != 0 || r.Beds[0].Available != nil {
		t.Fatalf("%+v", r)
	}
	for name, body := range map[string]string{
		"bad int":     head + "X,,https://s.invalid/,2026-10-20T00:00:00Z,,abc,\n",
		"bad bpjs":    head + "X,maybe,https://s.invalid/,2026-10-20T00:00:00Z,,,\n",
		"unknown col": "external_id,address\nX,y\n",
		"duplicate":   "external_id,external_id\nX,y\n",
		"empty":       "",
	} {
		if _, e := ParseCuration("a.csv", []byte(body)); e == nil {
			t.Errorf("%s accepted", name)
		}
	}
	// A verified_at column is parsed so the validator can reject it with a clear message.
	recs, e = ParseCuration("a.csv", []byte("external_id,verified_at,source_url,source_updated_at,svc_icu\nX,2026-10-20T00:00:00Z,https://s.invalid/,2026-10-20T00:00:00Z,available\n"))
	if e != nil {
		t.Fatal(e)
	}
	if out, _ := ValidateCuration(recs, curateNow); len(out) != 0 {
		t.Fatal("verified_at column must be rejected")
	}
}

func TestPlaceholderExamplesDryRun(t *testing.T) {
	for _, f := range []string{"example.placeholder.json", "example.placeholder.csv"} {
		var sb strings.Builder
		if e := RunCurate([]string{"../data/curation/" + f}, &sb); e != nil {
			t.Fatalf("%s: %v\n%s", f, e, sb.String())
		}
		for _, want := range []string{"DRY-RUN", "valid=2 ditolak=0", "placeholder=true", "verified_at=tidak-pernah-diisi"} {
			if !strings.Contains(sb.String(), want) {
				t.Errorf("%s: report lacks %q:\n%s", f, want, sb.String())
			}
		}
	}
}

func TestRunCurateGuards(t *testing.T) {
	t.Setenv("MONGODB_URI", "")
	var sb strings.Builder
	if e := RunCurate([]string{"--bogus", "x.json"}, &sb); e == nil {
		t.Error("unknown flag accepted")
	}
	if e := RunCurate([]string{"a.json", "b.json"}, &sb); e == nil {
		t.Error("two files accepted")
	}
	// Placeholder data is refused before any database is contacted.
	if e := RunCurate([]string{"../data/curation/example.placeholder.json", "--apply"}, &sb); e == nil || !strings.Contains(e.Error(), "placeholder") {
		t.Errorf("apply with placeholder: %v", e)
	}
	if _, e := os.Stat("../data/curation/README.md"); e != nil {
		t.Error("README missing")
	}
}

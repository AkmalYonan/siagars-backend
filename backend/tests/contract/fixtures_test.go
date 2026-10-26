package contract

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"siagars/backend/internal/domain"
)

// stripFreshness removes the only time-dependent fields (freshness is recomputed against the server clock).
func stripFreshness(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, c := range x {
			if k != "freshness" {
				out[k] = stripFreshness(c)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, c := range x {
			out[i] = stripFreshness(c)
		}
		return out
	}
	return v
}

func hospitalFromFixture(t *testing.T, fixture any) domain.Hospital {
	t.Helper()
	b, _ := json.Marshal(fixture)
	var h domain.Hospital
	if e := json.Unmarshal(b, &h); e != nil {
		t.Fatal(e)
	}
	return h
}

// Every fixture must itself satisfy the OpenAPI response schema it documents.
func TestFixturesMatchOpenAPI(t *testing.T) {
	s, fx := loadSpec(t), loadFixtures(t)
	for name, target := range map[string]struct {
		method, path string
		status       int
	}{
		"hospital_detail": {"GET", "/hospitals/{id}", 200}, "hospital_list": {"GET", "/hospitals", 200},
		"tracking": {"POST", "/referrals/lookup", 200}, "bpjs_found": {"POST", "/bpjs/check", 200}, "bpjs_not_found": {"POST", "/bpjs/check", 200}, "conflict": {"PATCH", "/admin/referrals/{id}/status", 409}, "admin_referrals": {"GET", "/admin/referrals", 200},
		"login_without_csrf": {"POST", "/auth/login", 200}, "validation_error": {"GET", "/hospitals", 422},
	} {
		schema, ok := s.responseSchema(target.method, target.path, target.status)
		if !ok || fx[name] == nil {
			t.Errorf("%s: skema atau fixture tidak ada", name)
			continue
		}
		for _, e := range s.validate(schema, any(fx[name])) {
			t.Errorf("fixture %s: %s", name, e)
		}
	}
	// Fixture freshness must agree with the documented thresholds evaluated at the fixture's own as_of.
	detail := obj(fx["hospital_detail"]["data"])
	asOf, _ := time.Parse(time.RFC3339, obj(fx["hospital_detail"]["meta"])["as_of"].(string))
	check := func(label string, item map[string]any) {
		var src *time.Time
		if v, ok := item["source_updated_at"].(string); ok {
			x, _ := time.Parse(time.RFC3339, v)
			src = &x
		}
		if got := domain.Freshness(src, asOf); got != item["freshness"] {
			t.Errorf("fixture %s: freshness %v, aturan menghasilkan %s", label, item["freshness"], got)
		}
	}
	check("hospital", detail)
	for _, x := range detail["services"].([]any) {
		check("service", obj(x))
	}
	for _, x := range detail["beds"].([]any) {
		check("bed", obj(x))
	}
	// null vs 0 in the fixture is the documented example: total unknown, available zero.
	if b := obj(detail["beds"].([]any)[0]); b["total"] != nil || !reflect.DeepEqual(b["available"], json.Number("0")) {
		t.Errorf("fixture bed null vs 0: %v", b)
	}
}

// The handlers must serve exactly what the fixtures promise (apart from the recomputed freshness).
func TestHandlersReproduceFixtures(t *testing.T) {
	fx := loadFixtures(t)
	hospital := hospitalFromFixture(t, fx["hospital_detail"]["data"])
	r := newRig(t, []domain.Hospital{hospital})
	r.store.referrals[0].DestinationHospitalID = &hospital.ID
	tr := obj(fx["tracking"]["data"])
	r.store.referrals[0].Status, r.store.referrals[0].PublicNote, r.store.referrals[0].NextStep = tr["public_status"].(string), tr["public_note"].(string), tr["next_step"].(string)
	r.store.referrals[0].UpdatedAt, _ = time.Parse(time.RFC3339, tr["updated_at"].(string))
	r.store.referrals[0].history = nil
	for _, e := range tr["history"].([]any) {
		h := obj(e)
		created, _ := time.Parse(time.RFC3339, h["created_at"].(string))
		r.store.referrals[0].history = append(r.store.referrals[0].history, domain.History{ToStatus: h["to_status"].(string), PublicNote: h["public_note"].(string), CreatedAt: created})
	}

	_, detail := r.do(call{method: "GET", path: "/api/v1/hospitals/" + hospital.ID})
	if !reflect.DeepEqual(stripFreshness(detail["data"]), stripFreshness(fx["hospital_detail"]["data"])) {
		t.Errorf("detail != fixture\n got: %v\nwant: %v", detail["data"], fx["hospital_detail"]["data"])
	}
	_, lst := r.do(call{method: "GET", path: "/api/v1/hospitals?dataset=simulation"})
	if !reflect.DeepEqual(stripFreshness(lst["data"]), stripFreshness(fx["hospital_list"]["data"])) {
		t.Errorf("daftar != fixture\n got: %v\nwant: %v", lst["data"], fx["hospital_list"]["data"])
	}
	wantMeta := obj(fx["hospital_list"]["meta"])
	for _, k := range []string{"page", "limit", "total", "total_pages", "data_scope"} {
		if !reflect.DeepEqual(obj(lst["meta"])[k], wantMeta[k]) {
			t.Errorf("meta.%s = %v, fixture %v", k, obj(lst["meta"])[k], wantMeta[k])
		}
	}
	_, look := r.do(call{method: "POST", path: "/api/v1/referrals/lookup", body: `{"tracking_code":"SR-DEMO-001"}`, remote: r.ip()})
	if !reflect.DeepEqual(stripFreshness(look["data"]), stripFreshness(fx["tracking"]["data"])) {
		t.Errorf("tracking != fixture\n got: %v\nwant: %v", look["data"], fx["tracking"]["data"])
	}
}

// Negative control: the validator must actually reject contract drift, otherwise every other test is vacuous.
func TestValidatorRejectsDrift(t *testing.T) {
	s, fx := loadSpec(t), loadFixtures(t)
	schema, _ := s.responseSchema("GET", "/hospitals/{id}", 200)
	mutations := map[string]func(d map[string]any){
		"field hilang":     func(d map[string]any) { delete(d, "verified_at") },
		"field tambahan":   func(d map[string]any) { d["internal_note"] = "x" },
		"enum salah":       func(d map[string]any) { d["freshness"] = "expired" },
		"null terlarang":   func(d map[string]any) { d["name"] = nil },
		"tipe salah":       func(d map[string]any) { d["latitude"] = "-6.2" },
		"waktu bukan UTC":  func(d map[string]any) { d["synced_at"] = "2026-10-23T08:01:00+07:00" },
		"uuid kapital":     func(d map[string]any) { d["id"] = "ABCDEF00-0000-4000-8000-000000000001" },
		"bed negatif":      func(d map[string]any) { obj(d["beds"].([]any)[0])["available"] = json.Number("-1") },
		"layanan tambahan": func(d map[string]any) { obj(d["services"].([]any)[0])["actor_id"] = "x" },
	}
	for name, mutate := range mutations {
		b, _ := json.Marshal(fx["hospital_detail"])
		var copy map[string]any
		if e := decodeJSON(b, &copy); e != nil {
			t.Fatal(e)
		}
		mutate(obj(copy["data"]))
		if len(s.validate(schema, any(copy))) == 0 {
			t.Errorf("validator menerima pelanggaran kontrak: %s", name)
		}
	}
}

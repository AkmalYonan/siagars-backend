package contract

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"siagars/backend/internal/domain"
)

func keys(m map[string]any) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// walkKeys visits every object key in a decoded JSON value.
func walkKeys(v any, visit func(path, key string)) {
	var walk func(v any, path string)
	walk = func(v any, path string) {
		switch x := v.(type) {
		case map[string]any:
			for k, c := range x {
				visit(path, k)
				walk(c, path+"."+k)
			}
		case []any:
			for _, c := range x {
				walk(c, path+"[]")
			}
		}
	}
	walk(v, "$")
}

func TestLookupPublicDTOHasNoInternalFields(t *testing.T) {
	dest := simHospital()
	r := newRig(t, []domain.Hospital{dest})
	r.store.referrals[0].PublicNote = "Skenario publik sintetis"
	w, body := r.do(call{method: "POST", path: "/api/v1/referrals/lookup", body: `{"tracking_code":"SR-DEMO-001"}`, remote: r.ip()})
	if w.Code != 200 {
		t.Fatal(w.Code, body)
	}
	data := obj(body["data"])
	if got, want := keys(data), []string{"destination", "history", "next_step", "public_note", "public_status", "updated_at"}; !reflect.DeepEqual(got, want) {
		t.Errorf("kunci lookup %v, kontrak %v", got, want)
	}
	for _, e := range data["history"].([]any) {
		if got, want := keys(obj(e)), []string{"created_at", "from_status", "public_note", "to_status"}; !reflect.DeepEqual(got, want) {
			t.Errorf("kunci history %v, kontrak %v", got, want)
		}
	}
	// No internal vocabulary anywhere in the payload, and none of the hidden values (owner hospital id, the code itself).
	forbidden := []string{"owner", "actor", "operator", "internal", "token", "hash", "audit", "password", "is_demo", "destination_hospital_id", "tracking_code", "email"}
	walkKeys(body, func(path, key string) {
		for _, f := range forbidden {
			if strings.Contains(strings.ToLower(key), f) {
				t.Errorf("kunci %s.%s terlarang di DTO publik", path, key)
			}
		}
	})
	if raw := w.Body.String(); strings.Contains(raw, "SR-DEMO-001") || strings.Contains(raw, "30000000-0000-4000") {
		t.Errorf("body memuat kode tracking atau ID operator: %s", raw)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control %q", cc)
	}
	if data["destination"] == nil || obj(data["destination"])["id"] != h1 {
		t.Errorf("destination: %v", data["destination"])
	}
}

func TestLookupWithoutDestinationIsNull(t *testing.T) {
	r := newRig(t, []domain.Hospital{simHospital()})
	r.store.referrals[1].code = "SR-DEMO-002"
	w, body := r.do(call{method: "POST", path: "/api/v1/referrals/lookup", body: `{"tracking_code":"SR-DEMO-002"}`, remote: r.ip()})
	if w.Code != 200 || obj(body["data"])["destination"] != nil {
		t.Errorf("%d %v", w.Code, body)
	}
	if h := obj(body["data"])["history"]; h == nil {
		t.Error("history harus [] bukan null")
	}
}

func TestAdminAndAuthDTOsDoNotLeakSecrets(t *testing.T) {
	r := newRig(t, []domain.Hospital{simHospital()})
	for _, c := range []call{
		{method: "GET", path: "/api/v1/auth/me", cookie: tokenA},
		{method: "GET", path: "/api/v1/auth/csrf", cookie: tokenA},
		{method: "GET", path: "/api/v1/admin/referrals", cookie: tokenA},
		mutate("PATCH", statusPath(refA), `{"status":"verified","public_note":"ok","version":1}`, tokenA),
	} {
		w, body := r.do(c)
		if w.Code != 200 {
			t.Fatalf("%s: %d %v", c.path, w.Code, body)
		}
		walkKeys(body, func(path, key string) {
			for _, f := range []string{"password", "hash", "token_hash", "session", "secret", "internal", "actor"} {
				if strings.Contains(key, f) {
					t.Errorf("%s: kunci terlarang %s.%s", c.path, path, key)
				}
			}
		})
		if strings.Contains(w.Body.String(), tokenA) {
			t.Errorf("%s: session token mentah muncul di body", c.path)
		}
	}
}

func TestLoginCookieAndLifecycle(t *testing.T) {
	for _, secure := range []bool{false, true} {
		r := newRig(t, []domain.Hospital{simHospital()})
		r.cfg.SecureCookie = secure
		r.reload()
		login := call{method: "POST", path: "/api/v1/auth/login", body: `{"email":"  Operator-A@Example.test ","password":"` + password + `"}`, origin: origin, remote: r.ip()}
		w, body := r.do(login)
		if w.Code != 200 {
			t.Fatalf("login: %d %v", w.Code, body)
		}
		cookies := w.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("cookies: %v", cookies)
		}
		c := cookies[0]
		if c.Name != "siagars_session" || !c.HttpOnly || c.Secure != secure || c.Path != "/api/v1" || c.Domain != "" || c.SameSite != http.SameSiteLaxMode || len(c.Value) != 64 {
			t.Errorf("cookie %+v (secure=%v)", c, secure)
		}
		if ttl := time.Duration(c.MaxAge) * time.Second; ttl != 8*time.Hour {
			t.Errorf("TTL %v, baseline 8 jam", ttl)
		}
		data := obj(body["data"])
		token := data["csrf_token"].(string)
		if strings.Contains(w.Body.String(), c.Value) {
			t.Error("session token mentah ada di body login")
		}
		// The issued CSRF token authorises mutations; /auth/csrf returns the same value.
		_, again := r.do(call{method: "GET", path: "/api/v1/auth/csrf", cookie: c.Value})
		if obj(again["data"])["csrf_token"] != token {
			t.Errorf("csrf berbeda: %v vs %v", again, token)
		}
		if w, b := r.do(call{method: "PATCH", path: statusPath(refA), body: `{"status":"verified","version":1}`, origin: origin, csrf: token, cookie: c.Value}); w.Code != 200 {
			t.Errorf("mutasi dengan csrf login: %d %v", w.Code, b)
		}
		_, me := r.do(call{method: "GET", path: "/api/v1/auth/me", cookie: c.Value})
		if obj(me["data"])["email"] != "operator-a@example.test" {
			t.Errorf("email dinormalisasi: %v", me)
		}
		// Logout revokes the session and clears the cookie; the old cookie is dead afterwards.
		lw, _ := r.do(call{method: "POST", path: "/api/v1/auth/logout", body: "{}", origin: origin, csrf: token, cookie: c.Value})
		if lw.Code != 200 || len(lw.Result().Cookies()) != 1 || lw.Result().Cookies()[0].MaxAge >= 0 {
			t.Errorf("logout: %d %v", lw.Code, lw.Result().Cookies())
		}
		if w, _ := r.do(call{method: "GET", path: "/api/v1/auth/me", cookie: c.Value}); w.Code != 401 {
			t.Errorf("session setelah logout: %d", w.Code)
		}
		// Login while holding an old cookie rotates it.
		login.remote = r.ip()
		login.cookie = tokenA
		r.do(login)
		if _, alive := r.store.sessions[tokenA]; alive {
			t.Error("session lama harus dicabut saat login baru (rotate)")
		}
	}
}

func TestNullIsNotZero(t *testing.T) {
	up := ago(time.Minute)
	r := newRig(t, []domain.Hospital{
		hosp(1, "Null vs nol", "curated", up, withBeds(
			bed("general", nil, nil, "curated", up),
			bed("icu", ptr(0), ptr(0), "curated", up),
			bed("nicu", nil, ptr(0), "curated", up),
			bed("picu", ptr(8), nil, "curated", up),
		), withServices(svc("nicu", "unknown", "curated", up)), at(-6.2, 106.8)),
		hosp(2, "Tanpa koordinat", "curated", up),
	})
	_, body := r.do(call{method: "GET", path: "/api/v1/hospitals/" + hid(1)})
	d := obj(body["data"])
	want := map[string][2]any{"general": {nil, nil}, "icu": {json.Number("0"), json.Number("0")}, "nicu": {nil, json.Number("0")}, "picu": {json.Number("8"), nil}}
	for _, b := range d["beds"].([]any) {
		bd := obj(b)
		w := want[bd["category"].(string)]
		if !reflect.DeepEqual(bd["total"], w[0]) || !reflect.DeepEqual(bd["available"], w[1]) {
			t.Errorf("bed %v: total=%#v available=%#v, harus %v", bd["category"], bd["total"], bd["available"], w)
		}
	}
	if s := obj(d["services"].([]any)[0]); s["status"] != "unknown" {
		t.Errorf("status layanan unknown harus tetap %q: %v", "unknown", s["status"])
	}
	for _, k := range []string{"address", "phone", "bpjs", "source_url", "verified_at", "distance_km"} {
		if v, present := d[k]; !present || v != nil {
			t.Errorf("%s harus ada dan null, dapat %#v (ada=%v)", k, v, present)
		}
	}
	if v, present := d["latitude"]; !present || v == nil {
		t.Error("latitude RS 1")
	}
	_, none := r.do(call{method: "GET", path: "/api/v1/hospitals/" + hid(2)})
	if obj(none["data"])["latitude"] != nil || obj(none["data"])["longitude"] != nil {
		t.Error("koordinat tak diketahui harus null, bukan 0")
	}
	// Empty collections are [] not null.
	if s := obj(none["data"])["services"]; s == nil || len(s.([]any)) != 0 {
		t.Errorf("services kosong: %#v", s)
	}
	if s := obj(none["data"])["beds"]; s == nil || len(s.([]any)) != 0 {
		t.Errorf("beds kosong: %#v", s)
	}
	// Bed mutation keeps null/0 distinct end to end.
	r2 := newRig(t, []domain.Hospital{simHospital()})
	// Ordered: each update must carry the version returned by the previous one.
	for _, step := range []struct {
		in   string
		want [2]any
	}{
		{`{"total":null,"available":null,"version":1}`, [2]any{nil, nil}},
		{`{"total":null,"available":0,"version":2}`, [2]any{nil, json.Number("0")}},
		{`{"total":0,"available":0,"version":3}`, [2]any{json.Number("0"), json.Number("0")}},
	} {
		in, want := step.in, step.want
		w, resp := r2.do(mutate("PATCH", "/api/v1/admin/hospitals/"+h1+"/beds/icu", in, tokenA))
		if w.Code != 200 || !reflect.DeepEqual(obj(resp["data"])["total"], want[0]) || !reflect.DeepEqual(obj(resp["data"])["available"], want[1]) {
			t.Errorf("%s: %d %v", in, w.Code, resp)
		}
	}
}

func TestFreshnessBoundaries(t *testing.T) {
	for name, c := range map[string]struct {
		age  *time.Duration
		want string
	}{
		"59 menit":                {dur(59 * time.Minute), "fresh"},
		"baru saja":               {dur(0), "fresh"},
		"61 menit":                {dur(61 * time.Minute), "aging"},
		"23 jam 59 menit":         {dur(23*time.Hour + 59*time.Minute), "aging"},
		"24 jam 1 menit":          {dur(24*time.Hour + time.Minute), "stale"},
		"7 hari":                  {dur(7 * 24 * time.Hour), "stale"},
		"di masa depan":           {dur(-time.Hour), "unknown"},
		"tanpa source_updated_at": {nil, "unknown"},
	} {
		up := (*time.Time)(nil)
		if c.age != nil {
			up = ago(*c.age)
		}
		// synced_at is always "now": freshness must follow source_updated_at only.
		r := newRig(t, []domain.Hospital{hosp(1, "RS", "curated", up, withServices(svc("nicu", "available", "curated", up)), withBeds(bed("icu", ptr(1), ptr(1), "curated", up)))})
		_, body := r.do(call{method: "GET", path: "/api/v1/hospitals/" + hid(1)})
		d := obj(body["data"])
		for level, got := range map[string]any{"rs": d["freshness"], "layanan": obj(d["services"].([]any)[0])["freshness"], "bed": obj(d["beds"].([]any)[0])["freshness"]} {
			if got != c.want {
				t.Errorf("%s/%s: freshness %v, harus %s", name, level, got, c.want)
			}
		}
	}
	// Each item carries its own freshness; simulation stays labelled even when fresh; verified_at is never auto-filled.
	up, old := ago(time.Minute), ago(30*time.Hour)
	r := newRig(t, []domain.Hospital{hosp(1, "RS", "simulation", up, withServices(svc("nicu", "available", "simulation", ago(2*time.Hour))), withBeds(bed("icu", ptr(1), ptr(1), "simulation", old)))})
	_, body := r.do(call{method: "GET", path: "/api/v1/hospitals/" + hid(1)})
	d := obj(body["data"])
	if d["freshness"] != "fresh" || d["data_origin"] != "simulation" || obj(d["services"].([]any)[0])["freshness"] != "aging" || obj(d["beds"].([]any)[0])["freshness"] != "stale" || d["verified_at"] != nil {
		t.Errorf("provenance per item: %v", d)
	}
	// meta.as_of is server time (RFC3339 UTC) taken during the request.
	before := time.Now().UTC().Add(-time.Second)
	_, list := r.do(call{method: "GET", path: "/api/v1/hospitals?dataset=all"})
	asOf, e := time.Parse(time.RFC3339, obj(list["meta"])["as_of"].(string))
	if e != nil || asOf.Before(before) || asOf.After(time.Now().UTC().Add(time.Second)) {
		t.Errorf("as_of %v (%v)", asOf, e)
	}
}
func dur(d time.Duration) *time.Duration { return &d }

func TestMutationResponsesCarryNewVersionAndServerTime(t *testing.T) {
	r := newRig(t, []domain.Hospital{simHospital()})
	_, ref := r.do(mutate("PATCH", statusPath(refA), `{"status":"verified","public_note":"Verifikasi simulasi","version":1}`, tokenA))
	if d := obj(ref["data"]); fmt.Sprint(d["version"]) != "2" || d["status"] != "verified" || d["public_note"] != "Verifikasi simulasi" {
		t.Errorf("referral: %v", d)
	}
	_, svcResp := r.do(mutate("PATCH", "/api/v1/admin/hospitals/"+h1+"/services/nicu", `{"status":"limited","version":1}`, tokenA))
	if d := obj(svcResp["data"]); fmt.Sprint(d["version"]) != "2" || d["status"] != "limited" || d["data_origin"] != "simulation" || d["verified_at"] != nil || d["freshness"] != "fresh" {
		t.Errorf("layanan: %v", d)
	}
	_, bedResp := r.do(mutate("PATCH", "/api/v1/admin/hospitals/"+h1+"/beds/icu", `{"total":10,"available":1,"version":1}`, tokenA))
	if d := obj(bedResp["data"]); fmt.Sprint(d["version"]) != "2" || fmt.Sprint(d["available"]) != "1" || d["verified_at"] != nil {
		t.Errorf("bed: %v", d)
	}
	if len(r.store.audit) != 3 {
		t.Errorf("setiap mutasi sukses harus menyertakan request id ke audit: %v", r.store.audit)
	}
	// A status change is immediately visible to the public lookup (the E2E flow, at handler level).
	_, look := r.do(call{method: "POST", path: "/api/v1/referrals/lookup", body: `{"tracking_code":"SR-DEMO-001"}`, remote: r.ip()})
	d := obj(look["data"])
	hist := d["history"].([]any)
	if d["public_status"] != "verified" || len(hist) != 2 || obj(hist[1])["to_status"] != "verified" || obj(hist[1])["from_status"] != "submitted" || obj(hist[0])["from_status"] != nil {
		t.Errorf("lookup setelah update: %v", d)
	}
}

func TestSecretsNeverReachLogs(t *testing.T) {
	r := newRig(t, []domain.Hospital{simHospital()})
	r.do(call{method: "POST", path: "/api/v1/referrals/lookup", body: `{"tracking_code":"SR-DEMO-001"}`, remote: r.ip()})
	r.do(call{method: "POST", path: "/api/v1/auth/login", body: `{"email":"operator-a@example.test","password":"` + password + `"}`, origin: origin, remote: r.ip()})
	r.do(call{method: "GET", path: "/api/v1/auth/me", cookie: tokenA})
	r.do(mutate("PATCH", statusPath(refA), `{"status":"verified","public_note":"catatan-rahasia-uji","version":1}`, tokenA))
	r.store.err = errBoom
	r.do(call{method: "GET", path: "/api/v1/hospitals"})
	out := logged()
	if out == "" {
		t.Fatal("log kosong; uji ini tidak bermakna")
	}
	for _, secret := range []string{"SR-DEMO-001", password, tokenA, csrf, "catatan-rahasia-uji", "secret-pw", "operator-a@example.test"} {
		if strings.Contains(out, secret) {
			t.Errorf("log memuat %q", secret)
		}
	}
}

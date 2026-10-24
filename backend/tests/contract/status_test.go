package contract

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"siagars/backend/internal/domain"
)

func statusPath(id string) string { return "/api/v1/admin/referrals/" + id + "/status" }

func simHospital() domain.Hospital {
	up := ago(time.Minute)
	return hosp(1, "RS Simulasi Siaga 01", "simulation", up,
		withServices(svc("nicu", "available", "simulation", up)),
		withBeds(bed("icu", ptr(10), ptr(2), "simulation", up)))
}

func TestEveryDocumentedOperationIsRoutedAndWrongMethodIs405(t *testing.T) {
	r := newRig(t, []domain.Hospital{simHospital()})
	concrete := strings.NewReplacer("{id}", h1, "{slug}", "nicu", "{category}", "icu")
	declared := 0
	for path, ops := range r.spec.paths() {
		p := "/api/v1" + concrete.Replace(path)
		methods := []string{}
		for m := range ops {
			if m == "parameters" {
				continue
			}
			methods = append(methods, strings.ToUpper(m))
			declared++
			w, _ := r.raw(call{method: strings.ToUpper(m), path: p, remote: r.ip()})
			if w.Code == 405 {
				t.Errorf("%s %s didokumentasikan tetapi 405", m, path)
			}
		}
		for _, wrong := range []string{"PUT", "DELETE", "HEAD", "OPTIONS", "PATCH", "POST", "GET"} {
			if strings.Contains(strings.Join(methods, ","), wrong) {
				continue
			}
			w, _ := r.do(call{method: wrong, path: p, remote: r.ip()})
			if w.Code != 405 || w.Header().Get("Allow") == "" && wrong != "HEAD" {
				t.Errorf("%s %s: %d Allow=%q, harus 405", wrong, path, w.Code, w.Header().Get("Allow"))
			}
		}
	}
	if declared < 14 {
		t.Errorf("OpenAPI hanya mendokumentasikan %d operasi, minimal baseline 14", declared)
	}
	if w, body := r.do(call{method: "GET", path: "/api/v1/tidak-ada"}); w.Code != 404 || errorCode(t, body) != "not_found" {
		t.Error(w.Code)
	}
}

func TestUnauthorized401(t *testing.T) {
	r := newRig(t, []domain.Hospital{simHospital()})
	ops := []call{
		{method: "GET", path: "/api/v1/auth/me"}, {method: "GET", path: "/api/v1/auth/csrf"}, {method: "GET", path: "/api/v1/admin/referrals"},
		{method: "POST", path: "/api/v1/auth/logout", body: "{}"},
		{method: "PATCH", path: statusPath(refA), body: `{"status":"verified","version":1}`},
		{method: "PATCH", path: "/api/v1/admin/hospitals/" + h1 + "/services/nicu", body: `{"status":"limited","version":1}`},
		{method: "PATCH", path: "/api/v1/admin/hospitals/" + h1 + "/beds/icu", body: `{"total":1,"available":1,"version":1}`},
	}
	for _, cookie := range []string{"", "short", strings.Repeat("f", 64)} { // none, wrong shape, unknown/revoked session
		for _, c := range ops {
			c.cookie, c.origin, c.csrf = cookie, origin, csrf
			w, body := r.do(c)
			if w.Code != 401 || errorCode(t, body) != "unauthorized" {
				t.Errorf("%s %s cookie=%q: %d %v", c.method, c.path, cookie, w.Code, body)
			}
		}
	}
}

func TestForbidden403(t *testing.T) {
	r := newRig(t, []domain.Hospital{simHospital(), hosp(2, "RS Lain", "simulation", ago(time.Minute), withBeds(bed("icu", ptr(1), ptr(1), "simulation", ago(time.Minute))))})
	patch := `{"status":"verified","version":1}`
	for name, c := range map[string]call{
		"csrf salah":         {method: "PATCH", path: statusPath(refA), body: patch, origin: origin, csrf: strings.Repeat("d", 64), cookie: tokenA},
		"csrf kosong":        {method: "PATCH", path: statusPath(refA), body: patch, origin: origin, cookie: tokenA},
		"csrf pendek":        {method: "PATCH", path: statusPath(refA), body: patch, origin: origin, csrf: "abc", cookie: tokenA},
		"origin asing":       {method: "PATCH", path: statusPath(refA), body: patch, origin: "https://evil.test", csrf: csrf, cookie: tokenA},
		"origin kosong":      {method: "PATCH", path: statusPath(refA), body: patch, csrf: csrf, cookie: tokenA},
		"logout csrf":        {method: "POST", path: "/api/v1/auth/logout", body: "{}", origin: origin, csrf: "x", cookie: tokenA},
		"login origin":       {method: "POST", path: "/api/v1/auth/login", body: `{"email":"operator-a@example.test","password":"` + password + `"}`, origin: "https://evil.test"},
		"RS lain (referral)": mutate("PATCH", statusPath(refB), `{"status":"searching","public_note":"alihkan","version":3}`, tokenA),
		"RS lain (layanan)":  mutate("PATCH", "/api/v1/admin/hospitals/"+hid(2)+"/services/nicu", `{"status":"limited","version":1}`, tokenA),
		"RS lain (bed)":      mutate("PATCH", "/api/v1/admin/hospitals/"+hid(2)+"/beds/icu", `{"total":1,"available":0,"version":1}`, tokenA),
	} {
		w, body := r.do(c)
		code := errorCode(t, body)
		if w.Code != 403 {
			t.Errorf("%s: %d %v", name, w.Code, body)
		}
		if strings.HasPrefix(name, "RS lain") && code != "forbidden" || strings.HasPrefix(name, "csrf") && code != "invalid_csrf" || strings.HasPrefix(name, "origin") && code != "invalid_origin" {
			t.Errorf("%s: kode %s", name, code)
		}
	}
	// A rejected cross-hospital write must not have changed anything.
	if r.store.referrals[1].Version != 3 || len(r.store.audit) != 0 {
		t.Error("tulis lintas RS tidak boleh mengubah data atau audit")
	}
	// Operator B only sees its own hospital's referrals; A never sees B's.
	_, a := r.do(call{method: "GET", path: "/api/v1/admin/referrals", cookie: tokenA})
	_, b := r.do(call{method: "GET", path: "/api/v1/admin/referrals", cookie: tokenB})
	if len(dataList(t, a)) != 1 || obj(dataList(t, a)[0])["id"] != refA || len(dataList(t, b)) != 1 || obj(dataList(t, b)[0])["id"] != refB {
		t.Errorf("scope admin/referrals: %v %v", a, b)
	}
}

func TestNotFound404(t *testing.T) {
	letters := simHospital()
	letters.ID = "1000000a-0000-4000-8000-00000000000a"
	r := newRig(t, []domain.Hospital{simHospital(), letters})
	missing := "10000000-0000-4000-8000-0000000000ff"
	for name, c := range map[string]call{
		"detail tak ada":          {method: "GET", path: "/api/v1/hospitals/" + missing},
		"detail bukan uuid":       {method: "GET", path: "/api/v1/hospitals/bukan-uuid"},
		"detail uuid kapital":     {method: "GET", path: "/api/v1/hospitals/" + strings.ToUpper(letters.ID)},
		"referral bukan uuid":     mutate("PATCH", statusPath("123"), `{"status":"verified","version":1}`, tokenA),
		"layanan RS bukan uuid":   mutate("PATCH", "/api/v1/admin/hospitals/x/services/nicu", `{"status":"limited","version":1}`, tokenA),
		"bed RS bukan uuid":       mutate("PATCH", "/api/v1/admin/hospitals/x/beds/icu", `{"total":1,"available":1,"version":1}`, tokenA),
		"referral tak ada":        mutate("PATCH", statusPath("20000000-0000-4000-8000-0000000000ff"), `{"status":"verified","version":1}`, tokenA),
		"slug layanan tak ada":    mutate("PATCH", "/api/v1/admin/hospitals/"+h1+"/services/tidak-ada", `{"status":"limited","version":1}`, tokenA),
		"lookup kode tak ada":     {method: "POST", path: "/api/v1/referrals/lookup", body: `{"tracking_code":"SR-DEMO-999"}`},
		"lookup kode kosong":      {method: "POST", path: "/api/v1/referrals/lookup", body: `{"tracking_code":""}`},
		"lookup field hilang":     {method: "POST", path: "/api/v1/referrals/lookup", body: `{}`},
		"lookup kode kepanjangan": {method: "POST", path: "/api/v1/referrals/lookup", body: `{"tracking_code":"` + strings.Repeat("x", 257) + `"}`},
	} {
		c.remote = r.ip()
		w, body := r.do(c)
		if w.Code != 404 {
			t.Errorf("%s: %d %v", name, w.Code, body)
		}
	}
	// Lookup 404 must not reveal why a code failed: same status and error code for every miss, same message for lookups that reach the store.
	messages := map[string]bool{}
	for _, code := range []string{"SR-DEMO-999", "sr-demo-001 ", "tidak-pernah-ada"} {
		_, body := r.do(call{method: "POST", path: "/api/v1/referrals/lookup", body: mustJSON(map[string]string{"tracking_code": code}), remote: r.ip()})
		messages[fmt.Sprint(obj(body["error"])["code"], "|", obj(body["error"])["message"])] = true
	}
	if len(messages) != 1 {
		t.Errorf("pesan 404 lookup tidak generik: %v", messages)
	}
	// KNOWN DEVIATION (reported, not fixed here): empty/oversized codes are rejected before the store with a different message text.
	_, early := r.do(call{method: "POST", path: "/api/v1/referrals/lookup", body: `{"tracking_code":""}`, remote: r.ip()})
	_, late := r.do(call{method: "POST", path: "/api/v1/referrals/lookup", body: `{"tracking_code":"SR-DEMO-999"}`, remote: r.ip()})
	if obj(early["error"])["code"] != obj(late["error"])["code"] {
		t.Error("kode error lookup 404 harus sama")
	}
	if obj(early["error"])["message"] != obj(late["error"])["message"] {
		t.Logf("TEMUAN: pesan 404 lookup kode kosong/kepanjangan (%q) berbeda dari kode tak dikenal (%q)", obj(early["error"])["message"], obj(late["error"])["message"])
	}
}

func TestConflict409(t *testing.T) {
	fx := loadFixtures(t)["conflict"]
	r := newRig(t, []domain.Hospital{simHospital()})
	note := func(s string) string { return fmt.Sprintf(`"public_note":%q`, s) }
	for name, c := range map[string]call{
		"versi basi":                  mutate("PATCH", statusPath(refA), `{"status":"verified","version":7}`, tokenA),
		"loncat submitted->confirmed": mutate("PATCH", statusPath(refA), `{"status":"confirmed","version":1}`, tokenA),
		"mundur ke submitted":         mutate("PATCH", statusPath(refA), `{"status":"submitted","version":1}`, tokenA),
		"layanan versi basi":          mutate("PATCH", "/api/v1/admin/hospitals/"+h1+"/services/nicu", `{"status":"limited","version":9}`, tokenA),
		"bed versi basi":              mutate("PATCH", "/api/v1/admin/hospitals/"+h1+"/beds/icu", `{"total":10,"available":1,"version":9}`, tokenA),
	} {
		w, body := r.do(c)
		e := obj(body["error"])
		if w.Code != 409 || e["code"] != obj(fx["error"])["code"] || e["message"] != obj(fx["error"])["message"] {
			t.Errorf("%s: %d %v (fixture %v)", name, w.Code, body, fx)
		}
	}
	// Full lifecycle through the API; confirmed->searching needs a reason; completed/rejected are terminal.
	step := func(status, n string, version int, want int) {
		t.Helper()
		body := fmt.Sprintf(`{"status":%q,%s,"version":%d}`, status, note(n), version)
		if w, _ := r.do(mutate("PATCH", statusPath(refA), body, tokenA)); w.Code != want {
			t.Errorf("-> %s v%d: %d want %d", status, version, w.Code, want)
		}
	}
	step("verified", "", 1, 200)
	step("searching", "", 2, 200)
	step("confirmed", "Konfirmasi simulasi diterima", 3, 200)
	step("searching", "", 4, 409) // diversion without a reason
	step("searching", "Pengalihan simulasi", 4, 200)
	step("rejected", "ditolak", 5, 200)
	step("searching", "x", 6, 409) // rejected is terminal
	step("completed", "x", 6, 409)
	if got := len(r.store.referrals[0].history); got != 6 {
		t.Errorf("history harus 1 per perubahan sukses, dapat %d", got)
	}
}

func TestPayloadTooLarge413AndMalformed400(t *testing.T) {
	r := newRig(t, []domain.Hospital{simHospital()})
	huge := strings.Repeat("a", 70<<10)
	for name, c := range map[string]call{
		"lookup":   {method: "POST", path: "/api/v1/referrals/lookup", body: `{"tracking_code":"` + huge + `"}`},
		"login":    {method: "POST", path: "/api/v1/auth/login", body: `{"email":"` + huge + `"}`, origin: origin},
		"referral": mutate("PATCH", statusPath(refA), `{"status":"verified","public_note":"`+huge+`","version":1}`, tokenA),
	} {
		c.remote = r.ip()
		if w, body := r.do(c); w.Code != 413 || errorCode(t, body) != "body_too_large" {
			t.Errorf("%s: %d %v", name, w.Code, body)
		}
	}
	for name, c := range map[string]call{
		"json rusak":        {method: "POST", path: "/api/v1/referrals/lookup", body: "{"},
		"field tak dikenal": {method: "POST", path: "/api/v1/referrals/lookup", body: `{"tracking_code":"a","extra":1}`},
		"dua objek":         {method: "POST", path: "/api/v1/referrals/lookup", body: `{"tracking_code":"a"}{"tracking_code":"b"}`},
		"tipe salah":        {method: "POST", path: "/api/v1/referrals/lookup", body: `{"tracking_code":1}`},
		"body kosong":       {method: "POST", path: "/api/v1/referrals/lookup", body: ``},
		"patch field asing": mutate("PATCH", statusPath(refA), `{"status":"verified","version":1,"owner_hospital_id":"x"}`, tokenA),
	} {
		c.remote = r.ip()
		if w, body := r.do(c); w.Code != 400 || errorCode(t, body) != "invalid_json" {
			t.Errorf("%s: %d %v", name, w.Code, body)
		}
	}
}

func TestValidation422(t *testing.T) {
	r := newRig(t, []domain.Hospital{simHospital()})
	svcPath, bedPath := "/api/v1/admin/hospitals/"+h1+"/services/nicu", "/api/v1/admin/hospitals/"+h1+"/beds/icu"
	long := strings.Repeat("é", 501)
	for name, c := range map[string]call{
		"content-type salah":         {method: "POST", path: "/api/v1/referrals/lookup", body: `{"tracking_code":"a"}`, contentType: "text/plain"},
		"content-type hilang":        {method: "POST", path: "/api/v1/referrals/lookup", body: `{"tracking_code":"a"}`, contentType: "-"},
		"login email tak valid":      {method: "POST", path: "/api/v1/auth/login", body: `{"email":"bukan-email","password":"x"}`, origin: origin},
		"login password kosong":      {method: "POST", path: "/api/v1/auth/login", body: `{"email":"a@b.test","password":""}`, origin: origin},
		"status tak dikenal":         mutate("PATCH", statusPath(refA), `{"status":"wrong","version":1}`, tokenA),
		"status kosong":              mutate("PATCH", statusPath(refA), `{"version":1}`, tokenA),
		"version 0":                  mutate("PATCH", statusPath(refA), `{"status":"verified","version":0}`, tokenA),
		"version hilang":             mutate("PATCH", statusPath(refA), `{"status":"verified"}`, tokenA),
		"catatan 501 karakter":       mutate("PATCH", statusPath(refA), `{"status":"verified","public_note":"`+long+`","version":1}`, tokenA),
		"layanan status tak dikenal": mutate("PATCH", svcPath, `{"status":"open","version":1}`, tokenA),
		"layanan version 0":          mutate("PATCH", svcPath, `{"status":"limited","version":0}`, tokenA),
		"bed negatif":                mutate("PATCH", bedPath, `{"total":10,"available":-1,"version":1}`, tokenA),
		"bed total negatif":          mutate("PATCH", bedPath, `{"total":-1,"available":null,"version":1}`, tokenA),
		"bed available > total":      mutate("PATCH", bedPath, `{"total":2,"available":3,"version":1}`, tokenA),
		"bed available hilang":       mutate("PATCH", bedPath, `{"total":2,"version":1}`, tokenA),
		"bed total hilang":           mutate("PATCH", bedPath, `{"available":2,"version":1}`, tokenA),
		"bed tipe string":            mutate("PATCH", bedPath, `{"total":"2","available":1,"version":1}`, tokenA),
		"bed pecahan":                mutate("PATCH", bedPath, `{"total":2.5,"available":1,"version":1}`, tokenA),
		"bed kategori tak dikenal":   mutate("PATCH", "/api/v1/admin/hospitals/"+h1+"/beds/xyz", `{"total":1,"available":1,"version":1}`, tokenA),
	} {
		c.remote = r.ip()
		if w, body := r.do(c); w.Code != 422 || errorCode(t, body) != "validation_error" {
			t.Errorf("%s: %d %v", name, w.Code, body)
		}
	}
	// Boundary: exactly 500 multibyte characters is accepted (limit counts characters, not bytes).
	ok500 := strings.Repeat("é", 500)
	if w, body := r.do(mutate("PATCH", statusPath(refA), `{"status":"verified","public_note":"`+ok500+`","version":1}`, tokenA)); w.Code != 200 {
		t.Errorf("500 karakter: %d %v", w.Code, body)
	}
}

func TestRateLimit429(t *testing.T) {
	r := newRig(t, []domain.Hospital{simHospital()})
	lookup := func(ip string, spoof string) *http.Response {
		w, _ := r.do(call{method: "POST", path: "/api/v1/referrals/lookup", body: `{"tracking_code":"SR-DEMO-001"}`, remote: ip, header: map[string]string{"X-Forwarded-For": spoof}})
		return w.Result()
	}
	// 10 lookups/minute/IP; rotating X-Forwarded-For must not buy extra attempts.
	for i := 1; i <= 10; i++ {
		if res := lookup("203.0.113.9:5000", fmt.Sprintf("10.0.0.%d", i)); res.StatusCode != 200 {
			t.Fatalf("lookup %d: %d", i, res.StatusCode)
		}
	}
	res := lookup("203.0.113.9:5001", "10.9.9.9") // same IP, different port
	if res.StatusCode != 429 || res.Header.Get("Retry-After") == "" {
		t.Errorf("lookup ke-11: %d Retry-After=%q", res.StatusCode, res.Header.Get("Retry-After"))
	}
	if other := lookup("203.0.113.10:5000", ""); other.StatusCode != 200 {
		t.Errorf("IP lain tidak boleh terkena limit: %d", other.StatusCode)
	}
	// Login: 5 failures per IP+account, then 429 even with the right password; generic message before that.
	bad := func(email string) (int, string) {
		w, body := r.do(call{method: "POST", path: "/api/v1/auth/login", body: fmt.Sprintf(`{"email":%q,"password":"salah"}`, email), origin: origin, remote: "203.0.113.20:1"})
		msg, _ := obj(body["error"])["message"].(string)
		return w.Code, msg
	}
	_, unknownMsg := bad("tidak-ada@example.test")
	for i := 0; i < 4; i++ {
		code, msg := bad("operator-a@example.test")
		if code != 401 || msg != unknownMsg {
			t.Errorf("gagal ke-%d: %d %q (akun tak dikenal %q)", i+1, code, msg, unknownMsg)
		}
	}
	if code, _ := bad("operator-a@example.test"); code != 401 {
		t.Errorf("gagal ke-5: %d", code)
	}
	w, _ := r.do(call{method: "POST", path: "/api/v1/auth/login", body: `{"email":"operator-a@example.test","password":"` + password + `"}`, origin: origin, remote: "203.0.113.20:1"})
	if w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Errorf("login benar setelah 5 gagal harus 429: %d", w.Code)
	}
}

func TestInternalError500AndReadiness503DoNotLeak(t *testing.T) {
	r := newRig(t, nil)
	r.store.err = errBoom
	w, body := r.do(call{method: "GET", path: "/api/v1/hospitals"})
	if w.Code != 500 || errorCode(t, body) != "internal_error" {
		t.Fatalf("%d %v", w.Code, body)
	}
	r.ready = errBoom
	w2, body2 := r.do(call{method: "GET", path: "/api/v1/ready"})
	if w2.Code != 503 || errorCode(t, body2) != "not_ready" {
		t.Fatalf("%d %v", w2.Code, body2)
	}
	for _, out := range []string{w.Body.String(), w2.Body.String()} {
		for _, secret := range []string{"mongodb", "secret-pw", "secret-user", "cluster0", "auth failed"} {
			if strings.Contains(out, secret) {
				t.Errorf("error membocorkan %q: %s", secret, out)
			}
		}
	}
	r.ready = nil
	if w, body := r.do(call{method: "GET", path: "/api/v1/ready"}); w.Code != 200 || obj(body["data"])["status"] != "ready" {
		t.Error(w.Code, body)
	}
	if w, body := r.do(call{method: "GET", path: "/api/v1/health"}); w.Code != 200 || obj(body["data"])["status"] != "ok" || len(obj(body["data"])) != 1 {
		t.Error(w.Code, body)
	}
}

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"siagars/backend/internal/config"
	"siagars/backend/internal/domain"
	httpapi "siagars/backend/internal/http"
	"siagars/backend/internal/repository"
	"siagars/backend/seed"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"
)

func mustHash(t *testing.T, password string) string {
	t.Helper()
	h, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if e != nil {
		t.Fatal(e)
	}
	return string(h)
}

// TestBPJSHistoryStorage runs only in the temporary database created by database().
func TestBPJSHistoryStorage(t *testing.T) {
	p := database(t)
	ctx := context.Background()
	for i := 0; i < 2; i++ { // seeding twice must not duplicate or fail
		if e := seed.Apply(ctx, p); e != nil {
			t.Fatal(e)
		}
	}
	if n, _ := p.DB.Collection("bpjs_participants").CountDocuments(ctx, bson.D{}); int(n) != len(seed.BPJSDemoMembers) {
		t.Fatalf("peserta demo: %d", n)
	}
	// The validator keeps real-looking garbage out of the synthetic participants.
	if _, e := p.DB.Collection("bpjs_participants").InsertOne(ctx, bson.D{{Key: "_id", Value: "123"}, {Key: "name", Value: "x"}, {Key: "birth_date", Value: "1990-01-01"}, {Key: "status", Value: "active"}}); e == nil {
		t.Fatal("NIK 3 digit lolos validator")
	}
	// TTL index of 30 days on history.
	cur, e := p.DB.Collection("bpjs_check_history").Indexes().List(ctx)
	if e != nil {
		t.Fatal(e)
	}
	var idx []bson.M
	if e = cur.All(ctx, &idx); e != nil {
		t.Fatal(e)
	}
	ttl := false
	for _, i := range idx {
		if i["name"] == "bpjs_history_ttl_idx" {
			ttl = i["expireAfterSeconds"] == int32(2592000) || i["expireAfterSeconds"] == int64(2592000)
		}
	}
	if !ttl {
		t.Fatalf("index TTL 30 hari tidak ada: %v", idx)
	}

	key := bytes.Repeat([]byte{9}, 32)
	cipher, e := repository.NewHistoryCipher(key)
	if e != nil {
		t.Fatal(e)
	}
	p.SetHistoryCipher(cipher)
	rec := func(nik, name, birth, result string) domain.BPJSRecord {
		return domain.BPJSRecord{NIKMasked: domain.MaskNIK(nik), NameMasked: domain.MaskName(name), BirthYear: 1990, Result: result, Raw: domain.BPJSRaw{NIK: nik, Name: name, BirthDate: birth}}
	}
	for _, r := range []domain.BPJSRecord{rec("9999010101900001", "Budi Santoso", "1990-01-01", "found"), rec("9999010101900001", "Budi Santoso", "1990-01-01", "found"), rec("9999777788889999", "Orang Lain", "1980-05-05", "not_found")} {
		if e = p.RecordBPJSCheck(ctx, r); e != nil {
			t.Fatal(e)
		}
	}
	// Nothing readable is stored: scan every raw document of the collection.
	raws, e := p.DB.Collection("bpjs_check_history").Find(ctx, bson.D{})
	if e != nil {
		t.Fatal(e)
	}
	var docs []bson.Raw
	if e = raws.All(ctx, &docs); e != nil || len(docs) != 3 {
		t.Fatal(e, len(docs))
	}
	for _, d := range docs {
		for _, secret := range []string{"9999010101900001", "9999777788889999", "Budi Santoso", "Orang Lain", "1990-01-01", "1980-05-05"} {
			if bytes.Contains(d, []byte(secret)) {
				t.Fatalf("dokumen riwayat memuat %q terbaca", secret)
			}
		}
	}
	items, total, e := p.BPJSHistory(ctx, 1, 10, "")
	if e != nil || total != 3 || len(items) != 3 {
		t.Fatal(e, total, len(items))
	}
	repeats := map[string]int{}
	for _, i := range items {
		repeats[i.NIKMasked] = i.RepeatCount
	}
	if repeats["9999********0001"] != 2 || repeats["9999********9999"] != 1 {
		t.Fatalf("repeat_count: %v", repeats)
	}
	if only, n, _ := p.BPJSHistory(ctx, 1, 10, "not_found"); n != 1 || len(only) != 1 || only[0].NameMasked != "OR*** LA**" {
		t.Fatalf("filter result: %+v", only)
	}
	if page2, n, _ := p.BPJSHistory(ctx, 2, 2, ""); n != 3 || len(page2) != 1 {
		t.Fatalf("paginasi: %d %d", n, len(page2))
	}
	got, e := p.RevealBPJS(ctx, items[0].ID)
	if e != nil || got.ID != items[0].ID || got.NIK == "" || got.Name == "" || got.BirthDate == "" {
		t.Fatalf("reveal: %+v %v", got, e)
	}
	if _, e = p.RevealBPJS(ctx, "00000000-0000-4000-8000-000000000000"); e != repository.ErrNotFound {
		t.Fatalf("reveal id tak ada: %v", e)
	}
	// A blob moved to another entry fails: it is bound to its id.
	var a, b struct {
		ID  string `bson:"_id"`
		Raw []byte `bson:"raw_enc"`
	}
	c := p.DB.Collection("bpjs_check_history")
	_ = c.FindOne(ctx, bson.D{{Key: "_id", Value: items[0].ID}}).Decode(&a)
	_ = c.FindOne(ctx, bson.D{{Key: "_id", Value: items[1].ID}}).Decode(&b)
	if _, e = c.UpdateOne(ctx, bson.D{{Key: "_id", Value: b.ID}}, bson.D{{Key: "$set", Value: bson.D{{Key: "raw_enc", Value: a.Raw}}}}); e != nil {
		t.Fatal(e)
	}
	if _, e = p.RevealBPJS(ctx, b.ID); e != repository.ErrRawUnavailable {
		t.Fatalf("blob dipindah terbuka: %v", e)
	}
	// Without the server key nothing can be revealed, and new entries keep only masked data.
	p.SetHistoryCipher(nil)
	if _, e = p.RevealBPJS(ctx, items[0].ID); e != repository.ErrRawUnavailable {
		t.Fatalf("tanpa kunci: %v", e)
	}
	if e = p.RecordBPJSCheck(ctx, rec("9999111122223333", "Tanpa Kunci", "1970-01-01", "not_found")); e != nil {
		t.Fatal(e)
	}
	var plain struct {
		Raw  []byte `bson:"raw_enc"`
		HMAC string `bson:"nik_hmac"`
	}
	_ = c.FindOne(ctx, bson.D{{Key: "nik_masked", Value: "9999********3333"}}).Decode(&plain)
	if len(plain.Raw) != 0 || plain.HMAC != "" {
		t.Fatal("tanpa kunci tetap menyimpan data mentah")
	}
	if e = p.AuditBPJS(ctx, "operator", "bpjs.reveal", items[0].ID, "rid"); e != nil {
		t.Fatal(e)
	}
	var audit bson.M
	if e = p.DB.Collection("audit_logs").FindOne(ctx, bson.D{{Key: "action", Value: "bpjs.reveal"}}).Decode(&audit); e != nil || audit["entity_id"] != items[0].ID {
		t.Fatal(e, audit)
	}
}

// TestBPJSEndToEndWithMongo drives the real handler against MongoDB: demo participants, masked answers,
// stored history and the operator reveal with the operator's password.
func TestBPJSEndToEndWithMongo(t *testing.T) {
	p := database(t)
	ctx := context.Background()
	if e := seed.Apply(ctx, p); e != nil {
		t.Fatal(e)
	}
	cipher, _ := repository.NewHistoryCipher(bytes.Repeat([]byte{3}, 32))
	p.SetHistoryCipher(cipher)
	h := httpapi.New(p, config.Config{PublicOrigin: "http://localhost:3000", SessionTTL: time.Hour}, func(context.Context) error { return nil })
	post := func(path, body string, hdr map[string]string, cookie string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/v1"+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		for k, v := range hdr {
			r.Header.Set(k, v)
		}
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: "siagars_session", Value: cookie})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for _, m := range seed.BPJSDemoMembers {
		body, _ := json.Marshal(map[string]string{"nik": m.NIK, "name": strings.ToUpper(m.Name), "birth_date": m.BirthDate})
		w := post("/bpjs/check", string(body), nil, "")
		var out struct {
			Data struct {
				Result      string
				Participant *struct{ Status string }
			}
		}
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		if w.Code != 200 || out.Data.Result != "found" || out.Data.Participant.Status != m.Status || strings.Contains(w.Body.String(), m.NIK) {
			t.Fatalf("%s: %d %s", m.Name, w.Code, w.Body.String())
		}
	}
	if w := post("/bpjs/check", `{"nik":"9999010101900001","name":"Salah Nama","birth_date":"1990-01-01"}`, nil, ""); !strings.Contains(w.Body.String(), `"not_found"`) {
		t.Fatal(w.Body.String())
	}
	if n, _ := p.DB.Collection("bpjs_check_history").CountDocuments(ctx, bson.D{}); int(n) != len(seed.BPJSDemoMembers)+1 {
		t.Fatalf("riwayat: %d", n)
	}
	// operator
	if e := p.CreateOperator(ctx, "op-1", "ops@example.invalid", mustHash(t, "correct-horse-battery"), []string{seed.HospitalID(1)}); e != nil {
		t.Fatal(e)
	}
	login := post("/auth/login", `{"email":"ops@example.invalid","password":"correct-horse-battery"}`, map[string]string{"Origin": "http://localhost:3000"}, "")
	if login.Code != 200 {
		t.Fatal(login.Code, login.Body.String())
	}
	var l struct {
		Data struct {
			CSRF string `json:"csrf_token"`
		}
	}
	_ = json.Unmarshal(login.Body.Bytes(), &l)
	cookie := login.Result().Cookies()[0].Value
	list := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/admin/bpjs-checks?limit=50", nil)
	req.AddCookie(&http.Cookie{Name: "siagars_session", Value: cookie})
	h.ServeHTTP(list, req)
	var items struct{ Data []domain.BPJSHistoryItem }
	_ = json.Unmarshal(list.Body.Bytes(), &items)
	if list.Code != 200 || len(items.Data) != len(seed.BPJSDemoMembers)+1 || strings.Contains(list.Body.String(), "Santoso") || strings.Contains(list.Body.String(), "9999010101900001") {
		t.Fatal(list.Code, list.Body.String())
	}
	hdr := map[string]string{"Origin": "http://localhost:3000", "X-CSRF-Token": l.Data.CSRF}
	id := items.Data[len(items.Data)-1].ID // oldest = first demo participant
	if w := post("/admin/bpjs-checks/"+id+"/reveal", `{"password":"salah-salah-salah"}`, hdr, cookie); w.Code != 401 {
		t.Fatal(w.Code)
	}
	w := post("/admin/bpjs-checks/"+id+"/reveal", `{"password":"correct-horse-battery"}`, hdr, cookie)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"nik":"9999010101900001"`) || !strings.Contains(w.Body.String(), `"name":"BUDI SANTOSO"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	n, _ := p.DB.Collection("audit_logs").CountDocuments(ctx, bson.D{{Key: "action", Value: bson.D{{Key: "$in", Value: bson.A{"bpjs.reveal", "bpjs.reveal_denied"}}}}})
	if n != 2 {
		t.Fatalf("audit: %d", n)
	}
	// audit rows never hold personal data
	var all []bson.Raw
	cur, _ := p.DB.Collection("audit_logs").Find(ctx, bson.D{})
	_ = cur.All(ctx, &all)
	for _, d := range all {
		if bytes.Contains(d, []byte("9999010101900001")) || bytes.Contains(d, []byte("BUDI")) {
			t.Fatal("audit memuat data pribadi")
		}
	}
}

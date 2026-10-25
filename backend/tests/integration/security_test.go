package integration

import (
	"context"
	"errors"
	"siagars/backend/internal/domain"
	"siagars/backend/internal/repository"
	"siagars/backend/internal/service"
	"siagars/backend/seed"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Operator and session rules enforced by the repository: inactive operators, independent and rotated
// sessions, expiry, and an empty hospital scope.
func TestOperatorSessionSecurity(t *testing.T) {
	p := database(t)
	ctx := context.Background()
	if e := seed.Apply(ctx, p); e != nil {
		t.Fatal(e)
	}
	const op = "30000000-0000-4000-8000-0000000000a1"
	if e := p.CreateOperator(ctx, op, "sec@example.invalid", "test-hash", []string{seed.HospitalID(1)}); e != nil {
		t.Fatal(e)
	}
	soon := time.Now().Add(time.Hour)
	for _, s := range [][2]string{{"token-a", "csrf-a"}, {"token-b", "csrf-b"}} {
		if e := p.CreateSession(ctx, op, s[0], s[1], soon, ""); e != nil {
			t.Fatal(e)
		}
	}
	// Only a hash of the token and of the CSRF token is stored.
	var raw bson.M
	if e := p.DB.Collection("sessions").FindOne(ctx, bson.D{{Key: "_id", Value: repository.Hash("token-a")}}).Decode(&raw); e != nil || raw["csrf_hash"] != repository.Hash("csrf-a") {
		t.Fatal("hash sesi", e, raw)
	}
	if n, e := p.DB.Collection("sessions").CountDocuments(ctx, bson.D{{Key: "_id", Value: "token-a"}}); e != nil || n != 0 {
		t.Fatal("token mentah tersimpan", n, e)
	}
	if got, csrf, e := p.Session(ctx, "token-a"); e != nil || got.ID != op || csrf != repository.Hash("csrf-a") || len(got.HospitalIDs) != 1 {
		t.Fatal(got, e)
	}
	// Revoking one session leaves the other alone.
	if e := p.Revoke(ctx, "token-a"); e != nil {
		t.Fatal(e)
	}
	if _, _, e := p.Session(ctx, "token-a"); !errors.Is(e, repository.ErrNotFound) {
		t.Fatal("sesi dicabut harus ditolak", e)
	}
	if _, _, e := p.Session(ctx, "token-b"); e != nil {
		t.Fatal("sesi lain harus hidup", e)
	}
	// Login with an old cookie revokes it in the same transaction as the new session is created.
	if e := p.CreateSession(ctx, op, "token-c", "csrf-c", soon, "token-b"); e != nil {
		t.Fatal(e)
	}
	if _, _, e := p.Session(ctx, "token-b"); !errors.Is(e, repository.ErrNotFound) {
		t.Fatal("cookie lama harus dicabut saat rotasi", e)
	}
	if _, _, e := p.Session(ctx, "token-c"); e != nil {
		t.Fatal(e)
	}
	// An expired session is invalid even before the TTL index removes it.
	if e := p.CreateSession(ctx, op, "token-old", "csrf-old", time.Now().Add(-time.Minute), ""); e != nil {
		t.Fatal(e)
	}
	if _, _, e := p.Session(ctx, "token-old"); !errors.Is(e, repository.ErrNotFound) {
		t.Fatal("sesi kedaluwarsa", e)
	}
	// Deactivating the operator ends credential lookup and every live session at once.
	if _, _, e := p.Credentials(ctx, "sec@example.invalid"); e != nil {
		t.Fatal(e)
	}
	if _, e := p.DB.Collection("operators").UpdateOne(ctx, bson.D{{Key: "_id", Value: op}}, bson.D{{Key: "$set", Value: bson.D{{Key: "active", Value: false}}}}); e != nil {
		t.Fatal(e)
	}
	if _, _, e := p.Credentials(ctx, "sec@example.invalid"); !errors.Is(e, repository.ErrNotFound) {
		t.Fatal("operator nonaktif tidak boleh login", e)
	}
	if _, _, e := p.Session(ctx, "token-c"); !errors.Is(e, repository.ErrNotFound) {
		t.Fatal("sesi operator nonaktif", e)
	}
	// Unknown e-mail is indistinguishable from an inactive operator at this layer.
	if _, _, e := p.Credentials(ctx, "nobody@example.invalid"); !errors.Is(e, repository.ErrNotFound) {
		t.Fatal(e)
	}
}

func TestEmptyScopeOperatorIsForbiddenEverywhere(t *testing.T) {
	p := database(t)
	ctx := context.Background()
	if e := seed.Apply(ctx, p); e != nil {
		t.Fatal(e)
	}
	const op = "30000000-0000-4000-8000-0000000000a2"
	if _, e := p.DB.Collection("operators").InsertOne(ctx, bson.D{{Key: "_id", Value: op}, {Key: "email", Value: "empty@example.invalid"}, {Key: "password_hash", Value: "x"}, {Key: "active", Value: true}, {Key: "hospital_ids", Value: bson.A{}}}); e != nil {
		t.Fatal(e)
	}
	status, zero := "limited", 1
	if e := p.UpdateFacility(ctx, op, seed.HospitalID(1), "icu", 1, &status, nil, nil, "rid"); !errors.Is(e, repository.ErrForbidden) {
		t.Fatal("layanan", e)
	}
	if e := p.UpdateFacility(ctx, op, seed.HospitalID(1), "icu", 1, nil, &zero, &zero, "rid"); !errors.Is(e, repository.ErrForbidden) {
		t.Fatal("bed", e)
	}
	if _, e := p.UpdateReferral(ctx, op, seed.ReferralID, "verified", "x", 1, "rid"); !errors.Is(e, repository.ErrForbidden) {
		t.Fatal("rujukan", e)
	}
	if refs, e := p.Referrals(ctx, op); e != nil || len(refs) != 0 {
		t.Fatal("baca rujukan", refs, e)
	}
	if n, e := p.DB.Collection("audit_logs").CountDocuments(ctx, bson.D{{Key: "actor_id", Value: op}}); e != nil || n != 0 {
		t.Fatal("penolakan tidak boleh menulis audit", n, e)
	}
}

// Lookup of a hospital by its upstream code, the regions listing and name suggestions against real MongoDB.
func TestExternalLookupRegionsAndSuggest(t *testing.T) {
	p := database(t)
	ctx := context.Background()
	if e := seed.Apply(ctx, p); e != nil {
		t.Fatal(e)
	}
	at := time.Now().UTC().Truncate(time.Millisecond)
	ok := true
	items := []domain.HospitalImport{
		{ExternalID: "ext-1", SourceNamespace: "satusehat:msi:sandbox", Name: "RS Contoh Satu", ProvinceCode: "31", ProvinceName: "DKI Jakarta", CityCode: "3171", CityName: "Kota Jakarta Pusat", SourceActive: true, FacilityClass: "B", FacilityClassName: "Kelas B", FacilitySubtype: "1041", FacilitySubtypeName: "RSU", Operational: &ok, SyncedAt: at},
		{ExternalID: "ext-2", SourceNamespace: "satusehat:msi:sandbox", Name: "RS Nonaktif", ProvinceCode: "31", ProvinceName: "DKI Jakarta", CityCode: "3171", CityName: "Kota Jakarta Pusat", SourceActive: false, SyncedAt: at},
		{ExternalID: "$ne", SourceNamespace: "satusehat:msi:sandbox", Name: "RS $where Dollar", ProvinceCode: "31", ProvinceName: "DKI Jakarta", CityCode: "3171", CityName: "Kota Jakarta Pusat", SourceActive: true, SyncedAt: at},
	}
	if r, e := p.UpsertSATUSEHATHospitals(ctx, items); e != nil || r.Created != 3 {
		t.Fatal(r, e)
	}
	h, e := p.HospitalByExternalID(ctx, "ext-1")
	if e != nil || h.Name != "RS Contoh Satu" || h.ExternalID == nil || *h.ExternalID != "ext-1" || h.FacilityClass == nil || h.FacilityClass.Code != "B" || h.FacilityClass.Name == nil || *h.FacilityClass.Name != "Kelas B" || h.FacilitySubtype == nil || h.Operational == nil || !*h.Operational {
		t.Fatalf("lookup kode: %+v %v", h, e)
	}
	h.Refresh(at)
	if h.Summary.ICU != "unknown" || h.Summary.BPJS != nil || h.Summary.InpatientBeds.Total != nil {
		t.Fatalf("RS impor harus unknown: %+v", h.Summary)
	}
	if _, e := p.HospitalByExternalID(ctx, "ext-2"); !errors.Is(e, repository.ErrNotFound) {
		t.Fatal("RS nonaktif harus tersembunyi", e)
	}
	// The code is a string value, never an operator: "$ne" matches only the hospital whose code is literally "$ne".
	if h, e := p.HospitalByExternalID(ctx, "$ne"); e != nil || h.Name != "RS $where Dollar" {
		t.Fatal(h, e)
	}
	if _, e := p.HospitalByExternalID(ctx, "missing"); !errors.Is(e, repository.ErrNotFound) {
		t.Fatal(e)
	}

	regions, e := p.Regions(ctx)
	if e != nil {
		t.Fatal(e)
	}
	codes := map[string]string{}
	for _, r := range regions {
		codes[r.Code] = r.Kind
	}
	if codes["31"] != "province" || codes["31.71"] != "city" || codes["3171"] != "city" {
		t.Fatalf("kode seed (31.71) dan MSI (3171) harus sama-sama ada apa adanya: %v", codes)
	}

	real, e := p.SuggestHospitals(ctx, "contoh", "real", 10)
	if e != nil || len(real) != 1 || real[0].Name != "RS Contoh Satu" || real[0].ExternalID == nil {
		t.Fatal(real, e)
	}
	if sim, e := p.SuggestHospitals(ctx, "SIMULASI siaga 0", "real", 10); e != nil || len(sim) != 0 {
		t.Fatal("dataset real tidak memuat simulasi", sim, e)
	}
	if sim, e := p.SuggestHospitals(ctx, "SIMULASI siaga 0", "simulation", 3); e != nil || len(sim) != 3 || sim[0].Name > sim[1].Name {
		t.Fatal(sim, e)
	}
	if dollar, e := p.SuggestHospitals(ctx, "$where", "all", 10); e != nil || len(dollar) != 1 || dollar[0].Name != "RS $where Dollar" {
		t.Fatal("q berawalan $ adalah teks biasa", dollar, e)
	}
	if none, e := p.SuggestHospitals(ctx, "$name", "all", 10); e != nil || len(none) != 0 {
		t.Fatal("q=$name tidak boleh menjadi path field", none, e)
	}
	// Search `q` shares the same literal handling.
	if res, _, e := p.SearchHospitals(ctx, service.Search{Q: "$name", Dataset: "all", Page: 1, Limit: 20, Sort: "name"}, at); e != nil || len(res) != 0 {
		t.Fatal("search q=$name", res, e)
	}
}

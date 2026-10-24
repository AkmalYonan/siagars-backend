package contract

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"siagars/backend/internal/domain"
)

func ptr[T any](v T) *T { return &v }

func hid(n int) string { return fmt.Sprintf("10000000-0000-4000-8000-%012d", n) }

func prov(origin string, source *time.Time) domain.Provenance {
	synced := time.Now().UTC()
	return domain.Provenance{DataOrigin: origin, SourceUpdatedAt: source, SyncedAt: &synced}
}
func svc(slug, status, origin string, source *time.Time) domain.Service {
	names := map[string]string{"icu": "ICU", "nicu": "NICU", "igd": "IGD"}
	return domain.Service{Slug: slug, Name: names[slug], Status: status, Provenance: prov(origin, source), Version: 1}
}
func bed(category string, total, available *int, origin string, source *time.Time) domain.Bed {
	return domain.Bed{Category: category, Total: total, Available: available, Provenance: prov(origin, source), Version: 1}
}
func hosp(n int, name, origin string, source *time.Time, mods ...func(*domain.Hospital)) domain.Hospital {
	h := domain.Hospital{ID: hid(n), Name: name, CityCode: "31.71", CityName: "Jakarta Selatan", ProvinceCode: "31", ProvinceName: "DKI Jakarta",
		Provenance: prov(origin, source), Services: []domain.Service{}, Beds: []domain.Bed{}}
	for _, m := range mods {
		m(&h)
	}
	return h
}
func at(lat, lng float64) func(*domain.Hospital) {
	return func(h *domain.Hospital) { h.Latitude, h.Longitude = &lat, &lng }
}
func withServices(s ...domain.Service) func(*domain.Hospital) {
	return func(h *domain.Hospital) { h.Services = s }
}
func withBeds(b ...domain.Bed) func(*domain.Hospital) { return func(h *domain.Hospital) { h.Beds = b } }

func ago(d time.Duration) *time.Time { t := time.Now().UTC().Add(-d); return &t }

func list(t *testing.T, r *rig, query string) map[string]any {
	t.Helper()
	w, body := r.do(call{method: "GET", path: "/api/v1/hospitals?" + query, remote: r.ip()})
	if w.Code != 200 {
		t.Fatalf("%s: %d %s", query, w.Code, w.Body.String())
	}
	return body
}
func listStatus(r *rig, query string) (int, map[string]any) {
	w, body := r.do(call{method: "GET", path: "/api/v1/hospitals?" + query, remote: r.ip()})
	return w.Code, body
}

func TestFilterServicesIsAND(t *testing.T) {
	fresh := ago(5 * time.Minute)
	both := func(a, b string) func(*domain.Hospital) {
		return withServices(svc("nicu", a, "curated", fresh), svc("icu", b, "curated", fresh))
	}
	r := newRig(t, []domain.Hospital{
		hosp(1, "A nicu+icu available", "curated", fresh, both("available", "available")),
		hosp(2, "B nicu only", "curated", fresh, withServices(svc("nicu", "available", "curated", fresh))),
		hosp(3, "C nicu available icu limited", "curated", fresh, both("available", "limited")),
		hosp(4, "D icu unavailable", "curated", fresh, both("available", "unavailable")),
		hosp(5, "E icu unknown", "curated", fresh, both("available", "unknown")),
		hosp(6, "F no services", "curated", fresh),
	})
	for query, want := range map[string][]string{
		"services=nicu,icu": {hid(1), hid(3)},
		"services=icu,nicu": {hid(1), hid(3)},
		"services=nicu":     {hid(1), hid(2), hid(3), hid(4), hid(5)},
		"services=icu":      {hid(1), hid(3)},
	} {
		if got := ids(t, list(t, r, query)); !equalStrings(got, want) {
			t.Errorf("%s: got %v want %v", query, got, want)
		}
	}
	for _, bad := range []string{"services=nicu,nicu", "services=nicu,", "services=nicu,%20icu", "services=tidak-ada", "services=nicu&services=icu"} {
		if code, body := listStatus(r, bad); code != 422 || errorCode(t, body) != "validation_error" {
			t.Errorf("%s: %d %v", bad, code, body)
		}
	}
}

func TestFilterBPJSNeverConfusesFalseWithUnknown(t *testing.T) {
	up := ago(time.Minute)
	r := newRig(t, []domain.Hospital{
		hosp(1, "Yes", "curated", up, func(h *domain.Hospital) { h.BPJS = ptr(true) }),
		hosp(2, "No", "curated", up, func(h *domain.Hospital) { h.BPJS = ptr(false) }),
		hosp(3, "Unknown", "curated", up),
	})
	for query, want := range map[string][]string{"bpjs=true": {hid(1)}, "bpjs=false": {hid(2)}} {
		if got := ids(t, list(t, r, query)); !equalStrings(got, want) {
			t.Errorf("%s: got %v want %v", query, got, want)
		}
	}
	all := list(t, r, "page=1")
	for _, x := range dataList(t, all) {
		h := obj(x)
		if h["id"] == hid(3) && h["bpjs"] != nil {
			t.Errorf("bpjs tidak diketahui harus null, dapat %v", h["bpjs"])
		}
	}
	if code, _ := listStatus(r, "bpjs=maybe"); code != 422 {
		t.Error(code)
	}
}

func TestFilterAvailableBedsOnlyFreshNonSimulationPositive(t *testing.T) {
	bedOf := func(available *int, origin string, source *time.Time) func(*domain.Hospital) {
		return withBeds(bed("icu", ptr(10), available, origin, source))
	}
	r := newRig(t, []domain.Hospital{
		hosp(1, "ok fresh curated", "curated", ago(time.Minute), bedOf(ptr(3), "curated", ago(time.Minute))),
		hosp(2, "simulation fresh", "simulation", ago(time.Minute), bedOf(ptr(5), "simulation", ago(time.Minute))),
		hosp(3, "stale", "curated", ago(time.Minute), bedOf(ptr(4), "curated", ago(25*time.Hour))),
		hosp(4, "aging", "curated", ago(time.Minute), bedOf(ptr(4), "curated", ago(2*time.Hour))),
		hosp(5, "zero", "curated", ago(time.Minute), bedOf(ptr(0), "curated", ago(time.Minute))),
		hosp(6, "unknown available", "curated", ago(time.Minute), bedOf(nil, "curated", ago(time.Minute))),
		hosp(7, "no source time", "curated", ago(time.Minute), bedOf(ptr(4), "curated", nil)),
	})
	if got := ids(t, list(t, r, "has_available_beds=true&dataset=all")); !equalStrings(got, []string{hid(1)}) {
		t.Errorf("has_available_beds=true: %v", got)
	}
	// false is an explicit "no filter", not "only hospitals without beds".
	if got := list(t, r, "has_available_beds=false&dataset=all"); num(obj(got["meta"])["total"]) != 7 {
		t.Errorf("has_available_beds=false menyaring: %v", got["meta"])
	}
}

func TestDatasetSeparation(t *testing.T) {
	up := ago(time.Minute)
	r := newRig(t, []domain.Hospital{
		hosp(1, "Real curated", "curated", up),
		hosp(2, "Real sandbox", "satusehat_sandbox", nil),
		hosp(3, "Fake", "simulation", up),
	})
	for query, want := range map[string][]string{
		"": {hid(1), hid(2)}, "dataset=real": {hid(1), hid(2)}, "dataset=simulation": {hid(3)},
	} {
		if got := ids(t, list(t, r, query)); !equalStrings(got, want) {
			t.Errorf("%q: got %v want %v", query, got, want)
		}
	}
	if got := ids(t, list(t, r, "dataset=all")); len(got) != 3 {
		t.Errorf("dataset=all: %v", got)
	}
	meta := obj(list(t, r, "dataset=simulation")["meta"])
	if meta["dataset"] != "simulation" || meta["data_scope"] != "pilot" {
		t.Errorf("meta: %v", meta)
	}
	if obj(list(t, r, "")["meta"])["dataset"] != "real" {
		t.Error("default dataset harus real")
	}
	// Detail still serves simulated hospitals (lookup destination of SR-DEMO-001).
	if w, _ := r.do(call{method: "GET", path: "/api/v1/hospitals/" + hid(3)}); w.Code != 200 {
		t.Error(w.Code)
	}
}

func TestQueryValidationMatchesSpec(t *testing.T) {
	r := newRig(t, []domain.Hospital{hosp(1, "A", "curated", ago(time.Minute), at(-6.2, 106.8))})
	limit := r.spec.queryParam("limit")
	max, def := int(num(limit["maximum"])), int(num(limit["default"]))
	if max != 50 || def != 20 {
		t.Fatalf("OpenAPI limit default/max berubah: %d/%d", def, max)
	}
	if got := num(obj(list(t, r, "")["meta"])["limit"]); got != 20 {
		t.Errorf("default limit %v", got)
	}
	if code, _ := listStatus(r, fmt.Sprintf("limit=%d", max)); code != 200 {
		t.Errorf("limit=max: %d", code)
	}
	for _, bad := range []string{
		fmt.Sprintf("limit=%d", max+1), "limit=0", "limit=-1", "limit=abc", "limit=", "page=0", "page=abc", "page=1000001",
		"unknown=1", "q=a&q=b", "sort=random", "sort=distance", "lat=-6.2", "lng=106.8", "lat=91&lng=0", "lat=-91&lng=0", "lat=0&lng=181", "lat=NaN&lng=0", "lat=x&lng=0",
		"province_code=99", "city_code=99.99", "city_code=31", "province_code=31.71", "dataset=fake", "bpjs=1", "has_available_beds=yes",
		"q=" + strings.Repeat("x", 101), "q=" + strings.Repeat("é", 101),
	} {
		code, body := listStatus(r, bad)
		if code != 422 || errorCode(t, body) != "validation_error" {
			t.Errorf("%q: %d %v", bad, code, body)
		}
	}
	for _, good := range []string{"q=" + strings.Repeat("x", 100), "q=" + strings.Repeat("é", 100), "lat=90&lng=180", "lat=-90&lng=-180", "province_code=31", "city_code=31.71", "sort=distance&lat=-6.2&lng=106.8", "sort=match", "q=jakarta"} {
		if code, body := listStatus(r, good); code != 200 {
			t.Errorf("%q: %d %v", good, code, body)
		}
	}
	// Enum values for sort and dataset documented in OpenAPI must be accepted.
	for _, name := range []string{"sort", "dataset"} {
		enum, _ := r.spec.queryParam(name)["enum"].([]any)
		if len(enum) == 0 {
			t.Errorf("enum %s tidak ada di OpenAPI", name)
		}
		for _, v := range enum {
			q := fmt.Sprintf("%s=%v", name, v)
			if v == "distance" {
				q += "&lat=0&lng=0"
			}
			if code, _ := listStatus(r, q); code != 200 {
				t.Errorf("%s: %d", q, code)
			}
		}
	}
}
func TestTextAndRegionFilters(t *testing.T) {
	up := ago(time.Minute)
	r := newRig(t, []domain.Hospital{
		hosp(1, "Siaga Utama", "curated", up),
		hosp(2, "Medika Barat", "curated", up, func(h *domain.Hospital) {
			h.CityCode, h.CityName, h.ProvinceCode, h.ProvinceName = "32.73", "Bandung", "32", "Jawa Barat"
		}),
	})
	for query, want := range map[string][]string{
		"q=siaga": {hid(1)}, "q=SIAGA%20utama": {hid(1)}, "q=bandung": {hid(2)}, "q=jawa": {hid(2)}, "q=tidak-ada": {},
		"province_code=32": {hid(2)}, "city_code=31.71": {hid(1)}, "province_code=31&q=siaga": {hid(1)}, "province_code=32&q=siaga": {},
	} {
		if got := ids(t, list(t, r, query)); !equalStrings(got, want) {
			t.Errorf("%s: got %v want %v", query, got, want)
		}
	}
}

func TestPaginationMeta(t *testing.T) {
	up := ago(time.Minute)
	hs := []domain.Hospital{}
	for i := 1; i <= 7; i++ {
		hs = append(hs, hosp(i, fmt.Sprintf("RS %d", (i+1)/2), "curated", up)) // pairs share a name -> ID tie-break
	}
	r := newRig(t, hs)
	seen := []string{}
	for page, size := range []int{3, 3, 1} {
		body := list(t, r, fmt.Sprintf("limit=3&page=%d", page+1))
		meta := obj(body["meta"])
		for k, want := range map[string]string{"page": fmt.Sprint(page + 1), "limit": "3", "total": "7", "total_pages": "3"} {
			if fmt.Sprint(meta[k]) != want {
				t.Errorf("page %d meta.%s=%v want %s", page+1, k, meta[k], want)
			}
		}
		if got := ids(t, body); len(got) != size {
			t.Errorf("page %d size %d want %d", page+1, len(got), size)
		} else {
			seen = append(seen, got...)
		}
	}
	want := []string{}
	for i := 1; i <= 7; i++ {
		want = append(want, hid(i))
	}
	if !equalStrings(seen, want) {
		t.Errorf("urutan name lalu ID: %v", seen)
	}
	beyond := list(t, r, "limit=3&page=4")
	if len(dataList(t, beyond)) != 0 || fmt.Sprint(obj(beyond["meta"])["total"]) != "7" {
		t.Errorf("halaman di luar jangkauan: %v", beyond)
	}
	empty := list(t, r, "q=tidak-ada")
	if len(dataList(t, empty)) != 0 || fmt.Sprint(obj(empty["meta"])["total"]) != "0" || fmt.Sprint(obj(empty["meta"])["total_pages"]) != "0" {
		t.Errorf("hasil kosong harus [] dengan total 0: %v", empty)
	}
	if _, ok := obj(list(t, r, "")["meta"])["as_of"].(string); !ok {
		t.Error("meta.as_of")
	}
}

func haversine(a, b, c, d float64) float64 {
	const rad = math.Pi / 180
	x := math.Pow(math.Sin((c-a)*rad/2), 2) + math.Cos(a*rad)*math.Cos(c*rad)*math.Pow(math.Sin((d-b)*rad/2), 2)
	return 6371 * 2 * math.Asin(math.Sqrt(x))
}

func TestDistanceSortAndNullDistance(t *testing.T) {
	up := ago(time.Minute)
	r := newRig(t, []domain.Hospital{
		hosp(1, "Jauh", "curated", up, at(-6.9, 107.6)),
		hosp(2, "Tanpa koordinat", "curated", up),
		hosp(3, "Dekat", "curated", up, at(-6.21, 106.82)),
		hosp(4, "Tengah", "curated", up, at(-6.5, 107.0)),
	})
	body := list(t, r, "sort=distance&lat=-6.2&lng=106.8")
	if got := ids(t, body); !equalStrings(got, []string{hid(3), hid(4), hid(1), hid(2)}) {
		t.Errorf("urutan jarak: %v", got)
	}
	for _, x := range dataList(t, body) {
		h := obj(x)
		switch h["id"] {
		case hid(2):
			if h["distance_km"] != nil {
				t.Errorf("tanpa koordinat harus distance_km null: %v", h["distance_km"])
			}
		case hid(3):
			got := num(h["distance_km"])
			if want := haversine(-6.2, 106.8, -6.21, 106.82); math.Abs(got-want) > 0.001 {
				t.Errorf("Haversine km %v want %v", got, want)
			}
		}
	}
	// Without lat/lng the distance is null for every hospital.
	for _, x := range dataList(t, list(t, r, "")) {
		if obj(x)["distance_km"] != nil {
			t.Errorf("distance_km harus null tanpa lat/lng: %v", obj(x)["distance_km"])
		}
	}
}

func TestMatchSortRanksStatusThenFreshnessAndIgnoresSimulation(t *testing.T) {
	mk := func(n int, name, status, origin string, age time.Duration) domain.Hospital {
		return hosp(n, name, origin, ago(age), withServices(svc("nicu", status, origin, ago(age))))
	}
	r := newRig(t, []domain.Hospital{
		mk(1, "B limited fresh", "limited", "curated", time.Minute),
		mk(2, "A simulation available fresh", "available", "simulation", time.Minute),
		mk(3, "C available aging", "available", "curated", 2*time.Hour),
		mk(4, "D available fresh", "available", "curated", time.Minute),
	})
	got := ids(t, list(t, r, "sort=match&services=nicu&dataset=all"))
	// D 13, C 11, B 8, simulation 0 (no factual bonus).
	if want := []string{hid(4), hid(3), hid(1), hid(2)}; !equalStrings(got, want) {
		t.Errorf("match: got %v want %v", got, want)
	}
}

func TestCompareTwoToThreeUniqueIDs(t *testing.T) {
	up := ago(time.Minute)
	r := newRig(t, []domain.Hospital{
		hosp(1, "Satu", "simulation", up, withBeds(bed("icu", nil, ptr(0), "simulation", up))),
		hosp(2, "Dua", "curated", up),
		hosp(3, "Tiga", "curated", up),
		hosp(4, "Empat", "curated", up),
	})
	get := func(ids string) (int, map[string]any) {
		w, body := r.do(call{method: "GET", path: "/api/v1/hospitals/compare?ids=" + ids, remote: r.ip()})
		return w.Code, body
	}
	if code, body := get(hid(3) + "," + hid(1) + "," + hid(2)); code != 200 || !equalStrings(ids(t, body), []string{hid(3), hid(1), hid(2)}) || num(obj(body["meta"])["count"]) != 3 {
		t.Errorf("urutan hasil harus mengikuti ids, simulasi tetap bisa dibandingkan: %d %v", code, body)
	}
	if code, body := get(hid(1) + "," + hid(2)); code != 200 {
		t.Errorf("%d %v", code, body)
	} else if bd := obj(obj(dataList(t, body)[0])["beds"].([]any)[0]); bd["total"] != nil || num(bd["available"]) != 0 {
		t.Errorf("null vs 0 pada compare: %v", bd)
	}
	for name, ids := range map[string]string{
		"satu ID": hid(1), "empat ID": hid(1) + "," + hid(2) + "," + hid(3) + "," + hid(4), "duplikat": hid(1) + "," + hid(1),
		"bukan UUID": hid(1) + ",abc", "spasi": hid(1) + ",%20" + hid(2), "kosong": "",
	} {
		if code, body := get(ids); code != 422 || errorCode(t, body) != "validation_error" {
			t.Errorf("%s: %d %v", name, code, body)
		}
	}
	missing := "10000000-0000-4000-8000-0000000000ff"
	if code, body := get(hid(1) + "," + missing); code != 404 || !strings.Contains(obj(body["error"])["message"].(string), missing) {
		t.Errorf("semua-atau-tidak-sama-sekali: %d %v", code, body)
	}
	w, _ := r.do(call{method: "GET", path: "/api/v1/hospitals/compare", remote: r.ip()})
	w2, _ := r.do(call{method: "GET", path: "/api/v1/hospitals/compare?ids=" + hid(1) + "," + hid(2) + "&x=1", remote: r.ip()})
	if w.Code != 422 || w2.Code != 422 {
		t.Errorf("ids hilang / parameter tak dikenal harus 422: %d %d", w.Code, w2.Code)
	}
}

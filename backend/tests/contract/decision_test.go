package contract

import (
	"testing"
	"time"

	"siagars/backend/internal/domain"
)

// Every r.do call below is also checked against the OpenAPI response schema (rig.conform), so these
// tests cover both behavior and the 0.4.0 contract of /recommendations and /hospitals/markers.

func decisionRig(t *testing.T) *rig {
	fresh := ago(10 * time.Minute)
	near := hosp(1, "RS Dekat NICU", "curated", fresh, at(-6.21, 106.8), withServices(svc("nicu", "available", "curated", fresh)), withBeds(bed("nicu", ptr(4), ptr(2), "curated", fresh)), func(h *domain.Hospital) { h.BPJS = ptr(true) })
	sim := hosp(2, "RS Simulasi NICU", "simulation", fresh, at(-6.205, 106.8), withServices(svc("nicu", "available", "simulation", fresh)), withBeds(bed("nicu", ptr(4), ptr(2), "simulation", fresh)), func(h *domain.Hospital) { h.BPJS = ptr(true) })
	none := hosp(3, "RS Tanpa NICU", "curated", fresh, at(-6.22, 106.8), withServices(svc("nicu", "unavailable", "curated", fresh)))
	noBPJS := hosp(4, "RS Non BPJS", "curated", fresh, at(-6.23, 106.8), withServices(svc("nicu", "available", "curated", fresh)), func(h *domain.Hospital) { h.BPJS = ptr(false) })
	far := hosp(5, "RS Jauh", "curated", fresh, at(-7.5, 106.8), withServices(svc("nicu", "available", "curated", fresh)))
	unknown := hosp(6, "RS Belum Diketahui", "curated", nil, at(-6.24, 106.8))
	return newRig(t, []domain.Hospital{near, sim, none, noBPJS, far, unknown})
}

func TestRecommendationsEndToEnd(t *testing.T) {
	r := decisionRig(t)
	w, body := r.do(call{method: "GET", path: "/api/v1/recommendations?services=nicu&lat=-6.2&lng=106.8&radius_km=20&bpjs=required&dataset=all", remote: r.ip()})
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	data := dataList(t, body)
	meta := obj(body["meta"])
	excluded := obj(meta["excluded"])
	if num(meta["total_considered"]) != 5 || num(excluded["service_unavailable"]) != 1 || num(excluded["bpjs_false"]) != 1 || meta["algorithm_version"] != domain.AlgorithmVersion || num(meta["radius_km"]) != 20 {
		t.Fatalf("meta: %v", meta)
	}
	names := []string{}
	for _, x := range data {
		names = append(names, obj(obj(x)["hospital"])["name"].(string))
	}
	// Curated beats the closer simulated twin (penalty 0.8); unknown data ranks last but is shown.
	if !equalStrings(names, []string{"RS Dekat NICU", "RS Simulasi NICU", "RS Belum Diketahui"}) {
		t.Fatalf("urutan: %v", names)
	}
	last := obj(data[2])
	if obj(data[1])["flags"].([]any)[0] != "contains_simulated_data" || len(last["unknowns"].([]any)) == 0 {
		t.Fatalf("flags/unknowns: %v %v", obj(data[1])["flags"], last["unknowns"])
	}
	// Without a location the same need by city works; the distance component disappears.
	w, body = r.do(call{method: "GET", path: "/api/v1/recommendations?services=nicu&city_code=3171&dataset=all", remote: r.ip()})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, x := range dataList(t, body) {
		for _, c := range obj(x)["components"].([]any) {
			if obj(c)["key"] == "distance" || obj(c)["key"] == "bpjs" {
				t.Fatalf("komponen %v tidak boleh ada tanpa lat/lng / bpjs=any", obj(c)["key"])
			}
		}
		if obj(x)["distance_km"] != nil {
			t.Fatal("distance_km harus null tanpa lokasi")
		}
	}
	if obj(body["meta"])["radius_km"] != nil {
		t.Fatal("radius_km harus null tanpa lat/lng")
	}
	// The default dataset is real: the simulated hospital is not a candidate.
	_, body = r.do(call{method: "GET", path: "/api/v1/recommendations?services=nicu&lat=-6.2&lng=106.8", remote: r.ip()})
	for _, x := range dataList(t, body) {
		if obj(obj(x)["hospital"])["data_origin"] == "simulation" {
			t.Fatal("dataset default real memuat simulasi")
		}
	}
}

func TestRecommendations422(t *testing.T) {
	r := decisionRig(t)
	for _, q := range []string{
		"", "services=nicu", "lat=-6.2&lng=106.8", "services=&lat=-6.2&lng=106.8", "services=nicu,nicu&city_code=3171", "services=tidak-ada&city_code=3171",
		"services=nicu&lat=-6.2", "services=nicu&lat=91&lng=0", "services=nicu&city_code=3171&radius_km=20", "services=nicu&lat=-6.2&lng=106.8&radius_km=15",
		"services=nicu&city_code=9999", "services=nicu&city_code=31.71", "services=nicu&city_code=3171&bpjs=true", "services=nicu&city_code=3171&limit=21",
		"services=nicu&city_code=3171&limit=0", "services=nicu&city_code=3171&dataset=x", "services=nicu&city_code=3171&x=1", "services=nicu&city_code=3171&city_code=3171",
	} {
		w, body := r.do(call{method: "GET", path: "/api/v1/recommendations?" + q, remote: r.ip()})
		if w.Code != 422 || errorCode(t, body) != "validation_error" {
			t.Errorf("%q: %d", q, w.Code)
		}
	}
	if w, _ := r.do(call{method: "POST", path: "/api/v1/recommendations", remote: r.ip()}); w.Code != 405 {
		t.Error("POST:", w.Code)
	}
}

func TestMarkers(t *testing.T) {
	r := decisionRig(t)
	w, body := r.do(call{method: "GET", path: "/api/v1/hospitals/markers?bbox=106,-7,107,-6&services=nicu&dataset=all", remote: r.ip()})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	data := dataList(t, body)
	if len(data) != 5 || obj(body["meta"])["truncated"] != false {
		t.Fatalf("RS di luar bbox ikut atau ada yang hilang: %d", len(data))
	}
	byName := map[string]map[string]any{}
	for _, x := range data {
		byName[obj(x)["name"].(string)] = obj(x)
	}
	if byName["RS Tanpa NICU"]["match"] != "none" || byName["RS Dekat NICU"]["match"] != "all" || byName["RS Simulasi NICU"]["has_simulated_data"] != true || byName["RS Dekat NICU"]["has_simulated_data"] != false {
		t.Fatalf("%v", byName)
	}
	_, body = r.do(call{method: "GET", path: "/api/v1/hospitals/markers?bbox=106,-7,107,-6&bpjs=true&dataset=all", remote: r.ip()})
	if len(dataList(t, body)) != 2 {
		t.Fatalf("filter bpjs=true (null tidak cocok): %v", dataList(t, body))
	}
	for _, q := range []string{"", "bbox=1,2,3", "bbox=106,-7,108.5,-6", "bbox=107,-7,106,-6", "bbox=a,b,c,d", "bbox=106,-7,107,-6&services=x", "bbox=106,-7,107,-6&bpjs=1", "bbox=106,-7,107,-6&zoom=3", "bbox=-181,-7,-180,-6"} {
		if w, _ := r.do(call{method: "GET", path: "/api/v1/hospitals/markers?" + q, remote: r.ip()}); w.Code != 422 {
			t.Errorf("%q: %d", q, w.Code)
		}
	}
}

func TestDataQualityOnEveryHospitalDTO(t *testing.T) {
	r := decisionRig(t)
	for _, path := range []string{"/api/v1/hospitals?dataset=all", "/api/v1/hospitals/" + hid(1), "/api/v1/hospitals/compare?ids=" + hid(1) + "," + hid(6)} {
		_, body := r.do(call{method: "GET", path: path, remote: r.ip()})
		items := []any{}
		switch d := body["data"].(type) {
		case []any:
			items = d
		default:
			items = append(items, d)
		}
		for _, x := range items {
			dq := obj(obj(x)["data_quality"])
			if dq["label"] == nil || dq["score"] == nil {
				t.Fatalf("%s: data_quality hilang: %v", path, obj(x))
			}
		}
	}
}

// TestInsights checks the four /insights/* endpoints against the OpenAPI schemas (via r.do) and the
// numbers the rig's six hospitals must produce.
func TestInsights(t *testing.T) {
	r := decisionRig(t)
	w, body := r.do(call{method: "GET", path: "/api/v1/insights/summary?dataset=all", remote: r.ip()})
	if w.Code != 200 || w.Header().Get("Cache-Control") != "public, max-age=300" {
		t.Fatalf("%d %q %s", w.Code, w.Header().Get("Cache-Control"), w.Body.String())
	}
	s := obj(body["data"])
	if num(s["hospital_count"]) != 6 || num(s["city_count"]) != 1 || num(s["curated_count"]) != 5 || num(s["simulated_facility_count"]) != 2 {
		t.Fatalf("summary: %v", s)
	}
	if num(s["fresh_ratio"]) != 5.0/6 || num(s["with_known_bpjs_ratio"]) != 3.0/6 {
		t.Fatalf("rasio: %v", s)
	}
	// Default dataset is real: the simulated hospital drops out.
	_, body = r.do(call{method: "GET", path: "/api/v1/insights/summary", remote: r.ip()})
	if num(obj(body["data"])["hospital_count"]) != 5 {
		t.Fatalf("real: %v", body["data"])
	}
	_, body = r.do(call{method: "GET", path: "/api/v1/insights/service-gaps?service=nicu&dataset=all&province_code=31", remote: r.ip()})
	g := obj(dataList(t, body)[0])
	if num(g["available"]) != 4 || num(g["unavailable"]) != 1 || num(g["unknown"]) != 1 || g["gap"] != false {
		t.Fatalf("service-gaps: %v", g)
	}
	// A province with no hospital in the data is valid and yields an empty list, not a gap claim.
	_, body = r.do(call{method: "GET", path: "/api/v1/insights/coverage?province_code=32", remote: r.ip()})
	if len(dataList(t, body)) != 0 {
		t.Fatalf("coverage 32: %v", body["data"])
	}
	_, body = r.do(call{method: "GET", path: "/api/v1/insights/freshness?dataset=all", remote: r.ip()})
	if len(dataList(t, body)) != 2 {
		t.Fatalf("freshness: %v", body["data"])
	}
	for _, q := range []string{"summary?x=1", "summary?dataset=x", "summary?province_code=99", "coverage?service=nicu", "service-gaps", "service-gaps?service=nope", "freshness?province_code=31&province_code=32"} {
		if w, _ := r.do(call{method: "GET", path: "/api/v1/insights/" + q, remote: r.ip()}); w.Code != 422 || w.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s: %d %q", q, w.Code, w.Header().Get("Cache-Control"))
		}
	}
	if w, _ := r.do(call{method: "POST", path: "/api/v1/insights/summary", remote: r.ip()}); w.Code != 405 {
		t.Error("POST:", w.Code)
	}
}

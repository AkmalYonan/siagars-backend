package domain

import (
	"math"
	"slices"
	"testing"
	"time"
)

// Origin of the request in the tests below; offsets in latitude give exact-ish distances (1° ≈ 111.19 km).
var origin = struct{ lat, lng float64 }{-6.2, 106.8}

func atKM(km float64) (*float64, *float64) {
	lat := origin.lat + km/111.195
	lng := origin.lng
	return &lat, &lng
}

// rsA is the contract §4.4 hospital: NICU available (curated), NICU bed 2 (curated), 6 km, BPJS true,
// data quality 80 (forced through the fields below).
func rsA(origin string) Hospital {
	lat, lng := atKM(6)
	h := Hospital{ID: "a", Name: "RS A", Latitude: lat, Longitude: lng, BPJS: bptr(true), BPJSDataOrigin: sptr("curated"), Provenance: Provenance{DataOrigin: "satusehat_sandbox"}}
	h.Services = []Service{svcQ("nicu", "available", origin, ago(time.Minute))}
	h.Beds = []Bed{bedQ("nicu", iptr(2), origin, ago(time.Minute))}
	return h
}

func nicuReq() RecommendRequest {
	return RecommendRequest{Services: []string{"nicu"}, Lat: &origin.lat, Lng: &origin.lng, RadiusKM: 20, BPJSRequired: true, Limit: 10}
}

func component(r Recommendation, key string) (ScoreComponent, bool) {
	for _, c := range r.Components {
		if c.Key == key {
			return c, true
		}
	}
	return ScoreComponent{}, false
}

// manual recomputes the score from the components of a recommendation as the contract states it.
func manual(base map[string]float64, values map[string]float64, penalty map[string]float64) int {
	var total, raw float64
	for k := range values {
		total += base[k]
	}
	for k, v := range values {
		p := penalty[k]
		if p == 0 {
			p = 1
		}
		raw += base[k] / total * v * p
	}
	return int(math.Round(100*raw + 1e-9))
}

func TestRecommendContractExample(t *testing.T) {
	// The contract fixes data_quality at 80 for the example; check the arithmetic with that value.
	values := map[string]float64{"service_match": 1, "bed_availability": 1, "distance": 0.7, "bpjs": 1, "data_confidence": 0.8}
	if got := manual(BaseWeights, values, nil); got != 92 {
		t.Fatalf("contoh kurasi: %d, ingin 92", got)
	}
	if got := manual(BaseWeights, values, map[string]float64{"service_match": SimulationPenalty, "bed_availability": SimulationPenalty}); got != 80 {
		t.Fatalf("contoh simulasi: %d, ingin 80", got)
	}
	// The same through Recommend: components carry exactly these values, so the score is recomputable.
	res := Recommend([]Hospital{rsA("curated")}, nicuReq(), qNow)
	r := res.Recommendations[0]
	dq := float64(r.Hospital.DataQuality.Score) / 100
	if d, _ := component(r, "distance"); math.Abs(d.Value-0.7) > 0.001 {
		t.Fatalf("distance value %v", d.Value)
	}
	want := manual(BaseWeights, map[string]float64{"service_match": 1, "bed_availability": 1, "distance": 0.7, "bpjs": 1, "data_confidence": dq}, nil)
	if r.Score != want || r.Level != RecommendLevel(want) || slices.Contains(r.Flags, "contains_simulated_data") {
		t.Fatalf("score %d ingin %d: %+v", r.Score, want, r)
	}
	sim := Recommend([]Hospital{rsA("simulation")}, nicuReq(), qNow).Recommendations[0]
	dqSim := float64(sim.Hospital.DataQuality.Score) / 100
	wantSim := manual(BaseWeights, map[string]float64{"service_match": 1, "bed_availability": 1, "distance": 0.7, "bpjs": 1, "data_confidence": dqSim}, map[string]float64{"service_match": SimulationPenalty, "bed_availability": SimulationPenalty})
	if sim.Score != wantSim || !slices.Contains(sim.Flags, "contains_simulated_data") || sim.Score >= r.Score {
		t.Fatalf("simulasi %d ingin %d (kurasi %d)", sim.Score, wantSim, r.Score)
	}
}

func TestRecommendTable(t *testing.T) {
	type tc struct {
		name  string
		hs    func() []Hospital
		req   func() RecommendRequest
		check func(t *testing.T, res RecommendResult)
	}
	one := func(f func(*Hospital)) func() []Hospital {
		return func() []Hospital { h := rsA("curated"); f(&h); return []Hospital{h} }
	}
	excluded := func(reason string) func(*testing.T, RecommendResult) {
		return func(t *testing.T, res RecommendResult) {
			if len(res.Recommendations) != 0 || res.Excluded[reason] != 1 || res.Considered != 1 {
				t.Fatalf("ingin dikeluarkan %s: %+v", reason, res)
			}
		}
	}
	first := func(f func(*testing.T, Recommendation)) func(*testing.T, RecommendResult) {
		return func(t *testing.T, res RecommendResult) {
			if len(res.Recommendations) != 1 {
				t.Fatalf("hasil: %+v", res)
			}
			f(t, res.Recommendations[0])
		}
	}
	cases := []tc{
		{"layanan unavailable dikeluarkan", one(func(h *Hospital) { h.Services[0].Status = "unavailable" }), nicuReq, excluded(ExcludedServiceUnavailable)},
		{"bpjs false dikeluarkan saat required", one(func(h *Hospital) { h.BPJS = bptr(false) }), nicuReq, excluded(ExcludedBPJSFalse)},
		{"bpjs false tetap ikut saat any", one(func(h *Hospital) { h.BPJS = bptr(false) }), func() RecommendRequest { r := nicuReq(); r.BPJSRequired = false; return r }, first(func(t *testing.T, r Recommendation) {
			if _, has := component(r, "bpjs"); has {
				t.Fatal("komponen bpjs harus dihapus saat any")
			}
		})},
		{"bpjs null = 0,3, bukan false", one(func(h *Hospital) { h.BPJS = nil }), nicuReq, first(func(t *testing.T, r Recommendation) {
			if c, _ := component(r, "bpjs"); c.Value != 0.3 || !slices.Contains(r.Unknowns, "bpjs") {
				t.Fatalf("%+v", c)
			}
		})},
		{"layanan unknown = 0,2 dan tercatat", one(func(h *Hospital) { h.Services[0].Status = "unknown" }), nicuReq, first(func(t *testing.T, r Recommendation) {
			if c, _ := component(r, "service_match"); c.Value != 0.2 || !slices.Contains(r.Unknowns, "service_nicu") {
				t.Fatalf("%+v %v", c, r.Unknowns)
			}
		})},
		{"layanan tidak ada = unknown, bukan unavailable", one(func(h *Hospital) { h.Services = nil }), nicuReq, first(func(t *testing.T, r Recommendation) {
			if c, _ := component(r, "service_match"); c.Value != 0.2 {
				t.Fatalf("%+v", c)
			}
		})},
		{"limited = 0,6", one(func(h *Hospital) { h.Services[0].Status = "limited" }), nicuReq, first(func(t *testing.T, r Recommendation) {
			if c, _ := component(r, "service_match"); c.Value != 0.6 {
				t.Fatalf("%+v", c)
			}
		})},
		{"bed null = 0,3, bed 0 = 0 (null tidak pernah 0)", func() []Hospital {
			a, b := rsA("curated"), rsA("curated")
			a.ID, b.ID = "null", "zero"
			a.Beds[0].Available, b.Beds[0].Available = nil, iptr(0)
			return []Hospital{a, b}
		}, nicuReq, func(t *testing.T, res RecommendResult) {
			for _, r := range res.Recommendations {
				c, _ := component(r, "bed_availability")
				if (r.Hospital.ID == "null" && (c.Value != 0.3 || !slices.Contains(r.Unknowns, "bed_nicu"))) || (r.Hospital.ID == "zero" && c.Value != 0) {
					t.Fatalf("%s: %+v", r.Hospital.ID, c)
				}
			}
			if res.Recommendations[0].Hospital.ID != "null" {
				t.Fatal("bed tidak diketahui harus di atas bed penuh")
			}
		}},
		{"tanpa kategori bed relevan → komponen bed dihapus", one(func(h *Hospital) { h.Services[0].Slug = "igd-24h" }), func() RecommendRequest { r := nicuReq(); r.Services = []string{"igd-24h"}; return r }, first(func(t *testing.T, r Recommendation) {
			if _, has := component(r, "bed_availability"); has {
				t.Fatal("komponen bed harus dihapus")
			}
		})},
		{"tanpa lat/lng → jarak dihapus & bobot dinormalisasi", one(func(*Hospital) {}), func() RecommendRequest { r := nicuReq(); r.Lat, r.Lng = nil, nil; return r }, first(func(t *testing.T, r Recommendation) {
			if _, has := component(r, "distance"); has || r.DistanceKM != nil {
				t.Fatal("jarak harus dihapus")
			}
			var sum float64
			for _, c := range r.Components {
				sum += c.Weight
			}
			if math.Abs(sum-1) > 0.001 {
				t.Fatalf("bobot tidak dinormalisasi: %v", sum)
			}
			if c, _ := component(r, "service_match"); math.Abs(c.Weight-0.5) > 0.0001 {
				t.Fatalf("bobot service_match %v, ingin 0,5", c.Weight)
			}
		})},
		{"radius boundary: tepat di dalam ikut", one(func(h *Hospital) { h.Latitude, h.Longitude = atKM(19.99) }), nicuReq, first(func(t *testing.T, r Recommendation) {
			if c, _ := component(r, "distance"); c.Value < 0 || c.Value > 0.01 {
				t.Fatalf("%+v", c)
			}
		})},
		{"radius boundary: di luar dikeluarkan", one(func(h *Hospital) { h.Latitude, h.Longitude = atKM(20.05) }), nicuReq, excluded(ExcludedOutOfRadius)},
		{"RS tanpa koordinat dikeluarkan saat lokasi diberikan", one(func(h *Hospital) { h.Latitude = nil }), nicuReq, excluded(ExcludedLocationUnknown)},
		{"RS nonaktif dikeluarkan", one(func(h *Hospital) { h.Operational = bptr(false) }), nicuReq, excluded(ExcludedNotOperational)},
		{"operational null tetap ikut", one(func(h *Hospital) { h.Operational = nil }), nicuReq, first(func(*testing.T, Recommendation) {})},
		{"penalti simulasi hanya pada layanan & bed", one(func(h *Hospital) { h.Services[0].DataOrigin = "simulation" }), nicuReq, first(func(t *testing.T, r Recommendation) {
			c, _ := component(r, "service_match")
			b, _ := component(r, "bed_availability")
			if c.Contribution != 32 || b.Contribution != 20 || !slices.Contains(r.Flags, "contains_simulated_data") {
				t.Fatalf("%+v %+v", c, b)
			}
		})},
		{"seri diurutkan jarak lalu nama", func() []Hospital {
			mk := func(id, name string, km float64) Hospital {
				h := rsA("curated")
				h.ID, h.Name = id, name
				h.Latitude, h.Longitude = atKM(km)
				return h
			}
			// Equal data and distance give equal scores, so only the case-insensitive name decides.
			return []Hospital{mk("3", "RS Cempaka", 6), mk("1", "rs anggrek", 6), mk("2", "RS Bougenville", 6)}
		}, nicuReq, func(t *testing.T, res RecommendResult) {
			got := []string{}
			for _, r := range res.Recommendations {
				got = append(got, r.Hospital.Name)
			}
			if !slices.Equal(got, []string{"rs anggrek", "RS Bougenville", "RS Cempaka"}) {
				t.Fatal(got)
			}
		}},
		{"skor tertinggi dulu, lalu jarak", func() []Hospital {
			near, far, better := rsA("curated"), rsA("curated"), rsA("curated")
			near.ID, far.ID, better.ID = "near", "far", "better"
			near.Latitude, near.Longitude = atKM(2)
			far.Latitude, far.Longitude = atKM(10)
			near.Services[0].Status = "limited"
			better.Latitude, better.Longitude = atKM(3)
			return []Hospital{far, near, better}
		}, nicuReq, func(t *testing.T, res RecommendResult) {
			ids := []string{}
			for _, r := range res.Recommendations {
				ids = append(ids, r.Hospital.ID)
			}
			if !slices.Equal(ids, []string{"better", "far", "near"}) {
				t.Fatal(ids, res.Recommendations[0].Score, res.Recommendations[1].Score, res.Recommendations[2].Score)
			}
		}},
		{"limit memotong hasil, excluded tetap terhitung", func() []Hospital {
			hs := []Hospital{}
			for i := 0; i < 5; i++ {
				h := rsA("curated")
				h.ID = string(rune('a' + i))
				hs = append(hs, h)
			}
			hs[4].Services[0].Status = "unavailable"
			return hs
		}, func() RecommendRequest { r := nicuReq(); r.Limit = 2; return r }, func(t *testing.T, res RecommendResult) {
			if len(res.Recommendations) != 2 || res.Considered != 5 || res.Excluded[ExcludedServiceUnavailable] != 1 {
				t.Fatalf("%+v", res)
			}
		}},
		{"level high/medium/low", one(func(*Hospital) {}), nicuReq, first(func(t *testing.T, r Recommendation) {
			for score, want := range map[int]string{75: "high", 74: "medium", 50: "medium", 49: "low"} {
				if RecommendLevel(score) != want {
					t.Fatal(score)
				}
			}
		})},
		{"alasan berbahasa Indonesia dengan sumber & tanggal", one(func(*Hospital) {}), nicuReq, first(func(t *testing.T, r Recommendation) {
			c, _ := component(r, "service_match")
			if c.Reason != "NICU tersedia (kurasi, 26 Okt 2026)" {
				t.Fatal(c.Reason)
			}
			d, _ := component(r, "distance")
			if d.Reason != "6,0 km garis lurus dari lokasi Anda (radius 20,0 km); bukan waktu tempuh" {
				t.Fatal(d.Reason)
			}
		})},
		{"tanpa kandidat → daftar kosong, bukan null", func() []Hospital { return nil }, nicuReq, func(t *testing.T, res RecommendResult) {
			if res.Recommendations == nil || len(res.Recommendations) != 0 || res.Considered != 0 {
				t.Fatalf("%+v", res)
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { c.check(t, Recommend(c.hs(), c.req(), qNow)) })
	}
	if len(cases) < 15 {
		t.Fatalf("PRD meminta >= 15 kasus, ada %d", len(cases))
	}
}

func TestRecommendDoesNotMutateInput(t *testing.T) {
	h := rsA("curated")
	in := []Hospital{h}
	Recommend(in, nicuReq(), qNow)
	if in[0].Services[0].Freshness != "" || in[0].DataQuality.Score != 0 {
		t.Fatal("input berubah")
	}
}

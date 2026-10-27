package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"siagars/backend/internal/domain"
	"siagars/backend/internal/repository"
	"siagars/backend/internal/service"
	"siagars/backend/seed"
	"sort"
	"strconv"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// syntheticHospitals adds fictional, mixed-origin hospitals around the seed so every filter has
// both matching and non-matching rows, including freshness boundaries.
func syntheticHospitals(n int, now time.Time) []repository.HospitalDoc {
	r := rand.New(rand.NewSource(7))
	cities := [][2]string{{"3171", "31"}, {"3271", "32"}, {"3275", "32"}, {"3671", "36"}}
	words := []string{"Sehat", "Harapan", "Medika", "Bunda", "Sentosa", "Mulia", "Kasih", "Prima"}
	slugs := []string{"icu", "nicu", "picu", "igd-24h", "ambulance"}
	statuses := []string{"available", "limited", "unavailable", "unknown"}
	ages := []time.Duration{-time.Minute, 0, time.Hour - time.Millisecond, time.Hour, time.Hour + time.Millisecond, 5 * time.Hour, 72 * time.Hour}
	at := func(d time.Duration) *time.Time { v := now.Add(-d); return &v }
	out := make([]repository.HospitalDoc, 0, n)
	for i := 0; i < n; i++ {
		c := cities[r.Intn(len(cities))]
		origin := []string{"curated", "satusehat_sandbox", "simulation"}[r.Intn(3)]
		d := repository.HospitalDoc{ID: fmt.Sprintf("20000000-0000-4000-8000-%012d", i), Name: fmt.Sprintf("%s %s %s %d", []string{"RS", "rs", "RSU", "Klinik"}[r.Intn(4)], words[r.Intn(len(words))], words[r.Intn(len(words))], r.Intn(40)),
			CityCode: c[0], ProvinceCode: c[1], ProvenanceDoc: repository.ProvenanceDoc{DataOrigin: origin, SourceUpdatedAt: at(ages[r.Intn(len(ages))])}, Services: []repository.ServiceDoc{}, Beds: []repository.BedDoc{}}
		if origin != "curated" && origin != "simulation" {
			ext, ns := strconv.Itoa(i), "test-ns"
			d.ExternalID, d.SourceNamespace = &ext, &ns
		}
		if r.Intn(4) > 0 {
			lat, lng := -6.0-r.Float64(), 106.5+r.Float64()
			d.Latitude, d.Longitude = &lat, &lng
		}
		if b := r.Intn(3); b < 2 {
			v := b == 0
			d.BPJS = &v
		}
		for _, s := range slugs {
			if r.Intn(2) == 0 {
				d.Services = append(d.Services, repository.ServiceDoc{Slug: s, Status: statuses[r.Intn(4)], ProvenanceDoc: repository.ProvenanceDoc{DataOrigin: []string{"curated", "simulation"}[r.Intn(2)], SourceUpdatedAt: at(ages[r.Intn(len(ages))])}, Version: 1})
			}
		}
		for _, cat := range []string{"icu", "nicu", "picu", "general"} {
			if r.Intn(2) == 0 {
				var total, avail *int
				if r.Intn(4) > 0 {
					t, a := 10, r.Intn(3)
					total, avail = &t, &a
				}
				d.Beds = append(d.Beds, repository.BedDoc{Category: cat, Total: total, Available: avail, ProvenanceDoc: repository.ProvenanceDoc{DataOrigin: []string{"curated", "simulation", "satusehat_sandbox"}[r.Intn(3)], SourceUpdatedAt: at(ages[r.Intn(len(ages))])}, Version: 1})
			}
		}
		out = append(out, d)
	}
	return out
}

func loadSynthetic(t *testing.T, p *repository.Mongo, n int, now time.Time) {
	t.Helper()
	docs := make([]any, 0, n)
	for _, d := range syntheticHospitals(n, now) {
		docs = append(docs, d)
	}
	if _, e := p.DB.Collection("hospitals").InsertMany(context.Background(), docs); e != nil {
		t.Fatal(e)
	}
}

func canonical(t *testing.T, v any) string {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func ids(hs []domain.Hospital) []string {
	out := make([]string, len(hs))
	for i, h := range hs {
		out[i] = h.ID
	}
	return out
}

// TestSearchPushdownMatchesInMemory is the equivalence guard: for many generated queries the
// database path must return exactly what service.Filter returns over the full snapshot.
func TestSearchPushdownMatchesInMemory(t *testing.T) {
	p := database(t)
	ctx := context.Background()
	if e := seed.Apply(ctx, p); e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	loadSynthetic(t, p, 300, now)
	inactive := bson.D{{Key: "_id", Value: bson.D{{Key: "$regex", Value: "^20000000-0000-4000-8000-00000000000"}}}}
	if _, e := p.DB.Collection("hospitals").UpdateMany(ctx, inactive, bson.D{{Key: "$set", Value: bson.D{{Key: "source_active", Value: false}}}}); e != nil {
		t.Fatal(e)
	}
	all, e := p.Hospitals(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if len(all) != 30+300-10 {
		t.Fatal("snapshot size", len(all))
	}
	r := rand.New(rand.NewSource(11))
	slugs := []string{"icu", "nicu", "picu", "igd-24h", "ambulance"}
	lat, lng := -6.3, 106.8
	yes, no := true, false
	nonEmpty := 0
	for i := 0; i < 160; i++ {
		s := service.Search{Page: 1 + r.Intn(3), Limit: []int{1, 5, 20, 50}[r.Intn(4)], Sort: []string{"name", "name", "distance", "match"}[r.Intn(4)], Dataset: []string{"real", "simulation", "all"}[r.Intn(3)]}
		if r.Intn(3) == 0 {
			s.Province = []string{"31", "32", "36"}[r.Intn(3)]
		}
		if r.Intn(3) == 0 {
			s.City = []string{"3171", "3271", "3275", "3671", "3674"}[r.Intn(5)]
		}
		s.Q = []string{"", "", "rs", "SEHAT", "sehat jakarta", "rs sehat", "bogor", "banten", "zzz-none", "% (.*)", "simulasi siaga 0"}[r.Intn(11)]
		s.BPJS = []*bool{nil, &yes, &no}[r.Intn(3)]
		s.HasBeds = r.Intn(3) == 0
		for _, slug := range slugs {
			if r.Intn(5) == 0 {
				s.Services = append(s.Services, slug)
			}
		}
		if s.Sort == "distance" || r.Intn(3) == 0 {
			s.Lat, s.Lng = &lat, &lng
		}
		want, wantTotal := service.Filter(all, s, now)
		got, gotTotal, e := p.SearchHospitals(ctx, s, now)
		if e != nil {
			t.Fatalf("%+v: %v", s, e)
		}
		if gotTotal != wantTotal || canonical(t, got) != canonical(t, want) {
			t.Fatalf("case %d differs for %+v\ntotal got %d want %d\ngot  %v\nwant %v", i, s, gotTotal, wantTotal, ids(got), ids(want))
		}
		if wantTotal > 0 {
			nonEmpty++
		}
	}
	t.Logf("pushdown equals in-memory for 160 generated queries (%d non-empty)", nonEmpty)
	if nonEmpty < 40 {
		t.Fatal("generator too selective to be meaningful", nonEmpty)
	}
	if _, e := p.Hospital(ctx, "20000000-0000-4000-8000-000000000000"); e != repository.ErrNotFound {
		t.Fatal("inactive hospital must be hidden", e)
	}
	h, e := p.Hospital(ctx, seed.HospitalID(1))
	if e != nil || h.CityName == "" || len(h.Services) != 6 {
		t.Fatal("detail", h, e)
	}
}

// TestSearchLatency measures read latency, in-memory path versus pushdown, against a temporary
// test database. It runs only with SEARCH_BENCH=1 and prints numbers; it asserts nothing about speed.
// Latency includes the network round trip to whatever TEST_MONGODB_URI points at.
func TestSearchLatency(t *testing.T) {
	if os.Getenv("SEARCH_BENCH") != "1" {
		t.Skip("set SEARCH_BENCH=1 to measure")
	}
	p := database(t)
	ctx := context.Background()
	if e := seed.Apply(ctx, p); e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	size, _ := strconv.Atoi(os.Getenv("SEARCH_BENCH_N"))
	if size == 0 {
		size = 3000
	}
	loadSynthetic(t, p, size, now)
	lat, lng := -6.3, 106.8
	cases := map[string]service.Search{
		"default page 1":        {Page: 1, Limit: 20, Sort: "name", Dataset: "real"},
		"province+services":     {Page: 1, Limit: 20, Sort: "name", Dataset: "real", Province: "32", Services: []string{"icu"}},
		"q=sehat":               {Page: 1, Limit: 20, Sort: "name", Dataset: "real", Q: "sehat"},
		"sort=distance":         {Page: 1, Limit: 20, Sort: "distance", Dataset: "real", Lat: &lat, Lng: &lng},
		"has_available_beds+q":  {Page: 1, Limit: 20, Sort: "name", Dataset: "all", Q: "rs", HasBeds: true},
		"sort=match 2 services": {Page: 1, Limit: 20, Sort: "match", Dataset: "real", Services: []string{"icu", "nicu"}},
	}
	names := make([]string, 0, len(cases))
	for n := range cases {
		names = append(names, n)
	}
	sort.Strings(names)
	measure := func(f func() error) (p50, p95 time.Duration) {
		var d []time.Duration
		for i := 0; i < 40; i++ {
			start := time.Now()
			if e := f(); e != nil {
				t.Fatal(e)
			}
			d = append(d, time.Since(start))
		}
		sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
		return d[len(d)/2], d[(len(d)*95+99)/100-1]
	}
	t.Logf("hospitals in test database: %d synthetic + 30 seed; 40 runs per case", size)
	for _, n := range names {
		s := cases[n]
		a50, a95 := measure(func() error {
			hs, e := p.Hospitals(ctx)
			if e == nil {
				service.Filter(hs, s, now)
			}
			return e
		})
		b50, b95 := measure(func() error { _, _, e := p.SearchHospitals(ctx, s, now); return e })
		t.Logf("%-24s in-memory p50=%-8v p95=%-8v | pushdown p50=%-8v p95=%v", n, a50, a95, b50, b95)
	}
}

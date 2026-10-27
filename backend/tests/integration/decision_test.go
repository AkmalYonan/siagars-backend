package integration

import (
	"context"
	"net/url"
	"siagars/backend/internal/service"
	"siagars/backend/seed"
	"sort"
	"strings"
	"testing"
)

// TestAreaQueriesMatchInMemory checks that the database candidate and marker queries return exactly the
// hospitals the in-memory fallback would, on the seed (30 synthetic hospitals around Jabodetabek).
func TestAreaQueriesMatchInMemory(t *testing.T) {
	p := database(t)
	ctx := context.Background()
	if e := seed.Apply(ctx, p); e != nil {
		t.Fatal(e)
	}
	all, e := p.Hospitals(ctx)
	if e != nil {
		t.Fatal(e)
	}
	idsOf := func(hs []string) []string { sort.Strings(hs); return hs }
	for _, raw := range []string{"services=nicu&lat=-6.25&lng=106.8&radius_km=10&dataset=simulation", "services=icu&city_code=3275&dataset=all", "services=nicu&province_code=32&dataset=simulation", "services=nicu&lat=-6.25&lng=106.8&dataset=real"} {
		v, _ := url.ParseQuery(raw)
		q, e := service.ParseRecommend(v)
		if e != nil {
			t.Fatal(raw, e)
		}
		db, _, e := p.RecommendCandidates(ctx, q)
		if e != nil {
			t.Fatal(e)
		}
		mem, _ := service.RecommendCandidates(all, q)
		a, b := []string{}, []string{}
		for _, h := range db {
			a = append(a, h.ID)
		}
		for _, h := range mem {
			b = append(b, h.ID)
		}
		if got, want := strings.Join(idsOf(a), ","), strings.Join(idsOf(b), ","); got != want {
			t.Fatalf("%s: db %v, memori %v", raw, got, want)
		}
	}
	v, _ := url.ParseQuery("bbox=106.6,-6.4,107.0,-6.1&dataset=simulation&bpjs=true")
	mq, e := service.ParseMarkers(v)
	if e != nil {
		t.Fatal(e)
	}
	db, _, e := p.MarkerHospitals(ctx, mq)
	if e != nil {
		t.Fatal(e)
	}
	mem, _ := service.MarkerHospitals(all, mq)
	if len(db) != len(mem) || len(db) == 0 {
		t.Fatalf("markers db %d, memori %d", len(db), len(mem))
	}
}

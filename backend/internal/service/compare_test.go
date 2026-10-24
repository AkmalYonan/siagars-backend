package service

import (
	"net/url"
	"siagars/backend/internal/domain"
	"strings"
	"testing"
	"time"
)

const (
	idA       = "10000000-0000-4000-8000-000000000001"
	idB       = "10000000-0000-4000-8000-000000000002"
	idC       = "10000000-0000-4000-8000-000000000003"
	idD       = "10000000-0000-4000-8000-000000000004"
	idLetters = "abcdef00-0000-4000-8000-000000000001"
)

func uuidLike(s string) bool {
	return len(s) == 36 && strings.Count(s, "-") == 4 && s == strings.ToLower(s)
}

func TestParseCompare(t *testing.T) {
	good := map[string][]string{
		"ids=" + idA + "," + idB:              {idA, idB},
		"ids=" + idC + "," + idA + "," + idB:  {idC, idA, idB},
		"ids=" + url.QueryEscape(idB+","+idA): {idB, idA},
	}
	for raw, want := range good {
		q, _ := url.ParseQuery(raw)
		got, e := ParseCompare(q, uuidLike)
		if e != nil || strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("%s: %v %v", raw, got, e)
		}
	}
	bad := []string{"", "ids=", "ids=" + idA, "ids=" + idA + "," + idA, "ids=" + idA + "," + idB + "," + idA,
		"ids=" + idA + "," + idB + "," + idC + "," + idD, "ids=" + idA + ",", "ids=," + idB, "ids=" + idA + ",xyz",
		"ids=" + idA + "," + strings.ToUpper(idLetters), "ids=" + idA + "," + idB + "&ids=" + idC, "ids=" + idA + "," + idB + "&page=1",
		"ids=" + idA + ", " + idB, "foo=1"}
	for _, raw := range bad {
		q, _ := url.ParseQuery(raw)
		if got, e := ParseCompare(q, uuidLike); e == nil {
			t.Errorf("accepted %q -> %v", raw, got)
		}
	}
}

func bed(avail *int, origin string, src *time.Time) domain.Bed {
	return domain.Bed{Category: "icu", Available: avail, Provenance: domain.Provenance{DataOrigin: origin, SourceUpdatedAt: src}}
}

// has_available_beds=true: available>0, source_updated_at strictly younger than one hour and not in
// the future, origin not simulation. Unknown (null) and 0 never match; false does not filter.
func TestHasAvailableBedsSemantics(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) *time.Time { v := now.Add(-d); return &v }
	zero, two := 0, 2
	cases := []struct {
		name string
		bed  domain.Bed
		want bool
	}{
		{"fresh available", bed(&two, "curated", ago(time.Minute)), true},
		{"1ms under one hour", bed(&two, "curated", ago(time.Hour-time.Millisecond)), true},
		{"exactly one hour is aging", bed(&two, "curated", ago(time.Hour)), false},
		{"stale", bed(&two, "curated", ago(48*time.Hour)), false},
		{"future timestamp is unknown", bed(&two, "curated", ago(-time.Second)), false},
		{"null source_updated_at", bed(&two, "satusehat_sandbox", nil), false},
		{"zero available", bed(&zero, "curated", ago(time.Minute)), false},
		{"unknown available (null)", bed(nil, "curated", ago(time.Minute)), false},
		{"simulation never proves availability", bed(&two, "simulation", ago(time.Minute)), false},
		{"sandbox fresh counts", bed(&two, "satusehat_sandbox", ago(time.Minute)), true},
	}
	for _, c := range cases {
		hs := []domain.Hospital{{ID: "h", DataOrigin: "curated", Beds: []domain.Bed{c.bed}}}
		_, n := Filter(hs, Search{Page: 1, Limit: 20, Dataset: "all", HasBeds: true}, now)
		if (n == 1) != c.want {
			t.Errorf("%s: matched=%d want %v", c.name, n, c.want)
		}
		if _, n = Filter(hs, Search{Page: 1, Limit: 20, Dataset: "all"}, now); n != 1 {
			t.Errorf("%s: without the filter the hospital must remain", c.name)
		}
	}
}

func place(id, name string, lat, lng *float64) domain.Hospital {
	return domain.Hospital{ID: id, Name: name, DataOrigin: "curated", Latitude: lat, Longitude: lng}
}

func order(hs []domain.Hospital) string {
	var ids []string
	for _, h := range hs {
		ids = append(ids, h.ID)
	}
	return strings.Join(ids, ",")
}

// sort=distance: nearest first; unknown coordinates last (distance null, never 0); ties by name then ID.
func TestSortDistanceSemantics(t *testing.T) {
	now := time.Now().UTC()
	f := func(v float64) *float64 { return &v }
	origin := Search{Page: 1, Limit: 20, Dataset: "all", Sort: "distance", Lat: f(-6.2), Lng: f(106.8)}
	hs := []domain.Hospital{
		place("unknown", "Aaa", nil, nil), place("far", "Bbb", f(-6.9), f(107.6)), place("near", "Zzz", f(-6.21), f(106.81)),
		place("tie2", "Same", f(-6.3), f(106.8)), place("tie1", "Same", f(-6.3), f(106.8)), place("at-origin", "Mmm", f(-6.2), f(106.8)),
	}
	out, n := Filter(hs, origin, now)
	if n != 6 || order(out) != "at-origin,near,tie1,tie2,far,unknown" {
		t.Fatal(order(out))
	}
	for _, h := range out {
		if (h.ID == "unknown") != (h.DistanceKM == nil) {
			t.Errorf("%s distance %v", h.ID, h.DistanceKM)
		}
	}
	if *out[0].DistanceKM != 0 || *out[1].DistanceKM <= 0 {
		t.Error("distance in km must be Haversine and 0 at the origin")
	}
}

// sort=match: wanted-service status (available +10, limited +5), then freshness (+3/+1), then
// distance when lat/lng exist, then name. Simulation earns no bonus.
func TestSortMatchSemantics(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	f := func(v float64) *float64 { return &v }
	src := func(d time.Duration) *time.Time { v := now.Add(-d); return &v }
	svc := func(status, origin string, age time.Duration) []domain.Service {
		return []domain.Service{{Slug: "icu", Status: status, Provenance: domain.Provenance{DataOrigin: origin, SourceUpdatedAt: src(age)}}}
	}
	mk := func(id string, lat float64, s []domain.Service) domain.Hospital {
		h := place(id, id, f(lat), f(106.8))
		h.Services = s
		return h
	}
	hs := []domain.Hospital{
		mk("limited-fresh", -6.2, svc("limited", "curated", time.Minute)),       // 5+3
		mk("avail-stale", -6.2, svc("available", "curated", 72*time.Hour)),      // 10
		mk("avail-aging", -6.2, svc("available", "curated", 5*time.Hour)),       // 11
		mk("avail-fresh-far", -6.9, svc("available", "curated", time.Minute)),   // 13
		mk("avail-fresh-near", -6.21, svc("available", "curated", time.Minute)), // 13
		mk("simulated", -6.2, svc("available", "simulation", time.Minute)),      // 0
	}
	s := Search{Page: 1, Limit: 20, Dataset: "all", Sort: "match", Services: []string{"icu"}, Lat: f(-6.2), Lng: f(106.8)}
	out, _ := Filter(hs, s, now)
	if got := order(out); got != "avail-fresh-near,avail-fresh-far,avail-aging,avail-stale,limited-fresh,simulated" {
		t.Fatal(got)
	}
	// Without lat/lng the tie falls to name, never to an arbitrary order.
	s.Lat, s.Lng = nil, nil
	out, _ = Filter(hs, s, now)
	if got := order(out); got != "avail-fresh-far,avail-fresh-near,avail-aging,avail-stale,limited-fresh,simulated" {
		t.Fatal(got)
	}
}

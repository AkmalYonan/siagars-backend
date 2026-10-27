package domain

import (
	"testing"
	"time"
)

func insightFixture(now time.Time) []Hospital {
	ago := func(d time.Duration) *time.Time { t := now.Add(-d); return &t }
	yes := true
	cls := func(c string) *Coded { return &Coded{Code: c} }
	return []Hospital{
		{ID: "1", CityCode: "3171", CityName: "Jakarta Selatan", FacilityClass: cls("A"), BPJS: &yes, Provenance: Provenance{DataOrigin: "curated", SourceUpdatedAt: ago(30 * time.Minute)},
			Services: []Service{{Slug: "nicu", Status: "available"}}},
		{ID: "2", CityCode: "3171", CityName: "Jakarta Selatan", FacilityClass: cls("b"), Provenance: Provenance{DataOrigin: "simulation", SourceUpdatedAt: ago(3 * time.Hour)},
			Services: []Service{{Slug: "nicu", Status: "unknown", Provenance: Provenance{DataOrigin: "simulation"}}, {Slug: "icu", Status: "limited"}},
			Beds:     []Bed{{Category: "icu", Provenance: Provenance{DataOrigin: "simulation"}}}},
		{ID: "3", CityCode: "3275", CityName: "Bekasi", Provenance: Provenance{DataOrigin: "satusehat", SourceUpdatedAt: ago(48 * time.Hour)},
			Services: []Service{{Slug: "nicu", Status: "unavailable"}}},
		{ID: "4", CityCode: "3275", CityName: "Bekasi", FacilityClass: cls("Z"), Provenance: Provenance{DataOrigin: "satusehat"}},
	}
}

func TestSummarize(t *testing.T) {
	now := time.Now().UTC()
	s := Summarize(insightFixture(now), now)
	if s.HospitalCount != 4 || s.CityCount != 2 || s.CuratedCount != 1 || s.SimulatedFacilityCount != 2 {
		t.Fatalf("%+v", s)
	}
	if s.FreshRatio == nil || *s.FreshRatio != 0.25 || *s.WithKnownBPJSRatio != 0.25 {
		t.Fatalf("%v %v", *s.FreshRatio, *s.WithKnownBPJSRatio)
	}
	if e := Summarize(nil, now); e.FreshRatio != nil || e.WithKnownBPJSRatio != nil || e.HospitalCount != 0 {
		t.Fatalf("kosong: %+v", e)
	}
}

func TestCoverage(t *testing.T) {
	c := Coverage(insightFixture(time.Now().UTC()))
	if len(c) != 2 || c[0].CityName != "Bekasi" || c[1].CityCode != "3171" {
		t.Fatalf("%+v", c)
	}
	if c[1].ByClass != (ClassCounts{A: 1, B: 1}) || c[1].CuratedCount != 1 || c[0].ByClass.Unknown != 2 {
		t.Fatalf("%+v", c)
	}
}

func TestServiceGaps(t *testing.T) {
	g := ServiceGaps(insightFixture(time.Now().UTC()), "nicu")
	if g[0].CityName != "Bekasi" || g[0].Unavailable != 1 || g[0].Unknown != 1 || !g[0].Gap {
		t.Fatalf("%+v", g[0])
	}
	if g[1].Available != 1 || g[1].Unknown != 1 || g[1].Gap {
		t.Fatalf("%+v", g[1])
	}
	// Only limited/available count against a gap; a service nobody records is unknown, not unavailable.
	icu := ServiceGaps(insightFixture(time.Now().UTC()), "icu")
	if icu[1].Limited != 1 || icu[1].Unknown != 1 || icu[1].Gap || !icu[0].Gap || icu[0].Unavailable != 0 {
		t.Fatalf("%+v", icu)
	}
}

func TestFreshnessByOrigin(t *testing.T) {
	now := time.Now().UTC()
	f := FreshnessByOrigin(insightFixture(now), now)
	want := []OriginFreshness{{DataOrigin: "curated", Fresh: 1}, {DataOrigin: "satusehat", Stale: 1, Unknown: 1}, {DataOrigin: "simulation", Aging: 1}}
	if len(f) != 3 || f[0] != want[0] || f[1] != want[1] || f[2] != want[2] {
		t.Fatalf("%+v", f)
	}
}

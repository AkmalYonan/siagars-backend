package domain

import (
	"sort"
	"strings"
	"time"
)

// Insights are aggregates over hospitals already filtered by dataset/province. They are pure functions of
// their input and now: a ratio is nil when its denominator is 0, and an unknown value is counted as
// unknown, never as 0 or false (contract §6).

type InsightSummary struct {
	HospitalCount          int      `json:"hospital_count"`
	CityCount              int      `json:"city_count"`
	CuratedCount           int      `json:"curated_count"`
	SimulatedFacilityCount int      `json:"simulated_facility_count"`
	FreshRatio             *float64 `json:"fresh_ratio"`
	WithKnownBPJSRatio     *float64 `json:"with_known_bpjs_ratio"`
}

type ClassCounts struct {
	A       int `json:"A"`
	B       int `json:"B"`
	C       int `json:"C"`
	D       int `json:"D"`
	Unknown int `json:"unknown"`
}

type CityCoverage struct {
	CityCode      string      `json:"city_code"`
	CityName      string      `json:"city_name"`
	HospitalCount int         `json:"hospital_count"`
	ByClass       ClassCounts `json:"by_class"`
	CuratedCount  int         `json:"curated_count"`
}

type CityServiceGap struct {
	CityCode    string `json:"city_code"`
	CityName    string `json:"city_name"`
	Available   int    `json:"available"`
	Limited     int    `json:"limited"`
	Unavailable int    `json:"unavailable"`
	Unknown     int    `json:"unknown"`
	// Gap means no hospital in the city is recorded as offering the service in our data; it is not a
	// factual claim about the region.
	Gap bool `json:"gap"`
}

type OriginFreshness struct {
	DataOrigin string `json:"data_origin"`
	Fresh      int    `json:"fresh"`
	Aging      int    `json:"aging"`
	Stale      int    `json:"stale"`
	Unknown    int    `json:"unknown"`
}

func ratio(n, d int) *float64 {
	if d == 0 {
		return nil
	}
	v := float64(n) / float64(d)
	return &v
}

func Summarize(hs []Hospital, now time.Time) InsightSummary {
	s := InsightSummary{HospitalCount: len(hs)}
	cities := map[string]bool{}
	fresh, bpjs := 0, 0
	for _, h := range hs {
		cities[h.CityCode] = true
		if h.DataOrigin == "curated" {
			s.CuratedCount++
		}
		if Freshness(h.SourceUpdatedAt, now) == "fresh" {
			fresh++
		}
		if h.BPJS != nil {
			bpjs++
		}
		for _, x := range h.Services {
			if x.DataOrigin == "simulation" {
				s.SimulatedFacilityCount++
			}
		}
		for _, b := range h.Beds {
			if b.DataOrigin == "simulation" {
				s.SimulatedFacilityCount++
			}
		}
	}
	s.CityCount = len(cities)
	s.FreshRatio, s.WithKnownBPJSRatio = ratio(fresh, len(hs)), ratio(bpjs, len(hs))
	return s
}

func classOf(h Hospital) string {
	if h.FacilityClass != nil {
		switch c := strings.ToUpper(strings.TrimSpace(h.FacilityClass.Code)); c {
		case "A", "B", "C", "D":
			return c
		}
	}
	return "unknown"
}

// cityKeys returns city codes ordered by name then code, so output is stable.
func cityKeys(names map[string]string) []string {
	keys := make([]string, 0, len(names))
	for k := range names {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if names[keys[i]] != names[keys[j]] {
			return names[keys[i]] < names[keys[j]]
		}
		return keys[i] < keys[j]
	})
	return keys
}

func Coverage(hs []Hospital) []CityCoverage {
	names, by := map[string]string{}, map[string]*CityCoverage{}
	for _, h := range hs {
		c := by[h.CityCode]
		if c == nil {
			names[h.CityCode] = h.CityName
			c = &CityCoverage{CityCode: h.CityCode, CityName: h.CityName}
			by[h.CityCode] = c
		}
		c.HospitalCount++
		if h.DataOrigin == "curated" {
			c.CuratedCount++
		}
		switch classOf(h) {
		case "A":
			c.ByClass.A++
		case "B":
			c.ByClass.B++
		case "C":
			c.ByClass.C++
		case "D":
			c.ByClass.D++
		default:
			c.ByClass.Unknown++
		}
	}
	out := make([]CityCoverage, 0, len(by))
	for _, k := range cityKeys(names) {
		out = append(out, *by[k])
	}
	return out
}

// ServiceGaps counts, per city, how each hospital records one service. A hospital with no entry for the
// service is unknown.
func ServiceGaps(hs []Hospital, slug string) []CityServiceGap {
	names, by := map[string]string{}, map[string]*CityServiceGap{}
	for _, h := range hs {
		g := by[h.CityCode]
		if g == nil {
			names[h.CityCode] = h.CityName
			g = &CityServiceGap{CityCode: h.CityCode, CityName: h.CityName}
			by[h.CityCode] = g
		}
		status := "unknown"
		for _, x := range h.Services {
			if x.Slug == slug && ValidServiceStatus(x.Status) {
				status = x.Status
				break
			}
		}
		switch status {
		case "available":
			g.Available++
		case "limited":
			g.Limited++
		case "unavailable":
			g.Unavailable++
		default:
			g.Unknown++
		}
	}
	out := make([]CityServiceGap, 0, len(by))
	for _, k := range cityKeys(names) {
		g := *by[k]
		g.Gap = g.Available+g.Limited == 0
		out = append(out, g)
	}
	return out
}

// FreshnessByOrigin buckets hospitals by data_origin and hospital-level freshness. An empty origin is
// reported as "unknown".
func FreshnessByOrigin(hs []Hospital, now time.Time) []OriginFreshness {
	by := map[string]*OriginFreshness{}
	for _, h := range hs {
		o := h.DataOrigin
		if o == "" {
			o = "unknown"
		}
		f := by[o]
		if f == nil {
			f = &OriginFreshness{DataOrigin: o}
			by[o] = f
		}
		switch Freshness(h.SourceUpdatedAt, now) {
		case "fresh":
			f.Fresh++
		case "aging":
			f.Aging++
		case "stale":
			f.Stale++
		default:
			f.Unknown++
		}
	}
	keys := make([]string, 0, len(by))
	for k := range by {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]OriginFreshness, 0, len(keys))
	for _, k := range keys {
		out = append(out, *by[k])
	}
	return out
}

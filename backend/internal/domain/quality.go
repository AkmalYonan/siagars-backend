package domain

import (
	"math"
	"time"
)

// DataQuality summarizes how complete, fresh and well-sourced a hospital's data is (contract §5). It is
// computed when the DTO is rendered and never stored. It rates the data, not the hospital.
type DataQuality struct {
	Score         int     `json:"score"`
	Label         string  `json:"label"`
	Completeness  float64 `json:"completeness"`
	Freshness     string  `json:"freshness"`
	PrimaryOrigin string  `json:"primary_origin"`
}

// QualityServices and QualityBeds are the key facility items counted by completeness, together with
// phone, coordinates and BPJS: 3 + 6 + 4 = 13 items.
var (
	QualityServices = []string{"icu", "nicu", "picu", "igd-24h", "ambulance", "rawat-inap"}
	QualityBeds     = []string{"icu", "nicu", "picu", "general"}
)

var (
	freshnessFactor = map[string]float64{"fresh": 1, "aging": 0.7, "stale": 0.3, "unknown": 0.2}
	freshnessRank   = map[string]int{"unknown": 0, "stale": 1, "aging": 2, "fresh": 3}
	originFactor    = map[string]float64{"curated": 1, "satusehat_sandbox": 0.8, "satusehat_production": 0.8, "simulation": 0.5}
)

// QualityLabel maps a score to baik (>= 75), cukup (50-74) or terbatas (< 50).
func QualityLabel(score int) string {
	switch {
	case score >= 75:
		return "baik"
	case score >= 50:
		return "cukup"
	}
	return "terbatas"
}

// ComputeDataQuality is a pure function of the hospital and now:
//
//	completeness = known key items / 13 (phone, lat+lng, bpjs, 6 service statuses, 4 bed categories)
//	freshness    = the worst freshness among the services and beds present (unknown when there are none)
//	origin       = the majority origin of those items, else the hospital's own origin
//	score        = round(100 × (0.5·completeness + 0.25·freshness_factor + 0.25·origin_factor))
//
// A service counts as known when its status is not "unknown"; a bed when "available" is not null (0 is
// known). Ties in the majority origin resolve to the lower-trust origin.
func ComputeDataQuality(h Hospital, now time.Time) DataQuality {
	known, total := 0, 3+len(QualityServices)+len(QualityBeds)
	if h.Phone != nil && *h.Phone != "" {
		known++
	}
	if h.Latitude != nil && h.Longitude != nil {
		known++
	}
	if h.BPJS != nil {
		known++
	}
	worst, items := "", 0
	origins := map[string]int{}
	note := func(p Provenance) {
		items++
		f := Freshness(p.SourceUpdatedAt, now)
		if worst == "" || freshnessRank[f] < freshnessRank[worst] {
			worst = f
		}
		if p.DataOrigin != "" {
			origins[p.DataOrigin]++
		}
	}
	for _, slug := range QualityServices {
		for _, s := range h.Services {
			if s.Slug == slug {
				if ValidServiceStatus(s.Status) && s.Status != "unknown" {
					known++
				}
				break
			}
		}
	}
	for _, category := range QualityBeds {
		for _, b := range h.Beds {
			if b.Category == category {
				if b.Available != nil {
					known++
				}
				break
			}
		}
	}
	for _, s := range h.Services {
		note(s.Provenance)
	}
	for _, b := range h.Beds {
		note(b.Provenance)
	}
	if items == 0 {
		worst = "unknown"
	}
	primary := h.DataOrigin
	best := 0
	for _, o := range []string{"simulation", "satusehat_sandbox", "satusehat_production", "curated"} {
		if origins[o] > best {
			primary, best = o, origins[o]
		}
	}
	completeness := float64(known) / float64(total)
	score := int(math.Round(100 * (0.5*completeness + 0.25*freshnessFactor[worst] + 0.25*originFactor[primary])))
	return DataQuality{Score: score, Label: QualityLabel(score), Completeness: math.Round(completeness*100) / 100, Freshness: worst, PrimaryOrigin: primary}
}

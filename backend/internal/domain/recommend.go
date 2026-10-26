package domain

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// AlgorithmVersion identifies the scoring formula below (contract §4). Raise it whenever a weight,
// value table, penalty or filter changes.
const AlgorithmVersion = "1"

// BaseWeights are the weights before renormalization; removed components hand theirs to the others.
var BaseWeights = map[string]float64{"service_match": 0.40, "bed_availability": 0.20, "distance": 0.20, "bpjs": 0.10, "data_confidence": 0.10}

// componentOrder fixes the order of components in a response.
var componentOrder = []string{"service_match", "bed_availability", "distance", "bpjs", "data_confidence"}

// SimulationPenalty multiplies service_match and bed_availability when the values they used include
// simulation data, so synthetic data never beats equivalent curated data.
const SimulationPenalty = 0.8

// BedCategoryFor maps a requested service to the bed category that measures its capacity.
var BedCategoryFor = map[string]string{"icu": "icu", "nicu": "nicu", "picu": "picu", "rawat-inap": "general"}

// RecommendRequest is a validated recommendation query. Lat/Lng are both set or both nil; RadiusKM
// applies only with them. ServiceNames labels requested slugs in reasons (from the service master).
type RecommendRequest struct {
	Services     []string
	ServiceNames map[string]string
	Lat, Lng     *float64
	RadiusKM     float64
	BPJSRequired bool
	Limit        int
}

// ScoreComponent is one explainable part of a score. Weight is the renormalized weight; Contribution is
// round(100 × weight × value × penalty).
type ScoreComponent struct {
	Key          string  `json:"key"`
	Label        string  `json:"label"`
	Weight       float64 `json:"weight"`
	Value        float64 `json:"value"`
	Contribution int     `json:"contribution"`
	Reason       string  `json:"reason"`
}

// RecommendedHospital is the compact hospital card of a recommendation.
type RecommendedHospital struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Address         *string         `json:"address"`
	CityName        string          `json:"city_name"`
	ProvinceName    string          `json:"province_name"`
	Phone           *string         `json:"phone"`
	Lat             *float64        `json:"lat"`
	Lng             *float64        `json:"lng"`
	BPJS            *bool           `json:"bpjs"`
	DataOrigin      string          `json:"data_origin"`
	FacilitySummary FacilitySummary `json:"facility_summary"`
	DataQuality     DataQuality     `json:"data_quality"`
}

// Recommendation is one scored hospital.
type Recommendation struct {
	Hospital   RecommendedHospital `json:"hospital"`
	Score      int                 `json:"score"`
	Level      string              `json:"level"`
	DistanceKM *float64            `json:"distance_km"`
	Components []ScoreComponent    `json:"components"`
	Unknowns   []string            `json:"unknowns"`
	Flags      []string            `json:"flags"`
}

// RecommendResult is the full answer: the top Limit recommendations and why the others were left out.
type RecommendResult struct {
	Recommendations []Recommendation
	Considered      int
	Excluded        map[string]int
}

// Exclusion reasons reported in meta.excluded.
const (
	ExcludedServiceUnavailable = "service_unavailable"
	ExcludedBPJSFalse          = "bpjs_false"
	ExcludedOutOfRadius        = "out_of_radius"
	ExcludedLocationUnknown    = "location_unknown"
	ExcludedNotOperational     = "not_operational"
)

// RecommendLevel maps a score to high (>= 75), medium (50-74) or low (< 50).
func RecommendLevel(score int) string {
	switch {
	case score >= 75:
		return "high"
	case score >= 50:
		return "medium"
	}
	return "low"
}

// Recommend scores candidates with the contract §4 algorithm. It is a pure function: no I/O, and now is
// the only clock. Candidates are hospitals the repository already limited to the requested region or
// bounding box and dataset; hard filters here exclude the rest and are counted per reason.
//
// Hard filters: a requested service "unavailable"; bpjs=false when BPJS is required; outside the radius,
// or no coordinates, when a location was given; operational=false. null is never treated as 0/false.
func Recommend(candidates []Hospital, req RecommendRequest, now time.Time) RecommendResult {
	out := RecommendResult{Considered: len(candidates), Excluded: map[string]int{}}
	for _, h := range candidates {
		// Refresh writes freshness into the item slices; copy them so the caller's data is untouched.
		h.Services, h.Beds = append([]Service(nil), h.Services...), append([]Bed(nil), h.Beds...)
		h.Refresh(now)
		if reason := exclusion(h, req); reason != "" {
			out.Excluded[reason]++
			continue
		}
		out.Recommendations = append(out.Recommendations, score(h, req))
	}
	sort.SliceStable(out.Recommendations, func(i, j int) bool {
		a, b := out.Recommendations[i], out.Recommendations[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if (a.DistanceKM == nil) != (b.DistanceKM == nil) {
			return a.DistanceKM != nil
		}
		if a.DistanceKM != nil && *a.DistanceKM != *b.DistanceKM {
			return *a.DistanceKM < *b.DistanceKM
		}
		an, bn := strings.ToLower(a.Hospital.Name), strings.ToLower(b.Hospital.Name)
		if an != bn {
			return an < bn
		}
		return a.Hospital.ID < b.Hospital.ID
	})
	if req.Limit > 0 && len(out.Recommendations) > req.Limit {
		out.Recommendations = out.Recommendations[:req.Limit]
	}
	if out.Recommendations == nil {
		out.Recommendations = []Recommendation{}
	}
	return out
}

func serviceOf(h Hospital, slug string) (Service, bool) {
	for _, s := range h.Services {
		if s.Slug == slug {
			return s, true
		}
	}
	return Service{}, false
}
func bedOf(h Hospital, category string) (Bed, bool) {
	for _, b := range h.Beds {
		if b.Category == category {
			return b, true
		}
	}
	return Bed{}, false
}

func exclusion(h Hospital, req RecommendRequest) string {
	if h.Operational != nil && !*h.Operational {
		return ExcludedNotOperational
	}
	for _, slug := range req.Services {
		if s, found := serviceOf(h, slug); found && s.Status == "unavailable" {
			return ExcludedServiceUnavailable
		}
	}
	if req.BPJSRequired && h.BPJS != nil && !*h.BPJS {
		return ExcludedBPJSFalse
	}
	if req.Lat != nil {
		if h.Latitude == nil || h.Longitude == nil {
			return ExcludedLocationUnknown
		}
		if Distance(*req.Lat, *req.Lng, *h.Latitude, *h.Longitude) > req.RadiusKM {
			return ExcludedOutOfRadius
		}
	}
	return ""
}

type part struct {
	key, label, reason string
	value, penalty     float64
}

func score(h Hospital, req RecommendRequest) Recommendation {
	rec := Recommendation{Unknowns: []string{}, Flags: []string{}}
	simulated := h.DataOrigin == "simulation"
	parts := []part{}

	// service_match: available 1.0 · limited 0.6 · unknown/missing 0.2.
	var sum float64
	var reasons []string
	penalty := 1.0
	for _, slug := range req.Services {
		s, found := serviceOf(h, slug)
		name := serviceName(req, h, slug)
		switch {
		case found && s.Status == "available":
			sum += 1
		case found && s.Status == "limited":
			sum += 0.6
		default:
			sum += 0.2
			rec.Unknowns = append(rec.Unknowns, "service_"+slug)
		}
		if found && s.Status != "unknown" && s.DataOrigin == "simulation" {
			penalty, simulated = SimulationPenalty, true
		}
		status := "belum diketahui"
		if found {
			status = map[string]string{"available": "tersedia", "limited": "terbatas", "unknown": "belum diketahui"}[s.Status]
		}
		reasons = append(reasons, name+" "+status+provenanceNote(found && s.Status != "unknown", s.Provenance))
	}
	parts = append(parts, part{"service_match", "Layanan yang dibutuhkan", strings.Join(reasons, "; "), sum / float64(len(req.Services)), penalty})

	// bed_availability: available > 0 → 1.0 · 0 → 0.0 · null/missing → 0.3; removed without a relevant category.
	sum, reasons, penalty = 0, nil, 1.0
	categories := 0
	for _, slug := range req.Services {
		category, relevant := BedCategoryFor[slug]
		if !relevant {
			continue
		}
		categories++
		b, found := bedOf(h, category)
		label := "Bed " + bedLabel(category)
		switch {
		case found && b.Available != nil && *b.Available > 0:
			sum += 1
			reasons = append(reasons, fmt.Sprintf("%s: %d tersedia%s", label, *b.Available, provenanceNote(true, b.Provenance)))
		case found && b.Available != nil:
			reasons = append(reasons, fmt.Sprintf("%s: penuh (0 tersedia)%s", label, provenanceNote(true, b.Provenance)))
		default:
			sum += 0.3
			reasons = append(reasons, label+": jumlah belum diketahui")
			rec.Unknowns = append(rec.Unknowns, "bed_"+category)
		}
		if found && b.Available != nil && b.DataOrigin == "simulation" {
			penalty, simulated = SimulationPenalty, true
		}
	}
	if categories > 0 {
		parts = append(parts, part{"bed_availability", "Ketersediaan tempat tidur", strings.Join(reasons, "; "), sum / float64(categories), penalty})
	}

	// distance: max(0, 1 − d/radius), straight-line Haversine; removed without a location.
	if req.Lat != nil && h.Latitude != nil && h.Longitude != nil {
		d := Distance(*req.Lat, *req.Lng, *h.Latitude, *h.Longitude)
		rec.DistanceKM = &d
		parts = append(parts, part{"distance", "Jarak", fmt.Sprintf("%s km garis lurus dari lokasi Anda (radius %s km); bukan waktu tempuh", decimalID(d), decimalID(req.RadiusKM)), math.Max(0, 1-d/req.RadiusKM), 1})
	}

	// bpjs: only when required; true 1.0 · null 0.3 (false was excluded).
	if req.BPJSRequired {
		if h.BPJS != nil {
			origin := Provenance{}
			if h.BPJSDataOrigin != nil {
				origin.DataOrigin = *h.BPJSDataOrigin
				if origin.DataOrigin == "simulation" {
					simulated = true
				}
			}
			origin.SourceUpdatedAt = h.BPJSSourceUpdatedAt
			parts = append(parts, part{"bpjs", "Menerima BPJS", "Menerima pasien BPJS" + provenanceNote(h.BPJSDataOrigin != nil, origin), 1, 1})
		} else {
			rec.Unknowns = append(rec.Unknowns, "bpjs")
			parts = append(parts, part{"bpjs", "Menerima BPJS", "Status kerja sama BPJS belum diketahui", 0.3, 1})
		}
	}

	// data_confidence: data_quality.score / 100.
	parts = append(parts, part{"data_confidence", "Keyakinan data", fmt.Sprintf("Kualitas data %s (%d/100)", h.DataQuality.Label, h.DataQuality.Score), float64(h.DataQuality.Score) / 100, 1})

	var total float64
	for _, p := range parts {
		total += BaseWeights[p.key]
	}
	var raw float64
	for _, key := range componentOrder {
		for _, p := range parts {
			if p.key != key {
				continue
			}
			w := BaseWeights[p.key] / total
			raw += w * p.value * p.penalty
			rec.Components = append(rec.Components, ScoreComponent{Key: p.key, Label: p.label, Weight: round4(w), Value: round4(p.value), Contribution: int(math.Round(100 * w * p.value * p.penalty)), Reason: p.reason})
		}
	}
	// A tiny epsilon keeps exact halves (e.g. 91.99999999) from rounding down through float error.
	rec.Score = int(math.Round(100*raw + 1e-9))
	rec.Level = RecommendLevel(rec.Score)
	if simulated {
		rec.Flags = append(rec.Flags, "contains_simulated_data")
	}
	if f := h.DataQuality.Freshness; f == "stale" || f == "unknown" {
		rec.Flags = append(rec.Flags, "stale_or_unknown_freshness")
	}
	rec.Hospital = RecommendedHospital{ID: h.ID, Name: h.Name, Address: h.Address, CityName: h.CityName, ProvinceName: h.ProvinceName, Phone: h.Phone, Lat: h.Latitude, Lng: h.Longitude, BPJS: h.BPJS, DataOrigin: h.DataOrigin, FacilitySummary: h.Summary, DataQuality: h.DataQuality}
	return rec
}

func serviceName(req RecommendRequest, h Hospital, slug string) string {
	if s, found := serviceOf(h, slug); found && s.Name != "" {
		return s.Name
	}
	if n := req.ServiceNames[slug]; n != "" {
		return n
	}
	return strings.ToUpper(slug)
}

func bedLabel(category string) string {
	if category == "general" {
		return "rawat inap"
	}
	return strings.ToUpper(category)
}

var originLabel = map[string]string{"curated": "kurasi", "simulation": "simulasi", "satusehat_sandbox": "SATUSEHAT sandbox", "satusehat_production": "SATUSEHAT"}

// provenanceNote renders " (kurasi, 25 Okt 2026)" for a known value; empty when the value is unknown.
func provenanceNote(known bool, p Provenance) string {
	if !known || p.DataOrigin == "" {
		return ""
	}
	label := originLabel[p.DataOrigin]
	if label == "" {
		label = p.DataOrigin
	}
	if p.SourceUpdatedAt != nil {
		return " (" + label + ", " + dateID(*p.SourceUpdatedAt) + ")"
	}
	return " (" + label + ")"
}

var monthsID = [...]string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}
var wibZone = time.FixedZone("WIB", 7*60*60)

// dateID formats a date the Indonesian way in WIB, e.g. "25 Okt 2026".
func dateID(t time.Time) string {
	t = t.In(wibZone)
	return fmt.Sprintf("%d %s %d", t.Day(), monthsID[t.Month()-1], t.Year())
}

// decimalID formats one decimal with a comma, e.g. 6.04 → "6,0".
func decimalID(f float64) string { return strings.Replace(fmt.Sprintf("%.1f", f), ".", ",", 1) }

func round4(f float64) float64 { return math.Round(f*10000) / 10000 }

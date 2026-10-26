package service

import (
	"fmt"
	"math"
	"net/url"
	"siagars/backend/internal/domain"
	"strconv"
	"strings"
)

// MaxCandidates caps how many hospitals are scored for one recommendation request (PRD §6).
const MaxCandidates = 500

// MaxMarkers caps one markers response (contract §7).
const MaxMarkers = 500

// BBox is a longitude/latitude rectangle.
type BBox struct{ MinLng, MinLat, MaxLng, MaxLat float64 }

// Contains reports whether a hospital has coordinates inside the box (edges included).
func (b BBox) Contains(h domain.Hospital) bool {
	return h.Latitude != nil && h.Longitude != nil && *h.Latitude >= b.MinLat && *h.Latitude <= b.MaxLat && *h.Longitude >= b.MinLng && *h.Longitude <= b.MaxLng
}

// RadiusBBox is a box that surely contains the circle of radius km around (lat, lng); the exact
// Haversine test happens in domain.Recommend. It widens a little to stay safe near the box corners.
func RadiusBBox(lat, lng, km float64) BBox {
	dLat := km / 110.574 * 1.01
	cos := math.Cos(lat * math.Pi / 180)
	dLng := 180.0
	if cos > 0.01 {
		dLng = km / (111.320 * cos) * 1.01
	}
	return BBox{MinLng: math.Max(-180, lng-dLng), MinLat: math.Max(-90, lat-dLat), MaxLng: math.Min(180, lng+dLng), MaxLat: math.Min(90, lat+dLat)}
}

// RecommendQuery is a validated GET /recommendations query: the scoring request plus where to look.
type RecommendQuery struct {
	domain.RecommendRequest
	Province, City, Dataset string
	// Area is set when lat/lng were given: the radius box candidates are taken from.
	Area *BBox
}

var radii = map[string]float64{"5": 5, "10": 10, "20": 20, "50": 50}

// ParseRecommend validates GET /recommendations (contract §4.1). services is required (1..6 distinct
// slugs; existence is checked by the handler); lat+lng or province_code/city_code is required;
// radius_km only with lat/lng. Unknown, empty or repeated parameters are rejected.
func ParseRecommend(v url.Values) (RecommendQuery, error) {
	q := RecommendQuery{Dataset: "real"}
	q.RadiusKM, q.Limit = 20, 10
	allowed := map[string]bool{"services": true, "lat": true, "lng": true, "radius_km": true, "province_code": true, "city_code": true, "bpjs": true, "dataset": true, "limit": true}
	for k, values := range v {
		if !allowed[k] || len(values) != 1 || values[0] == "" {
			return q, fmt.Errorf("parameter tidak didukung, kosong atau berulang: %s", k)
		}
	}
	raw := v.Get("services")
	if raw == "" {
		return q, fmt.Errorf("services wajib diisi (minimal 1 slug layanan)")
	}
	seen := map[string]bool{}
	for _, slug := range strings.Split(raw, ",") {
		if slug == "" || strings.TrimSpace(slug) != slug || seen[slug] {
			return q, fmt.Errorf("services tidak valid")
		}
		seen[slug] = true
		q.Services = append(q.Services, slug)
	}
	if len(q.Services) > 6 {
		return q, fmt.Errorf("services maksimal 6 slug")
	}
	if (v.Get("lat") == "") != (v.Get("lng") == "") {
		return q, fmt.Errorf("lat/lng harus berpasangan")
	}
	if v.Get("lat") != "" {
		lat, lng, e := parseLatLng(v.Get("lat"), v.Get("lng"))
		if e != nil {
			return q, e
		}
		q.Lat, q.Lng = &lat, &lng
	}
	if x := v.Get("radius_km"); x != "" {
		r, known := radii[x]
		if !known {
			return q, fmt.Errorf("radius_km harus 5, 10, 20 atau 50")
		}
		if q.Lat == nil {
			return q, fmt.Errorf("radius_km hanya berlaku bersama lat/lng")
		}
		q.RadiusKM = r
	}
	q.Province, q.City = v.Get("province_code"), v.Get("city_code")
	if q.Lat == nil && q.Province == "" && q.City == "" {
		return q, fmt.Errorf("lokasi wajib: lat+lng atau province_code/city_code")
	}
	switch x := v.Get("bpjs"); x {
	case "", "any":
	case "required":
		q.BPJSRequired = true
	default:
		return q, fmt.Errorf("bpjs harus required atau any")
	}
	if x := v.Get("dataset"); x != "" {
		if x != "real" && x != "simulation" && x != "all" {
			return q, fmt.Errorf("dataset harus real, simulation atau all")
		}
		q.Dataset = x
	}
	if x := v.Get("limit"); x != "" {
		n, e := strconv.Atoi(x)
		if e != nil || n < 1 || n > 20 {
			return q, fmt.Errorf("limit harus 1 sampai 20")
		}
		q.Limit = n
	}
	if q.Lat != nil {
		box := RadiusBBox(*q.Lat, *q.Lng, q.RadiusKM)
		q.Area = &box
	}
	return q, nil
}

func parseLatLng(rawLat, rawLng string) (float64, float64, error) {
	lat, e := strconv.ParseFloat(rawLat, 64)
	if e != nil || math.IsNaN(lat) || math.IsInf(lat, 0) || lat < -90 || lat > 90 {
		return 0, 0, fmt.Errorf("lat tidak valid")
	}
	lng, e := strconv.ParseFloat(rawLng, 64)
	if e != nil || math.IsNaN(lng) || math.IsInf(lng, 0) || lng < -180 || lng > 180 {
		return 0, 0, fmt.Errorf("lng tidak valid")
	}
	return lat, lng, nil
}

// inDataset mirrors the dataset rule of search: real = everything except simulation.
func inDataset(h domain.Hospital, dataset string) bool {
	simulated := h.DataOrigin == "simulation"
	switch dataset {
	case "simulation":
		return simulated
	case "all":
		return true
	}
	return !simulated
}

// RecommendCandidates is the in-memory equivalent of the repository's candidate query, for stores that
// cannot filter in the database: dataset, region and the radius box. It returns at most MaxCandidates
// and whether more existed.
func RecommendCandidates(hs []domain.Hospital, q RecommendQuery) ([]domain.Hospital, bool) {
	out := []domain.Hospital{}
	for _, h := range hs {
		if !inDataset(h, q.Dataset) || q.Province != "" && h.ProvinceCode != q.Province || q.City != "" && h.CityCode != q.City || q.Area != nil && !q.Area.Contains(h) {
			continue
		}
		if len(out) == MaxCandidates {
			return out, true
		}
		out = append(out, h)
	}
	return out, false
}

// MarkersQuery is a validated GET /hospitals/markers query.
type MarkersQuery struct {
	BBox
	Services []string
	BPJS     *bool
	Dataset  string
}

// ParseMarkers validates GET /hospitals/markers (contract §7): bbox=minLng,minLat,maxLng,maxLat is
// required, min < max, and at most 2° × 2°; services (optional, AND for match), bpjs true|false,
// dataset real|simulation|all.
func ParseMarkers(v url.Values) (MarkersQuery, error) {
	q := MarkersQuery{Dataset: "real"}
	for k, values := range v {
		if (k != "bbox" && k != "services" && k != "bpjs" && k != "dataset") || len(values) != 1 || values[0] == "" {
			return q, fmt.Errorf("parameter tidak didukung, kosong atau berulang: %s", k)
		}
	}
	parts := strings.Split(v.Get("bbox"), ",")
	if len(parts) != 4 {
		return q, fmt.Errorf("bbox wajib: minLng,minLat,maxLng,maxLat")
	}
	var n [4]float64
	for i, p := range parts {
		f, e := strconv.ParseFloat(p, 64)
		if e != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return q, fmt.Errorf("bbox harus berisi 4 angka")
		}
		n[i] = f
	}
	q.BBox = BBox{MinLng: n[0], MinLat: n[1], MaxLng: n[2], MaxLat: n[3]}
	if q.MinLng < -180 || q.MaxLng > 180 || q.MinLat < -90 || q.MaxLat > 90 || q.MinLng >= q.MaxLng || q.MinLat >= q.MaxLat {
		return q, fmt.Errorf("bbox tidak valid (min harus lebih kecil dari max, dalam rentang koordinat)")
	}
	if q.MaxLng-q.MinLng > 2 || q.MaxLat-q.MinLat > 2 {
		return q, fmt.Errorf("bbox maksimal 2° × 2°; perbesar zoom peta")
	}
	if x := v.Get("services"); x != "" {
		seen := map[string]bool{}
		for _, slug := range strings.Split(x, ",") {
			if slug == "" || strings.TrimSpace(slug) != slug || seen[slug] {
				return q, fmt.Errorf("services tidak valid")
			}
			seen[slug] = true
			q.Services = append(q.Services, slug)
		}
	}
	if x := v.Get("bpjs"); x != "" {
		if x != "true" && x != "false" {
			return q, fmt.Errorf("bpjs harus true/false")
		}
		b := x == "true"
		q.BPJS = &b
	}
	if x := v.Get("dataset"); x != "" {
		if x != "real" && x != "simulation" && x != "all" {
			return q, fmt.Errorf("dataset harus real, simulation atau all")
		}
		q.Dataset = x
	}
	return q, nil
}

// MarkerHospitals is the in-memory equivalent of the repository's markers query: dataset, bbox and
// the bpjs filter (null never matches). It returns at most MaxMarkers and whether more existed.
func MarkerHospitals(hs []domain.Hospital, q MarkersQuery) ([]domain.Hospital, bool) {
	out := []domain.Hospital{}
	for _, h := range hs {
		if !inDataset(h, q.Dataset) || !q.BBox.Contains(h) || q.BPJS != nil && (h.BPJS == nil || *h.BPJS != *q.BPJS) {
			continue
		}
		if len(out) == MaxMarkers {
			return out, true
		}
		out = append(out, h)
	}
	return out, false
}

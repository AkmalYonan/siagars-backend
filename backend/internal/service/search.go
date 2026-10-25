package service

import (
	"fmt"
	"math"
	"net/url"
	"siagars/backend/internal/domain"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Search struct {
	Q, Province, City, Sort string
	// Dataset is "real" (everything except simulation; the default), "simulation" or "all".
	Dataset     string
	Services    []string
	BPJS        *bool
	HasBeds     bool
	Lat, Lng    *float64
	Page, Limit int
}

func ParseSearch(v url.Values) (Search, error) {
	s := Search{Page: 1, Limit: 20, Sort: "name", Dataset: "real"}
	allowed := map[string]bool{"q": true, "province_code": true, "city_code": true, "services": true, "bpjs": true, "has_available_beds": true, "lat": true, "lng": true, "sort": true, "page": true, "limit": true, "dataset": true}
	for k, values := range v {
		if !allowed[k] || len(values) != 1 || values[0] == "" {
			return s, fmt.Errorf("parameter tidak didukung, kosong atau berulang: %s", k)
		}
	}
	s.Q = strings.TrimSpace(v.Get("q"))
	s.Province = v.Get("province_code")
	s.City = v.Get("city_code")
	if utf8.RuneCountInString(s.Q) > 100 {
		return s, fmt.Errorf("q maksimal 100 karakter")
	}
	if x := v.Get("dataset"); x != "" {
		if x != "real" && x != "simulation" && x != "all" {
			return s, fmt.Errorf("dataset harus real, simulation atau all")
		}
		s.Dataset = x
	}
	if x := v.Get("sort"); x != "" {
		s.Sort = x
	}
	if s.Sort != "name" && s.Sort != "distance" && s.Sort != "match" {
		return s, fmt.Errorf("sort tidak valid")
	}
	for _, k := range []string{"page", "limit"} {
		if x := v.Get(k); x != "" {
			n, e := strconv.Atoi(x)
			if e != nil || n < 1 || (k == "limit" && n > 50) || n > 1000000 {
				return s, fmt.Errorf("%s tidak valid", k)
			}
			if k == "page" {
				s.Page = n
			} else {
				s.Limit = n
			}
		}
	}
	if x := v.Get("services"); x != "" {
		seen := map[string]bool{}
		for _, slug := range strings.Split(x, ",") {
			if slug == "" || strings.TrimSpace(slug) != slug || seen[slug] {
				return s, fmt.Errorf("services tidak valid")
			}
			seen[slug] = true
			s.Services = append(s.Services, slug)
		}
	}
	for _, k := range []string{"bpjs", "has_available_beds"} {
		if x := v.Get(k); x != "" {
			if x != "true" && x != "false" {
				return s, fmt.Errorf("%s harus true/false", k)
			}
			b := x == "true"
			if k == "bpjs" {
				s.BPJS = &b
			} else {
				s.HasBeds = b
			}
		}
	}
	if (v.Get("lat") == "") != (v.Get("lng") == "") {
		return s, fmt.Errorf("lat/lng harus berpasangan")
	}
	if v.Get("lat") != "" {
		a, e := strconv.ParseFloat(v.Get("lat"), 64)
		if e != nil || math.IsNaN(a) || math.IsInf(a, 0) || a < -90 || a > 90 {
			return s, fmt.Errorf("lat tidak valid")
		}
		b, e := strconv.ParseFloat(v.Get("lng"), 64)
		if e != nil || math.IsNaN(b) || math.IsInf(b, 0) || b < -180 || b > 180 {
			return s, fmt.Errorf("lng tidak valid")
		}
		s.Lat = &a
		s.Lng = &b
	}
	if s.Sort == "distance" && s.Lat == nil {
		return s, fmt.Errorf("sort distance memerlukan lat/lng")
	}
	return s, nil
}

// Decorate computes freshness and the optional distance for a hospital that the database
// already filtered, exactly as Filter does for every hospital it returns.
func Decorate(h *domain.Hospital, s Search, now time.Time) {
	h.Refresh(now)
	setDistance(h, s)
}
func setDistance(h *domain.Hospital, s Search) {
	if s.Lat != nil && h.Latitude != nil && h.Longitude != nil {
		d := domain.Distance(*s.Lat, *s.Lng, *h.Latitude, *h.Longitude)
		h.DistanceKM = &d
	}
}
func Filter(hs []domain.Hospital, s Search, now time.Time) ([]domain.Hospital, int) {
	out := []domain.Hospital{}
	for _, h := range hs {
		h.Refresh(now)
		// An empty Dataset behaves like "real" so zero-value searches never mix in demo data.
		if simulated := h.DataOrigin == "simulation"; (s.Dataset == "simulation" && !simulated) || (s.Dataset != "simulation" && s.Dataset != "all" && simulated) {
			continue
		}
		if s.Q != "" && !strings.Contains(strings.ToLower(h.Name+" "+h.CityName+" "+h.ProvinceName), strings.ToLower(s.Q)) {
			continue
		}
		if s.Province != "" && h.ProvinceCode != s.Province || s.City != "" && h.CityCode != s.City {
			continue
		}
		if s.BPJS != nil && (h.BPJS == nil || *h.BPJS != *s.BPJS) {
			continue
		}
		matches := true
		for _, slug := range s.Services {
			found := false
			for _, x := range h.Services {
				if x.Slug == slug && (x.Status == "available" || x.Status == "limited") {
					found = true
				}
			}
			if !found {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}
		if s.HasBeds {
			found := false
			for _, b := range h.Beds {
				if b.Available != nil && *b.Available > 0 && b.Freshness == "fresh" && b.DataOrigin != "simulation" {
					found = true
				}
			}
			if !found {
				continue
			}
		}
		if s.Lat != nil && h.Latitude != nil && h.Longitude != nil {
			d := domain.Distance(*s.Lat, *s.Lng, *h.Latitude, *h.Longitude)
			h.DistanceKM = &d
		}
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if s.Sort == "match" {
			as, bs := matchScore(a, s.Services), matchScore(b, s.Services)
			if as != bs {
				return as > bs
			}
		}
		if s.Sort == "distance" || (s.Sort == "match" && s.Lat != nil) {
			if (a.DistanceKM == nil) != (b.DistanceKM == nil) {
				return a.DistanceKM != nil
			}
			if a.DistanceKM != nil && *a.DistanceKM != *b.DistanceKM {
				return *a.DistanceKM < *b.DistanceKM
			}
		}
		an, bn := strings.ToLower(a.Name), strings.ToLower(b.Name)
		if an != bn {
			return an < bn
		}
		return a.ID < b.ID
	})
	total := len(out)
	start := (s.Page - 1) * s.Limit
	if start >= total {
		return []domain.Hospital{}, total
	}
	end := start + s.Limit
	if end > total {
		end = total
	}
	return out[start:end], total
}

// Demo match ordering is deterministic and not a clinical score. Simulation has no factual bonus.
func matchScore(h domain.Hospital, slugs []string) int {
	score := 0
	for _, x := range h.Services {
		wanted := len(slugs) == 0
		for _, s := range slugs {
			if s == x.Slug {
				wanted = true
			}
		}
		if !wanted || x.DataOrigin == "simulation" {
			continue
		}
		if x.Status == "available" {
			score += 10
		} else if x.Status == "limited" {
			score += 5
		}
		switch x.Freshness {
		case "fresh":
			score += 3
		case "aging":
			score++
		}
	}
	return score
}

// Compare limits: a comparison needs at least two and at most three distinct hospitals.
const (
	CompareMin = 2
	CompareMax = 3
)

// ParseCompare validates GET /hospitals/compare?ids=a,b,c and returns the IDs in request order.
// Unknown, empty or repeated query parameters, malformed or duplicate IDs, and fewer than
// CompareMin or more than CompareMax IDs are rejected.
func ParseCompare(v url.Values, validID func(string) bool) ([]string, error) {
	for k, values := range v {
		if k != "ids" || len(values) != 1 || values[0] == "" {
			return nil, fmt.Errorf("parameter tidak didukung, kosong atau berulang: %s", k)
		}
	}
	raw := v.Get("ids")
	if raw == "" {
		return nil, fmt.Errorf("ids wajib diisi (%d sampai %d ID dipisah koma)", CompareMin, CompareMax)
	}
	ids := strings.Split(raw, ",")
	if len(ids) < CompareMin || len(ids) > CompareMax {
		return nil, fmt.Errorf("ids harus berisi %d sampai %d ID", CompareMin, CompareMax)
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !validID(id) {
			return nil, fmt.Errorf("ids memuat ID yang bukan UUID valid atau kode fasyankes")
		}
		if seen[id] {
			return nil, fmt.Errorf("ids tidak boleh memuat ID duplikat")
		}
		seen[id] = true
	}
	return ids, nil
}

// Suggest is a validated GET /hospitals/suggest query.
type Suggest struct {
	Q, Dataset string
	Limit      int
}

// ParseSuggest accepts q (1..100 characters, required), dataset (real|simulation|all, default real)
// and limit (1..20, default 10). Unknown, empty or repeated parameters are rejected.
func ParseSuggest(v url.Values) (Suggest, error) {
	s := Suggest{Dataset: "real", Limit: 10}
	for k, values := range v {
		if (k != "q" && k != "dataset" && k != "limit") || len(values) != 1 || values[0] == "" {
			return s, fmt.Errorf("parameter tidak didukung, kosong atau berulang: %s", k)
		}
	}
	s.Q = strings.TrimSpace(v.Get("q"))
	if s.Q == "" || utf8.RuneCountInString(s.Q) > 100 {
		return s, fmt.Errorf("q wajib diisi, maksimal 100 karakter")
	}
	if x := v.Get("dataset"); x != "" {
		if x != "real" && x != "simulation" && x != "all" {
			return s, fmt.Errorf("dataset harus real, simulation atau all")
		}
		s.Dataset = x
	}
	if x := v.Get("limit"); x != "" {
		n, e := strconv.Atoi(x)
		if e != nil || n < 1 || n > 20 {
			return s, fmt.Errorf("limit harus 1 sampai 20")
		}
		s.Limit = n
	}
	return s, nil
}

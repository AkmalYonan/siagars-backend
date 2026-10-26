package httpapi

import (
	"context"
	"net/http"
	"siagars/backend/internal/domain"
	"siagars/backend/internal/service"
	"time"
)

// AreaReader is an optional Store capability: candidate and marker queries filtered in the database.
// Stores without it fall back to Hospitals plus the equivalent in-memory filter.
type AreaReader interface {
	RecommendCandidates(context.Context, service.RecommendQuery) ([]domain.Hospital, bool, error)
	MarkerHospitals(context.Context, service.MarkersQuery) ([]domain.Hospital, bool, error)
}

// knownServices checks every slug against the service master and returns slug → display name.
func (s *Server) knownServices(w http.ResponseWriter, r *http.Request, slugs []string) (map[string]string, bool) {
	masters, e := s.repo.Services(r.Context())
	if e != nil {
		s.dbError(w, r, e)
		return nil, false
	}
	names := map[string]string{}
	for _, m := range masters {
		names[m.Slug] = m.Name
	}
	for _, slug := range slugs {
		if _, found := names[slug]; !found {
			fail(w, r, 422, "validation_error", "Slug layanan tidak dikenal")
			return nil, false
		}
	}
	return names, true
}

// knownRegions answers 422 for a province/city code that is not in the region master.
func (s *Server) knownRegions(w http.ResponseWriter, r *http.Request, province, city string) bool {
	for _, x := range []struct{ code, kind string }{{city, "city"}, {province, "province"}} {
		if x.code == "" {
			continue
		}
		exists, e := s.repo.RegionExists(r.Context(), x.code, x.kind)
		if e != nil {
			s.dbError(w, r, e)
			return false
		}
		if !exists {
			fail(w, r, 422, "validation_error", "Kode wilayah tidak dikenal")
			return false
		}
	}
	return true
}

// recommendations scores hospitals for a need (contract §4). The scoring itself is the pure
// domain.Recommend; this handler only validates, loads candidates and shapes the response.
func (s *Server) recommendations(w http.ResponseWriter, r *http.Request) {
	q, e := service.ParseRecommend(r.URL.Query())
	if e != nil {
		fail(w, r, 422, "validation_error", e.Error())
		return
	}
	if !s.knownRegions(w, r, q.Province, q.City) {
		return
	}
	names, yes := s.knownServices(w, r, q.Services)
	if !yes {
		return
	}
	q.ServiceNames = names
	var candidates []domain.Hospital
	var truncated bool
	if area, ok := s.repo.(AreaReader); ok {
		candidates, truncated, e = area.RecommendCandidates(r.Context(), q)
	} else {
		var hs []domain.Hospital
		if hs, e = s.repo.Hospitals(r.Context()); e == nil {
			candidates, truncated = service.RecommendCandidates(hs, q)
		}
	}
	if e != nil {
		s.dbError(w, r, e)
		return
	}
	now := time.Now().UTC()
	res := domain.Recommend(candidates, q.RecommendRequest, now)
	meta := map[string]any{"total_considered": res.Considered, "excluded": res.Excluded, "weights": domain.BaseWeights, "generated_at": now, "algorithm_version": domain.AlgorithmVersion,
		"dataset": q.Dataset, "candidates_truncated": truncated, "radius_km": nil}
	if q.Lat != nil {
		meta["radius_km"] = q.RadiusKM
	}
	send(w, 200, map[string]any{"data": res.Recommendations, "meta": meta})
}

// markers returns lightweight pins for every hospital in a map box (contract §7), at most 500.
func (s *Server) markers(w http.ResponseWriter, r *http.Request) {
	q, e := service.ParseMarkers(r.URL.Query())
	if e != nil {
		fail(w, r, 422, "validation_error", e.Error())
		return
	}
	if _, yes := s.knownServices(w, r, q.Services); !yes {
		return
	}
	var hs []domain.Hospital
	var truncated bool
	if area, ok := s.repo.(AreaReader); ok {
		hs, truncated, e = area.MarkerHospitals(r.Context(), q)
	} else {
		var all []domain.Hospital
		if all, e = s.repo.Hospitals(r.Context()); e == nil {
			hs, truncated = service.MarkerHospitals(all, q)
		}
	}
	if e != nil {
		s.dbError(w, r, e)
		return
	}
	now := time.Now().UTC()
	out := make([]domain.Marker, 0, len(hs))
	for _, h := range hs {
		h.Refresh(now)
		out = append(out, domain.MarkerOf(h, q.Services))
	}
	send(w, 200, map[string]any{"data": out, "meta": map[string]any{"count": len(out), "truncated": truncated, "dataset": q.Dataset, "as_of": now}})
}

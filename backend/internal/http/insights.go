package httpapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"siagars/backend/internal/domain"
	"siagars/backend/internal/service"
	"sort"
	"sync"
	"time"
)

const (
	insightsTTL      = 5 * time.Minute
	insightsMaxCache = 256
)

// insightCache keeps rendered insight bodies for insightsTTL. Keys are built from validated parameters
// only (dataset, known province, known service), so the key space is small; the cap is a safety net.
type insightCache struct {
	mu      sync.Mutex
	entries map[string]insightEntry
}
type insightEntry struct {
	body    []byte
	expires time.Time
}

func (c *insightCache) get(key string, now time.Time) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || !now.Before(e.expires) {
		delete(c.entries, key)
		return nil, false
	}
	return e.body, true
}

func (c *insightCache) put(key string, body []byte, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil || len(c.entries) >= insightsMaxCache {
		c.entries = map[string]insightEntry{}
	}
	c.entries[key] = insightEntry{body, now.Add(insightsTTL)}
}

// insight serves one /insights/* endpoint (contract §6): validate, answer from the 5-minute cache or
// aggregate over the filtered hospitals, and mark the success response publicly cacheable.
func (s *Server) insight(name string, withService bool, build func(hs []domain.Hospital, q service.InsightsQuery, now time.Time) any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, e := service.ParseInsights(r.URL.Query(), withService)
		if e != nil {
			fail(w, r, 422, "validation_error", e.Error())
			return
		}
		if !s.knownRegions(w, r, q.Province, "") {
			return
		}
		if withService {
			if _, yes := s.knownServices(w, r, []string{q.Service}); !yes {
				return
			}
		}
		now := time.Now().UTC()
		key := name + "?" + url.Values{"dataset": {q.Dataset}, "province_code": {q.Province}, "service": {q.Service}}.Encode()
		body, hit := s.insights.get(key, now)
		if !hit {
			all, e := s.repo.Hospitals(r.Context())
			if e != nil {
				s.dbError(w, r, e)
				return
			}
			hs := service.InsightHospitals(all, q)
			origins := map[string]bool{}
			for _, h := range hs {
				if h.DataOrigin != "" {
					origins[h.DataOrigin] = true
				}
			}
			sources := make([]string, 0, len(origins))
			for o := range origins {
				sources = append(sources, o)
			}
			sort.Strings(sources)
			var province any
			if q.Province != "" {
				province = q.Province
			}
			body, e = json.Marshal(map[string]any{"data": build(hs, q, now), "meta": map[string]any{"generated_at": now, "sources": sources, "dataset": q.Dataset, "province_code": province}})
			if e != nil {
				s.dbError(w, r, e)
				return
			}
			body = append(body, '\n')
			s.insights.put(key, body, now)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.WriteHeader(200)
		_, _ = w.Write(body)
	}
}

func (s *Server) insightHandlers() (summary, coverage, gaps, freshness http.HandlerFunc) {
	summary = s.insight("summary", false, func(hs []domain.Hospital, _ service.InsightsQuery, now time.Time) any {
		return domain.Summarize(hs, now)
	})
	coverage = s.insight("coverage", false, func(hs []domain.Hospital, _ service.InsightsQuery, _ time.Time) any { return domain.Coverage(hs) })
	gaps = s.insight("service-gaps", true, func(hs []domain.Hospital, q service.InsightsQuery, _ time.Time) any {
		return domain.ServiceGaps(hs, q.Service)
	})
	freshness = s.insight("freshness", false, func(hs []domain.Hospital, _ service.InsightsQuery, now time.Time) any {
		return domain.FreshnessByOrigin(hs, now)
	})
	return
}

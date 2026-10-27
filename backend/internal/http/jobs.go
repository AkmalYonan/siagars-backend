package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"siagars/backend/internal/domain"
	"strings"
	"time"
)

// SimulationRefresher runs the daily simulation refresh (BE-05); stores without it answer 501.
type SimulationRefresher interface {
	RefreshSimulationJob(ctx context.Context, actor, rid string, at time.Time) (domain.SimulationRefresh, error)
}

// jobSecret returns the secret a job caller presented: X-Job-Secret on POST (manual run, GitHub Actions)
// or the bearer token Vercel Cron sends on GET when CRON_SECRET is set to the same value.
func jobSecret(r *http.Request) string {
	if r.Method == http.MethodGet {
		token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !found {
			return ""
		}
		return token
	}
	return r.Header.Get("X-Job-Secret")
}

// refreshSimulation keeps simulation data fresh. It is internal (not in the public OpenAPI contract) and
// answers only to JOB_SECRET, compared in constant time; without JOB_SECRET every call is 401. The job
// writes simulation items only and is audited as system:cron.
func (s *Server) refreshSimulation(w http.ResponseWriter, r *http.Request) {
	if s.cfg.JobSecret == "" || !equal(jobSecret(r), s.cfg.JobSecret) {
		fail(w, r, 401, "unauthorized", "Secret job tidak valid")
		return
	}
	if len(r.URL.Query()) > 0 {
		fail(w, r, 422, "validation_error", "Parameter tidak didukung")
		return
	}
	job, yes := s.repo.(SimulationRefresher)
	if !yes {
		fail(w, r, 501, "not_implemented", "Fitur ini belum tersedia di server")
		return
	}
	out, e := job.RefreshSimulationJob(r.Context(), "system:cron", rid(r), time.Now())
	if e != nil {
		s.dbError(w, r, e)
		return
	}
	slog.Info("job refresh-simulation", "request_id", rid(r), "imported_hospitals", out.ImportedHospitals, "simulation_hospitals", out.SimulationHospitals, "services", out.Services, "beds", out.Beds)
	ok(w, out)
}

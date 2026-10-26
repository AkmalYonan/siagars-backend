package seed

import (
	"context"
	"siagars/backend/internal/domain"
	"siagars/backend/internal/repository"
	"time"
)

// RefreshSimulation is the daily simulation refresh (BE-05). It first runs the dummy-facilities logic
// so imported hospitals without simulation items get them, then restamps every simulation item so its
// freshness reflects the run, and finally writes one audit entry. Curated and upstream items are never
// written, and verified_at is never set. Values stay deterministic; only their timestamps move.
func RefreshSimulation(ctx context.Context, p *repository.Mongo, actor, rid string, at time.Time) (domain.SimulationRefresh, error) {
	at = at.UTC().Truncate(time.Millisecond)
	out := domain.SimulationRefresh{At: at}
	dummy, e := ApplyDummyFacilities(ctx, p.DB, at)
	if e != nil {
		return out, e
	}
	out.ImportedHospitals = dummy.Hospitals
	if out.SimulationHospitals, e = p.RestampSimulation(ctx, at); e != nil {
		return out, e
	}
	if out.Services, out.Beds, e = p.CountSimulationItems(ctx, at); e != nil {
		return out, e
	}
	e = p.AuditJob(ctx, actor, "job.refresh_simulation", "simulation", rid, map[string]int{"imported_hospitals": out.ImportedHospitals, "simulation_hospitals": out.SimulationHospitals, "services": out.Services, "beds": out.Beds})
	return out, e
}

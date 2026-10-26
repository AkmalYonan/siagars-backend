package app

import (
	"context"
	"siagars/backend/internal/domain"
	"siagars/backend/internal/repository"
	"siagars/backend/seed"
	"time"
)

// store is the API's view of MongoDB: every repository capability plus jobs that combine repository
// steps (the daily simulation refresh lives in package seed next to dummy-facilities).
type store struct{ *repository.Mongo }

func (s store) RefreshSimulationJob(ctx context.Context, actor, rid string, at time.Time) (domain.SimulationRefresh, error) {
	return seed.RefreshSimulation(ctx, s.Mongo, actor, rid, at)
}

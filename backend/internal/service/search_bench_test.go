package service

import (
	"fmt"
	"math/rand"
	"siagars/backend/internal/domain"
	"testing"
	"time"
)

// benchHospitals builds a deterministic synthetic snapshot; it measures the in-memory Filter only
// (no database, no network), as a reference for the pre-pushdown search path.
func benchHospitals(n int, now time.Time) []domain.Hospital {
	r := rand.New(rand.NewSource(1))
	yes := true
	avail := 2
	out := make([]domain.Hospital, n)
	for i := range out {
		lat, lng := -6.0-r.Float64(), 106.5+r.Float64()
		out[i] = domain.Hospital{ID: fmt.Sprintf("%08d", i), Name: fmt.Sprintf("RS Sintetis %d", r.Intn(n)), CityCode: "3171", ProvinceCode: "31", CityName: "Jakarta Pusat", ProvinceName: "DKI Jakarta",
			Latitude: &lat, Longitude: &lng, BPJS: &yes, Provenance: domain.Provenance{DataOrigin: "curated", SourceUpdatedAt: &now},
			Services: []domain.Service{{Slug: "icu", Status: "available"}, {Slug: "nicu", Status: "limited"}},
			Beds:     []domain.Bed{{Category: "icu", Available: &avail, Provenance: domain.Provenance{DataOrigin: "curated", SourceUpdatedAt: &now}}}}
	}
	return out
}

func BenchmarkFilter(b *testing.B) {
	now := time.Now().UTC()
	lat, lng := -6.5, 107.0
	for _, n := range []int{30, 500, 5000} {
		hs := benchHospitals(n, now)
		for name, s := range map[string]Search{
			"name":     {Page: 1, Limit: 20, Sort: "name", Dataset: "real"},
			"distance": {Page: 1, Limit: 20, Sort: "distance", Dataset: "real", Lat: &lat, Lng: &lng, Services: []string{"icu", "nicu"}, HasBeds: true},
			"q":        {Page: 1, Limit: 20, Sort: "match", Dataset: "real", Q: "sintetis 1"},
		} {
			b.Run(fmt.Sprintf("n=%d/%s", n, name), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					Filter(hs, s, now)
				}
			})
		}
	}
}

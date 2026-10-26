package domain

import (
	"testing"
	"time"
)

var qNow = time.Date(2026, 10, 26, 3, 0, 0, 0, time.UTC)

func ago(d time.Duration) *time.Time { t := qNow.Add(-d); return &t }
func iptr(n int) *int                { return &n }
func sptr(s string) *string          { return &s }
func bptr(b bool) *bool              { return &b }
func fptr(f float64) *float64        { return &f }

func svcQ(slug, status, origin string, at *time.Time) Service {
	return Service{Slug: slug, Status: status, Provenance: Provenance{DataOrigin: origin, SourceUpdatedAt: at}}
}
func bedQ(category string, available *int, origin string, at *time.Time) Bed {
	return Bed{Category: category, Total: iptr(10), Available: available, Provenance: Provenance{DataOrigin: origin, SourceUpdatedAt: at}}
}

// complete is a hospital with all 13 key items known, curated and fresh.
func complete() Hospital {
	h := Hospital{Name: "RS Lengkap", Phone: sptr("021-1"), Latitude: fptr(-6.2), Longitude: fptr(106.8), BPJS: bptr(true), Provenance: Provenance{DataOrigin: "satusehat_sandbox"}}
	for _, s := range QualityServices {
		h.Services = append(h.Services, svcQ(s, "available", "curated", ago(time.Minute)))
	}
	for _, b := range QualityBeds {
		h.Beds = append(h.Beds, bedQ(b, iptr(1), "curated", ago(time.Minute)))
	}
	return h
}

func TestDataQualityTable(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Hospital)
		score  int
		label  string
		fresh  string
		origin string
		compl  float64
	}{
		{"lengkap kurasi segar", func(*Hospital) {}, 100, "baik", "fresh", "curated", 1},
		{"telepon null", func(h *Hospital) { h.Phone = nil }, 96, "baik", "fresh", "curated", 0.92},
		{"bpjs false tetap diketahui", func(h *Hospital) { h.BPJS = bptr(false) }, 100, "baik", "fresh", "curated", 1},
		{"bed 0 diketahui, bukan unknown", func(h *Hospital) { h.Beds[0].Available = iptr(0) }, 100, "baik", "fresh", "curated", 1},
		{"bed null = unknown", func(h *Hospital) { h.Beds[0].Available = nil }, 96, "baik", "fresh", "curated", 0.92},
		{"satu item aging menurunkan kesegaran", func(h *Hospital) { h.Services[0].SourceUpdatedAt = ago(2 * time.Hour) }, 93, "baik", "aging", "curated", 1},
		{"satu item stale", func(h *Hospital) { h.Beds[3].SourceUpdatedAt = ago(25 * time.Hour) }, 83, "baik", "stale", "curated", 1},
		{"tanggal di masa depan = unknown", func(h *Hospital) { future := qNow.Add(time.Hour); h.Beds[3].SourceUpdatedAt = &future }, 80, "baik", "unknown", "curated", 1},
		{"semua simulasi", func(h *Hospital) {
			for i := range h.Services {
				h.Services[i].DataOrigin = "simulation"
			}
			for i := range h.Beds {
				h.Beds[i].DataOrigin = "simulation"
			}
		}, 88, "baik", "fresh", "simulation", 1},
		{"seri mayoritas condong ke origin lebih rendah", func(h *Hospital) {
			h.Services = h.Services[:5]
			for i := range h.Services {
				h.Services[i].DataOrigin = "simulation"
			}
			h.Beds = append(h.Beds, bedQ("x", nil, "curated", ago(time.Minute)))
		}, 84, "baik", "fresh", "simulation", 0.92},
		{"RS impor tanpa fasilitas", func(h *Hospital) { h.Services, h.Beds, h.BPJS, h.Phone = nil, nil, nil, nil }, 29, "terbatas", "unknown", "satusehat_sandbox", 0.08},
		{"status layanan unknown tidak dihitung", func(h *Hospital) {
			for i := range h.Services {
				h.Services[i].Status = "unknown"
			}
		}, 77, "baik", "fresh", "curated", 0.54},
	}
	for _, c := range cases {
		h := complete()
		c.mutate(&h)
		got := ComputeDataQuality(h, qNow)
		if got.Score != c.score || got.Label != c.label || got.Freshness != c.fresh || got.PrimaryOrigin != c.origin || got.Completeness != c.compl {
			t.Errorf("%s: %+v", c.name, got)
		}
	}
}

func TestDataQualityLabelBoundaries(t *testing.T) {
	for score, want := range map[int]string{100: "baik", 75: "baik", 74: "cukup", 50: "cukup", 49: "terbatas", 0: "terbatas"} {
		if got := QualityLabel(score); got != want {
			t.Errorf("%d: %s", score, got)
		}
	}
}

func TestRefreshFillsDataQuality(t *testing.T) {
	h := complete()
	h.Refresh(qNow)
	if h.DataQuality.Score != 100 || h.DataQuality.Label != "baik" {
		t.Fatalf("%+v", h.DataQuality)
	}
}

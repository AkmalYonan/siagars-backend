package service

import (
	"net/url"
	"siagars/backend/internal/domain"
	"testing"
	"time"
)

func TestInvalidQueries(t *testing.T) {
	for _, raw := range []string{"foo=1", "page=0", "limit=51", "page=1&page=2", "services=icu,,nicu", "lat=NaN&lng=1", "lat=1", "sort=distance", "bpjs=1", "q=", "sort=foo", "dataset=", "dataset=fake", "dataset=real&dataset=all", "dataset=REAL"} {
		q, _ := url.ParseQuery(raw)
		if _, e := ParseSearch(q); e == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}
func TestANDAndUnknownAndBeds(t *testing.T) {
	now := time.Now().UTC()
	yes, no := true, false
	one := 1
	fresh := domain.Provenance{DataOrigin: "curated", SourceUpdatedAt: &now}
	hs := []domain.Hospital{{ID: "a", BPJS: &yes, Services: []domain.Service{{Slug: "icu", Status: "available"}, {Slug: "nicu", Status: "limited"}}, Beds: []domain.Bed{{Available: &one, Provenance: fresh}}}, {ID: "b", BPJS: &no, Services: []domain.Service{{Slug: "icu", Status: "available"}}}, {ID: "c", BPJS: nil}}
	q := Search{Page: 1, Limit: 20, Services: []string{"icu", "nicu"}}
	out, n := Filter(hs, q, now)
	if n != 1 || out[0].ID != "a" {
		t.Fatal(out)
	}
	q.Services = nil
	q.BPJS = &no
	out, n = Filter(hs, q, now)
	if n != 1 || out[0].ID != "b" {
		t.Fatal("unknown must not match false")
	}
	q.BPJS = nil
	q.HasBeds = true
	out, n = Filter(hs, q, now)
	if n != 1 {
		t.Fatal(out)
	}
	hs[0].Beds[0].DataOrigin = "simulation"
	_, n = Filter(hs, q, now)
	if n != 0 {
		t.Fatal("simulation cannot prove availability")
	}
	hs[0].Beds[0].DataOrigin = "curated"
	old := now.Add(-time.Hour)
	hs[0].Beds[0].SourceUpdatedAt = &old
	_, n = Filter(hs, q, now)
	if n != 0 {
		t.Fatal("aging excluded")
	}
}
func TestSortPagination(t *testing.T) {
	now := time.Now()
	lat, lng, far := 0.0, 0.0, 1.0
	hs := []domain.Hospital{{ID: "b", Name: "Same"}, {ID: "a", Name: "Same", Latitude: &lat, Longitude: &lng}, {ID: "c", Name: "A", Latitude: &lat, Longitude: &far}}
	out, n := Filter(hs, Search{Page: 1, Limit: 1, Sort: "distance", Lat: &lat, Lng: &lng}, now)
	if n != 3 || out[0].ID != "a" {
		t.Fatal(out)
	}
	out, _ = Filter(hs, Search{Page: 99, Limit: 20}, now)
	if len(out) != 0 {
		t.Fatal(out)
	}
}
func TestDatasetSeparation(t *testing.T) {
	now := time.Now()
	hs := []domain.Hospital{
		{ID: "s", Name: "S", Provenance: domain.Provenance{DataOrigin: "simulation"}},
		{ID: "n", Name: "N", Provenance: domain.Provenance{DataOrigin: "satusehat_sandbox"}},
		{ID: "c", Name: "C", Provenance: domain.Provenance{DataOrigin: "curated"}},
	}
	for _, c := range []struct {
		raw  string
		want string
	}{{"", "cn"}, {"dataset=real", "cn"}, {"dataset=simulation", "s"}, {"dataset=all", "cns"}} {
		q, _ := url.ParseQuery(c.raw)
		search, e := ParseSearch(q)
		if e != nil {
			t.Fatal(c.raw, e)
		}
		out, n := Filter(hs, search, now)
		got := ""
		for _, h := range out {
			got += h.ID
		}
		if got != c.want || n != len(c.want) {
			t.Errorf("%q: got %q want %q", c.raw, got, c.want)
		}
	}
	if out, _ := Filter(hs, Search{Page: 1, Limit: 20}, now); len(out) != 2 {
		t.Fatal("zero-value search must behave like dataset=real")
	}
}

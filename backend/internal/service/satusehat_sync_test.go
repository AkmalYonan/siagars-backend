package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"siagars/backend/internal/domain"
	"siagars/backend/internal/integrations/satusehat"
	"testing"
	"time"
)

type pages struct {
	batches map[int]satusehat.HospitalBatch
	byScope map[string]map[int]satusehat.HospitalBatch
	fail    map[int]error
	calls   []int
	scopes  []string
}

func (p *pages) FetchPage(_ context.Context, scope string, page, limit int) (satusehat.HospitalBatch, error) {
	p.calls = append(p.calls, page)
	p.scopes = append(p.scopes, scope)
	if e := p.fail[page]; e != nil {
		return satusehat.HospitalBatch{}, e
	}
	if p.byScope != nil {
		return p.byScope[scope][page], nil
	}
	return p.batches[page], nil
}

type importer struct {
	batches [][]domain.HospitalImport
	result  domain.ImportResult
	err     error
}

func (i *importer) UpsertSATUSEHATHospitals(_ context.Context, items []domain.HospitalImport) (domain.ImportResult, error) {
	i.batches = append(i.batches, items)
	r := i.result
	if r == (domain.ImportResult{}) {
		r.Created = len(items)
	}
	return r, i.err
}
func record(id string) satusehat.MSIFacility { return recordIn(id, "3171") }
func recordIn(id, city string) satusehat.MSIFacility {
	var f satusehat.MSIFacility
	body := fmt.Sprintf(`{"kode_satusehat":%q,"nama":"RS %s","provinsi":{"kode":%q,"nama":"Provinsi"},"kabkota":{"kode":%q,"nama":"Kab/Kota"},"jenis_sarana":{"kode":"104"}}`, id, id, city[:2], city)
	if e := json.Unmarshal([]byte(body), &f); e != nil {
		panic(e)
	}
	return f
}
func batch(page, total int, ids ...string) satusehat.HospitalBatch {
	b := satusehat.HospitalBatch{Page: page, TotalPage: total, Records: []satusehat.MSIFacility{}}
	for _, id := range ids {
		b.Records = append(b.Records, record(id))
	}
	if len(ids) > 0 && page < total {
		next := page + 1
		b.NextPage = &next
	}
	return b
}
func clock() func() time.Time {
	t := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	return func() time.Time { t = t.Add(time.Second); return t }
}

func TestSyncWalksAllPages(t *testing.T) {
	src := &pages{batches: map[int]satusehat.HospitalBatch{1: batch(1, 3, "a", "b"), 2: batch(2, 3, "c", "a"), 3: batch(3, 3, "d")}}
	bad := record("x")
	bad.Name = ""
	b := src.batches[3]
	b.Records = append(b.Records, bad)
	src.batches[3] = b
	imp := &importer{}
	s, e := SyncSATUSEHAT(context.Background(), src, imp, nil, 2, clock())
	if e != nil || fmt.Sprint(src.calls) != "[1 2 3]" || s.Pages != 3 || s.Fetched != 6 || s.Created != 4 || s.Skipped != 2 || s.SkipReasons["duplicate"] != 1 || s.SkipReasons[satusehat.SkipName] != 1 {
		t.Fatalf("%+v %v %v", s, src.calls, e)
	}
	if s.Environment != "sandbox" || !s.FinishedAt.After(s.StartedAt) || s.ErrorCategory != "" {
		t.Fatalf("%+v", s)
	}
	for _, items := range imp.batches {
		for _, h := range items {
			if h.SourceNamespace != satusehat.Namespace || h.SyncedAt.IsZero() {
				t.Fatal(h)
			}
		}
	}
}
func TestSyncEmptyFirstPage(t *testing.T) {
	src := &pages{batches: map[int]satusehat.HospitalBatch{1: batch(1, 0)}}
	if s, e := SyncSATUSEHAT(context.Background(), src, &importer{}, nil, 10, clock()); e != nil || s.Pages != 1 || len(src.calls) != 1 {
		t.Fatal(s, e)
	}
}
func TestSyncStopsOnFailedPageAndKeepsEarlierPages(t *testing.T) {
	upstream := &satusehat.Error{Op: "msi", Category: satusehat.CategoryUpstream, Status: 503}
	src := &pages{batches: map[int]satusehat.HospitalBatch{1: batch(1, 3, "a")}, fail: map[int]error{2: upstream}}
	imp := &importer{}
	s, e := SyncSATUSEHAT(context.Background(), src, imp, nil, 1, clock())
	if !errors.Is(e, upstream) || s.FailedPage != 2 || s.ErrorCategory != string(satusehat.CategoryUpstream) || len(imp.batches) != 1 || s.Created != 1 {
		t.Fatalf("%+v %v", s, e)
	}
	// Token/upstream failure before any page: nothing is written.
	src = &pages{fail: map[int]error{1: &satusehat.Error{Op: "oauth", Category: satusehat.CategoryAuth}}}
	imp = &importer{}
	if s, e = SyncSATUSEHAT(context.Background(), src, imp, nil, 1, clock()); e == nil || len(imp.batches) != 0 || s.FailedPage != 1 || s.ErrorCategory != string(satusehat.CategoryAuth) {
		t.Fatalf("%+v %v", s, e)
	}
}
func TestSyncRejectsInconsistentPagination(t *testing.T) {
	jump := batch(1, 5, "a")
	five := 5
	jump.NextPage = &five
	beyond := batch(1, 1, "a")
	two := 2
	beyond.NextPage = &two
	for name, src := range map[string]*pages{
		"skip ahead":     {batches: map[int]satusehat.HospitalBatch{1: jump}},
		"beyond total":   {batches: map[int]satusehat.HospitalBatch{1: beyond}},
		"total changes":  {batches: map[int]satusehat.HospitalBatch{1: batch(1, 3, "a"), 2: batch(2, 9, "b")}},
		"repeating page": {batches: map[int]satusehat.HospitalBatch{1: batch(1, 3, "a"), 2: {Page: 2, TotalPage: 3, Records: []satusehat.MSIFacility{record("b")}, NextPage: &two}}},
	} {
		s, e := SyncSATUSEHAT(context.Background(), src, &importer{}, nil, 1, clock())
		if !errors.Is(e, ErrInvalidPaginate) || s.ErrorCategory != "invalid_pagination" || len(src.calls) > 2 {
			t.Fatalf("%s: %+v %v %v", name, s, e, src.calls)
		}
	}
}
func TestSyncReportsRejectedWrites(t *testing.T) {
	src := &pages{batches: map[int]satusehat.HospitalBatch{1: batch(1, 1, "a", "b")}}
	s, e := SyncSATUSEHAT(context.Background(), src, &importer{result: domain.ImportResult{Created: 1, Failed: 1}}, nil, 10, clock())
	if !errors.Is(e, ErrPartialSync) || s.Failed != 1 || s.Created != 1 || s.ErrorCategory != "write_rejected" {
		t.Fatalf("%+v %v", s, e)
	}
	src = &pages{batches: map[int]satusehat.HospitalBatch{1: batch(1, 2, "a")}}
	if s, e = SyncSATUSEHAT(context.Background(), src, &importer{err: errors.New("db down")}, nil, 10, clock()); e == nil || s.ErrorCategory != "database" || s.FailedPage != 1 || len(src.calls) != 1 {
		t.Fatalf("%+v %v", s, e)
	}
}
func TestSyncWalksEachScope(t *testing.T) {
	jaksel := batch(1, 2, "a")
	jaksel.Records = append(jaksel.Records, recordIn("stray", "3271"))
	src := &pages{byScope: map[string]map[int]satusehat.HospitalBatch{
		"3171": {1: jaksel, 2: batch(2, 2, "b")},
		"3276": {1: {Page: 1, TotalPage: 1, Records: []satusehat.MSIFacility{recordIn("c", "3276"), recordIn("a", "3276")}}},
	}}
	imp := &importer{}
	s, e := SyncSATUSEHAT(context.Background(), src, imp, []string{"3171", "3276"}, 10, clock())
	if e != nil || fmt.Sprint(src.scopes) != "[3171 3171 3276]" || s.Scopes != 2 || s.Pages != 3 || s.Fetched != 5 || s.Created != 3 || s.SkipReasons["out_of_scope"] != 1 || s.SkipReasons["duplicate"] != 1 {
		t.Fatalf("%+v %v %v", s, src.scopes, e)
	}
	upstream := &satusehat.Error{Op: "msi", Category: satusehat.CategoryRateLimited, Status: 429}
	src = &pages{byScope: map[string]map[int]satusehat.HospitalBatch{"3171": {1: batch(1, 1, "a")}}, fail: map[int]error{}}
	failing := &failScope{pages: src, scope: "3276", err: upstream}
	if s, e = SyncSATUSEHAT(context.Background(), failing, &importer{}, []string{"3171", "3276", "3671"}, 10, clock()); !errors.Is(e, upstream) || s.FailedScope != "3276" || s.FailedPage != 1 || s.Created != 1 || s.Scopes != 2 {
		t.Fatalf("%+v %v", s, e)
	}
}

type failScope struct {
	*pages
	scope string
	err   error
}

func (f *failScope) FetchPage(ctx context.Context, scope string, page, limit int) (satusehat.HospitalBatch, error) {
	if scope == f.scope {
		return satusehat.HospitalBatch{}, f.err
	}
	return f.pages.FetchPage(ctx, scope, page, limit)
}

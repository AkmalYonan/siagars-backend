package service

import (
	"context"
	"errors"
	"fmt"
	"siagars/backend/internal/domain"
	"siagars/backend/internal/integrations/satusehat"
	"time"
)

type HospitalImporter interface {
	UpsertSATUSEHATHospitals(context.Context, []domain.HospitalImport) (domain.ImportResult, error)
}

// SyncSummary contains counts and categories only, never record content or credentials.
type SyncSummary struct {
	Environment   string
	Scopes        int
	Pages         int
	Fetched       int
	Created       int
	Updated       int
	Unchanged     int
	Skipped       int
	SkipReasons   map[string]int
	Failed        int
	FailedScope   string
	FailedPage    int
	ErrorCategory string
	StartedAt     time.Time
	FinishedAt    time.Time
}

var (
	ErrPartialSync     = errors.New("some records were rejected by the database")
	ErrInvalidPaginate = errors.New("upstream pagination is inconsistent")
)

// SyncSATUSEHAT walks MSI pages from 1 to total_page for every kab/kota scope (or once nationally when
// scopes is empty) and upserts each page as one batch. A failed page stops the run; pages already written
// stay in place (last-known-good is never deleted), and records that were not received keep their synced_at.
func SyncSATUSEHAT(ctx context.Context, source satusehat.HospitalSource, importer HospitalImporter, scopes []string, limit int, now func() time.Time) (SyncSummary, error) {
	s := SyncSummary{Environment: "sandbox", SkipReasons: map[string]int{}, StartedAt: now().UTC()}
	finish := func(scope string, page int, e error) (SyncSummary, error) {
		s.FinishedAt = now().UTC()
		if e != nil {
			s.FailedScope, s.FailedPage = scope, page
			var upstream *satusehat.Error
			switch {
			case errors.As(e, &upstream):
				s.ErrorCategory = string(upstream.Category)
			case errors.Is(e, ErrInvalidPaginate):
				s.ErrorCategory = "invalid_pagination"
			case errors.Is(e, ErrPartialSync):
				s.ErrorCategory = "write_rejected"
				s.FailedScope, s.FailedPage = "", 0
			default:
				s.ErrorCategory = "database"
			}
		}
		return s, e
	}
	if len(scopes) == 0 {
		scopes = []string{""}
	}
	seen := map[string]bool{}
	for _, scope := range scopes {
		s.Scopes++
		lastPage := 0
		for page := 1; ; {
			batch, e := source.FetchPage(ctx, scope, page, limit)
			if e != nil {
				return finish(scope, page, e)
			}
			if page == 1 {
				lastPage = batch.TotalPage
			}
			// total_page may not shrink or grow mid-run in a way that would loop forever.
			if batch.TotalPage != lastPage || (batch.NextPage != nil && (*batch.NextPage != page+1 || *batch.NextPage > lastPage)) {
				return finish(scope, page, ErrInvalidPaginate)
			}
			s.Pages++
			s.Fetched += len(batch.Records)
			syncedAt := now()
			items := make([]domain.HospitalImport, 0, len(batch.Records))
			for _, record := range batch.Records {
				item, reason, ok := satusehat.Map(record, satusehat.Namespace, syncedAt)
				switch {
				case !ok:
				case scope != "" && item.CityCode != scope:
					// Defends against an upstream that ignores the region filter.
					reason, ok = "out_of_scope", false
				case seen[item.ExternalID]:
					// Pagination can shift between requests; a repeated record is written once per run.
					reason, ok = "duplicate", false
				}
				if !ok {
					s.Skipped++
					s.SkipReasons[reason]++
					continue
				}
				seen[item.ExternalID] = true
				items = append(items, item)
			}
			result, e := importer.UpsertSATUSEHATHospitals(ctx, items)
			s.Created += result.Created
			s.Updated += result.Updated
			s.Unchanged += result.Unchanged
			s.Failed += result.Failed
			if e != nil {
				return finish(scope, page, fmt.Errorf("page %d: %w", page, e))
			}
			if batch.NextPage == nil {
				break
			}
			page = *batch.NextPage
		}
	}
	if s.Failed > 0 {
		return finish("", 0, ErrPartialSync)
	}
	return finish("", 0, nil)
}

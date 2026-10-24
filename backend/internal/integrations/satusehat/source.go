// Package satusehat is the read-only boundary to SATUSEHAT Sandbox.
// It understands HTTP and MSI payloads only; persistence and workflow live elsewhere.
package satusehat

import (
	"context"
	"errors"
)

// SandboxBaseURL is the only upstream this phase may call. There is deliberately no production default.
const SandboxBaseURL = "https://api-satusehat-stg.dto.kemkes.go.id"

// Namespace scopes external_id so sandbox records can never collide with production or curated data.
const Namespace = "satusehat:msi:sandbox"

// HospitalJenisSarana is the documented MSI facility type code for Rumah Sakit.
const HospitalJenisSarana = "104"

type HospitalBatch struct {
	Records   []MSIFacility
	Page      int
	TotalPage int
	NextPage  *int
}

// HospitalSource fetches one page. kabkota is a 4-digit MSI kode_kabkota filter, or "" for no region filter.
type HospitalSource interface {
	FetchPage(ctx context.Context, kabkota string, page, limit int) (HospitalBatch, error)
}

var ErrDisabled = errors.New("SATUSEHAT disabled")

type DisabledSource struct{}

func (DisabledSource) FetchPage(context.Context, string, int, int) (HospitalBatch, error) {
	return HospitalBatch{}, ErrDisabled
}

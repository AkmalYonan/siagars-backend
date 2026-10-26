package domain

import (
	"math"
	"strings"
	"time"
)

type Provenance struct {
	DataOrigin      string     `json:"data_origin"`
	SourceURL       *string    `json:"source_url"`
	SourceUpdatedAt *time.Time `json:"source_updated_at"`
	SyncedAt        *time.Time `json:"synced_at"`
	VerifiedAt      *time.Time `json:"verified_at"`
	Freshness       string     `json:"freshness"`
}
type Service struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Category string `json:"category,omitempty"`
	Status   string `json:"status,omitempty"`
	Provenance
	Version int `json:"version,omitempty"`
}
type Bed struct {
	Category  string `json:"category"`
	Total     *int   `json:"total"`
	Available *int   `json:"available"`
	Provenance
	Version int `json:"version"`
}

// Coded is an upstream code with its display name; Name is null until the source has been re-synced.
type Coded struct {
	Code string  `json:"code"`
	Name *string `json:"name"`
}

// InpatientBeds summarizes the general (rawat inap) bed category. Null means unknown, never 0.
type InpatientBeds struct {
	Total     *int   `json:"total"`
	Available *int   `json:"available"`
	Freshness string `json:"freshness"`
}

// FacilitySummary is a read-only roll-up of what is known about ICU/NICU/PICU, inpatient care and BPJS,
// so clients need not walk the services/beds arrays. Missing data stays "unknown"/null.
type FacilitySummary struct {
	ICU       string `json:"icu"`
	NICU      string `json:"nicu"`
	PICU      string `json:"picu"`
	RawatInap string `json:"rawat_inap"`
	BPJS      *bool  `json:"bpjs"`
	// DataOrigin is "simulation" when any figure above is synthetic; otherwise the origin of the known
	// figures, or null when nothing is known.
	DataOrigin          *string       `json:"data_origin"`
	InpatientBeds       InpatientBeds `json:"inpatient_beds"`
	LastSourceUpdatedAt *time.Time    `json:"last_source_updated_at"`
}
type Hospital struct {
	ID              string   `json:"id"`
	ExternalID      *string  `json:"external_id"`
	SourceNamespace *string  `json:"source_namespace"`
	Name            string   `json:"name"`
	Address         *string  `json:"address"`
	CityCode        string   `json:"city_code"`
	CityName        string   `json:"city_name"`
	ProvinceCode    string   `json:"province_code"`
	ProvinceName    string   `json:"province_name"`
	Latitude        *float64 `json:"latitude"`
	Longitude       *float64 `json:"longitude"`
	Phone           *string  `json:"phone"`
	BPJS            *bool    `json:"bpjs"`
	DistanceKM      *float64 `json:"distance_km"`
	// BPJS provenance is separate from the hospital's: it is set only by curation with a dated source.
	BPJSDataOrigin      *string    `json:"bpjs_data_origin"`
	BPJSSourceURL       *string    `json:"bpjs_source_url"`
	BPJSSourceUpdatedAt *time.Time `json:"bpjs_source_updated_at"`
	FacilityClass       *Coded     `json:"facility_class"`
	FacilitySubtype     *Coded     `json:"facility_subtype"`
	Operational         *bool      `json:"operational"`
	Provenance
	Summary  FacilitySummary `json:"facility_summary"`
	Services []Service       `json:"services"`
	Beds     []Bed           `json:"beds"`
}

// Region is one province or city from the regions master; codes are used exactly as stored.
type Region struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	ParentCode *string `json:"parent_code"`
}

// Suggestion is a compact hospital entry for autocomplete.
type Suggestion struct {
	ID         string  `json:"id"`
	ExternalID *string `json:"external_id"`
	Name       string  `json:"name"`
	CityName   string  `json:"city_name"`
	DataOrigin string  `json:"data_origin"`
}
type History struct {
	FromStatus *string   `json:"from_status"`
	ToStatus   string    `json:"to_status"`
	PublicNote string    `json:"public_note"`
	CreatedAt  time.Time `json:"created_at"`
}
type Referral struct {
	ID                    string    `json:"id"`
	OwnerHospitalID       string    `json:"owner_hospital_id"`
	DestinationHospitalID *string   `json:"destination_hospital_id"`
	Status                string    `json:"status"`
	PublicNote            string    `json:"public_note"`
	NextStep              string    `json:"next_step"`
	Version               int       `json:"version"`
	IsDemo                bool      `json:"is_demo"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// AdminReferral is the operator list view of a referral: the referral plus the owner hospital's name,
// joined by the server so clients need no per-row hospital lookup.
type AdminReferral struct {
	Referral
	HospitalName string `json:"hospital_name"`
}
type Operator struct {
	ID          string   `json:"id"`
	Email       string   `json:"email"`
	HospitalIDs []string `json:"hospital_ids"`
}

func Freshness(source *time.Time, now time.Time) string {
	if source == nil || source.After(now) {
		return "unknown"
	}
	age := now.Sub(*source)
	if age < time.Hour {
		return "fresh"
	}
	if age <= 24*time.Hour {
		return "aging"
	}
	return "stale"
}
func CanTransition(from, to, note string) bool {
	switch from {
	case "submitted":
		return to == "verified"
	case "verified":
		return to == "searching" || to == "rejected"
	case "searching":
		return to == "confirmed" || to == "rejected"
	case "confirmed":
		return to == "completed" || (to == "searching" && strings.TrimSpace(note) != "")
	}
	return false
}
func ValidStatus(s string) bool {
	switch s {
	case "submitted", "verified", "searching", "confirmed", "completed", "rejected":
		return true
	}
	return false
}
func ValidServiceStatus(s string) bool {
	switch s {
	case "available", "limited", "unavailable", "unknown":
		return true
	}
	return false
}
func ValidBeds(t, a *int) bool {
	return (t == nil || *t >= 0) && (a == nil || *a >= 0) && (t == nil || a == nil || *a <= *t)
}
func Distance(a, b, c, d float64) float64 {
	const r = math.Pi / 180
	x := math.Pow(math.Sin((c-a)*r/2), 2) + math.Cos(a*r)*math.Cos(c*r)*math.Pow(math.Sin((d-b)*r/2), 2)
	return 6371 * 2 * math.Asin(math.Sqrt(math.Min(1, x)))
}
func (h *Hospital) Refresh(now time.Time) {
	h.Freshness = Freshness(h.SourceUpdatedAt, now)
	for i := range h.Services {
		h.Services[i].Freshness = Freshness(h.Services[i].SourceUpdatedAt, now)
	}
	for i := range h.Beds {
		h.Beds[i].Freshness = Freshness(h.Beds[i].SourceUpdatedAt, now)
	}
	h.Summary = h.summary()
}

func (h *Hospital) summary() FacilitySummary {
	status := func(slug string) string {
		for _, s := range h.Services {
			if s.Slug == slug && ValidServiceStatus(s.Status) {
				return s.Status
			}
		}
		return "unknown"
	}
	sum := FacilitySummary{ICU: status("icu"), NICU: status("nicu"), PICU: status("picu"), RawatInap: status("rawat-inap"), BPJS: h.BPJS, InpatientBeds: InpatientBeds{Freshness: "unknown"}}
	latest := func(t *time.Time) {
		if t != nil && (sum.LastSourceUpdatedAt == nil || t.After(*sum.LastSourceUpdatedAt)) {
			sum.LastSourceUpdatedAt = t
		}
	}
	var origin string
	note := func(o string) {
		if o != "" && origin != "simulation" {
			origin = o
		}
	}
	if h.BPJS != nil && h.BPJSDataOrigin != nil {
		note(*h.BPJSDataOrigin)
	}
	for _, s := range h.Services {
		latest(s.SourceUpdatedAt)
		if s.Status != "unknown" {
			note(s.DataOrigin)
		}
	}
	for _, b := range h.Beds {
		latest(b.SourceUpdatedAt)
		note(b.DataOrigin)
		if b.Category == "general" {
			sum.InpatientBeds = InpatientBeds{Total: b.Total, Available: b.Available, Freshness: b.Freshness}
		}
	}
	if origin != "" {
		sum.DataOrigin = &origin
	}
	return sum
}

// HospitalImport is a normalized upstream facility record, independent of the upstream payload format.
type HospitalImport struct {
	ExternalID, SourceNamespace, Name                             string
	Address, Phone                                                *string
	ProvinceCode, ProvinceName, CityCode, CityName                string
	Latitude, Longitude                                           *float64
	SourceActive                                                  bool
	SourceStatus, FacilityCode, FacilityTypeCode, FacilitySubtype string
	FacilityClass                                                 string
	FacilityClassName, FacilitySubtypeName                        string
	Operational                                                   *bool
	SyncedAt                                                      time.Time
}
type ImportResult struct {
	Created, Updated, Unchanged, Failed int
}

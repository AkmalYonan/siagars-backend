package satusehat

import (
	"math"
	"regexp"
	"siagars/backend/internal/domain"
	"strconv"
	"strings"
	"time"
)

// Skip reasons are counted in the sync summary; they never carry record content.
const (
	SkipType   = "not_hospital"
	SkipID     = "missing_id"
	SkipName   = "missing_name"
	SkipRegion = "invalid_region"
)

var (
	provinceCode = regexp.MustCompile(`^[0-9]{2}$`)
	cityCode     = regexp.MustCompile(`^[0-9]{4}$`)
)

func clean(t Text) string { return strings.TrimSpace(string(t)) }
func optional(t Text) *string {
	if s := clean(t); s != "" {
		return &s
	}
	return nil
}
func code(v *MSICodedValue) string {
	if v == nil {
		return ""
	}
	return clean(v.Code)
}

// Coordinates are parsed as a pair: if either is empty, invalid or out of range both are unknown.
// 0,0 is treated as unknown because it is a common placeholder and lies far outside Indonesia.
func coordinates(lat, lng Text) (*float64, *float64) {
	a, e1 := strconv.ParseFloat(clean(lat), 64)
	b, e2 := strconv.ParseFloat(clean(lng), 64)
	if e1 != nil || e2 != nil || math.IsNaN(a) || math.IsNaN(b) || math.IsInf(a, 0) || math.IsInf(b, 0) || a < -90 || a > 90 || b < -180 || b > 180 || (a == 0 && b == 0) {
		return nil, nil
	}
	return &a, &b
}

// Map normalizes one MSI record. It never fills BPJS, services, beds, source_updated_at
// or verified_at, because MSI does not provide evidence for them.
func Map(f MSIFacility, namespace string, syncedAt time.Time) (domain.HospitalImport, string, bool) {
	if code(f.FacilityType) != HospitalJenisSarana {
		return domain.HospitalImport{}, SkipType, false
	}
	id := clean(f.SATUSEHATCode)
	if id == "" {
		return domain.HospitalImport{}, SkipID, false
	}
	name := clean(f.Name)
	if name == "" {
		return domain.HospitalImport{}, SkipName, false
	}
	if f.Province == nil || f.City == nil {
		return domain.HospitalImport{}, SkipRegion, false
	}
	province, city := clean(f.Province.Code), clean(f.City.Code)
	provinceName, cityName := clean(f.Province.Name), clean(f.City.Name)
	if !provinceCode.MatchString(province) || !cityCode.MatchString(city) || !strings.HasPrefix(city, province) || provinceName == "" || cityName == "" {
		return domain.HospitalImport{}, SkipRegion, false
	}
	lat, lng := coordinates(f.Latitude, f.Longitude)
	return domain.HospitalImport{
		ExternalID: id, SourceNamespace: namespace, Name: name, Address: optional(f.Address), Phone: optional(f.Phone),
		ProvinceCode: province, ProvinceName: provinceName, CityCode: city, CityName: cityName, Latitude: lat, Longitude: lng,
		// Only an explicit status_aktif=false deactivates a facility.
		SourceActive: !f.Active.Valid || f.Active.Value,
		SourceStatus: clean(f.FacilityStatus), FacilityCode: clean(f.FacilityCode), FacilityTypeCode: code(f.FacilityType),
		FacilitySubtype: code(f.Subtype), FacilityClass: code(f.FacilityClass), SyncedAt: syncedAt.UTC().Truncate(time.Millisecond),
	}, "", true
}

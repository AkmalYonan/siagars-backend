package service

import (
	"fmt"
	"net/url"
	"siagars/backend/internal/domain"
)

// InsightsQuery is the validated query of /insights/*: dataset (default real), optional province and, for
// service-gaps only, the service slug.
type InsightsQuery struct {
	Dataset, Province, Service string
}

// ParseInsights validates the query. withService is true for service-gaps, where "service" is required;
// elsewhere it is an unknown parameter. Unknown, empty or repeated parameters are errors.
func ParseInsights(v url.Values, withService bool) (InsightsQuery, error) {
	q := InsightsQuery{Dataset: "real"}
	for k, values := range v {
		if (k != "province_code" && k != "dataset" && (k != "service" || !withService)) || len(values) != 1 || values[0] == "" {
			return q, fmt.Errorf("parameter tidak didukung, kosong atau berulang: %s", k)
		}
	}
	if x := v.Get("dataset"); x != "" {
		if x != "real" && x != "simulation" && x != "all" {
			return q, fmt.Errorf("dataset harus real, simulation atau all")
		}
		q.Dataset = x
	}
	q.Province, q.Service = v.Get("province_code"), v.Get("service")
	if withService && q.Service == "" {
		return q, fmt.Errorf("service wajib diisi")
	}
	return q, nil
}

// InsightHospitals keeps the hospitals an insight is computed over: the dataset rule of search plus the
// optional province.
func InsightHospitals(hs []domain.Hospital, q InsightsQuery) []domain.Hospital {
	out := make([]domain.Hospital, 0, len(hs))
	for _, h := range hs {
		if inDataset(h, q.Dataset) && (q.Province == "" || h.ProvinceCode == q.Province) {
			out = append(out, h)
		}
	}
	return out
}

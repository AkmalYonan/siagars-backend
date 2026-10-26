package domain

// Marker is the lightweight map pin of a hospital (contract §7).
type Marker struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Lat              float64 `json:"lat"`
	Lng              float64 `json:"lng"`
	Match            string  `json:"match"`
	DataQualityLabel string  `json:"data_quality_label"`
	HasSimulatedData bool    `json:"has_simulated_data"`
}

// MarkerOf builds the pin of a refreshed hospital that has coordinates. match is "all" when every
// requested service is available/limited (or none was requested), "partial" when some are, else
// "none". has_simulated_data is true when the hospital, a service, a bed or the BPJS value is simulated.
func MarkerOf(h Hospital, services []string) Marker {
	m := Marker{ID: h.ID, Name: h.Name, Match: "all", DataQualityLabel: h.DataQuality.Label}
	if h.Latitude != nil && h.Longitude != nil {
		m.Lat, m.Lng = *h.Latitude, *h.Longitude
	}
	matched := 0
	for _, slug := range services {
		if s, found := serviceOf(h, slug); found && (s.Status == "available" || s.Status == "limited") {
			matched++
		}
	}
	switch {
	case len(services) == 0 || matched == len(services):
	case matched > 0:
		m.Match = "partial"
	default:
		m.Match = "none"
	}
	m.HasSimulatedData = h.DataOrigin == "simulation" || h.BPJSDataOrigin != nil && *h.BPJSDataOrigin == "simulation"
	for _, s := range h.Services {
		m.HasSimulatedData = m.HasSimulatedData || s.DataOrigin == "simulation"
	}
	for _, b := range h.Beds {
		m.HasSimulatedData = m.HasSimulatedData || b.DataOrigin == "simulation"
	}
	return m
}

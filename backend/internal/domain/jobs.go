package domain

import "time"

// SimulationRefresh reports one run of the simulation refresh job. Counts cover simulation items only;
// curated and upstream data are never touched by the job.
type SimulationRefresh struct {
	At                  time.Time `json:"at"`
	ImportedHospitals   int       `json:"imported_hospitals"`
	SimulationHospitals int       `json:"simulation_hospitals"`
	Services            int       `json:"services"`
	Beds                int       `json:"beds"`
}

package satusehat

import (
	"bytes"
	"encoding/json"
	"errors"
)

// Text accepts a JSON string, number or null. The MSI documentation types several fields
// (region codes, coordinates, expires_in) inconsistently between schema and examples.
type Text string

func (t *Text) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	switch {
	case bytes.Equal(b, []byte("null")):
		*t = ""
	case len(b) > 0 && b[0] == '"':
		var s string
		if e := json.Unmarshal(b, &s); e != nil {
			return e
		}
		*t = Text(s)
	default:
		var n json.Number
		if e := json.Unmarshal(b, &n); e != nil {
			return errors.New("expected string, number or null")
		}
		*t = Text(n.String())
	}
	return nil
}

// Flag is an optional boolean; Valid is false when the field is absent or null.
type Flag struct{ Value, Valid bool }

func (f *Flag) UnmarshalJSON(b []byte) error {
	switch string(bytes.TrimSpace(b)) {
	case "null":
		*f = Flag{}
	case "true", `"true"`:
		*f = Flag{true, true}
	case "false", `"false"`:
		*f = Flag{false, true}
	default:
		return errors.New("expected boolean or null")
	}
	return nil
}

type MSIResponse struct {
	StatusCode int           `json:"status_code"`
	Message    string        `json:"message"`
	Page       int           `json:"page"`
	TotalPage  int           `json:"total_page"`
	Data       []MSIFacility `json:"data"`
}
type MSIRegion struct {
	Code Text `json:"kode"`
	Name Text `json:"nama"`
}
type MSICodedValue struct {
	Code Text `json:"kode"`
	Name Text `json:"nama"`
}

// MSIFacility holds only the fields this phase maps. Unknown fields (email, website,
// sarana_administrasi, ...) are ignored by the decoder. Nested objects may be null.
type MSIFacility struct {
	SATUSEHATCode  Text           `json:"kode_satusehat"`
	FacilityCode   Text           `json:"kode_sarana"`
	Name           Text           `json:"nama"`
	Phone          Text           `json:"telp"`
	Address        Text           `json:"alamat"`
	Longitude      Text           `json:"longitude"`
	Latitude       Text           `json:"latitude"`
	Operational    Flag           `json:"operasional"`
	FacilityStatus Text           `json:"status_sarana"`
	Active         Flag           `json:"status_aktif"`
	Province       *MSIRegion     `json:"provinsi"`
	City           *MSIRegion     `json:"kabkota"`
	FacilityType   *MSICodedValue `json:"jenis_sarana"`
	Subtype        *MSICodedValue `json:"subjenis"`
	FacilityClass  *MSICodedValue `json:"kelas_sarana"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   Text   `json:"expires_in"`
}

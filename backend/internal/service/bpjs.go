package service

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// BPJSInput is a validated membership check request. Name has tidy spaces; NameKey is the lower-case
// form used only for comparison.
type BPJSInput struct {
	NIK, Name, NameKey, BirthDate string
}

var wib = time.FixedZone("WIB", 7*60*60)

// ParseBPJSCheck validates the request fields. Error messages never repeat the submitted values.
func ParseBPJSCheck(nik, name, birthDate string, now time.Time) (BPJSInput, error) {
	var in BPJSInput
	if len(nik) != 16 || strings.Trim(nik, "0123456789") != "" {
		return in, fmt.Errorf("NIK harus tepat 16 digit angka")
	}
	name = strings.Join(strings.Fields(name), " ")
	if n := utf8.RuneCountInString(name); n < 1 || n > 100 {
		return in, fmt.Errorf("nama wajib diisi, maksimal 100 karakter")
	}
	letters := 0
	for _, r := range name {
		switch {
		case unicode.IsLetter(r):
			letters++
		case unicode.IsMark(r), r == ' ', r == '.', r == '\'', r == '-':
		default:
			return in, fmt.Errorf("nama hanya boleh berisi huruf, spasi, titik, apostrof, dan tanda hubung")
		}
	}
	if letters == 0 {
		return in, fmt.Errorf("nama harus memuat huruf")
	}
	birth, e := time.Parse("2006-01-02", birthDate)
	if e != nil || birth.Format("2006-01-02") != birthDate {
		return in, fmt.Errorf("tanggal lahir harus berformat YYYY-MM-DD dan valid")
	}
	if birth.Before(time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)) {
		return in, fmt.Errorf("tanggal lahir tidak boleh sebelum 1900-01-01")
	}
	if today := now.In(wib).Format("2006-01-02"); birthDate > today {
		return in, fmt.Errorf("tanggal lahir tidak boleh di masa depan")
	}
	return BPJSInput{NIK: nik, Name: name, NameKey: strings.ToLower(name), BirthDate: birthDate}, nil
}

// NameKey is the comparison form of a stored name: tidy spaces and lower case, like BPJSInput.NameKey.
func NameKey(name string) string { return strings.ToLower(strings.Join(strings.Fields(name), " ")) }

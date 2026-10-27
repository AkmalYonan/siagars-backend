package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

// BPJSMember is a synthetic participant used by the BPJS membership simulation. It never describes a
// real person and is never returned unmasked by a public endpoint.
type BPJSMember struct {
	NIK, Name, BirthDate string
	Status               string
	InactiveReason       *string
	MembershipType       *Coded
	CareClass            *string
	PrimaryFacility      *Coded
}

// BPJSParticipant is the public, masked view of a matched member.
type BPJSParticipant struct {
	NIKMasked       string  `json:"nik_masked"`
	NameMasked      string  `json:"name_masked"`
	Status          string  `json:"status"`
	InactiveReason  *string `json:"inactive_reason"`
	MembershipType  *Coded  `json:"membership_type"`
	CareClass       *string `json:"care_class"`
	PrimaryFacility *Coded  `json:"primary_facility"`
}

// BPJSCheck is the public result of a membership check. Participant is nil when nothing matched.
type BPJSCheck struct {
	Result      string           `json:"result"`
	Participant *BPJSParticipant `json:"participant"`
	DataOrigin  string           `json:"data_origin"`
	CheckedAt   time.Time        `json:"checked_at"`
}

// PublicParticipant masks a member for public display.
func (m BPJSMember) PublicParticipant() *BPJSParticipant {
	return &BPJSParticipant{NIKMasked: MaskNIK(m.NIK), NameMasked: MaskName(m.Name), Status: m.Status, InactiveReason: m.InactiveReason,
		MembershipType: m.MembershipType, CareClass: m.CareClass, PrimaryFacility: m.PrimaryFacility}
}

// BPJSRecord is one history entry to store. NIK is only an input for the keyed fingerprint (nik_hmac);
// the repository never writes it, and no other unmasked input is kept (decision D-03).
type BPJSRecord struct {
	NIKMasked, NameMasked string
	BirthYear             int
	Result                string
	Status                *string
	NIK                   string
}

// BPJSHistoryItem is the masked list view of a stored check.
type BPJSHistoryItem struct {
	ID          string    `json:"id"`
	NIKMasked   string    `json:"nik_masked"`
	NameMasked  string    `json:"name_masked"`
	BirthYear   int       `json:"birth_year"`
	Result      string    `json:"result"`
	Status      *string   `json:"status"`
	CheckedAt   time.Time `json:"checked_at"`
	RepeatCount int       `json:"repeat_count"`
}

// MaskNIK keeps the first four and last four digits; anything that is not 16 bytes is fully masked.
func MaskNIK(nik string) string {
	if len(nik) != 16 {
		return strings.Repeat("*", 16)
	}
	return nik[:4] + strings.Repeat("*", 8) + nik[12:]
}

// MaskName keeps the first two letters of every word, upper-cased, and masks the rest.
func MaskName(name string) string {
	words := strings.Fields(name)
	for i, w := range words {
		runes := []rune(strings.ToUpper(w))
		keep := min(2, len(runes))
		words[i] = string(runes[:keep]) + strings.Repeat("*", utf8.RuneCountInString(w)-keep)
	}
	return strings.Join(words, " ")
}

package repository

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// HistoryHasher derives a stable keyed fingerprint of the NIK (HMAC-SHA256) so repeated checks can be
// counted without storing the NIK in any form, encrypted or not (decision D-03). The key never leaves
// the backend environment.
type HistoryHasher struct {
	macKey []byte
}

// NewHistoryHasher requires exactly 32 bytes of key material. The derivation label is unchanged from
// v0.3.0, so fingerprints written before migration 006 still match.
func NewHistoryHasher(key []byte) (*HistoryHasher, error) {
	if len(key) != 32 {
		return nil, errors.New("history key must be 32 bytes")
	}
	s := sha256.Sum256(append([]byte("siagars-bpjs-history-v1:mac:"), key...))
	return &HistoryHasher{macKey: s[:]}, nil
}

// Fingerprint is a keyed hash that lets repeated NIKs be counted without storing the NIK itself.
func (c *HistoryHasher) Fingerprint(value string) string {
	m := hmac.New(sha256.New, c.macKey)
	m.Write([]byte(value))
	return hex.EncodeToString(m.Sum(nil))
}

package repository

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// ErrRawUnavailable means a history entry has no encrypted original (no key was configured when stored,
// or no key is configured now).
var ErrRawUnavailable = errors.New("raw history unavailable")

// HistoryCipher encrypts the original input of BPJS checks (AES-256-GCM) and derives a stable keyed
// fingerprint of the NIK (HMAC-SHA256). Both subkeys come from one server secret that never leaves the
// backend environment.
type HistoryCipher struct {
	aead   cipher.AEAD
	macKey []byte
}

// NewHistoryCipher requires exactly 32 bytes of key material.
func NewHistoryCipher(key []byte) (*HistoryCipher, error) {
	if len(key) != 32 {
		return nil, errors.New("history key must be 32 bytes")
	}
	derive := func(label string) []byte { s := sha256.Sum256(append([]byte("siagars-bpjs-history-v1:"+label+":"), key...)); return s[:] }
	block, e := aes.NewCipher(derive("enc"))
	if e != nil {
		return nil, e
	}
	aead, e := cipher.NewGCM(block)
	if e != nil {
		return nil, e
	}
	return &HistoryCipher{aead: aead, macKey: derive("mac")}, nil
}

// Seal returns nonce||ciphertext, bound to aad (the history entry id) so a blob cannot be moved to
// another entry.
func (c *HistoryCipher) Seal(plain, aad []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, e := rand.Read(nonce); e != nil {
		return nil, e
	}
	return c.aead.Seal(nonce, nonce, plain, aad), nil
}

// Open reverses Seal; a wrong key, altered blob or different aad fails.
func (c *HistoryCipher) Open(blob, aad []byte) ([]byte, error) {
	n := c.aead.NonceSize()
	if len(blob) < n+c.aead.Overhead() {
		return nil, errors.New("invalid ciphertext")
	}
	return c.aead.Open(nil, blob[:n], blob[n:], aad)
}

// Fingerprint is a keyed hash that lets repeated NIKs be counted without storing the NIK itself.
func (c *HistoryCipher) Fingerprint(value string) string {
	m := hmac.New(sha256.New, c.macKey)
	m.Write([]byte(value))
	return hex.EncodeToString(m.Sum(nil))
}

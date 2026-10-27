package repository

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestHistoryHasher(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	c, e := NewHistoryHasher(key)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = NewHistoryHasher(key[:31]); e == nil {
		t.Fatal("kunci pendek diterima")
	}
	other, _ := NewHistoryHasher(bytes.Repeat([]byte{8}, 32))
	if c.Fingerprint("x") != c.Fingerprint("x") || c.Fingerprint("x") == c.Fingerprint("y") || c.Fingerprint("x") == other.Fingerprint("x") {
		t.Fatal("fingerprint tidak stabil/berbeda")
	}
	if fp := c.Fingerprint("9999010101900001"); len(fp) != 64 || bytes.Contains([]byte(fp), []byte("9999010101900001")) {
		t.Fatal("fingerprint bukan hex HMAC-SHA256 atau memuat input:", fp)
	}
	// The derivation must stay compatible with fingerprints stored by v0.3.0: subkey = SHA256(label+key).
	sub := sha256.Sum256(append([]byte("siagars-bpjs-history-v1:mac:"), key...))
	m := hmac.New(sha256.New, sub[:])
	m.Write([]byte("9999010101900001"))
	if want := hex.EncodeToString(m.Sum(nil)); c.Fingerprint("9999010101900001") != want {
		t.Fatal("derivasi kunci HMAC berubah; repeat_count riwayat lama akan terputus")
	}
}

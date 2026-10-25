package repository

import (
	"bytes"
	"testing"
)

func TestHistoryCipher(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	c, e := NewHistoryCipher(key)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = NewHistoryCipher(key[:31]); e == nil {
		t.Fatal("kunci pendek diterima")
	}
	plain := []byte(`{"nik":"9999010101900001"}`)
	a, _ := c.Seal(plain, []byte("id-1"))
	b, _ := c.Seal(plain, []byte("id-1"))
	if bytes.Equal(a, b) {
		t.Fatal("nonce berulang")
	}
	if bytes.Contains(a, []byte("9999010101900001")) {
		t.Fatal("ciphertext memuat plaintext")
	}
	if got, e := c.Open(a, []byte("id-1")); e != nil || !bytes.Equal(got, plain) {
		t.Fatalf("round trip: %q %v", got, e)
	}
	if _, e = c.Open(a, []byte("id-2")); e == nil {
		t.Fatal("aad berbeda diterima")
	}
	tampered := append([]byte(nil), a...)
	tampered[len(tampered)-1] ^= 1
	if _, e = c.Open(tampered, []byte("id-1")); e == nil {
		t.Fatal("blob diubah diterima")
	}
	if _, e = c.Open(a[:5], []byte("id-1")); e == nil {
		t.Fatal("blob pendek diterima")
	}
	other, _ := NewHistoryCipher(bytes.Repeat([]byte{8}, 32))
	if _, e = other.Open(a, []byte("id-1")); e == nil {
		t.Fatal("kunci lain diterima")
	}
	if c.Fingerprint("x") != c.Fingerprint("x") || c.Fingerprint("x") == c.Fingerprint("y") || c.Fingerprint("x") == other.Fingerprint("x") {
		t.Fatal("fingerprint tidak stabil/berbeda")
	}
}

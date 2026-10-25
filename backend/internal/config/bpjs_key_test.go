package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestBPJSHistoryKey(t *testing.T) {
	baseline(t)
	t.Setenv("BPJS_HISTORY_KEY", "")
	if c, e := Load(); e != nil || c.BPJSHistoryKey != nil {
		t.Fatalf("tanpa kunci: %v %v", c.BPJSHistoryKey, e)
	}
	good := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	t.Setenv("BPJS_HISTORY_KEY", good)
	if c, e := Load(); e != nil || len(c.BPJSHistoryKey) != 32 {
		t.Fatalf("kunci sah: %v", e)
	}
	for _, bad := range []string{"bukan-base64!!", base64.StdEncoding.EncodeToString([]byte("pendek")), base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 33)))} {
		t.Setenv("BPJS_HISTORY_KEY", bad)
		_, e := Load()
		if e == nil {
			t.Errorf("kunci %q diterima", bad)
		} else if strings.Contains(e.Error(), bad) {
			t.Errorf("galat memuat nilai kunci")
		}
	}
}

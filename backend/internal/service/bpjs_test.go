package service

import (
	"strings"
	"testing"
	"time"
)

func TestParseBPJSCheck(t *testing.T) {
	now := time.Date(2026, 10, 25, 20, 0, 0, 0, time.UTC) // 26 Oct 03:00 WIB
	good := func(nik, name, birth string) {
		t.Helper()
		if _, e := ParseBPJSCheck(nik, name, birth, now); e != nil {
			t.Errorf("(%q,%q,%q): %v", nik, name, birth, e)
		}
	}
	bad := func(nik, name, birth string) {
		t.Helper()
		_, e := ParseBPJSCheck(nik, name, birth, now)
		if e == nil {
			t.Errorf("(%q,%q,%q) diterima", nik, name, birth)
			return
		}
		for _, v := range []string{nik, name, birth} {
			if len(v) > 3 && strings.Contains(e.Error(), v) {
				t.Errorf("pesan menggemakan input %q: %s", v, e)
			}
		}
	}
	good("9999010101900001", "Budi Santoso", "1990-01-01")
	good("9999010101900001", "José Álvarez-Núñez", "1990-01-01")
	good("9999010101900001", "D'Arc Jr.", "1900-01-01")
	good("9999010101900001", "Budi", "2026-10-26") // already 26 Oct in WIB
	bad("999901010190000", "Budi", "1990-01-01")   // 15 digits
	bad("99990101019000011", "Budi", "1990-01-01") // 17 digits
	bad("99990101019000a1", "Budi", "1990-01-01")
	bad("９９９９０１０１０１９０００１０", "Budi", "1990-01-01") // full-width digits
	bad("9999010101900001", "", "1990-01-01")
	bad("9999010101900001", "   ", "1990-01-01")
	bad("9999010101900001", strings.Repeat("a", 101), "1990-01-01")
	bad("9999010101900001", "Budi123", "1990-01-01")
	bad("9999010101900001", "...", "1990-01-01")
	bad("9999010101900001", "Budi", "1990-1-1")
	bad("9999010101900001", "Budi", "1990-02-30")
	bad("9999010101900001", "Budi", "1899-12-31")
	bad("9999010101900001", "Budi", "2026-10-27") // future in WIB
	in, _ := ParseBPJSCheck("9999010101900001", "  Budi   SANTOSO ", "1990-01-01", now)
	if in.Name != "Budi SANTOSO" || in.NameKey != "budi santoso" || NameKey("budi  santoso") != in.NameKey {
		t.Errorf("normalisasi: %+v", in)
	}
}

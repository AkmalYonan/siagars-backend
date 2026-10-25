package domain

import "testing"

func TestMasking(t *testing.T) {
	for in, want := range map[string]string{"3171010101900001": "3171********0001", "123": "****************", "": "****************"} {
		if got := MaskNIK(in); got != want {
			t.Errorf("MaskNIK(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"Budi Santoso":       "BU** SA*****",
		"  budi   santoso  ": "BU** SA*****",
		"A":                  "A",
		"Al":                 "AL",
		"José Álvarez-Núñez": "JO** ÁL***********",
		"":                   "",
	} {
		if got := MaskName(in); got != want {
			t.Errorf("MaskName(%q) = %q, want %q", in, got, want)
		}
	}
}

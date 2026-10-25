package seed

import (
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestDummyFacilitiesDeterministicAndValid(t *testing.T) {
	at := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	var withBeds, withoutBeds, bpjsTrue, bpjsFalse int
	for i := 0; i < 400; i++ {
		ext := "EXT-" + strconv.Itoa(i)
		class := []string{"A", "B", "C", "D", ""}[i%5]
		b1, s1, beds1 := DummyFacilities(ext, class, at)
		b2, s2, beds2 := DummyFacilities(ext, class, at)
		if b1 != b2 || !reflect.DeepEqual(s1, s2) || !reflect.DeepEqual(beds1, beds2) {
			t.Fatalf("%s: tidak deterministik", ext)
		}
		if len(s1) != 6 {
			t.Fatalf("%s: %d layanan", ext, len(s1))
		}
		for _, s := range s1 {
			if s.DataOrigin != "simulation" || s.SourceURL != nil || s.VerifiedAt != nil || s.Version != 1 {
				t.Fatalf("%s: provenance layanan %+v", ext, s)
			}
			switch s.Status {
			case "available", "limited", "unavailable", "unknown":
			default:
				t.Fatalf("%s: status %q", ext, s.Status)
			}
		}
		for _, b := range beds1 {
			if b.DataOrigin != "simulation" || b.SourceURL != nil || b.VerifiedAt != nil || b.Total == nil || b.Available == nil || *b.Available > *b.Total || *b.Available < 0 {
				t.Fatalf("%s: bed %+v", ext, b)
			}
		}
		if len(beds1) > 0 {
			withBeds++
		} else {
			withoutBeds++
		}
		if b1 {
			bpjsTrue++
		} else {
			bpjsFalse++
		}
	}
	// Both branches must be reachable so clients can exercise unknown and known paths.
	if withBeds == 0 || withoutBeds == 0 || bpjsTrue == 0 || bpjsFalse == 0 {
		t.Fatalf("variasi kurang: beds %d/%d bpjs %d/%d", withBeds, withoutBeds, bpjsTrue, bpjsFalse)
	}
}

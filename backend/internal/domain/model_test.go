package domain

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func TestFreshnessBoundaries(t *testing.T) {
	now := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		age  time.Duration
		want string
	}{{-time.Second, "unknown"}, {0, "fresh"}, {time.Hour - time.Nanosecond, "fresh"}, {time.Hour, "aging"}, {24 * time.Hour, "aging"}, {24*time.Hour + time.Nanosecond, "stale"}}
	if Freshness(nil, now) != "unknown" {
		t.Fatal("nil")
	}
	for _, c := range cases {
		source := now.Add(-c.age)
		if got := Freshness(&source, now); got != c.want {
			t.Errorf("%v: %s", c.age, got)
		}
	}
}
func TestTransitionsExhaustive(t *testing.T) {
	states := []string{"submitted", "verified", "searching", "confirmed", "completed", "rejected", "invalid"}
	legal := map[string]bool{"submitted:verified": true, "verified:searching": true, "verified:rejected": true, "searching:confirmed": true, "searching:rejected": true, "confirmed:completed": true, "confirmed:searching": true}
	for _, from := range states {
		for _, to := range states {
			if got := CanTransition(from, to, "alasan"); got != legal[from+":"+to] {
				t.Errorf("%s -> %s: %v", from, to, got)
			}
		}
	}
	if CanTransition("confirmed", "searching", " \t") {
		t.Fatal("diversion without reason")
	}
}
func TestBedsNullZero(t *testing.T) {
	zero, one, negative := 0, 1, -1
	for _, c := range []struct {
		total, available *int
		valid            bool
	}{{nil, nil, true}, {&zero, &zero, true}, {nil, &one, true}, {&zero, &one, false}, {&negative, nil, false}, {nil, &negative, false}} {
		if ValidBeds(c.total, c.available) != c.valid {
			t.Fatal(c)
		}
	}
	b, _ := json.Marshal(Bed{Total: nil, Available: &zero})
	if !strings.Contains(string(b), "\"total\":null") || !strings.Contains(string(b), "\"available\":0") {
		t.Fatal(string(b))
	}
}
func TestDistance(t *testing.T) {
	if Distance(0, 0, 0, 0) != 0 {
		t.Fatal("identity")
	}
	if math.Abs(Distance(0, 0, 0, 1)-111.1949) > 0.001 {
		t.Fatal("great circle")
	}
	if math.IsNaN(Distance(0, 0, 0, 180)) {
		t.Fatal("antipodes")
	}
}

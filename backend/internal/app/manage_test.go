package app

import "testing"

func TestMinOperatorPassword(t *testing.T) {
	for env, want := range map[string]int{"": 8, "development": 8, "test": 12, "production": 12} {
		if got := MinOperatorPassword(env); got != want {
			t.Fatalf("%q: got %d want %d", env, got, want)
		}
	}
}

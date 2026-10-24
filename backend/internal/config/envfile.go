package config

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var envKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// LoadEnvFile sets KEY=VALUE pairs from a dotenv file for local development. Variables already
// present in the process environment win, so explicit overrides keep working. It supports
// comments, blank lines, an optional "export " prefix and single or double quoted values.
// Errors report line numbers only, never values.
func LoadEnvFile(path string) (int, error) {
	f, e := os.Open(path)
	if e != nil {
		return 0, e
	}
	defer f.Close()
	loaded, line := 0, 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		key, value, found := strings.Cut(strings.TrimPrefix(text, "export "), "=")
		key = strings.TrimSpace(key)
		if !found || !envKey.MatchString(key) {
			return loaded, fmt.Errorf("%s line %d: expected KEY=VALUE", path, line)
		}
		value = strings.TrimSpace(value)
		if n := len(value); n >= 2 && (value[0] == '"' || value[0] == '\'') {
			if value[n-1] != value[0] {
				return loaded, fmt.Errorf("%s line %d: unterminated quote", path, line)
			}
			value = value[1 : n-1]
		} else if i := strings.Index(value, " #"); i >= 0 {
			value = strings.TrimSpace(value[:i])
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if e = os.Setenv(key, value); e != nil {
			return loaded, fmt.Errorf("%s line %d: %w", path, line, e)
		}
		loaded++
	}
	return loaded, scanner.Err()
}

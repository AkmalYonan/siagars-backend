package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestConfigurationGuardrails(t *testing.T) {
	for _, test := range []struct{ name, key, value string }{{"demo", "DEMO_MODE", "false"}, {"integration", "SATUSEHAT_ENABLED", "true"}, {"origin", "PUBLIC_ORIGIN", "http://localhost:3000/path"}, {"ttl", "SESSION_TTL_HOURS", "0"}, {"port", "PORT", "-1"}, {"environment", "APP_ENV", "unknown"}, {"production_http", "APP_ENV", "production"}, {"mongo_scheme", "MONGODB_URI", "postgres://localhost/demo"}, {"mongo_missing", "MONGODB_URI", ""}, {"database", "MONGODB_DATABASE", "bad/name"}} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("MONGODB_URI", "mongodb://localhost:27017")
			t.Setenv("MONGODB_DATABASE", "siagars")
			t.Setenv("PUBLIC_ORIGIN", "http://localhost:3000")
			t.Setenv("APP_ENV", "development")
			t.Setenv("SESSION_TTL_HOURS", "8")
			t.Setenv("PORT", "8080")
			t.Setenv("DEMO_MODE", "true")
			t.Setenv("SATUSEHAT_ENABLED", "false")
			t.Setenv(test.key, test.value)
			if _, e := Load(); e == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}

func baseline(t *testing.T) {
	t.Helper()
	for k, v := range map[string]string{"MONGODB_URI": "mongodb://localhost:27017", "MONGODB_DATABASE": "siagars", "PUBLIC_ORIGIN": "http://localhost:3000", "APP_ENV": "development", "SESSION_TTL_HOURS": "8", "PORT": "8080", "DEMO_MODE": "true",
		"SATUSEHAT_ENABLED": "", "SATUSEHAT_ENV": "", "SATUSEHAT_CLIENT_ID": "", "SATUSEHAT_CLIENT_SECRET": "", "SATUSEHAT_PAGE_LIMIT": "", "SATUSEHAT_HTTP_TIMEOUT_SECONDS": "", "SATUSEHAT_KABKOTA": ""} {
		t.Setenv(k, v)
	}
}
func TestSATUSEHATDefaultsDisabled(t *testing.T) {
	baseline(t)
	c, e := Load()
	if e != nil || c.SATUSEHAT.Enabled || c.SATUSEHAT.Environment != "sandbox" || c.SATUSEHAT.PageLimit != 200 || c.SATUSEHAT.HTTPTimeout != 10*time.Second {
		t.Fatal(c.SATUSEHAT, e)
	}
}
func TestSATUSEHATGuardrails(t *testing.T) {
	const id, secret = "placeholder-client-id", "placeholder-client-secret"
	for _, c := range []struct {
		name string
		env  map[string]string
		ok   bool
	}{
		{"enabled sandbox", map[string]string{"SATUSEHAT_ENABLED": "true", "SATUSEHAT_CLIENT_ID": id, "SATUSEHAT_CLIENT_SECRET": secret}, true},
		{"production", map[string]string{"SATUSEHAT_ENABLED": "true", "SATUSEHAT_ENV": "production", "SATUSEHAT_CLIENT_ID": id, "SATUSEHAT_CLIENT_SECRET": secret}, false},
		{"production disabled", map[string]string{"SATUSEHAT_ENV": "production"}, false},
		{"unknown env", map[string]string{"SATUSEHAT_ENV": "staging"}, false},
		{"missing secret", map[string]string{"SATUSEHAT_ENABLED": "true", "SATUSEHAT_CLIENT_ID": id}, false},
		{"blank id", map[string]string{"SATUSEHAT_ENABLED": "true", "SATUSEHAT_CLIENT_ID": "  ", "SATUSEHAT_CLIENT_SECRET": secret}, false},
		{"enabled typo", map[string]string{"SATUSEHAT_ENABLED": "yes"}, false},
		{"limit", map[string]string{"SATUSEHAT_PAGE_LIMIT": "2001"}, false},
		{"timeout", map[string]string{"SATUSEHAT_HTTP_TIMEOUT_SECONDS": "0"}, false},
		{"kabkota preset", map[string]string{"SATUSEHAT_KABKOTA": "Jabodetabek"}, true},
		{"kabkota list", map[string]string{"SATUSEHAT_KABKOTA": "3171, 3276"}, true},
		{"kabkota dotted", map[string]string{"SATUSEHAT_KABKOTA": "31.71"}, false},
		{"kabkota empty item", map[string]string{"SATUSEHAT_KABKOTA": "3171,,3276"}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			baseline(t)
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			_, e := LoadSATUSEHAT()
			if (e == nil) != c.ok {
				t.Fatal(e)
			}
			if e != nil && (strings.Contains(e.Error(), id) || strings.Contains(e.Error(), secret)) {
				t.Fatal("credential value in error message")
			}
			if _, e = Load(); (e == nil) != c.ok {
				t.Fatal("API config must apply the same guardrails", e)
			}
		})
	}
}
func TestSATUSEHATKabKota(t *testing.T) {
	baseline(t)
	t.Setenv("SATUSEHAT_KABKOTA", "jabodetabek")
	c, e := LoadSATUSEHAT()
	if e != nil || len(c.KabKota) != 14 || c.KabKota[0] != "3101" {
		t.Fatal(c.KabKota, e)
	}
	t.Setenv("SATUSEHAT_KABKOTA", "3171,3276,3171")
	if c, e = LoadSATUSEHAT(); e != nil || len(c.KabKota) != 2 {
		t.Fatal(c.KabKota, e)
	}
}
func TestLoadEnvFile(t *testing.T) {
	path := t.TempDir() + "/.env"
	body := "# comment\n\nSIAGARS_T_PLAIN=abc\nexport SIAGARS_T_EXPORT=1\nSIAGARS_T_DQ=\"quoted value\"\nSIAGARS_T_SQ='single'\nSIAGARS_T_COMMENT=v # note\nSIAGARS_T_EMPTY=\nSIAGARS_T_KEEP=from-file\n"
	if e := os.WriteFile(path, []byte(body), 0o600); e != nil {
		t.Fatal(e)
	}
	t.Setenv("SIAGARS_T_KEEP", "from-process")
	for _, k := range []string{"SIAGARS_T_PLAIN", "SIAGARS_T_EXPORT", "SIAGARS_T_DQ", "SIAGARS_T_SQ", "SIAGARS_T_COMMENT", "SIAGARS_T_EMPTY"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	n, e := LoadEnvFile(path)
	if e != nil || n != 6 {
		t.Fatal(n, e)
	}
	for k, want := range map[string]string{"SIAGARS_T_PLAIN": "abc", "SIAGARS_T_EXPORT": "1", "SIAGARS_T_DQ": "quoted value", "SIAGARS_T_SQ": "single", "SIAGARS_T_COMMENT": "v", "SIAGARS_T_EMPTY": "", "SIAGARS_T_KEEP": "from-process"} {
		if got := os.Getenv(k); got != want {
			t.Errorf("%s=%q want %q", k, got, want)
		}
	}
	for _, bad := range []string{"NOEQUALS\n", "1BAD=x\n", "SIAGARS_T_Q=\"open\n"} {
		os.WriteFile(path, []byte(bad), 0o600)
		if _, e = LoadEnvFile(path); e == nil || strings.Contains(e.Error(), "open") && strings.Contains(bad, "open") && strings.Contains(e.Error(), "\"open") {
			t.Fatal("malformed line accepted or value leaked", bad, e)
		}
	}
}

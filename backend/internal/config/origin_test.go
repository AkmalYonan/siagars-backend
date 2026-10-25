package config

import "testing"

func TestPublicOriginList(t *testing.T) {
	for _, test := range []struct {
		name, env, origins string
		ok                 bool
	}{
		{"single", "development", "http://localhost:3000", true},
		{"prod primary https plus localhost", "production", "https://fe.example,http://localhost:3000", true},
		{"prod loopback ip", "production", "https://fe.example,http://127.0.0.1:3000", true},
		{"prod http primary", "production", "http://localhost:3000,https://fe.example", false},
		{"prod http remote", "production", "https://fe.example,http://evil.example", false},
		{"path", "production", "https://fe.example,https://b.example/x", false},
		{"trailing slash", "production", "https://fe.example/", false},
		{"empty entry", "production", "https://fe.example,", false},
		{"wildcard", "production", "https://*.example.com", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			baseline(t)
			t.Setenv("APP_ENV", test.env)
			t.Setenv("PUBLIC_ORIGIN", test.origins)
			_, e := Load()
			if (e == nil) != test.ok {
				t.Fatalf("ok=%v err=%v", test.ok, e)
			}
		})
	}
}

func TestOriginAllowedExactMatch(t *testing.T) {
	baseline(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("PUBLIC_ORIGIN", "https://fe.example, http://localhost:3000")
	c, e := Load()
	if e != nil || c.PublicOrigin != "https://fe.example" || !c.SecureCookie {
		t.Fatalf("%+v %v", c, e)
	}
	for origin, want := range map[string]bool{"https://fe.example": true, "http://localhost:3000": true, "http://localhost:3001": false, "https://fe.example/": false, "HTTPS://FE.EXAMPLE": false, "null": false, "": false, "https://evil.example": false} {
		if got := c.OriginAllowed(origin); got != want {
			t.Errorf("%q: got %v", origin, got)
		}
	}
}

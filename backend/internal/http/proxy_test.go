package httpapi

import (
	"net/http/httptest"
	"net/netip"
	"siagars/backend/internal/config"
	"testing"
)

func TestClientIPTrustedProxies(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("192.168.1.7/32")}
	for _, c := range []struct {
		name, remote, xff string
		proxies           []netip.Prefix
		want              string
	}{
		{"no proxies configured ignores header", "10.0.0.1:1234", "203.0.113.9", nil, "10.0.0.1"},
		{"untrusted peer cannot spoof", "198.51.100.4:1234", "203.0.113.9", trusted, "198.51.100.4"},
		{"trusted peer uses header", "10.0.0.1:1234", "203.0.113.9", trusted, "203.0.113.9"},
		{"rightmost untrusted hop wins over spoofed prefix", "10.0.0.1:1234", "1.2.3.4, 203.0.113.9, 10.0.0.2", trusted, "203.0.113.9"},
		{"single host entry", "192.168.1.7:80", "203.0.113.9", trusted, "203.0.113.9"},
		{"missing header falls back to peer", "10.0.0.1:1234", "", trusted, "10.0.0.1"},
		{"garbage header falls back to peer", "10.0.0.1:1234", "not-an-ip", trusted, "10.0.0.1"},
		{"all hops trusted falls back to peer", "10.0.0.1:1234", "10.1.1.1, 10.2.2.2", trusted, "10.0.0.1"},
		{"ipv6 client", "10.0.0.1:1234", "2001:db8::1", trusted, "2001:db8::1"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := &Server{cfg: config.Config{TrustedProxies: c.proxies}}
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = c.remote
			if c.xff != "" {
				r.Header.Set("X-Forwarded-For", c.xff)
			}
			if got := s.clientIP(r); got != c.want {
				t.Fatalf("got %s want %s", got, c.want)
			}
		})
	}
}

func TestRateLimitUsesForwardedClient(t *testing.T) {
	s := &Server{cfg: config.Config{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}}
	for i, c := range []struct {
		xff  string
		want bool
	}{{"203.0.113.1", true}, {"203.0.113.1", false}, {"203.0.113.2", true}} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = "10.0.0.1:1"
		r.Header.Set("X-Forwarded-For", c.xff)
		if got := s.rate(httptest.NewRecorder(), r, "lookup:"+s.clientIP(r), 1, 60e9); got != c.want {
			t.Fatalf("request %d: got %v want %v", i, got, c.want)
		}
	}
}

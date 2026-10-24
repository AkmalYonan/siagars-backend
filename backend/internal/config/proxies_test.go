package config

import "testing"

func TestTrustedProxies(t *testing.T) {
	baseline(t)
	if c, e := Load(); e != nil || len(c.TrustedProxies) != 0 {
		t.Fatal("default must trust nobody", c.TrustedProxies, e)
	}
	t.Setenv("TRUSTED_PROXIES", " 10.0.0.0/8, 192.168.1.7 ,2001:db8::/32 ,")
	c, e := Load()
	if e != nil || len(c.TrustedProxies) != 3 || c.TrustedProxies[1].String() != "192.168.1.7/32" {
		t.Fatal(c.TrustedProxies, e)
	}
	for _, bad := range []string{"10.0.0.0/33", "proxy.local", "10.0.0.0/8;1.1.1.1"} {
		t.Setenv("TRUSTED_PROXIES", bad)
		if _, e := Load(); e == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

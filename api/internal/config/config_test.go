package config

import "testing"

func TestTrustedProxies(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("TRUSTED_PROXIES", "")
	c, err := Load()
	if err != nil || len(c.TrustedProxies) != 0 {
		t.Fatalf("empty: %v %v", c.TrustedProxies, err)
	}

	t.Setenv("TRUSTED_PROXIES", " 10.0.0.0/8, 172.16.0.0/12 ,192.168.1.10")
	c, err = Load()
	if err != nil || len(c.TrustedProxies) != 3 || c.TrustedProxies[2].String() != "192.168.1.10/32" {
		t.Fatalf("list: %v %v", c.TrustedProxies, err)
	}

	t.Setenv("TRUSTED_PROXIES", "10.0.0.0/8,nope")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid CIDR")
	}
}

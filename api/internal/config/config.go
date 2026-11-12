package config

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL  string
	AppBaseURL   string
	Addr         string
	CookieSecure bool
	Timezone     *time.Location
	// TrustedProxies (TRUSTED_PROXIES, comma-separated CIDRs or IPs) may set
	// X-Real-IP. Empty means the header is never trusted.
	TrustedProxies []netip.Prefix
}

func Load() (Config, error) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return Config{}, fmt.Errorf("load timezone: %w", err)
	}
	c := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		AppBaseURL:  os.Getenv("APP_BASE_URL"),
		Addr:        os.Getenv("ADDR"),
		Timezone:    loc,
	}
	if c.Addr == "" {
		c.Addr = ":8080"
	}
	if v := os.Getenv("COOKIE_SECURE"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("COOKIE_SECURE: %w", err)
		}
		c.CookieSecure = b
	}
	proxies, err := parsePrefixes(os.Getenv("TRUSTED_PROXIES"))
	if err != nil {
		return Config{}, fmt.Errorf("TRUSTED_PROXIES: %w", err)
	}
	c.TrustedProxies = proxies
	if c.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	return c, nil
}

func parsePrefixes(s string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !strings.Contains(part, "/") {
			a, err := netip.ParseAddr(part)
			if err != nil {
				return nil, err
			}
			out = append(out, netip.PrefixFrom(a, a.BitLen()))
			continue
		}
		p, err := netip.ParsePrefix(part)
		if err != nil {
			return nil, err
		}
		out = append(out, p.Masked())
	}
	return out, nil
}

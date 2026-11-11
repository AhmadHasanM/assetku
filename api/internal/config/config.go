package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL  string
	AppBaseURL   string
	Addr         string
	CookieSecure bool
	Timezone     *time.Location
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
	if c.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	return c, nil
}

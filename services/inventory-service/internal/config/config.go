package config

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL           string
	PricingURL            string
	Port                  string
	FaultSlowQuerySeconds float64
}

func FromEnv() (Config, error) {
	return Load(os.Getenv)
}

// Load reads the configuration through getenv. An empty DATABASE_URL is valid:
// pgx then falls back to the standard PGHOST, PGUSER, PGPASSWORD and PGDATABASE variables.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		DatabaseURL: getenv("DATABASE_URL"),
		PricingURL:  getenv("PRICING_URL"),
		Port:        getenv("PORT"),
	}
	if cfg.PricingURL == "" {
		return Config{}, errors.New("PRICING_URL is required")
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if raw := getenv("FAULT_SLOW_QUERY_SECONDS"); raw != "" {
		seconds, err := strconv.ParseFloat(raw, 64)
		if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 {
			return Config{}, fmt.Errorf("FAULT_SLOW_QUERY_SECONDS must be a finite non-negative number, got %q", raw)
		}
		cfg.FaultSlowQuerySeconds = seconds
	}
	return cfg, nil
}

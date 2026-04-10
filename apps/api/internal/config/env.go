package config

import (
	"os"
	"strconv"
	"time"
)

func String(name string, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}

	return value
}

func PositiveInt(name string, fallback int) int {
	value, ok := parsePositiveInt(os.Getenv(name))
	if !ok {
		return fallback
	}

	return value
}

func DurationFromSeconds(name string, fallback time.Duration) time.Duration {
	seconds, ok := parsePositiveInt(os.Getenv(name))
	if !ok {
		return fallback
	}

	return time.Duration(seconds) * time.Second
}

func parsePositiveInt(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, false
	}

	return value, true
}

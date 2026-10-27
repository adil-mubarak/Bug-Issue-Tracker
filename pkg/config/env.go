// Package config contains small helpers for environment-based configuration.
package config

import "os"

// String returns a non-empty environment value or fallback.
func String(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

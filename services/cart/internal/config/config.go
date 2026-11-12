// Package config loads service settings from environment variables.
package config

import "os"

// Config holds everything the cart service needs to start.
type Config struct {
	Port       string // HTTP listen port, e.g. "8082"
	RedisURL   string // Redis connection URL
	CatalogURL string // base URL of the catalog service
}

// Load reads the environment, falling back to local-dev defaults that match
// the repo's docker-compose.yml.
func Load() Config {
	return Config{
		Port:       getenv("PORT", "8082"),
		RedisURL:   getenv("REDIS_URL", "redis://localhost:6379"),
		CatalogURL: getenv("CATALOG_URL", "http://localhost:8081"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

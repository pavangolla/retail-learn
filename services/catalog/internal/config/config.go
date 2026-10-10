// Package config loads service settings from environment variables.
package config

import "os"

// Config holds everything the catalog service needs to start.
type Config struct {
	Port      string // HTTP listen port, e.g. "8081"
	MongoAddr string // host:port of MongoDB, used by the /health check
}

// Load reads the environment, falling back to local-dev defaults.
func Load() Config {
	return Config{
		Port:      getenv("PORT", "8081"),
		MongoAddr: getenv("MONGO_ADDR", "localhost:27017"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

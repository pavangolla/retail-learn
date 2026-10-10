// Package config loads service settings from environment variables.
package config

import "os"

// Config holds everything the catalog service needs to start.
type Config struct {
	Port     string // HTTP listen port, e.g. "8081"
	MongoURI string // MongoDB connection string
	MongoDB  string // database name
}

// Load reads the environment, falling back to local-dev defaults that match
// the repo's docker-compose.yml.
func Load() Config {
	return Config{
		Port:     getenv("PORT", "8081"),
		MongoURI: getenv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:  getenv("MONGO_DB", "catalog"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

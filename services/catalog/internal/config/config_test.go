package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("MONGO_URI", "")
	t.Setenv("MONGO_DB", "")

	got := Load()
	want := Config{Port: "8081", MongoURI: "mongodb://localhost:27017", MongoDB: "catalog"}
	if got != want {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	t.Setenv("PORT", "9000")
	t.Setenv("MONGO_URI", "mongodb://db:27017")
	t.Setenv("MONGO_DB", "shop")

	got := Load()
	want := Config{Port: "9000", MongoURI: "mongodb://db:27017", MongoDB: "shop"}
	if got != want {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}
}

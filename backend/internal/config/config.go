package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config encapsulates all runtime configuration for the application.
type Config struct {
	Port               string
	Environment        string
	MongoURI           string
	MongoDBName        string
	CORSAllowedOrigins []string
	SyncLeadTimeMs     int
	SeedOnStartup      bool
}

// Load reads configuration from environment variables, falling back to sensible defaults
// for development mode when optional variables are not set.
func Load() (*Config, error) {
	// Attempt to load .env file if present, ignoring error if missing
	_ = godotenv.Load()

	cfg := &Config{
		Port:           getEnv("PORT", "8080"),
		Environment:    getEnv("ENVIRONMENT", "development"),
		MongoURI:       getEnv("MONGODB_URI", "mongodb://localhost:27017"),
		MongoDBName:    getEnv("MONGODB_DATABASE", "media_sequencer"),
		SyncLeadTimeMs: getEnvAsInt("SYNC_LEAD_TIME_MS", 1000),
		SeedOnStartup:  getEnvAsBool("SEED_ON_STARTUP", true),
	}

	originsRaw := getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:3000,http://127.0.0.1:5173")
	cfg.CORSAllowedOrigins = parseCommaSeparated(originsRaw)

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// Validate ensures all required configuration invariants are satisfied.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Port) == "" {
		return errors.New("PORT is required")
	}
	if strings.TrimSpace(c.MongoURI) == "" {
		return errors.New("MONGODB_URI is required")
	}
	if strings.TrimSpace(c.MongoDBName) == "" {
		return errors.New("MONGODB_DATABASE is required")
	}
	if c.SyncLeadTimeMs <= 0 {
		return errors.New("SYNC_LEAD_TIME_MS must be greater than 0")
	}
	if len(c.CORSAllowedOrigins) == 0 {
		return errors.New("at least one CORS allowed origin must be specified")
	}
	return nil
}

// IsProduction returns true if running in production mode.
func (c *Config) IsProduction() bool {
	return strings.ToLower(c.Environment) == "production"
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	valStr := getEnv(key, "")
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

func getEnvAsBool(key string, defaultVal bool) bool {
	valStr := getEnv(key, "")
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.ParseBool(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

func parseCommaSeparated(raw string) []string {
	parts := strings.Split(raw, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

package config

import (
	"os"
	"testing"
)

func TestConfigLoadDefaults(t *testing.T) {
	// Clear any relevant env vars
	os.Unsetenv("PORT")
	os.Unsetenv("MONGODB_URI")
	os.Unsetenv("MONGODB_DATABASE")
	os.Unsetenv("SYNC_LEAD_TIME_MS")
	os.Unsetenv("CORS_ALLOWED_ORIGINS")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected config to load with defaults, got error: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("expected default Port '8080', got '%s'", cfg.Port)
	}
	if cfg.MongoDBName != "media_sequencer" {
		t.Errorf("expected default DB 'media_sequencer', got '%s'", cfg.MongoDBName)
	}
	if cfg.SyncLeadTimeMs != 1000 {
		t.Errorf("expected default SyncLeadTimeMs 1000, got %d", cfg.SyncLeadTimeMs)
	}
	if len(cfg.CORSAllowedOrigins) == 0 {
		t.Errorf("expected default CORS origins to be populated")
	}
	if cfg.IsProduction() {
		t.Errorf("expected default environment to not be production")
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(c *Config)
		wantErr bool
	}{
		{
			name: "empty port",
			modify: func(c *Config) {
				c.Port = ""
			},
			wantErr: true,
		},
		{
			name: "empty mongo uri",
			modify: func(c *Config) {
				c.MongoURI = "   "
			},
			wantErr: true,
		},
		{
			name: "empty db name",
			modify: func(c *Config) {
				c.MongoDBName = ""
			},
			wantErr: true,
		},
		{
			name: "non-positive sync lead time",
			modify: func(c *Config) {
				c.SyncLeadTimeMs = 0
			},
			wantErr: true,
		},
		{
			name: "empty cors allowed origins",
			modify: func(c *Config) {
				c.CORSAllowedOrigins = []string{}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Port:               "8080",
				Environment:        "development",
				MongoURI:           "mongodb://localhost:27017",
				MongoDBName:        "media_sequencer",
				CORSAllowedOrigins: []string{"http://localhost:3000"},
				SyncLeadTimeMs:     1000,
			}
			tt.modify(cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseCommaSeparated(t *testing.T) {
	raw := " http://a.com ,  http://b.com, ,http://c.com "
	parsed := parseCommaSeparated(raw)
	if len(parsed) != 3 {
		t.Fatalf("expected 3 items, got %d", len(parsed))
	}
	if parsed[0] != "http://a.com" || parsed[1] != "http://b.com" || parsed[2] != "http://c.com" {
		t.Errorf("unexpected parsed result: %v", parsed)
	}
}

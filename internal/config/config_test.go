package config

import (
	"strings"
	"testing"
	"time"
)

func TestConfigValidateAcceptsSafeHTTPAndHTTPSOrigins(t *testing.T) {
	t.Parallel()

	for _, cfg := range []Config{
		validConfig("http://localhost:8080", false),
		validConfig("https://gotter.example.com", true),
	} {
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
	}
}

func TestConfigValidateRejectsUnsafeSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "unsupported URL scheme",
			mutate: func(cfg *Config) {
				cfg.AppBaseURL = "ftp://gotter.example.com"
			},
			wantErr: "absolute http or https URL",
		},
		{
			name: "URL path",
			mutate: func(cfg *Config) {
				cfg.AppBaseURL = "https://gotter.example.com/subpath"
			},
			wantErr: "only an origin",
		},
		{
			name: "insecure HTTPS cookie",
			mutate: func(cfg *Config) {
				cfg.CookieSecure = false
			},
			wantErr: "must be true",
		},
		{
			name: "secure cookie over HTTP",
			mutate: func(cfg *Config) {
				cfg.AppBaseURL = "http://localhost:8080"
			},
			wantErr: "must be false",
		},
		{
			name: "invalid port",
			mutate: func(cfg *Config) {
				cfg.Port = "70000"
			},
			wantErr: "1 to 65535",
		},
		{
			name: "idle timeout exceeds lifetime",
			mutate: func(cfg *Config) {
				cfg.SessionIdleTimeout = 25 * time.Hour
			},
			wantErr: "must not exceed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := validConfig("https://gotter.example.com", true)
			tt.mutate(&cfg)
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func validConfig(baseURL string, secureCookie bool) Config {
	return Config{
		AppName:            "Gotter",
		AppBaseURL:         baseURL,
		Port:               "8080",
		DatabasePath:       "./data/gotter.db",
		ESAClientID:        "client-id",
		ESAClientSecret:    "client-secret",
		ESAAllowedTeam:     "s-union",
		CookieSecure:       secureCookie,
		SessionLifetime:    24 * time.Hour,
		SessionIdleTimeout: 8 * time.Hour,
	}
}

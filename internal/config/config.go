package config

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppName            string
	AppBaseURL         string
	Port               string
	DatabasePath       string
	ESAClientID        string
	ESAClientSecret    string
	ESAAllowedTeam     string
	CookieSecure       bool
	SessionLifetime    time.Duration
	SessionIdleTimeout time.Duration
}

func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppName:         getenv("APP_NAME", "Gotter"),
		AppBaseURL:      strings.TrimRight(getenv("APP_BASE_URL", "http://localhost:8080"), "/"),
		Port:            getenv("PORT", "8080"),
		DatabasePath:    getenv("DATABASE_PATH", "./data/gotter.db"),
		ESAClientID:     os.Getenv("ESA_CLIENT_ID"),
		ESAClientSecret: os.Getenv("ESA_CLIENT_SECRET"),
		ESAAllowedTeam:  getenv("ESA_ALLOWED_TEAM", "s-union"),
	}

	secure, err := parseBool(getenv("COOKIE_SECURE", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("COOKIE_SECURE: %w", err)
	}
	cfg.CookieSecure = secure

	sessionLifetime, err := time.ParseDuration(getenv("SESSION_LIFETIME", "24h"))
	if err != nil {
		return Config{}, fmt.Errorf("SESSION_LIFETIME: %w", err)
	}
	cfg.SessionLifetime = sessionLifetime

	sessionIdleTime, err := time.ParseDuration(getenv("SESSION_IDLE_TIMEOUT", "8h"))
	if err != nil {
		return Config{}, fmt.Errorf("SESSION_IDLE_TIMEOUT: %w", err)
	}
	cfg.SessionIdleTimeout = sessionIdleTime

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.AppName == "" {
		return errors.New("APP_NAME is required")
	}
	if c.AppBaseURL == "" {
		return errors.New("APP_BASE_URL is required")
	}
	u, err := url.Parse(c.AppBaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("APP_BASE_URL must be an absolute http or https URL: %q", c.AppBaseURL)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return fmt.Errorf("APP_BASE_URL must contain only an origin without credentials, path, query, or fragment: %q", c.AppBaseURL)
	}
	if u.Scheme == "https" && !c.CookieSecure {
		return errors.New("COOKIE_SECURE must be true when APP_BASE_URL uses https")
	}
	if u.Scheme == "http" && c.CookieSecure {
		return errors.New("COOKIE_SECURE must be false when APP_BASE_URL uses http")
	}
	port, err := strconv.Atoi(c.Port)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("PORT must be an integer from 1 to 65535: %q", c.Port)
	}
	if c.DatabasePath == "" {
		return errors.New("DATABASE_PATH is required")
	}
	if c.ESAClientID == "" {
		return errors.New("ESA_CLIENT_ID is required")
	}
	if c.ESAClientSecret == "" {
		return errors.New("ESA_CLIENT_SECRET is required")
	}
	if c.ESAAllowedTeam == "" {
		return errors.New("ESA_ALLOWED_TEAM is required")
	}
	if c.SessionLifetime <= 0 {
		return errors.New("SESSION_LIFETIME must be greater than zero")
	}
	if c.SessionIdleTimeout < 0 {
		return errors.New("SESSION_IDLE_TIMEOUT must not be negative")
	}
	if c.SessionIdleTimeout > c.SessionLifetime {
		return errors.New("SESSION_IDLE_TIMEOUT must not exceed SESSION_LIFETIME")
	}
	return nil
}

func (c Config) RedirectURL() string {
	return c.AppBaseURL + "/auth/esa/callback"
}

func getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func parseBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off", "":
		return false, nil
	default:
		return strconv.ParseBool(value)
	}
}

func loadDotEnv() error {
	file, err := os.Open(".env")
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
	return scanner.Err()
}

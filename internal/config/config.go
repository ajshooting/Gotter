package config

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppName         string
	AppBaseURL      string
	Port            string
	DatabasePath    string
	ESAClientID     string
	ESAClientSecret string
	ESAAllowedTeam  string
	CookieSecure    bool
}

func Load() (Config, error) {
	if err := loadDotEnv(".env"); err != nil {
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
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("APP_BASE_URL must be an absolute URL: %q", c.AppBaseURL)
	}
	if c.Port == "" {
		return errors.New("PORT is required")
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

func loadDotEnv(path string) error {
	file, err := os.Open(path)
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

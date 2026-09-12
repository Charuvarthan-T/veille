package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL             string
	Timezone                string
	CollectInterval         time.Duration
	NotifyInterval          time.Duration
	HTTPTimeout             time.Duration
	ShutdownTimeout         time.Duration
	NotificationMaxAttempts int
	ResendAPIKey            string
	EmailFrom               string
	EmailTo                 string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:             strings.TrimSpace(os.Getenv("DATABASE_URL")),
		Timezone:                envOr("TIMEZONE", "Asia/Kolkata"),
		CollectInterval:         durationOr("COLLECT_INTERVAL", 15*time.Minute),
		NotifyInterval:          durationOr("NOTIFY_INTERVAL", 1*time.Minute),
		HTTPTimeout:             durationOr("HTTP_TIMEOUT", 30*time.Second),
		ShutdownTimeout:         durationOr("SHUTDOWN_TIMEOUT", 20*time.Second),
		NotificationMaxAttempts: intOr("NOTIFICATION_MAX_ATTEMPTS", 5),
		ResendAPIKey:            strings.TrimSpace(os.Getenv("RESEND_API_KEY")),
		EmailFrom:               strings.TrimSpace(os.Getenv("EMAIL_FROM")),
		EmailTo:                 strings.TrimSpace(os.Getenv("EMAIL_TO")),
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	var missing []string
	required := map[string]string{
		"DATABASE_URL":   c.DatabaseURL,
		"RESEND_API_KEY": c.ResendAPIKey,
		"EMAIL_FROM":     c.EmailFrom,
		"EMAIL_TO":       c.EmailTo,
	}
	for name, value := range required {
		if value == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}
	if c.Timezone == "" {
		return fmt.Errorf("TIMEZONE must not be empty")
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return fmt.Errorf("invalid TIMEZONE %q: %w", c.Timezone, err)
	}
	if c.CollectInterval <= 0 {
		return fmt.Errorf("COLLECT_INTERVAL must be positive")
	}
	if c.NotifyInterval <= 0 {
		return fmt.Errorf("NOTIFY_INTERVAL must be positive")
	}
	if c.HTTPTimeout <= 0 {
		return fmt.Errorf("HTTP_TIMEOUT must be positive")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("SHUTDOWN_TIMEOUT must be positive")
	}
	if c.NotificationMaxAttempts <= 0 {
		return fmt.Errorf("NOTIFICATION_MAX_ATTEMPTS must be positive")
	}
	return nil
}

func envOr(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func durationOr(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return d
}

func intOr(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

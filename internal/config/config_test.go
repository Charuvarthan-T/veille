package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadFailsWhenRequiredMissing(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("RESEND_API_KEY", "")
	t.Setenv("EMAIL_FROM", "")
	t.Setenv("EMAIL_TO", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected configuration error")
	}
}

func TestLoadSucceedsWithValidEnv(t *testing.T) {
	setValidEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Timezone != "Asia/Kolkata" {
		t.Fatalf("timezone = %q, want Asia/Kolkata", cfg.Timezone)
	}
	if cfg.CollectInterval != 15*time.Minute {
		t.Fatalf("collect interval = %v", cfg.CollectInterval)
	}
}

func TestValidateRejectsInvalidTimezone(t *testing.T) {
	setValidEnv(t)
	_ = os.Setenv("TIMEZONE", "Not/AZone")
	_, err := Load()
	if err == nil {
		t.Fatal("expected invalid timezone error")
	}
}

func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://veille:veille@127.0.0.1:5433/veille?sslmode=disable")
	t.Setenv("TIMEZONE", "Asia/Kolkata")
	t.Setenv("RESEND_API_KEY", "re_test")
	t.Setenv("EMAIL_FROM", "alerts@example.com")
	t.Setenv("EMAIL_TO", "me@example.com")
}

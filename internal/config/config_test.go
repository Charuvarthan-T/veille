package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadFailsWhenRequiredMissing(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("TWILIO_ACCOUNT_SID", "")
	t.Setenv("TWILIO_AUTH_TOKEN", "")
	t.Setenv("TWILIO_WHATSAPP_FROM", "")
	t.Setenv("WHATSAPP_TO", "")
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
	if cfg.ReminderLead != 24*time.Hour {
		t.Fatalf("reminder lead = %v", cfg.ReminderLead)
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
	t.Setenv("DATABASE_URL", "postgres://veille:veille@localhost:5432/veille?sslmode=disable")
	t.Setenv("TIMEZONE", "Asia/Kolkata")
	t.Setenv("TWILIO_ACCOUNT_SID", "ACtest")
	t.Setenv("TWILIO_AUTH_TOKEN", "token")
	t.Setenv("TWILIO_WHATSAPP_FROM", "whatsapp:+14155238886")
	t.Setenv("WHATSAPP_TO", "whatsapp:+919999999999")
	t.Setenv("RESEND_API_KEY", "re_test")
	t.Setenv("EMAIL_FROM", "alerts@example.com")
	t.Setenv("EMAIL_TO", "me@example.com")
}

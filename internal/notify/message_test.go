package notify_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Charuvarthan-T/veille/internal/domain"
	"github.com/Charuvarthan-T/veille/internal/notify"
)

func TestBuildActiveMessageUsesConfiguredTimezone(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Fatal(err)
	}
	contest := domain.Contest{
		Platform:  domain.PlatformCodeforces,
		Name:      "Round 999",
		URL:       "https://codeforces.com/contest/999",
		StartTime: time.Date(2026, 9, 1, 14, 35, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 9, 1, 16, 35, 0, 0, time.UTC),
		Duration:  2 * time.Hour,
	}

	msg := notify.BuildActiveMessage(contest, loc)
	if msg.Subject == "" {
		t.Fatal("expected subject")
	}
	if !strings.Contains(msg.Subject, "LIVE") {
		t.Fatalf("subject = %q", msg.Subject)
	}
	if !strings.Contains(msg.Body, "20:05") {
		t.Fatalf("expected Kolkata local time in body, got %q", msg.Body)
	}
	if !strings.Contains(msg.Body, "Join contest:") {
		t.Fatalf("expected join URL in body, got %q", msg.Body)
	}
}

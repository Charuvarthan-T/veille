package notify

import (
	"time"

	"github.com/Charuvarthan-T/veille/internal/domain"
)

func ReminderDueAt(start time.Time, lead time.Duration) time.Time {
	return start.UTC().Add(-lead)
}

func IsWithinReminderWindow(now, start time.Time, lead, window time.Duration) bool {
	now = now.UTC()
	start = start.UTC()
	dueAt := ReminderDueAt(start, lead)
	if now.Before(dueAt) {
		return false
	}
	if !now.Before(start) {
		return false
	}
	elapsed := now.Sub(dueAt)
	return elapsed <= window
}

func ShouldSend(n domain.Notification, contest domain.Contest, now time.Time, lead, window time.Duration, maxAttempts int) bool {
	if n.Status == domain.NotificationStatusSent {
		return false
	}
	if n.AttemptCount > maxAttempts {
		return false
	}
	if contest.Status != domain.ContestStatusUpcoming {
		return false
	}
	return IsWithinReminderWindow(now, contest.StartTime, lead, window)
}

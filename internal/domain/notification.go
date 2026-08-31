package domain

import "time"

type Channel string

const (
	ChannelWhatsApp Channel = "whatsapp"
	ChannelEmail    Channel = "email"
)

type NotificationStatus string

const (
	NotificationStatusPending NotificationStatus = "pending"
	NotificationStatusSending NotificationStatus = "sending"
	NotificationStatusSent    NotificationStatus = "sent"
	NotificationStatusFailed  NotificationStatus = "failed"
)

type NotificationKind string

const (
	NotificationKindReminder24h NotificationKind = "reminder_24h"
)

type Notification struct {
	ID           int64
	ContestID    int64
	Channel      Channel
	Kind         NotificationKind
	Status       NotificationStatus
	DueAt        time.Time
	SentAt       *time.Time
	AttemptCount int
	LastError    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

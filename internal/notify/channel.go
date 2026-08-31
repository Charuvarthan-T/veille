package notify

import (
	"context"

	"github.com/Charuvarthan-T/veille/internal/domain"
)

type Message struct {
	Subject string
	Body    string
}

type ChannelSender interface {
	Channel() domain.Channel
	Send(ctx context.Context, msg Message) error
}

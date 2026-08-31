package resend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Charuvarthan-T/veille/internal/domain"
	"github.com/Charuvarthan-T/veille/internal/notify"
)

type Sender struct {
	client  *http.Client
	apiKey  string
	from    string
	to      string
	apiBase string
}

func New(client *http.Client, apiKey, from, to string) *Sender {
	return &Sender{
		client:  client,
		apiKey:  apiKey,
		from:    from,
		to:      to,
		apiBase: "https://api.resend.com",
	}
}

func NewWithBase(client *http.Client, apiKey, from, to, apiBase string) *Sender {
	s := New(client, apiKey, from, to)
	s.apiBase = strings.TrimRight(apiBase, "/")
	return s
}

func (s *Sender) Channel() domain.Channel {
	return domain.ChannelEmail
}

type payload struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
}

func (s *Sender) Send(ctx context.Context, msg notify.Message) error {
	body, err := json.Marshal(payload{
		From:    s.from,
		To:      []string{s.to},
		Subject: msg.Subject,
		Text:    msg.Body,
	})
	if err != nil {
		return fmt.Errorf("resend marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiBase+"/emails", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("resend send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("resend status %d: %s", resp.StatusCode, truncate(string(respBody), 300))
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

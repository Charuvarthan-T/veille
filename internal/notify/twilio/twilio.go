package twilio

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Charuvarthan-T/veille/internal/domain"
	"github.com/Charuvarthan-T/veille/internal/notify"
)

type Sender struct {
	client     *http.Client
	accountSID string
	authToken  string
	from       string
	to         string
	apiBase    string
}

func New(client *http.Client, accountSID, authToken, from, to string) *Sender {
	return &Sender{
		client:     client,
		accountSID: accountSID,
		authToken:  authToken,
		from:       from,
		to:         to,
		apiBase:    "https://api.twilio.com",
	}
}

func NewWithBase(client *http.Client, accountSID, authToken, from, to, apiBase string) *Sender {
	s := New(client, accountSID, authToken, from, to)
	s.apiBase = strings.TrimRight(apiBase, "/")
	return s
}

func (s *Sender) Channel() domain.Channel {
	return domain.ChannelWhatsApp
}

func (s *Sender) Send(ctx context.Context, msg notify.Message) error {
	endpoint := fmt.Sprintf("%s/2010-04-01/Accounts/%s/Messages.json", s.apiBase, s.accountSID)
	form := url.Values{}
	form.Set("From", s.from)
	form.Set("To", s.to)
	form.Set("Body", msg.Body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("twilio request: %w", err)
	}
	req.SetBasicAuth(s.accountSID, s.authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("twilio send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("twilio status %d: %s", resp.StatusCode, truncate(string(body), 300))
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

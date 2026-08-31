package codeforces

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Charuvarthan-T/veille/internal/domain"
)

const defaultURL = "https://codeforces.com/api/contest.list"

type Source struct {
	client  *http.Client
	baseURL string
}

func New(client *http.Client) *Source {
	return &Source{
		client:  client,
		baseURL: defaultURL,
	}
}

func NewWithURL(client *http.Client, baseURL string) *Source {
	return &Source{
		client:  client,
		baseURL: baseURL,
	}
}

func (s *Source) Name() string {
	return "codeforces"
}

func (s *Source) Platform() domain.Platform {
	return domain.PlatformCodeforces
}

type apiResponse struct {
	Status  string       `json:"status"`
	Comment string       `json:"comment"`
	Result  []apiContest `json:"result"`
}

type apiContest struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Phase            string `json:"phase"`
	DurationSeconds  int64  `json:"durationSeconds"`
	StartTimeSeconds int64  `json:"startTimeSeconds"`
}

func (s *Source) FetchUpcoming(ctx context.Context) ([]domain.Contest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("codeforces request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("codeforces fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("codeforces read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("codeforces status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var parsed apiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("codeforces decode: %w", err)
	}
	if parsed.Status != "OK" {
		return nil, fmt.Errorf("codeforces api status %q: %s", parsed.Status, parsed.Comment)
	}

	now := time.Now().UTC()
	out := make([]domain.Contest, 0)
	for _, item := range parsed.Result {
		if item.Phase != "BEFORE" {
			continue
		}
		if item.StartTimeSeconds <= 0 || item.DurationSeconds < 0 {
			continue
		}
		start := time.Unix(item.StartTimeSeconds, 0).UTC()
		if start.Before(now) {
			continue
		}
		duration := time.Duration(item.DurationSeconds) * time.Second
		externalID := strconv.FormatInt(item.ID, 10)
		out = append(out, domain.Contest{
			Platform:   domain.PlatformCodeforces,
			ExternalID: externalID,
			Name:       item.Name,
			URL:        "https://codeforces.com/contest/" + externalID,
			StartTime:  start,
			EndTime:    start.Add(duration),
			Duration:   duration,
			Status:     domain.ContestStatusUpcoming,
		})
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

package codechef

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const defaultContestDetailsURL = "https://www.codechef.com/api/contests/"

type contestDetailsResponse struct {
	Status               string `json:"status"`
	IsRatedContest       string `json:"isRatedContest"`
	IsParentContestRated string `json:"isParentContestRated"`
}

type ratedChecker struct {
	client  *http.Client
	baseURL string
}

func newRatedChecker(client *http.Client, listBaseURL string) *ratedChecker {
	baseURL := contestDetailsBaseURL(listBaseURL)
	return &ratedChecker{client: client, baseURL: baseURL}
}

func contestDetailsBaseURL(listBaseURL string) string {
	trimmed := strings.TrimSpace(listBaseURL)
	if trimmed == "" {
		return defaultContestDetailsURL
	}
	if strings.Contains(trimmed, "/api/list/contests/") {
		return strings.Replace(trimmed, "/api/list/contests/all", "/api/contests/", 1)
	}
	return defaultContestDetailsURL
}

func (c *ratedChecker) isRated(ctx context.Context, contestCode string) (bool, error) {
	url := c.baseURL + contestCode
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("codechef rated request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "veille-contest-radar/1.0")

	resp, err := c.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("codechef rated fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false, fmt.Errorf("codechef rated read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("codechef rated status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var parsed contestDetailsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return false, fmt.Errorf("codechef rated decode: %w", err)
	}
	if parsed.Status != "" && !strings.EqualFold(parsed.Status, "success") {
		return false, fmt.Errorf("codechef rated api status %q", parsed.Status)
	}
	return isCodeChefRated(parsed), nil
}

func isCodeChefRated(details contestDetailsResponse) bool {
	return details.IsRatedContest == "1" || details.IsParentContestRated == "1"
}

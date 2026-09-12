package codeforces

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type ratingChangesResponse struct {
	Status  string `json:"status"`
	Comment string `json:"comment"`
	Result  []any  `json:"result"`
}

func (s *Source) ratingChangesURL(id int64) string {
	endpoint := strings.TrimRight(s.baseURL, "/")
	if strings.HasSuffix(endpoint, "contest.list") {
		endpoint = strings.TrimSuffix(endpoint, "contest.list")
	} else {
		endpoint += "/"
	}
	return endpoint + "contest.ratingChanges?contestId=" + strconv.FormatInt(id, 10)
}

func (s *Source) isRatedContest(ctx context.Context, id int64, phase string) (bool, error) {
	url := s.ratingChangesURL(id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("codeforces rated request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("codeforces rated fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false, fmt.Errorf("codeforces rated read body: %w", err)
	}

	var parsed ratingChangesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return false, fmt.Errorf("codeforces rated decode: %w", err)
	}

	switch parsed.Status {
	case "OK":
		return len(parsed.Result) > 0, nil
	case "FAILED":
		if strings.Contains(parsed.Comment, "Rating changes are unavailable") {
			return phase == "BEFORE" || phase == "CODING", nil
		}
		return false, fmt.Errorf("codeforces rated status %q: %s", parsed.Status, parsed.Comment)
	default:
		return false, fmt.Errorf("codeforces rated status %q: %s", parsed.Status, parsed.Comment)
	}
}

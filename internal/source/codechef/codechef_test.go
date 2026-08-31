package codechef_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Charuvarthan-T/veille/internal/source/codechef"
)

func TestFetchUpcomingNormalizesFutureContests(t *testing.T) {
	start := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Second)
	end := start.Add(3 * time.Hour)
	payload := `{
		"status":"success",
		"future_contests":[
			{
				"contest_code":"START999",
				"contest_name":"Starters 999",
				"contest_start_date_iso":"` + start.Format(time.RFC3339) + `",
				"contest_end_date_iso":"` + end.Format(time.RFC3339) + `"
			}
		]
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer server.Close()

	src := codechef.NewWithURL(server.Client(), server.URL)
	contests, err := src.FetchUpcoming(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(contests) != 1 {
		t.Fatalf("got %d contests", len(contests))
	}
	if contests[0].ExternalID != "START999" {
		t.Fatalf("external id = %s", contests[0].ExternalID)
	}
	if contests[0].Duration != 3*time.Hour {
		t.Fatalf("duration = %s", contests[0].Duration)
	}
}

func TestFetchUpcomingHandlesMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not-json`))
	}))
	defer server.Close()

	src := codechef.NewWithURL(server.Client(), server.URL)
	_, err := src.FetchUpcoming(context.Background())
	if err == nil {
		t.Fatal("expected decode error")
	}
}

package resend_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Charuvarthan-T/veille/internal/domain"
	"github.com/Charuvarthan-T/veille/internal/notify"
	"github.com/Charuvarthan-T/veille/internal/notify/resend"
)

func TestSendPostsEmail(t *testing.T) {
	var auth string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &payload)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"email_1"}`))
	}))
	defer server.Close()

	sender := resend.NewWithBase(server.Client(), "re_test", "from@example.com", "to@example.com", server.URL)
	if sender.Channel() != domain.ChannelEmail {
		t.Fatal("unexpected channel")
	}
	err := sender.Send(context.Background(), notify.Message{Subject: "subj", Body: "body"})
	if err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer re_test" {
		t.Fatalf("auth = %q", auth)
	}
	if payload["subject"] != "subj" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestSendSurfacesProviderFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"nope"}`))
	}))
	defer server.Close()

	sender := resend.NewWithBase(server.Client(), "re_test", "from@example.com", "to@example.com", server.URL)
	err := sender.Send(context.Background(), notify.Message{Subject: "s", Body: "b"})
	if err == nil {
		t.Fatal("expected error")
	}
}

package twilio_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Charuvarthan-T/veille/internal/domain"
	"github.com/Charuvarthan-T/veille/internal/notify"
	"github.com/Charuvarthan-T/veille/internal/notify/twilio"
)

func TestSendPostsWhatsAppMessage(t *testing.T) {
	var gotBody string
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"sid":"SM123"}`))
	}))
	defer server.Close()

	sender := twilio.NewWithBase(server.Client(), "ACxxx", "token", "whatsapp:+111", "whatsapp:+222", server.URL)
	if sender.Channel() != domain.ChannelWhatsApp {
		t.Fatal("unexpected channel")
	}
	err := sender.Send(context.Background(), notify.Message{Body: "hello contest"})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth == "" {
		t.Fatal("expected basic auth")
	}
	if !strings.Contains(gotBody, "hello+contest") && !strings.Contains(gotBody, "hello contest") {
		t.Fatalf("unexpected body %q", gotBody)
	}
}

func TestSendSurfacesProviderFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"bad"}`))
	}))
	defer server.Close()

	sender := twilio.NewWithBase(server.Client(), "ACxxx", "token", "whatsapp:+111", "whatsapp:+222", server.URL)
	err := sender.Send(context.Background(), notify.Message{Body: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
}

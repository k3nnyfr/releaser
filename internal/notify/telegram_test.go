package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendTelegram(t *testing.T) {
	var got telegramPayload
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		json.NewDecoder(r.Body).Decode(&got) //nolint:errcheck
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	orig := telegramAPIBase
	telegramAPIBase = srv.URL
	defer func() { telegramAPIBase = orig }()

	if err := sendTelegram(context.Background(), "bot-token", "chat-1", Message{Version: "v1.2.3", Notes: "notes body"}); err != nil {
		t.Fatalf("sendTelegram: %v", err)
	}
	if !strings.Contains(path, "bot-token") {
		t.Errorf("path = %q, want to contain bot token", path)
	}
	if got.ChatID != "chat-1" {
		t.Errorf("ChatID = %q, want chat-1", got.ChatID)
	}
	if !strings.Contains(got.Text, "v1.2.3") || !strings.Contains(got.Text, "notes body") {
		t.Errorf("Text = %q", got.Text)
	}
}

func TestSendTelegramError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	orig := telegramAPIBase
	telegramAPIBase = srv.URL
	defer func() { telegramAPIBase = orig }()

	if err := sendTelegram(context.Background(), "bot-token", "chat-1", Message{Version: "v1.0.0"}); err == nil {
		t.Fatal("expected error for 400 response")
	}
}

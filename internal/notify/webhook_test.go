package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendWebhook(t *testing.T) {
	var got webhookPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&got) //nolint:errcheck
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := sendWebhook(context.Background(), srv.URL, Message{Version: "v1.2.3", Notes: "notes body"}); err != nil {
		t.Fatalf("sendWebhook: %v", err)
	}
	if got.Version != "v1.2.3" {
		t.Errorf("Version = %q, want v1.2.3", got.Version)
	}
	if got.Notes != "notes body" {
		t.Errorf("Notes = %q, want %q", got.Notes, "notes body")
	}
}

func TestSendWebhookError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if err := sendWebhook(context.Background(), srv.URL, Message{Version: "v1.0.0"}); err == nil {
		t.Fatal("expected error for 500 response")
	}
}

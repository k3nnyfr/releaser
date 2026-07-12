package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendGoogleChat(t *testing.T) {
	var got googleChatPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&got) //nolint:errcheck
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := sendGoogleChat(context.Background(), srv.URL, Message{Version: "v1.2.3", Notes: "- feat: y"}); err != nil {
		t.Fatalf("sendGoogleChat: %v", err)
	}
	if !strings.Contains(got.Text, "v1.2.3") || !strings.Contains(got.Text, "feat: y") {
		t.Errorf("payload text = %q", got.Text)
	}
}

func TestSendGoogleChatError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	if err := sendGoogleChat(context.Background(), srv.URL, Message{Version: "v1.0.0"}); err == nil {
		t.Fatal("expected error for 400 response")
	}
}

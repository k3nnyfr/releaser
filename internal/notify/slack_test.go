package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendSlack(t *testing.T) {
	var got slackPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q", ct)
		}
		json.NewDecoder(r.Body).Decode(&got) //nolint:errcheck
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := sendSlack(context.Background(), srv.URL, Message{Version: "v1.2.3", Notes: "- fix: x"}); err != nil {
		t.Fatalf("sendSlack: %v", err)
	}
	if !strings.Contains(got.Text, "v1.2.3") || !strings.Contains(got.Text, "fix: x") {
		t.Errorf("payload text = %q", got.Text)
	}
}

func TestSendSlackError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	if err := sendSlack(context.Background(), srv.URL, Message{Version: "v1.0.0"}); err == nil {
		t.Fatal("expected error for 403 response")
	}
}

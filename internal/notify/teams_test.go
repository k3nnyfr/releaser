package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendTeams(t *testing.T) {
	var got teamsPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&got) //nolint:errcheck
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := sendTeams(context.Background(), srv.URL, Message{Version: "v1.2.3", Notes: "- fix: z"}); err != nil {
		t.Fatalf("sendTeams: %v", err)
	}
	if got.Type != "MessageCard" {
		t.Errorf("Type = %q, want MessageCard", got.Type)
	}
	if !strings.Contains(got.Title, "v1.2.3") || !strings.Contains(got.Summary, "v1.2.3") {
		t.Errorf("Title/Summary = %q/%q, want to contain v1.2.3", got.Title, got.Summary)
	}
	if !strings.Contains(got.Text, "fix: z") {
		t.Errorf("Text = %q", got.Text)
	}
}

func TestSendTeamsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	if err := sendTeams(context.Background(), srv.URL, Message{Version: "v1.0.0"}); err == nil {
		t.Fatal("expected error for 503 response")
	}
}

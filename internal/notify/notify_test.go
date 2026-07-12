package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// alwaysFailTransport returns an error for every request.
type alwaysFailTransport struct{}

func (alwaysFailTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, &testTransportError{"connection refused"}
}

type testTransportError struct{ msg string }

func (e *testTransportError) Error() string { return e.msg }

func TestPostJSONSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := postJSON(context.Background(), srv.URL, map[string]string{"a": "b"}); err != nil {
		t.Fatalf("postJSON: %v", err)
	}
}

func TestPostJSONBadURL(t *testing.T) {
	err := postJSON(context.Background(), "http://\x00bad", map[string]string{"a": "b"})
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestPostJSONRequestFails(t *testing.T) {
	orig := httpClient
	httpClient = &http.Client{Transport: alwaysFailTransport{}}
	defer func() { httpClient = orig }()

	err := postJSON(context.Background(), "http://127.0.0.1:1", map[string]string{"a": "b"})
	if err == nil {
		t.Fatal("expected error when HTTP request fails")
	}
}

func TestPostJSONNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := postJSON(context.Background(), srv.URL, map[string]string{"a": "b"})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestSendAllNoneConfigured(t *testing.T) {
	errs := SendAll(context.Background(), Config{}, Message{Version: "v1.0.0"})
	if errs != nil {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestSendAllAllSucceed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	origBase := telegramAPIBase
	telegramAPIBase = srv.URL
	defer func() { telegramAPIBase = origBase }()

	cfg := Config{
		SlackWebhookURL:      srv.URL,
		TeamsWebhookURL:      srv.URL,
		GoogleChatWebhookURL: srv.URL,
		TelegramBotToken:     "tok",
		TelegramChatID:       "123",
		WebhookURL:           srv.URL,
	}
	errs := SendAll(context.Background(), cfg, Message{Version: "v1.0.0", Notes: "notes"})
	if errs != nil {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestSendAllAllFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	origBase := telegramAPIBase
	telegramAPIBase = srv.URL
	defer func() { telegramAPIBase = origBase }()

	cfg := Config{
		SlackWebhookURL:      srv.URL,
		TeamsWebhookURL:      srv.URL,
		GoogleChatWebhookURL: srv.URL,
		TelegramBotToken:     "tok",
		TelegramChatID:       "123",
		WebhookURL:           srv.URL,
	}
	errs := SendAll(context.Background(), cfg, Message{Version: "v1.0.0"})
	if len(errs) != 5 {
		t.Fatalf("expected 5 errors, got %d: %v", len(errs), errs)
	}
}

func TestSendAllTelegramRequiresBothFields(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	origBase := telegramAPIBase
	telegramAPIBase = srv.URL
	defer func() { telegramAPIBase = origBase }()

	// Only bot token set — chat ID missing — Telegram target must be skipped.
	errs := SendAll(context.Background(), Config{TelegramBotToken: "tok"}, Message{Version: "v1.0.0"})
	if errs != nil {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if calls != 0 {
		t.Fatalf("expected telegram to be skipped, got %d calls", calls)
	}
}

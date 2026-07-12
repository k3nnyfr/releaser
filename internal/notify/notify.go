// Package notify sends best-effort release notifications to chat and webhook targets.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Message is the content of a release notification, shared across all targets.
type Message struct {
	Version string // tag name, e.g. "v1.2.3"
	Notes   string // release notes body
}

// Config holds the destination for every supported notification target.
// Every field is opt-in — a target is skipped when its required fields are empty.
type Config struct {
	SlackWebhookURL      string
	TeamsWebhookURL      string
	GoogleChatWebhookURL string
	TelegramBotToken     string
	TelegramChatID       string
	WebhookURL           string
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

// SendAll sends msg to every target configured in cfg. Each target is attempted
// independently — a failure on one does not prevent the others from being tried.
// Returns one error per failed target, or nil if every configured target
// succeeded (or none were configured).
func SendAll(ctx context.Context, cfg Config, msg Message) []error {
	var errs []error

	if cfg.SlackWebhookURL != "" {
		if err := sendSlack(ctx, cfg.SlackWebhookURL, msg); err != nil {
			errs = append(errs, fmt.Errorf("slack: %w", err))
		}
	}
	if cfg.TeamsWebhookURL != "" {
		if err := sendTeams(ctx, cfg.TeamsWebhookURL, msg); err != nil {
			errs = append(errs, fmt.Errorf("teams: %w", err))
		}
	}
	if cfg.GoogleChatWebhookURL != "" {
		if err := sendGoogleChat(ctx, cfg.GoogleChatWebhookURL, msg); err != nil {
			errs = append(errs, fmt.Errorf("google chat: %w", err))
		}
	}
	if cfg.TelegramBotToken != "" && cfg.TelegramChatID != "" {
		if err := sendTelegram(ctx, cfg.TelegramBotToken, cfg.TelegramChatID, msg); err != nil {
			errs = append(errs, fmt.Errorf("telegram: %w", err))
		}
	}
	if cfg.WebhookURL != "" {
		if err := sendWebhook(ctx, cfg.WebhookURL, msg); err != nil {
			errs = append(errs, fmt.Errorf("webhook: %w", err))
		}
	}

	return errs
}

// postJSON POSTs payload as JSON to url and treats any non-2xx response as an error.
func postJSON(ctx context.Context, url string, payload any) error {
	body, _ := json.Marshal(payload) // payload fields are always plain strings — Marshal cannot fail here

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("returned status %d", resp.StatusCode)
	}
	return nil
}

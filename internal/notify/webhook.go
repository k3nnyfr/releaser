package notify

import "context"

// webhookPayload is a generic JSON envelope for arbitrary API webhooks
// (e.g. n8n, Zapier, a custom receiver) that don't follow a chat-app schema.
type webhookPayload struct {
	Version string `json:"version"`
	Notes   string `json:"notes"`
}

func newWebhookPayload(msg Message) webhookPayload {
	return webhookPayload(msg)
}

func sendWebhook(ctx context.Context, url string, msg Message) error {
	return postJSON(ctx, url, newWebhookPayload(msg))
}

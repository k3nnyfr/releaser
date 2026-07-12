package notify

import (
	"context"
	"fmt"
)

// googleChatPayload matches the Google Chat Incoming Webhook contract: a
// single "text" field.
type googleChatPayload struct {
	Text string `json:"text"`
}

func newGoogleChatPayload(msg Message) googleChatPayload {
	return googleChatPayload{Text: fmt.Sprintf("*Released %s*\n\n%s", msg.Version, msg.Notes)}
}

func sendGoogleChat(ctx context.Context, webhookURL string, msg Message) error {
	return postJSON(ctx, webhookURL, newGoogleChatPayload(msg))
}

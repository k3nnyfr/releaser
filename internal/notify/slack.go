package notify

import (
	"context"
	"fmt"
)

// slackPayload matches the Slack Incoming Webhook contract: a single "text"
// field, interpreted as Slack mrkdwn.
type slackPayload struct {
	Text string `json:"text"`
}

func newSlackPayload(msg Message) slackPayload {
	return slackPayload{Text: fmt.Sprintf("*Released %s*\n\n%s", msg.Version, msg.Notes)}
}

func sendSlack(ctx context.Context, webhookURL string, msg Message) error {
	return postJSON(ctx, webhookURL, newSlackPayload(msg))
}

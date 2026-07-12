package notify

import (
	"context"
	"fmt"
)

// teamsPayload is a Microsoft Teams "Incoming Webhook" O365 connector card
// (MessageCard schema) — the format Teams webhook URLs still accept.
type teamsPayload struct {
	Type    string `json:"@type"`
	Context string `json:"@context"`
	Summary string `json:"summary"`
	Title   string `json:"title"`
	Text    string `json:"text"`
}

func newTeamsPayload(msg Message) teamsPayload {
	title := fmt.Sprintf("Released %s", msg.Version)
	return teamsPayload{
		Type:    "MessageCard",
		Context: "http://schema.org/extensions",
		Summary: title,
		Title:   title,
		Text:    msg.Notes,
	}
}

func sendTeams(ctx context.Context, webhookURL string, msg Message) error {
	return postJSON(ctx, webhookURL, newTeamsPayload(msg))
}

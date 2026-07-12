package notify

import (
	"context"
	"fmt"
)

// telegramAPIBase is the Telegram Bot API host. Overridden in tests.
var telegramAPIBase = "https://api.telegram.org"

// telegramPayload matches the Telegram Bot API sendMessage contract. Notes
// are sent as plain text (no parse_mode) — release notes come from arbitrary
// commit messages and are not guaranteed to be valid Telegram Markdown/HTML,
// and a malformed entity makes the whole request fail with a 400.
type telegramPayload struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

func newTelegramPayload(chatID string, msg Message) telegramPayload {
	return telegramPayload{
		ChatID: chatID,
		Text:   fmt.Sprintf("Released %s\n\n%s", msg.Version, msg.Notes),
	}
}

func sendTelegram(ctx context.Context, botToken, chatID string, msg Message) error {
	url := fmt.Sprintf("%s/bot%s/sendMessage", telegramAPIBase, botToken)
	return postJSON(ctx, url, newTelegramPayload(chatID, msg))
}

package notify

import (
	"encoding/json"
	"strings"
	"testing"
)

// FuzzMessagePayloads verifies that building a notification payload never
// panics on arbitrary version/notes strings, no matter the target — release
// notes are generated from free-form commit messages and end up embedded in
// every payload below.
func FuzzMessagePayloads(f *testing.F) {
	f.Add("v1.2.3", "release notes")
	f.Add("", "")
	f.Add("v1.0.0", "* markdown _weird_ [chars] `code` <html> & \"quotes\"")
	f.Add("tag\nwith\nnewline", "notes\x00with\xffbinary")
	f.Add("v1.0.0", strings.Repeat("x", 10000))

	f.Fuzz(func(t *testing.T, version, notes string) {
		msg := Message{Version: version, Notes: notes}
		_, _ = json.Marshal(newSlackPayload(msg))
		_, _ = json.Marshal(newGoogleChatPayload(msg))
		_, _ = json.Marshal(newTeamsPayload(msg))
		_, _ = json.Marshal(newTelegramPayload("chat-id", msg))
		_, _ = json.Marshal(newWebhookPayload(msg))
	})
}

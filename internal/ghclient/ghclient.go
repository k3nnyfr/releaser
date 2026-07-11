package ghclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client is a minimal GitHub API client covering only the Releases endpoint.
type Client struct {
	token      string
	repo       string // "owner/repo"
	httpClient *http.Client
}

// New creates a Client. repo must be in "owner/repo" format.
func New(token, repo string) *Client {
	return &Client{
		token: token,
		repo:  repo,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type createReleaseRequest struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
}

// CreateRelease creates a GitHub release on an existing tag.
// The tag must already be pushed to the remote before calling this.
func (c *Client) CreateRelease(ctx context.Context, tagName, body string) error {
	payload, _ := json.Marshal(createReleaseRequest{
		TagName: tagName,
		Name:    tagName,
		Body:    body,
	})

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases", c.repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errBody struct {
			Message string `json:"message"`
		}
		json.NewDecoder(resp.Body).Decode(&errBody) //nolint:errcheck
		if errBody.Message != "" {
			return fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, errBody.Message)
		}
		return fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	return nil
}

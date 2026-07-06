package glclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a minimal GitLab API client covering only the Releases endpoint.
type Client struct {
	baseURL    string
	token      string
	projectID  string // numeric ID or URL-encoded namespace/project
	httpClient *http.Client
}

// New creates a Client. baseURL is the GitLab instance root (e.g. "https://gitlab.example.com").
// project can be a numeric ID ("123") or a namespace/project path ("group/myapp").
func New(baseURL, token, project string) *Client {
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		token:     token,
		projectID: encodeProjectPath(project),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// endpoint builds a full API URL for the given resource under the project.
// Uses url.URL{Path, RawPath} to preserve %2F encoding in the project path —
// url.Parse would silently decode %2F back to / in the Path field.
func (c *Client) endpoint(resource string) string {
	rawPath := fmt.Sprintf("/api/v4/projects/%s/%s", c.projectID, resource)
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return c.baseURL + rawPath // fallback, handles only simple cases
	}
	u.Path = strings.ReplaceAll(rawPath, "%2F", "/")
	u.RawPath = rawPath
	return u.String()
}

// encodeProjectPath URL-encodes a GitLab project identifier for use in an API path.
// Numeric IDs ("123") pass through unchanged.
// Path-style IDs ("group/project") have each segment encoded and slashes replaced with %2F,
// because url.PathEscape does not encode / (it is a valid path separator).
func encodeProjectPath(project string) string {
	parts := strings.Split(project, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "%2F")
}

type createReleaseRequest struct {
	Name        string `json:"name"`
	TagName     string `json:"tag_name"`
	Description string `json:"description"`
}

// CreateRelease creates a GitLab release on an existing tag.
// The tag must already be pushed to the remote before calling this.
func (c *Client) CreateRelease(ctx context.Context, tagName, description string) error {
	// json.Marshal never fails for a struct with only string fields
	body, _ := json.Marshal(createReleaseRequest{
		Name:        tagName,
		TagName:     tagName,
		Description: description,
	})

	endpoint := c.endpoint("releases")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("GitLab API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errBody struct {
			Message string `json:"message"`
		}
		json.NewDecoder(resp.Body).Decode(&errBody) //nolint:errcheck
		if errBody.Message != "" {
			return fmt.Errorf("GitLab API returned %d: %s", resp.StatusCode, errBody.Message)
		}
		return fmt.Errorf("GitLab API returned %d", resp.StatusCode)
	}

	return nil
}

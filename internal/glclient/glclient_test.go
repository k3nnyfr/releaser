package glclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateRelease(t *testing.T) {
	var received createReleaseRequest
	var gotToken string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		gotToken = r.Header.Get("PRIVATE-TOKEN")
		json.NewDecoder(r.Body).Decode(&received) //nolint:errcheck
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{}`)) //nolint:errcheck
	}))
	defer srv.Close()

	client := New(srv.URL, "my-token", "123")
	err := client.CreateRelease(context.Background(), "v1.2.4", "## v1.2.4\n\n### Bug Fixes\n- patch something")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotToken != "my-token" {
		t.Errorf("PRIVATE-TOKEN = %q, want %q", gotToken, "my-token")
	}
	if received.TagName != "v1.2.4" {
		t.Errorf("tag_name = %q, want %q", received.TagName, "v1.2.4")
	}
	if received.Name != "v1.2.4" {
		t.Errorf("name = %q, want %q", received.Name, "v1.2.4")
	}
	if received.Description == "" {
		t.Error("description should not be empty")
	}
}

func TestCreateReleaseAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"message":"Tag already has a release"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	client := New(srv.URL, "token", "42")
	err := client.CreateRelease(context.Background(), "v1.2.4", "notes")
	if err == nil {
		t.Fatal("expected error for 422 response")
	}
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}
}

func TestProjectPathEncoding(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// EscapedPath returns RawPath when set, preserving %2F encoding.
		// r.URL.Path is always decoded so we must not use it here.
		gotPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{}`)) //nolint:errcheck
	}))
	defer srv.Close()

	client := New(srv.URL, "token", "group/my-app")
	client.CreateRelease(context.Background(), "v1.0.0", "") //nolint:errcheck

	// "group/my-app" must be URL-encoded as "group%2Fmy-app"
	if gotPath != "/api/v4/projects/group%2Fmy-app/releases" {
		t.Errorf("path = %q, want /api/v4/projects/group%%2Fmy-app/releases", gotPath)
	}
}

func TestEndpointFallback(t *testing.T) {
	// "://" is an invalid URL that url.Parse will reject,
	// exercising the fallback branch in endpoint().
	c := New("://invalid", "token", "123")
	got := c.endpoint("releases")
	want := "://invalid/api/v4/projects/123/releases"
	if got != want {
		t.Errorf("endpoint fallback = %q, want %q", got, want)
	}
}

func TestCreateReleaseStatusNoMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`not json`)) //nolint:errcheck
	}))
	defer srv.Close()

	err := New(srv.URL, "token", "42").CreateRelease(context.Background(), "v1.0.0", "notes")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	// Should contain just the status code, no message field
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status 500: %v", err)
	}
}

func TestCreateReleaseInvalidURL(t *testing.T) {
	// "://" is unparseable as a URL scheme, so endpoint() returns the raw fallback
	// string and http.NewRequestWithContext fails when it tries to parse it.
	c := New("://", "token", "42")
	err := c.CreateRelease(context.Background(), "v1.0.0", "notes")
	if err == nil {
		t.Fatal("expected error for client with invalid base URL")
	}
}

func TestCreateReleaseNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // close immediately so the request fails at TCP level

	err := New(srv.URL, "token", "42").CreateRelease(context.Background(), "v1.0.0", "notes")
	if err == nil {
		t.Fatal("expected network error after server is closed")
	}
}

// FuzzEncodeProjectPath verifies that the encoder never panics and never emits
// a raw slash when the input contains one.
func FuzzEncodeProjectPath(f *testing.F) {
	f.Add("group/project")
	f.Add("123")
	f.Add("")
	f.Add("group/subgroup/project")
	f.Add("a/b/c/d")
	f.Add("/leading/slash")
	f.Add("trailing/slash/")
	f.Add(strings.Repeat("a/", 50))

	f.Fuzz(func(t *testing.T, project string) {
		result := encodeProjectPath(project)
		// A slash in the input must NOT appear as a literal slash in the result
		if strings.Contains(project, "/") && strings.Contains(result, "/") {
			t.Errorf("encodeProjectPath(%q) = %q: contains unencoded slash", project, result)
		}
	})
}

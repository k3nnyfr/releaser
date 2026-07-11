package ghclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	c := New("tok", "owner/repo")
	if c.token != "tok" || c.repo != "owner/repo" || c.httpClient == nil {
		t.Fatalf("New fields: token=%q repo=%q httpClient=%v", c.token, c.repo, c.httpClient)
	}
}

func TestCreateReleaseSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/releases") {
			t.Errorf("path = %q, want .../releases", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		var req struct {
			TagName string `json:"tag_name"`
			Name    string `json:"name"`
			Body    string `json:"body"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.TagName != "v1.0.0" {
			t.Errorf("tag_name = %q, want v1.0.0", req.TagName)
		}
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{}`)
	}))
	defer srv.Close()

	c := New("test-token", "owner/repo")
	c.httpClient = srv.Client()
	// Override the URL by pointing the client at the test server.
	// We can't easily override the URL without a custom transport, so use a
	// round-trip wrapper instead.
	c.httpClient.Transport = rewriteTransport{base: http.DefaultTransport, target: srv.URL}

	if err := c.CreateRelease(context.Background(), "v1.0.0", "release notes"); err != nil {
		t.Fatalf("CreateRelease: %v", err)
	}
}

func TestCreateReleaseAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		io.WriteString(w, `{"message":"Validation Failed"}`)
	}))
	defer srv.Close()

	c := New("tok", "owner/repo")
	c.httpClient = &http.Client{Transport: rewriteTransport{base: http.DefaultTransport, target: srv.URL}}

	err := c.CreateRelease(context.Background(), "v1.0.0", "body")
	if err == nil {
		t.Fatal("expected error for 422 response")
	}
	if !strings.Contains(err.Error(), "422") {
		t.Errorf("error should mention status code: %v", err)
	}
}

func TestCreateReleaseAPIErrorNoMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `not json`)
	}))
	defer srv.Close()

	c := New("tok", "owner/repo")
	c.httpClient = &http.Client{Transport: rewriteTransport{base: http.DefaultTransport, target: srv.URL}}

	err := c.CreateRelease(context.Background(), "v1.0.0", "body")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status code: %v", err)
	}
}

func TestCreateReleaseRequestFails(t *testing.T) {
	c := New("tok", "owner/repo")
	// Use a transport that always fails.
	c.httpClient = &http.Client{Transport: alwaysFailTransport{}}

	err := c.CreateRelease(context.Background(), "v1.0.0", "body")
	if err == nil {
		t.Fatal("expected error when HTTP request fails")
	}
}

func TestCreateReleaseBadURL(t *testing.T) {
	// A repo containing a null byte makes the URL unparseable by http.NewRequestWithContext.
	c := New("tok", "owner/repo\x00bad")

	err := c.CreateRelease(context.Background(), "v1.0.0", "body")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

// rewriteTransport redirects all requests to a test server URL.
type rewriteTransport struct {
	base   http.RoundTripper
	target string
}

func (rt rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = strings.TrimPrefix(rt.target, "http://")
	return rt.base.RoundTrip(req2)
}

// alwaysFailTransport returns an error for every request.
type alwaysFailTransport struct{}

func (alwaysFailTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, &testTransportError{"connection refused"}
}

type testTransportError struct{ msg string }

func (e *testTransportError) Error() string { return e.msg }

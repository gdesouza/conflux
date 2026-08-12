package confluence

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewUsesFiniteHTTPTimeout(t *testing.T) {
	client := New("https://example.atlassian.net/wiki/", "user", "token")
	if client.client.Timeout != defaultHTTPTimeout {
		t.Fatalf("timeout = %v, want %v", client.client.Timeout, defaultHTTPTimeout)
	}
	if strings.HasSuffix(client.baseURL, "/") {
		t.Fatalf("base URL retained trailing slash: %q", client.baseURL)
	}
}

func TestNewWithHTTPClientUsesInjectedClient(t *testing.T) {
	httpClient := &http.Client{Timeout: time.Second}
	client := NewWithHTTPClient("https://example.atlassian.net/wiki", "user", "token", nil, httpClient)
	if client.client != httpClient {
		t.Fatal("injected HTTP client was not retained")
	}
}

func TestListAttachmentsContextPaginatesAndAuthenticates(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		username, password, ok := r.BasicAuth()
		if !ok || username != "user" || password != "token" {
			t.Fatalf("unexpected authentication: %q %q %v", username, password, ok)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") == "next" {
			_, _ = io.WriteString(w, `{"results":[{"id":"2","title":"two.png"}],"_links":{}}`)
			return
		}
		_, _ = io.WriteString(w, `{"results":[{"id":"1","title":"one.png"}],"_links":{"next":"/api/v2/pages/123/attachments?cursor=next"}}`)
	}))
	defer server.Close()

	client := NewWithHTTPClient(server.URL, "user", "token", nil, server.Client())
	attachments, err := client.ListAttachmentsContext(context.Background(), "123")
	if err != nil {
		t.Fatalf("ListAttachmentsContext returned error: %v", err)
	}
	if requests != 2 || len(attachments) != 2 || attachments[1].ID != "2" {
		t.Fatalf("requests=%d attachments=%#v", requests, attachments)
	}
}

func TestListAttachmentsReturnsTypedErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer server.Close()

	client := NewWithHTTPClient(server.URL, "user", "token", nil, server.Client())
	_, err := client.ListAttachments("missing")
	if !IsNotFound(err) {
		t.Fatalf("error = %T %v, want typed not-found error", err, err)
	}
	var apiError *APIError
	if !errors.As(err, &apiError) || apiError.Body != "missing" {
		t.Fatalf("API error = %#v", apiError)
	}
}

func TestListAttachmentsRejectsMalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{not-json`)
	}))
	defer server.Close()

	client := NewWithHTTPClient(server.URL, "user", "token", nil, server.Client())
	_, err := client.ListAttachmentsContext(context.Background(), "123")
	if err == nil || !strings.Contains(err.Error(), "decode attachment list response") {
		t.Fatalf("error = %v, want malformed response error", err)
	}
}

func TestDownloadAttachment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/pages/123/attachments":
			_, _ = io.WriteString(w, `{"results":[{"id":"att-1","title":"diagram.png","_links":{"download":"/download/diagram.png"}}]}`)
		case "/download/diagram.png":
			_, _ = io.WriteString(w, "image bytes")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewWithHTTPClient(server.URL, "user", "token", nil, server.Client())
	body, err := client.DownloadAttachment(context.Background(), "123", "att-1")
	if err != nil {
		t.Fatalf("DownloadAttachment returned error: %v", err)
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read attachment: %v", err)
	}
	if string(data) != "image bytes" {
		t.Fatalf("attachment = %q", data)
	}
}

func TestDownloadAttachmentHonorsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := NewWithHTTPClient("https://example.invalid", "user", "token", nil, &http.Client{})
	_, err := client.DownloadAttachment(ctx, "123", "missing")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
}

func TestVersionConflictClassification(t *testing.T) {
	err := &APIError{StatusCode: http.StatusConflict, Method: http.MethodPut, URL: "https://example/page", Body: "conflict"}
	if !IsVersionConflict(err) || IsNotFound(err) {
		t.Fatalf("unexpected classification for %v", err)
	}
}

func TestResolveURLPreservesBasePath(t *testing.T) {
	client := New("https://example.atlassian.net/wiki", "user", "token")

	got, err := client.resolveURL("/download/attachments/123/diagram.png")
	if err != nil {
		t.Fatalf("resolveURL returned error: %v", err)
	}
	want := "https://example.atlassian.net/wiki/download/attachments/123/diagram.png"
	if got != want {
		t.Fatalf("resolveURL = %q, want %q", got, want)
	}
}

func TestResolveURLAcceptsSameOriginAbsoluteURL(t *testing.T) {
	client := New("https://example.atlassian.net/wiki", "user", "token")

	got, err := client.resolveURL("https://example.atlassian.net/wiki/diagram.png")
	if err != nil {
		t.Fatalf("resolveURL returned error: %v", err)
	}
	if got != "https://example.atlassian.net/wiki/diagram.png" {
		t.Fatalf("resolveURL = %q", got)
	}
}

func TestResolveURLRejectsAnotherOrigin(t *testing.T) {
	client := New("https://example.atlassian.net/wiki", "user", "token")

	_, err := client.resolveURL("https://attacker.example/diagram.png")
	if err == nil {
		t.Fatal("resolveURL unexpectedly accepted another origin")
	}
}

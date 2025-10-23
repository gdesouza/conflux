package confluence

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadAttachment(t *testing.T) {
	client, mockTransport := createTestClient()

	// Create a temporary file for testing
	tmpfile, err := os.CreateTemp("", "test_attachment.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	if _, err := tmpfile.WriteString("test content"); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Mock the upload response
	attachmentResponse := struct {
		Results []Attachment `json:"results"`
	}{
		Results: []Attachment{
			{
				ID:    "att1",
				Title: "test_attachment.txt",
			},
		},
	}
	mockTransport.addResponse("POST", "/wiki/rest/api/content/123/child/attachment", http.StatusOK, attachmentResponse)

	attachment, err := client.UploadAttachment("123", tmpfile.Name())
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if attachment.ID != "att1" {
		t.Errorf("Expected attachment ID 'att1', got '%s'", attachment.ID)
	}
}

func TestUploadAttachmentDuplicate(t *testing.T) {
	client, mockTransport := createTestClient()

	// Create a temporary file for testing
	tmpfile, err := os.CreateTemp("", "test_attachment.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	if _, err := tmpfile.WriteString("test content"); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	filename := filepath.Base(tmpfile.Name())

	// Mock the upload response for a duplicate file
	mockTransport.addResponse("POST", "/wiki/rest/api/content/123/child/attachment", http.StatusBadRequest, "A file with the same file name as an existing attachment already exists on this page.")

	// Mock the response for finding the existing attachment
	findAttachmentResponse := struct {
		Results []Attachment `json:"results"`
	}{
		Results: []Attachment{
			{
				ID:    "att1",
				Title: filename,
			},
		},
	}
	mockTransport.addResponse("GET", "/wiki/rest/api/content/123/child/attachment", http.StatusOK, findAttachmentResponse)

	attachment, err := client.UploadAttachment("123", tmpfile.Name())
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if attachment.ID != "att1" {
		t.Errorf("Expected attachment ID 'att1', got '%s'", attachment.ID)
	}
}

func TestListAttachments(t *testing.T) {
	client, mockTransport := createTestClient()

	// Mock the list attachments response
	attachmentsResponse := struct {
		Results []Attachment `json:"results"`
	}{
		Results: []Attachment{
			{ID: "att1", Title: "file1.txt"},
			{ID: "att2", Title: "file2.txt"},
		},
	}
	mockTransport.addResponse("GET", "/wiki/api/v2/pages/123/attachments", http.StatusOK, attachmentsResponse)

	attachments, err := client.ListAttachments("123")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(attachments) != 2 {
		t.Fatalf("Expected 2 attachments, got %d", len(attachments))
	}
}

func TestGetAttachmentDownloadURL(t *testing.T) {
	client, mockTransport := createTestClient()

	// Mock the list attachments response
	attachmentsResponse := struct {
		Results []Attachment `json:"results"`
	}{
		Results: []Attachment{
			{
				ID: "att1",
				Links: struct {
					Download string `json:"download"`
				}{Download: "/download/att1"},
			},
		},
	}
	mockTransport.addResponse("GET", "/wiki/api/v2/pages/123/attachments", http.StatusOK, attachmentsResponse)

	url, err := client.GetAttachmentDownloadURL("123", "att1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expectedURL := "https://test.atlassian.net/wiki/download/att1"
	if url != expectedURL {
		t.Errorf("Expected URL '%s', got '%s'", expectedURL, url)
	}
}

func TestDoAuthenticatedRequest(t *testing.T) {
	client, mockTransport := createTestClient()

	mockTransport.addResponse("GET", "/wiki/test-auth", http.StatusOK, "ok")

	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/test-auth", client.baseURL), nil)
	_, err := client.DoAuthenticatedRequest(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	lastReq := mockTransport.getLastRequest()
	username, password, ok := lastReq.BasicAuth()
	if !ok || username != client.username || password != client.apiToken {
		t.Error("Expected request to have correct basic auth")
	}
}

func TestGetPageAncestors(t *testing.T) {
	client, mockTransport := createTestClient()

	// Mock the get page ancestors response
	ancestorsResponse := struct {
		Ancestors []PageInfo `json:"ancestors"`
	}{
		Ancestors: []PageInfo{
			{ID: "1", Title: "Grandparent"},
			{ID: "2", Title: "Parent"},
		},
	}
	mockTransport.addResponse("GET", "/wiki/rest/api/content/3?expand=ancestors", http.StatusOK, ancestorsResponse)

	ancestors, err := client.GetPageAncestors("3")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(ancestors) != 2 {
		t.Fatalf("Expected 2 ancestors, got %d", len(ancestors))
	}
}

func TestGetChildPages(t *testing.T) {
	client, mockTransport := createTestClient()

	// Mock getting child pages
	childPages := []PageInfo{
		{ID: "456", Title: "Child 1"},
		{ID: "789", Title: "Child 2"},
	}
	mockTransport.addResponse("GET", "/wiki/rest/api/content/123/child/page?expand=children.page", http.StatusOK, struct {
		Results []PageInfo `json:"results"`
	}{
		Results: childPages,
	})
	mockTransport.addResponse("GET", "/wiki/rest/api/content/456/child/page?expand=children.page", http.StatusOK, struct {
		Results []PageInfo `json:"results"`
	}{
		Results: []PageInfo{},
	})
	mockTransport.addResponse("GET", "/wiki/rest/api/content/789/child/page?expand=children.page", http.StatusOK, struct {
		Results []PageInfo `json:"results"`
	}{
		Results: []PageInfo{},
	})

	pages, err := client.GetChildPages("123")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(pages) != 2 {
		t.Fatalf("Expected 2 pages, got %d", len(pages))
	}
}

func TestFindAttachmentByFilename(t *testing.T) {
	client, mockTransport := createTestClient()

	// Mock the response for finding the existing attachment
	findAttachmentResponse := struct {
		Results []Attachment `json:"results"`
	}{
		Results: []Attachment{
			{
				ID:    "att1",
				Title: "test_attachment.txt",
			},
		},
	}
	mockTransport.addResponse("GET", "/wiki/rest/api/content/123/child/attachment", http.StatusOK, findAttachmentResponse)

	attachment, err := client.findAttachmentByFilename("123", "test_attachment.txt")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if attachment.ID != "att1" {
		t.Errorf("Expected attachment ID 'att1', got '%s'", attachment.ID)
	}
}

func TestFindAttachmentByFilenameNotFound(t *testing.T) {
	client, mockTransport := createTestClient()

	// Mock the response for finding the existing attachment
	findAttachmentResponse := struct {
		Results []Attachment `json:"results"`
	}{
		Results: []Attachment{},
	}
	mockTransport.addResponse("GET", "/wiki/rest/api/content/123/child/attachment", http.StatusOK, findAttachmentResponse)

	_, err := client.findAttachmentByFilename("123", "not_found.txt")
	if err == nil {
		t.Fatal("Expected an error, got nil")
	}

	if !strings.Contains(err.Error(), "attachment with filename 'not_found.txt' not found") {
		t.Errorf("Expected error message about attachment not found, got '%s'", err.Error())
	}
}

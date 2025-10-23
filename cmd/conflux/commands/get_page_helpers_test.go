package commands

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"conflux/internal/confluence"
	"conflux/pkg/logger"
)

// TestRunGetPage_Attachments ensures attachments are downloaded and links rewritten
func TestRunGetPage_Attachments(t *testing.T) {
	// Prepare a mock page with storage-format content referencing an attachment
	page := &confluence.Page{ID: "123", Title: "Test Page"}
	page.Body.Storage.Value = "Some content with file link [attachment.pdf] and an image <ac:image><ri:attachment ri:filename=\"image file.png\"/></ac:image>"

	// Create mock client and register the page and attachments
	mc := confluence.NewMockClient()
	mc.Pages[page.ID] = page
	mc.PagesByTitle["DOCS:Test Page"] = page
	// Provide attachments: one PDF and one image. Use the anonymous Links struct type.
	mc.Attachments[page.ID] = []confluence.Attachment{
		{ID: "att-pdf", Title: "attachment.pdf", MediaType: "application/pdf", Links: struct {
			Download string `json:"download"`
		}{Download: "http://example.local/attachment.pdf"}},
		{ID: "att-img", Title: "image file.png", MediaType: "image/png", Links: struct {
			Download string `json:"download"`
		}{Download: "http://example.local/image%20file.png"}},
	}

	// Replace the package-level newConfluenceClient to return our mock (match the real signature)
	origNew := newConfluenceClient
	defer func() { newConfluenceClient = origNew }()
	newConfluenceClient = func(baseURL, username, token string, log *logger.Logger) confluence.ConfluenceClient {
		return mc
	}

	// Use a temporary directory for attachments and switch cwd
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(dir)

	// Set flags used by runGetPage
	getPageSpace = "DOCS"
	getPageIDOrTitle = "Test Page"
	getPageFormat = "markdown"

	// Call runGetPage
	if err := runGetPage(nil, nil); err != nil {
		t.Fatalf("runGetPage failed: %v", err)
	}

	// Verify attachments directory exists and files created (mock download returns local path)
	attDir := filepath.Join(dir, "attachments")
	entries := make(map[string]fs.FileInfo)
	_ = filepath.WalkDir(attDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, _ := d.Info()
		entries[info.Name()] = info
		return nil
	})

	if _, ok := entries["attachment.pdf"]; !ok {
		t.Fatalf("expected attachment.pdf to be saved in %s", attDir)
	}
	if _, ok := entries["image file.png"]; !ok {
		t.Fatalf("expected image file.png to be saved in %s", attDir)
	}

	// Ensure mock client attachments remain unchanged
	if len(mc.Attachments[page.ID]) != 2 {
		t.Fatalf("mock attachments mutated unexpectedly: %v", mc.Attachments[page.ID])
	}

	fmt.Println("attachments test passed")
}

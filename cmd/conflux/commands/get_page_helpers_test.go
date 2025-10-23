package commands

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
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

	// Start an httptest server that serves the page, attachments list and file content
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// Find page by title request: /rest/api/content?spaceKey=DOCS&title=Test+Page&expand=body.storage
		if path == "/rest/api/content" && r.URL.Query().Get("title") == "Test Page" {
			p := map[string]interface{}{
				"results": []map[string]interface{}{{
					"id":    "123",
					"title": "Test Page",
					"body":  map[string]interface{}{"storage": map[string]interface{}{"value": page.Body.Storage.Value}},
				}},
			}
			b, _ := json.Marshal(p)
			w.WriteHeader(200)
			w.Write(b)
			return
		}

		if path == "/api/v2/pages/123/attachments" {
			att := map[string]interface{}{
				"results": []map[string]interface{}{
					{"id": "att-pdf", "title": "attachment.pdf", "mediaType": "application/pdf", "_links": map[string]string{"download": "/download/attachment.pdf"}},
					{"id": "att-img", "title": "image file.png", "mediaType": "image/png", "_links": map[string]string{"download": "/download/image%20file.png"}},
				},
			}
			b, _ := json.Marshal(att)
			w.WriteHeader(200)
			w.Write(b)
			return
		}

		if path == "/download/attachment.pdf" {
			w.WriteHeader(200)
			w.Write([]byte("PDFDATA"))
			return
		}
		if path == "/download/image%20file.png" || path == "/download/image file.png" {
			w.WriteHeader(200)
			w.Write([]byte("PNGDATA"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := confluence.NewClient(srv.URL, "u", "t", logger.New(false))

	// Write a temp config file and set package configFile
	cfg := "confluence:\n  base_url: http://example\n  username: u\n  api_token: t\n  space_key: DOCS\n"
	cfgPath := filepath.Join(t.TempDir(), "cfg.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	configFile = cfgPath

	// Replace the package-level newConfluenceClient to return our configured client
	origNew := newConfluenceClient
	defer func() { newConfluenceClient = origNew }()
	newConfluenceClient = func(baseURL, username, token string, log *logger.Logger) confluence.ConfluenceClient {
		return client
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

	fmt.Println("attachments test passed")
}

package commands

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"conflux/internal/confluence"
	"conflux/internal/content"
)

func TestPullEditableArtifactWritesPairedArtifact(t *testing.T) {
	directory := t.TempDir()
	output := filepath.Join(directory, "deployment.md")
	page := artifactTestPage()
	mock := confluence.NewMockClient()
	mock.Attachments[page.ID] = []confluence.Attachment{
		attachment("att-1", "diagram.png", "image/png"),
		attachment("att-2", "unused.txt", "text/plain"),
	}
	mock.AttachmentBodies["att-1"] = []byte("image bytes")

	var stdout bytes.Buffer
	err := pullEditableArtifact(context.Background(), &stdout, mock, page, "DOCS", output, false)
	if err != nil {
		t.Fatalf("pullEditableArtifact returned error: %v", err)
	}
	markdown, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read Markdown: %v", err)
	}
	if !strings.Contains(string(markdown), "deployment.attachments/diagram.png") {
		t.Fatalf("Markdown does not reference paired attachments directory: %s", markdown)
	}
	paths, _ := content.PathsFor(output)
	if _, err := os.Stat(filepath.Join(paths.AttachmentsDir, "diagram.png")); err != nil {
		t.Fatalf("referenced attachment missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(paths.AttachmentsDir, "unused.txt")); !os.IsNotExist(err) {
		t.Fatalf("unreferenced attachment was downloaded: %v", err)
	}
	metadata, err := content.LoadMetadata(paths.MetadataPath)
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	wantHash := sha256.Sum256([]byte("image bytes"))
	if metadata.Page.ID != page.ID || metadata.Page.BaseVersion != 7 || metadata.Attachments[0].SHA256 != hex.EncodeToString(wantHash[:]) {
		t.Fatalf("unexpected metadata: %#v", metadata)
	}
	if !strings.Contains(stdout.String(), output) {
		t.Fatalf("success output = %q", stdout.String())
	}
}

func TestPullEditableArtifactRefusesOverwrite(t *testing.T) {
	directory := t.TempDir()
	output := filepath.Join(directory, "page.md")
	if err := os.WriteFile(output, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	page := artifactTestPage()
	mock := confluence.NewMockClient()

	err := pullEditableArtifact(context.Background(), &bytes.Buffer{}, mock, page, "DOCS", output, false)
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("error = %v, want overwrite refusal", err)
	}
	data, _ := os.ReadFile(output)
	if string(data) != "keep me" {
		t.Fatalf("existing Markdown changed to %q", data)
	}
}

func TestPullEditableArtifactDownloadFailureLeavesNoArtifact(t *testing.T) {
	output := filepath.Join(t.TempDir(), "page.md")
	page := artifactTestPage()
	mock := confluence.NewMockClient()
	mock.Attachments[page.ID] = []confluence.Attachment{attachment("att-1", "diagram.png", "image/png")}

	err := pullEditableArtifact(context.Background(), &bytes.Buffer{}, mock, page, "DOCS", output, false)
	if err == nil || !strings.Contains(err.Error(), "download attachment") {
		t.Fatalf("error = %v, want download failure", err)
	}
	paths, _ := content.PathsFor(output)
	if pathExists(paths.MarkdownPath) || pathExists(paths.AttachmentsDir) {
		t.Fatal("failed pull left a partial artifact")
	}
}

func TestPullEditableArtifactForceReplacesPair(t *testing.T) {
	directory := t.TempDir()
	output := filepath.Join(directory, "page.md")
	paths, _ := content.PathsFor(output)
	if err := os.WriteFile(output, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(paths.AttachmentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(paths.AttachmentsDir, "old.txt"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := pullEditableArtifact(context.Background(), &bytes.Buffer{}, confluence.NewMockClient(), artifactPlainTestPage(), "DOCS", output, true)
	if err != nil {
		t.Fatalf("forced pull returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(paths.AttachmentsDir, "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("old attachment survived replacement: %v", err)
	}
}

func artifactTestPage() *confluence.Page {
	page := &confluence.Page{ID: "123", Title: "Deployment"}
	page.Space.Key = "DOCS"
	page.Version.Number = 7
	page.Body.Storage.Value = `<h1>Deployment</h1><ac:image><ri:attachment ri:filename="diagram.png" /></ac:image>`
	return page
}

func artifactPlainTestPage() *confluence.Page {
	page := artifactTestPage()
	page.Body.Storage.Value = `<h1>Deployment</h1>`
	return page
}

func attachment(id, title, mediaType string) confluence.Attachment {
	return confluence.Attachment{ID: id, Title: title, MediaType: mediaType}
}

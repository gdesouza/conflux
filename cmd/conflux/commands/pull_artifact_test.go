package commands

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
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

func TestPullEditableArtifactRejectsInvalidOutput(t *testing.T) {
	err := pullEditableArtifact(context.Background(), &bytes.Buffer{}, confluence.NewMockClient(), artifactPlainTestPage(), "DOCS", "page.txt", false)
	if err == nil || !strings.Contains(err.Error(), "resolve output artifact") {
		t.Fatalf("error = %v, want invalid output error", err)
	}
}

func TestPullEditableArtifactRequiresDownloader(t *testing.T) {
	client := &clientWithoutDownloader{ConfluenceClient: confluence.NewMockClient()}
	err := pullEditableArtifact(context.Background(), &bytes.Buffer{}, client, artifactPlainTestPage(), "DOCS", filepath.Join(t.TempDir(), "page.md"), false)
	if err == nil || !strings.Contains(err.Error(), "does not support attachment downloads") {
		t.Fatalf("error = %v, want downloader error", err)
	}
}

func TestPullEditableArtifactReportsListFailure(t *testing.T) {
	client := &failingAttachmentClient{ConfluenceClient: confluence.NewMockClient(), err: errors.New("list failed")}
	err := pullEditableArtifact(context.Background(), &bytes.Buffer{}, client, artifactPlainTestPage(), "DOCS", filepath.Join(t.TempDir(), "page.md"), false)
	if err == nil || !strings.Contains(err.Error(), "list page attachments") {
		t.Fatalf("error = %v, want list error", err)
	}
}

func TestPullEditableArtifactReportsRenderFailure(t *testing.T) {
	err := pullEditableArtifact(context.Background(), &bytes.Buffer{}, confluence.NewMockClient(), artifactPlainTestPage(), "", filepath.Join(t.TempDir(), "page.md"), false)
	if err == nil || !strings.Contains(err.Error(), "render editable artifact") {
		t.Fatalf("error = %v, want render error", err)
	}
}

func TestDownloadAttachmentRejectsDuplicateDestination(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "diagram.png")
	if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	mock := confluence.NewMockClient()
	mock.AttachmentBodies["att-1"] = []byte("new")
	_, err := downloadAttachment(context.Background(), mock, "123", content.AttachmentDownload{ID: "att-1", Filename: "diagram.png"}, directory)
	if err == nil || !strings.Contains(err.Error(), "create staged attachment") {
		t.Fatalf("error = %v, want exclusive-create error", err)
	}
}

func TestDownloadAttachmentReportsReadFailure(t *testing.T) {
	downloader := errorDownloader{reader: io.NopCloser(errorReader{})}
	_, err := downloadAttachment(context.Background(), downloader, "123", content.AttachmentDownload{ID: "att-1", Filename: "diagram.png"}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "write attachment") {
		t.Fatalf("error = %v, want copy error", err)
	}
}

func TestInstallPulledArtifactRefusesExistingAttachments(t *testing.T) {
	directory := t.TempDir()
	paths, _ := content.PathsFor(filepath.Join(directory, "page.md"))
	if err := os.Mkdir(paths.AttachmentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	err := installPulledArtifact(paths, "missing.md", "missing.attachments", false)
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("error = %v, want overwrite refusal", err)
	}
}

func TestInstallPulledArtifactRollsBackWhenMarkdownInstallFails(t *testing.T) {
	directory := t.TempDir()
	paths, _ := content.PathsFor(filepath.Join(directory, "page.md"))
	if err := os.WriteFile(paths.MarkdownPath, []byte("old markdown"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(paths.AttachmentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(paths.AttachmentsDir, "old.txt"), []byte("old attachment"), 0o600); err != nil {
		t.Fatal(err)
	}
	stageAttachments := filepath.Join(directory, "stage.attachments")
	if err := os.Mkdir(stageAttachments, 0o755); err != nil {
		t.Fatal(err)
	}

	err := installPulledArtifact(paths, filepath.Join(directory, "missing.md"), stageAttachments, true)
	if err == nil || !strings.Contains(err.Error(), "install Markdown") {
		t.Fatalf("error = %v, want install error", err)
	}
	markdown, _ := os.ReadFile(paths.MarkdownPath)
	attachment, attachmentErr := os.ReadFile(filepath.Join(paths.AttachmentsDir, "old.txt"))
	if string(markdown) != "old markdown" || attachmentErr != nil || string(attachment) != "old attachment" {
		t.Fatalf("rollback did not restore prior artifact: markdown=%q attachment=%q err=%v", markdown, attachment, attachmentErr)
	}
}

func TestMockDownloadAttachmentHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := confluence.NewMockClient().DownloadAttachment(ctx, "123", "att-1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
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

type clientWithoutDownloader struct {
	confluence.ConfluenceClient
}

type failingAttachmentClient struct {
	confluence.ConfluenceClient
	err error
}

func (c *failingAttachmentClient) ListAttachments(string) ([]confluence.Attachment, error) {
	return nil, c.err
}

func (c *failingAttachmentClient) DownloadAttachment(context.Context, string, string) (io.ReadCloser, error) {
	return nil, c.err
}

type errorDownloader struct {
	reader io.ReadCloser
}

func (d errorDownloader) DownloadAttachment(context.Context, string, string) (io.ReadCloser, error) {
	return d.reader, nil
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

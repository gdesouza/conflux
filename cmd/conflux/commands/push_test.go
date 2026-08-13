package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"conflux/internal/confluence"
	"conflux/internal/content"
	"conflux/pkg/logger"
)

type pushTestClient struct {
	*confluence.MockClient
	expectedVersions []int
	lastAttachmentID string
	failGet          bool
	failUpdate       bool
	failUpload       bool
	invalidUpdate    bool
}

func (m *pushTestClient) GetPage(pageID string) (*confluence.Page, error) {
	if m.failGet {
		return nil, fmt.Errorf("configured get failure")
	}
	return m.MockClient.GetPage(pageID)
}

func (m *pushTestClient) UpdatePageAtVersion(pageID, title, body string, baseVersion int) (*confluence.Page, error) {
	m.expectedVersions = append(m.expectedVersions, baseVersion)
	if m.failUpdate {
		return nil, fmt.Errorf("configured update failure")
	}
	if m.invalidUpdate {
		return &confluence.Page{}, nil
	}
	page := m.Pages[pageID]
	if page == nil || page.Version.Number != baseVersion {
		return nil, fmt.Errorf("version conflict")
	}
	page.Title = title
	page.Body.Storage.Value = body
	page.Version.Number++
	m.UpdateCalls = append(m.UpdateCalls, title)
	return page, nil
}

func (m *pushTestClient) UploadAttachment(pageID, filePath string) (*confluence.Attachment, error) {
	if m.failUpload {
		return nil, fmt.Errorf("configured upload failure")
	}
	return m.MockClient.UploadAttachment(pageID, filePath)
}

func (m *pushTestClient) UploadAttachmentVersion(pageID, attachmentID, filePath string) (*confluence.Attachment, error) {
	if m.failUpload {
		return nil, fmt.Errorf("configured upload failure")
	}
	m.lastAttachmentID = attachmentID
	m.LastUploadedFile = filePath
	return &confluence.Attachment{ID: attachmentID, Title: filepath.Base(filePath)}, nil
}

const pushTestConfigYAML = `confluence:
  base_url: http://example
  username: u
  api_token: t
  space_key: DOCS
local:
  markdown_dir: ./docs
mermaid:
  mode: preserve
projects:
  - name: team-docs
    space_key: TEAM
    local:
      markdown_dir: ./team-docs
`

func writePushTempConfig(t *testing.T) string {
	f, err := os.CreateTemp(t.TempDir(), "cfg-*.yaml")
	if err != nil {
		t.Fatalf("temp config: %v", err)
	}
	if _, err := f.WriteString(pushTestConfigYAML); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close cfg: %v", err)
	}
	return f.Name()
}

func resetPushFlags() {
	pushFile = ""
	pushSpace = ""
	pushParent = ""
	pushProject = ""
	pushForce = false
}

func TestPushEditableArtifactAdvancesMetadataVersion(t *testing.T) {
	file, metadata := writePushArtifact(t, "Body.\n", nil)
	mock := artifactPushMock(metadata)
	configurePushTest(t, file, mock)

	if err := runPush(pushCmd, nil); err != nil {
		t.Fatalf("runPush returned error: %v", err)
	}
	updated, err := content.LoadArtifactMetadata(file)
	if err != nil {
		t.Fatalf("load updated metadata: %v", err)
	}
	if updated.Page.BaseVersion != 8 {
		t.Fatalf("base version = %d, want 8", updated.Page.BaseVersion)
	}
	if len(mock.expectedVersions) != 1 || mock.expectedVersions[0] != 7 {
		t.Fatalf("expected versions = %v, want [7]", mock.expectedVersions)
	}
}

func TestPushEditableArtifactRejectsStaleRemote(t *testing.T) {
	file, metadata := writePushArtifact(t, "Body.\n", nil)
	mock := artifactPushMock(metadata)
	mock.Pages[metadata.Page.ID].Version.Number = 9
	configurePushTest(t, file, mock)

	err := runPush(pushCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "pull again or use --force") {
		t.Fatalf("error = %v, want stale-page guidance", err)
	}
	if len(mock.UpdateCalls) != 0 {
		t.Fatal("stale artifact updated the page")
	}
	unchanged, _ := content.LoadArtifactMetadata(file)
	if unchanged.Page.BaseVersion != 7 {
		t.Fatalf("metadata changed after conflict: %d", unchanged.Page.BaseVersion)
	}
}

func TestPushEditableArtifactForceUsesRemoteVersion(t *testing.T) {
	file, metadata := writePushArtifact(t, "Body.\n", nil)
	mock := artifactPushMock(metadata)
	mock.Pages[metadata.Page.ID].Version.Number = 9
	configurePushTest(t, file, mock)
	pushForce = true

	if err := runPush(pushCmd, nil); err != nil {
		t.Fatalf("runPush returned error: %v", err)
	}
	if len(mock.expectedVersions) != 1 || mock.expectedVersions[0] != 9 {
		t.Fatalf("expected versions = %v, want [9]", mock.expectedVersions)
	}
}

func TestPushEditableArtifactSkipsUnchangedAttachment(t *testing.T) {
	body := []byte("same image")
	digest := sha256.Sum256(body)
	attachment := content.AttachmentMetadata{ID: "att-1", Filename: "diagram.png", MediaType: "image/png", SHA256: hex.EncodeToString(digest[:])}
	file, metadata := writePushArtifact(t, "![diagram](page.attachments/diagram.png)\n", []content.AttachmentMetadata{attachment})
	paths, _ := content.PathsFor(file)
	if err := os.WriteFile(filepath.Join(paths.AttachmentsDir, "diagram.png"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	mock := artifactPushMock(metadata)
	configurePushTest(t, file, mock)

	if err := runPush(pushCmd, nil); err != nil {
		t.Fatalf("runPush returned error: %v", err)
	}
	if mock.LastUploadedFile != "" {
		t.Fatalf("unchanged attachment uploaded from %q", mock.LastUploadedFile)
	}
}

func TestPushEditableArtifactVersionsChangedAttachment(t *testing.T) {
	oldDigest := sha256.Sum256([]byte("old image"))
	attachment := content.AttachmentMetadata{ID: "att-1", Filename: "diagram.png", MediaType: "image/png", SHA256: hex.EncodeToString(oldDigest[:])}
	file, metadata := writePushArtifact(t, "![diagram](page.attachments/diagram.png)\n", []content.AttachmentMetadata{attachment})
	paths, _ := content.PathsFor(file)
	if err := os.WriteFile(filepath.Join(paths.AttachmentsDir, "diagram.png"), []byte("new image"), 0o600); err != nil {
		t.Fatal(err)
	}
	mock := artifactPushMock(metadata)
	configurePushTest(t, file, mock)

	if err := runPush(pushCmd, nil); err != nil {
		t.Fatalf("runPush returned error: %v", err)
	}
	if mock.lastAttachmentID != "att-1" || strings.HasSuffix(mock.LastUploadedFile, "metadata.json") {
		t.Fatalf("attachment id=%q uploaded file=%q", mock.lastAttachmentID, mock.LastUploadedFile)
	}
	updated, _ := content.LoadArtifactMetadata(file)
	wantDigest := sha256.Sum256([]byte("new image"))
	if updated.Attachments[0].SHA256 != hex.EncodeToString(wantDigest[:]) {
		t.Fatalf("attachment hash was not updated")
	}
}

func TestPushEditableArtifactFailedUpdatePreservesMetadata(t *testing.T) {
	file, metadata := writePushArtifact(t, "Body.\n", nil)
	mock := artifactPushMock(metadata)
	mock.failUpdate = true
	configurePushTest(t, file, mock)

	if err := runPush(pushCmd, nil); err == nil {
		t.Fatal("runPush unexpectedly succeeded")
	}
	unchanged, _ := content.LoadArtifactMetadata(file)
	if unchanged.Page.BaseVersion != metadata.Page.BaseVersion {
		t.Fatalf("metadata changed after failed update: %d", unchanged.Page.BaseVersion)
	}
}

func TestPushEditableArtifactRejectsSpaceMismatch(t *testing.T) {
	file, metadata := writePushArtifact(t, "Body.\n", nil)
	mock := artifactPushMock(metadata)
	configurePushTest(t, file, mock)
	pushSpace = "TEAM"

	err := runPush(pushCmd, nil)
	if err == nil || !strings.Contains(err.Error(), `belongs to space "DOCS"`) {
		t.Fatalf("error = %v, want space mismatch", err)
	}
}

func TestPushEditableArtifactRejectsMalformedMetadata(t *testing.T) {
	file, _ := writePushArtifact(t, "Body.\n", nil)
	paths, _ := content.PathsFor(file)
	if err := os.WriteFile(paths.MetadataPath, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	resetPushFlags()
	t.Cleanup(resetPushFlags)
	pushFile = file

	err := runPush(pushCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "load editable artifact") {
		t.Fatalf("error = %v, want metadata error", err)
	}
}

func TestPushEditableArtifactRequiresSafeAdapter(t *testing.T) {
	file, _ := writePushArtifact(t, "Body.\n", nil)
	configurePushTest(t, file, confluence.NewMockClient())

	err := runPush(pushCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "does not support safe artifact pushes") {
		t.Fatalf("error = %v, want adapter error", err)
	}
}

func TestPushEditableArtifactHandlesRemoteLookupFailures(t *testing.T) {
	for _, test := range []struct {
		name    string
		prepare func(*pushTestClient, content.Metadata)
		want    string
	}{
		{name: "request", prepare: func(mock *pushTestClient, _ content.Metadata) { mock.failGet = true }, want: "get current page"},
		{name: "missing", prepare: func(mock *pushTestClient, metadata content.Metadata) { delete(mock.Pages, metadata.Page.ID) }, want: "was not found"},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, metadata := writePushArtifact(t, "Body.\n", nil)
			mock := artifactPushMock(metadata)
			test.prepare(mock, metadata)
			configurePushTest(t, file, mock)
			if err := runPush(pushCmd, nil); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestPushEditableArtifactCreatesNewAttachment(t *testing.T) {
	file, metadata := writePushArtifact(t, "[runbook](page.attachments/runbook.pdf)\n", nil)
	paths, _ := content.PathsFor(file)
	if err := os.WriteFile(filepath.Join(paths.AttachmentsDir, "runbook.pdf"), []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	mock := artifactPushMock(metadata)
	configurePushTest(t, file, mock)

	if err := runPush(pushCmd, nil); err != nil {
		t.Fatalf("runPush returned error: %v", err)
	}
	updated, _ := content.LoadArtifactMetadata(file)
	if len(updated.Attachments) != 1 || updated.Attachments[0].Filename != "runbook.pdf" || updated.Attachments[0].ID == "" {
		t.Fatalf("attachment metadata = %#v", updated.Attachments)
	}
}

func TestPushEditableArtifactPreservesMetadataOnRemoteFailures(t *testing.T) {
	for _, test := range []struct {
		name    string
		prepare func(*pushTestClient)
		want    string
	}{
		{name: "upload", prepare: func(mock *pushTestClient) { mock.failUpload = true }, want: "upload attachment"},
		{name: "invalid update", prepare: func(mock *pushTestClient) { mock.invalidUpdate = true }, want: "no valid version"},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, metadata := writePushArtifact(t, "[runbook](page.attachments/runbook.pdf)\n", nil)
			paths, _ := content.PathsFor(file)
			if err := os.WriteFile(filepath.Join(paths.AttachmentsDir, "runbook.pdf"), []byte("pdf"), 0o600); err != nil {
				t.Fatal(err)
			}
			mock := artifactPushMock(metadata)
			test.prepare(mock)
			configurePushTest(t, file, mock)
			if err := runPush(pushCmd, nil); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			unchanged, _ := content.LoadArtifactMetadata(file)
			if unchanged.Page.BaseVersion != 7 || len(unchanged.Attachments) != 0 {
				t.Fatalf("metadata changed after failure: %#v", unchanged)
			}
		})
	}
}

func TestReadLocalAttachmentsRejectsNonRegularEntry(t *testing.T) {
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readLocalAttachments(directory); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("error = %v, want non-regular error", err)
	}
}

func writePushArtifact(t *testing.T, markdown string, attachments []content.AttachmentMetadata) (string, content.Metadata) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "page.md")
	if err := os.WriteFile(file, []byte(markdown), 0o600); err != nil {
		t.Fatal(err)
	}
	metadata := content.Metadata{
		SchemaVersion:      content.SchemaVersion,
		Page:               content.PageMetadata{ID: "page-123", SpaceKey: "DOCS", Title: "Page", BaseVersion: 7},
		PreservedFragments: map[string]string{}, Attachments: attachments,
	}
	if err := content.SaveArtifactMetadata(file, metadata); err != nil {
		t.Fatal(err)
	}
	return file, metadata
}

func artifactPushMock(metadata content.Metadata) *pushTestClient {
	mock := &pushTestClient{MockClient: confluence.NewMockClient()}
	page := &confluence.Page{ID: metadata.Page.ID, Title: metadata.Page.Title}
	page.Version.Number = metadata.Page.BaseVersion
	mock.Pages[page.ID] = page
	return mock
}

func configurePushTest(t *testing.T, file string, mock confluence.ConfluenceClient) {
	t.Helper()
	resetPushFlags()
	t.Cleanup(resetPushFlags)
	configFile = writePushTempConfig(t)
	verbose = false
	pushFile = file
	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient { return mock }
}

func TestPushProjectSelectionInfersSpace(t *testing.T) {
	resetPushFlags()
	t.Cleanup(resetPushFlags)

	dir := t.TempDir()
	file := filepath.Join(dir, "profile.md")
	if err := os.WriteFile(file, []byte("# Profile Page\n\nBody."), 0600); err != nil {
		t.Fatalf("write markdown: %v", err)
	}

	configFile = writePushTempConfig(t)
	verbose = false
	pushFile = file
	pushProject = "team-docs"

	mock := confluence.NewMockClient()
	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient {
		return mock
	}

	if err := runPush(pushCmd, nil); err != nil {
		t.Fatalf("runPush returned error: %v", err)
	}
	if _, exists := mock.PagesByTitle["TEAM:Profile Page"]; !exists {
		t.Fatal("push did not use the project space")
	}
}

func TestPushCreatesNewPage(t *testing.T) {
	// Prepare a temporary markdown file
	dir := t.TempDir()
	file := filepath.Join(dir, "test.md")
	content := "# Test Title\n\nSome body text." // Title should be "Test Title"
	if err := os.WriteFile(file, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	// Temp config
	configFile = writePushTempConfig(t)
	verbose = false
	// Substitute global flags
	pushFile = file
	pushSpace = "DOCS"
	pushParent = ""

	// Use a mock client explicitly to inspect results
	mock := confluence.NewMockClient()
	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient { return mock }

	// Run command logic
	if err := runPush(pushCmd, nil); err != nil {
		t.Fatalf("runPush returned error: %v", err)
	}

	if len(mock.CreateCalls) != 1 {
		t.Fatalf("expected 1 create call, got %d", len(mock.CreateCalls))
	}
	if mock.CreateCalls[0] != "Test Title" {
		t.Fatalf("unexpected created title: %s", mock.CreateCalls[0])
	}
}

func TestPushUpdatesExistingPage(t *testing.T) {
	mock := confluence.NewMockClient()
	// Seed existing page
	_, _ = mock.CreatePage("DOCS", "Existing Page", "old content")

	// Prepare file with same title (extracted from heading)
	dir := t.TempDir()
	file := filepath.Join(dir, "page.md")
	content := "# Existing Page\n\nNew body." // Title matches existing
	if err := os.WriteFile(file, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	// Temp config
	configFile = writePushTempConfig(t)
	verbose = false

	pushFile = file
	pushSpace = "DOCS"
	pushParent = ""

	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient { return mock }

	if err := runPush(pushCmd, nil); err != nil {
		t.Fatalf("runPush returned error: %v", err)
	}

	if len(mock.UpdateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(mock.UpdateCalls))
	}
	if mock.UpdateCalls[0] != "Existing Page" {
		t.Fatalf("unexpected updated title: %s", mock.UpdateCalls[0])
	}
}

func TestPushParentResolutionNumeric(t *testing.T) {
	mock := confluence.NewMockClient()

	// temp file
	dir := t.TempDir()
	file := filepath.Join(dir, "child.md")
	if err := os.WriteFile(file, []byte("# Child\nBody"), 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	// Temp config
	configFile = writePushTempConfig(t)
	verbose = false

	pushFile = file
	pushSpace = "DOCS"
	pushParent = "12345" // numeric treated as ID

	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient { return mock }

	if err := runPush(pushCmd, nil); err != nil {
		t.Fatalf("runPush returned error: %v", err)
	}

	if len(mock.CreateCalls) != 1 {
		t.Fatalf("expected create call, got %d", len(mock.CreateCalls))
	}
}

func TestPushParentResolutionByTitle(t *testing.T) {
	mock := confluence.NewMockClient()
	// Seed parent page
	parent, _ := mock.CreatePage("DOCS", "Parent", "content")
	if parent == nil {
		t.Fatalf("failed to seed parent page")
	}

	// temp file
	dir := t.TempDir()
	file := filepath.Join(dir, "child2.md")
	if err := os.WriteFile(file, []byte("# Child2\nBody"), 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	configFile = writePushTempConfig(t)
	verbose = false

	pushFile = file
	pushSpace = "DOCS"
	pushParent = "Parent" // title should resolve to ID

	newConfluenceClient = func(baseURL, username, apiToken string, log *logger.Logger) confluence.ConfluenceClient { return mock }

	if err := runPush(pushCmd, nil); err != nil {
		t.Fatalf("runPush returned error: %v", err)
	}

	if len(mock.CreateCalls) == 0 {
		t.Fatalf("expected create call")
	}
}

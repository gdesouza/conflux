package content

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPathsFor(t *testing.T) {
	paths, err := PathsFor(filepath.Join("docs", "deployment.md"))
	if err != nil {
		t.Fatalf("PathsFor returned error: %v", err)
	}

	wantAttachments := filepath.Join("docs", "deployment.attachments")
	if paths.AttachmentsDir != wantAttachments {
		t.Fatalf("attachments dir = %q, want %q", paths.AttachmentsDir, wantAttachments)
	}
	if paths.MetadataPath != filepath.Join(wantAttachments, "metadata.json") {
		t.Fatalf("metadata path = %q", paths.MetadataPath)
	}
}

func TestPathsForRejectsInvalidMarkdownPath(t *testing.T) {
	for _, path := range []string{"", "page.txt", "docs/", ".md"} {
		t.Run(path, func(t *testing.T) {
			if _, err := PathsFor(path); err == nil {
				t.Fatalf("PathsFor(%q) unexpectedly succeeded", path)
			}
		})
	}
}

func TestMetadataRoundTrip(t *testing.T) {
	metadata := validMetadata()
	path := filepath.Join(t.TempDir(), "deployment.attachments", "metadata.json")

	if err := SaveMetadata(path, metadata); err != nil {
		t.Fatalf("SaveMetadata returned error: %v", err)
	}
	loaded, err := LoadMetadata(path)
	if err != nil {
		t.Fatalf("LoadMetadata returned error: %v", err)
	}
	if !reflect.DeepEqual(loaded, metadata) {
		t.Fatalf("loaded metadata differs\n got: %#v\nwant: %#v", loaded, metadata)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat metadata: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("metadata permissions = %o, want 600", info.Mode().Perm())
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".metadata-*.tmp"))
	if err != nil {
		t.Fatalf("glob temporary metadata: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary metadata files remain: %v", matches)
	}
}

func TestArtifactMetadataUsesMatchingMarkdownBasename(t *testing.T) {
	dir := t.TempDir()
	markdownPath := filepath.Join(dir, "deployment.md")
	metadata := validMetadata()

	if err := SaveArtifactMetadata(markdownPath, metadata); err != nil {
		t.Fatalf("SaveArtifactMetadata returned error: %v", err)
	}
	loaded, err := LoadArtifactMetadata(markdownPath)
	if err != nil {
		t.Fatalf("LoadArtifactMetadata returned error: %v", err)
	}
	if !reflect.DeepEqual(loaded, metadata) {
		t.Fatalf("loaded metadata differs\n got: %#v\nwant: %#v", loaded, metadata)
	}

	wrongPath := filepath.Join(dir, "other.md")
	if _, err := LoadArtifactMetadata(wrongPath); !errors.Is(err, ErrMetadataNotFound) {
		t.Fatalf("other markdown resolved matching metadata: %v", err)
	}
}

func TestLoadMetadataNotFound(t *testing.T) {
	_, err := LoadMetadata(filepath.Join(t.TempDir(), "metadata.json"))
	if !errors.Is(err, ErrMetadataNotFound) {
		t.Fatalf("error = %v, want ErrMetadataNotFound", err)
	}
}

func TestLoadMetadataRejectsUnreadablePath(t *testing.T) {
	_, err := LoadMetadata(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "read artifact metadata") {
		t.Fatalf("error = %v, want read failure", err)
	}
}

func TestLoadMetadataRejectsMalformedAndUnknownFields(t *testing.T) {
	tests := map[string]string{
		"malformed":     `{`,
		"unknown field": `{"schema_version":1,"unknown":true}`,
		"trailing data": `{"schema_version":1} {}`,
	}
	for name, document := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "metadata.json")
			if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
				t.Fatalf("write metadata: %v", err)
			}
			if _, err := LoadMetadata(path); err == nil {
				t.Fatal("LoadMetadata unexpectedly succeeded")
			}
		})
	}
}

func TestMetadataValidation(t *testing.T) {
	tests := map[string]func(*Metadata){
		"schema":  func(metadata *Metadata) { metadata.SchemaVersion = 2 },
		"page id": func(metadata *Metadata) { metadata.Page.ID = "" },
		"space":   func(metadata *Metadata) { metadata.Page.SpaceKey = "" },
		"title":   func(metadata *Metadata) { metadata.Page.Title = "" },
		"version": func(metadata *Metadata) { metadata.Page.BaseVersion = 0 },
		"path traversal": func(metadata *Metadata) {
			metadata.Attachments[0].Filename = "../secret.txt"
		},
		"reserved filename": func(metadata *Metadata) {
			metadata.Attachments[0].Filename = "metadata.json"
		},
		"duplicate filename": func(metadata *Metadata) {
			metadata.Attachments = append(metadata.Attachments, AttachmentMetadata{Filename: "DIAGRAM.PNG"})
		},
		"empty fragment": func(metadata *Metadata) { metadata.PreservedFragments["fragment-0001"] = "" },
		"empty fragment id": func(metadata *Metadata) {
			metadata.PreservedFragments = map[string]string{"": "<p>fragment</p>"}
		},
		"empty attachment filename": func(metadata *Metadata) { metadata.Attachments[0].Filename = "" },
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			metadata := validMetadata()
			mutate(&metadata)
			if err := metadata.Validate(); err == nil {
				t.Fatal("Validate unexpectedly succeeded")
			}
		})
	}
}

func TestArtifactMetadataRejectsInvalidMarkdownPath(t *testing.T) {
	if _, err := LoadArtifactMetadata("page.txt"); err == nil {
		t.Fatal("LoadArtifactMetadata accepted a non-Markdown path")
	}
	if err := SaveArtifactMetadata("page.txt", validMetadata()); err == nil {
		t.Fatal("SaveArtifactMetadata accepted a non-Markdown path")
	}
}

func TestSaveMetadataRejectsInvalidMetadata(t *testing.T) {
	metadata := validMetadata()
	metadata.Page.ID = ""
	if err := SaveMetadata(filepath.Join(t.TempDir(), "metadata.json"), metadata); err == nil {
		t.Fatal("SaveMetadata accepted invalid metadata")
	}
}

func TestSaveMetadataRejectsInvalidDirectoryAndReplacement(t *testing.T) {
	root := t.TempDir()
	parentFile := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(parentFile, []byte("file"), 0o600); err != nil {
		t.Fatalf("write parent file: %v", err)
	}
	if err := SaveMetadata(filepath.Join(parentFile, "metadata.json"), validMetadata()); err == nil {
		t.Fatal("SaveMetadata created metadata beneath a file")
	}

	targetDirectory := filepath.Join(root, "metadata.json")
	if err := os.Mkdir(targetDirectory, 0o755); err != nil {
		t.Fatalf("create target directory: %v", err)
	}
	if err := SaveMetadata(targetDirectory, validMetadata()); err == nil || !strings.Contains(err.Error(), "replace artifact metadata") {
		t.Fatalf("error = %v, want replacement failure", err)
	}
}

func TestValidateArtifact(t *testing.T) {
	metadata := validMetadata()
	markdown := "# Deployment\n\n<!-- conflux:preserved id=\"fragment-0001\" -->\n"

	result, err := ValidateArtifact(markdown, &metadata)
	if err != nil {
		t.Fatalf("ValidateArtifact returned error: %v", err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", result.Warnings)
	}
}

func TestValidateStandaloneMarkdownWithoutMetadata(t *testing.T) {
	if _, err := ValidateArtifact("# New page\n", nil); err != nil {
		t.Fatalf("standalone Markdown was rejected: %v", err)
	}
}

func TestValidateArtifactRejectsInvalidMetadata(t *testing.T) {
	metadata := validMetadata()
	metadata.Page.ID = ""
	if _, err := ValidateArtifact("# Deployment\n", &metadata); err == nil {
		t.Fatal("ValidateArtifact accepted invalid metadata")
	}
}

func TestValidateArtifactRejectsUnsafeMarkers(t *testing.T) {
	metadata := validMetadata()
	tests := map[string]string{
		"missing metadata": "<!-- conflux:preserved id=\"fragment-0001\" -->",
		"malformed":        "<!-- conflux:preserved id='fragment-0001' -->",
		"duplicate":        "<!-- conflux:preserved id=\"fragment-0001\" -->\n<!-- conflux:preserved id=\"fragment-0001\" -->",
		"unknown":          "<!-- conflux:preserved id=\"fragment-9999\" -->",
	}

	for name, markdown := range tests {
		t.Run(name, func(t *testing.T) {
			var artifactMetadata *Metadata
			if name != "missing metadata" {
				artifactMetadata = &metadata
			}
			if _, err := ValidateArtifact(markdown, artifactMetadata); err == nil {
				t.Fatal("ValidateArtifact unexpectedly succeeded")
			}
		})
	}
}

func TestValidateArtifactWarnsForUnreferencedFragments(t *testing.T) {
	metadata := validMetadata()
	result, err := ValidateArtifact("# Deployment\n", &metadata)
	if err != nil {
		t.Fatalf("ValidateArtifact returned error: %v", err)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "fragment-0001") {
		t.Fatalf("warnings = %v", result.Warnings)
	}
}

func TestPreservationMarker(t *testing.T) {
	marker, err := PreservationMarker("fragment-0001")
	if err != nil {
		t.Fatalf("PreservationMarker returned error: %v", err)
	}
	if marker != `<!-- conflux:preserved id="fragment-0001" -->` {
		t.Fatalf("marker = %q", marker)
	}
	if _, err := PreservationMarker("../fragment"); err == nil {
		t.Fatal("PreservationMarker accepted an invalid id")
	}
}

func validMetadata() Metadata {
	return Metadata{
		SchemaVersion: SchemaVersion,
		Page: PageMetadata{
			ID:          "12345",
			SpaceKey:    "DOCS",
			Title:       "Deployment",
			BaseVersion: 17,
		},
		PreservedFragments: map[string]string{
			"fragment-0001": `<ac:structured-macro ac:name="status" />`,
		},
		Attachments: []AttachmentMetadata{
			{ID: "67890", Filename: "diagram.png", MediaType: "image/png", SHA256: "abc123"},
		},
	}
}

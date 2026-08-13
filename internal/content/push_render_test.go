package content

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestRenderArtifactConvertsSupportedMarkdownAndRestoresFragment(t *testing.T) {
	metadata := pushMetadata()
	markdown := "# Deployment\n\nUse **care** and `conflux`.\n\n- Build\n- Push\n\n" +
		`<!-- conflux:preserved id="fragment-0001" -->` + "\n\n```go\nfmt.Println(\"hi\")\n```\n"

	artifact, err := RenderArtifact(markdown, metadata, nil)
	if err != nil {
		t.Fatalf("RenderArtifact returned error: %v", err)
	}
	for _, expected := range []string{"<h1>Deployment</h1>", "<strong>care</strong>", "<code>conflux</code>", "<ul><li>Build</li><li>Push</li></ul>", metadata.PreservedFragments["fragment-0001"], `ac:name="language">go`} {
		if !strings.Contains(artifact.Storage, expected) {
			t.Fatalf("storage does not contain %q: %s", expected, artifact.Storage)
		}
	}
	if artifact.PageID != "123" || artifact.BaseVersion != 7 || len(artifact.Uploads) != 0 {
		t.Fatalf("unexpected artifact: %#v", artifact)
	}
}

func TestRenderArtifactCreatesUploadForChangedAttachment(t *testing.T) {
	metadata := pushMetadata()
	metadata.PreservedFragments = map[string]string{}
	oldHash := sha256.Sum256([]byte("old"))
	metadata.Attachments = []AttachmentMetadata{{ID: "att-1", Filename: "diagram.png", MediaType: "image/png", SHA256: hex.EncodeToString(oldHash[:])}}
	markdown := "![diagram](page.attachments/diagram.png)\n"

	artifact, err := RenderArtifact(markdown, metadata, []LocalAttachment{{Filename: "diagram.png", MediaType: "image/png", Content: []byte("new")}})
	if err != nil {
		t.Fatalf("RenderArtifact returned error: %v", err)
	}
	if len(artifact.Uploads) != 1 || artifact.Uploads[0].Filename != "diagram.png" {
		t.Fatalf("uploads = %#v", artifact.Uploads)
	}
	if !strings.Contains(artifact.Storage, `ri:filename="diagram.png"`) {
		t.Fatalf("storage = %s", artifact.Storage)
	}
}

func TestRenderArtifactSkipsUnchangedAttachment(t *testing.T) {
	content := []byte("same")
	digest := sha256.Sum256(content)
	metadata := pushMetadata()
	metadata.PreservedFragments = map[string]string{}
	metadata.Attachments = []AttachmentMetadata{{ID: "att-1", Filename: "diagram.png", SHA256: hex.EncodeToString(digest[:])}}

	artifact, err := RenderArtifact("![diagram](page.attachments/diagram.png)\n", metadata, []LocalAttachment{{Filename: "diagram.png", Content: content}})
	if err != nil {
		t.Fatalf("RenderArtifact returned error: %v", err)
	}
	if len(artifact.Uploads) != 0 {
		t.Fatalf("unchanged attachment produced uploads: %#v", artifact.Uploads)
	}
}

func TestRenderArtifactRejectsMissingReferencedAttachment(t *testing.T) {
	metadata := pushMetadata()
	metadata.PreservedFragments = map[string]string{}
	_, err := RenderArtifact("![diagram](page.attachments/diagram.png)\n", metadata, nil)
	if err == nil || !strings.Contains(err.Error(), "is missing") {
		t.Fatalf("error = %v, want missing attachment", err)
	}
}

func TestRenderArtifactRejectsDeletedAndCorruptMarkers(t *testing.T) {
	metadata := pushMetadata()
	for name, markdown := range map[string]string{
		"deleted": "# Deployment\n",
		"corrupt": `<!-- conflux:preserved id='fragment-0001' -->`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := RenderArtifact(markdown, metadata, nil); err == nil {
				t.Fatal("RenderArtifact unexpectedly succeeded")
			}
		})
	}
}

func TestRenderArtifactRejectsInlinePreservationMarker(t *testing.T) {
	metadata := pushMetadata()
	markdown := `Before <!-- conflux:preserved id="fragment-0001" --> after`
	_, err := RenderArtifact(markdown, metadata, nil)
	if err == nil || !strings.Contains(err.Error(), "own line") {
		t.Fatalf("error = %v, want standalone marker error", err)
	}
}

func TestRenderArtifactRejectsUnsafeAndDuplicateLocalAttachments(t *testing.T) {
	metadata := pushMetadata()
	metadata.PreservedFragments = map[string]string{}
	for name, attachments := range map[string][]LocalAttachment{
		"unsafe":    {{Filename: "../secret"}},
		"metadata":  {{Filename: "metadata.json"}},
		"duplicate": {{Filename: "diagram.png"}, {Filename: "DIAGRAM.PNG"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := RenderArtifact("# Page\n", metadata, attachments); err == nil {
				t.Fatal("RenderArtifact unexpectedly succeeded")
			}
		})
	}
}

func TestRenderArtifactRejectsUnclosedCodeFence(t *testing.T) {
	metadata := pushMetadata()
	metadata.PreservedFragments = map[string]string{}
	_, err := RenderArtifact("```go\ncode\n", metadata, nil)
	if err == nil || !strings.Contains(err.Error(), "unclosed") {
		t.Fatalf("error = %v, want unclosed fence", err)
	}
}

func TestRenderArtifactConvertsOrderedListsLinksAndEmphasis(t *testing.T) {
	metadata := pushMetadata()
	metadata.PreservedFragments = map[string]string{}
	artifact, err := RenderArtifact("1. *Read* [guide](https://example.com?a=1&b=2)\n2. Apply\n", metadata, nil)
	if err != nil {
		t.Fatalf("RenderArtifact returned error: %v", err)
	}
	for _, expected := range []string{"<ol>", "<em>Read</em>", `<a href="https://example.com?a=1&amp;b=2">guide</a>`, "</ol>"} {
		if !strings.Contains(artifact.Storage, expected) {
			t.Fatalf("storage does not contain %q: %s", expected, artifact.Storage)
		}
	}
}

func TestRenderArtifactProtectsFormattingSyntaxInsideCodeAndURLs(t *testing.T) {
	metadata := pushMetadata()
	metadata.PreservedFragments = map[string]string{}
	artifact, err := RenderArtifact("Use `a*b*` and [guide](https://example.com/a_b).\n", metadata, nil)
	if err != nil {
		t.Fatalf("RenderArtifact returned error: %v", err)
	}
	for _, expected := range []string{`<code>a*b*</code>`, `<a href="https://example.com/a_b">guide</a>`} {
		if !strings.Contains(artifact.Storage, expected) {
			t.Fatalf("storage does not contain %q: %s", expected, artifact.Storage)
		}
	}
}

func TestRenderArtifactConvertsBlockquoteAndAttachmentLink(t *testing.T) {
	metadata := pushMetadata()
	metadata.PreservedFragments = map[string]string{}
	markdown := "> Important\n\n[runbook](page.attachments/runbook.pdf)\n"
	artifact, err := RenderArtifact(markdown, metadata, []LocalAttachment{{Filename: "runbook.pdf", Content: []byte("pdf")}})
	if err != nil {
		t.Fatalf("RenderArtifact returned error: %v", err)
	}
	for _, expected := range []string{"<blockquote><p>Important</p></blockquote>", `<ac:link><ri:attachment ri:filename="runbook.pdf" />`, "<ac:link-body>runbook</ac:link-body>"} {
		if !strings.Contains(artifact.Storage, expected) {
			t.Fatalf("storage does not contain %q: %s", expected, artifact.Storage)
		}
	}
}

func TestRenderArtifactDeduplicatesUploadIntents(t *testing.T) {
	metadata := pushMetadata()
	metadata.PreservedFragments = map[string]string{}
	markdown := "![first](page.attachments/diagram.png) ![second](page.attachments/diagram.png)\n"
	artifact, err := RenderArtifact(markdown, metadata, []LocalAttachment{{Filename: "diagram.png", Content: []byte("new")}})
	if err != nil {
		t.Fatalf("RenderArtifact returned error: %v", err)
	}
	if len(artifact.Uploads) != 1 {
		t.Fatalf("uploads = %#v", artifact.Uploads)
	}
}

func TestRenderArtifactEscapesCDATAEndSequence(t *testing.T) {
	metadata := pushMetadata()
	metadata.PreservedFragments = map[string]string{}
	artifact, err := RenderArtifact("```\na ]]> b\n```\n", metadata, nil)
	if err != nil {
		t.Fatalf("RenderArtifact returned error: %v", err)
	}
	if !strings.Contains(artifact.Storage, "]]]]><![CDATA[>") {
		t.Fatalf("CDATA terminator was not split: %s", artifact.Storage)
	}
}

func TestRenderStorageArtifactNoEditRoundTrip(t *testing.T) {
	macro := `<ac:structured-macro ac:name="status"><ac:parameter ac:name="title">READY</ac:parameter></ac:structured-macro>`
	page := testStoragePage(`<h2>Release</h2><p>Before <strong>deploy</strong>.</p>` + macro + `<ac:image><ri:attachment ri:filename="diagram.png" /></ac:image>`)
	page.AttachmentDirectory = "release.attachments"
	page.Attachments = []AttachmentMetadata{{ID: "att-1", Filename: "diagram.png", MediaType: "image/png"}}
	pulled, err := RenderStorage(page)
	if err != nil {
		t.Fatalf("RenderStorage returned error: %v", err)
	}
	pushed, err := RenderArtifact(pulled.Markdown, pulled.Metadata, []LocalAttachment{{Filename: "diagram.png", MediaType: "image/png", Content: []byte("image")}})
	if err != nil {
		t.Fatalf("RenderArtifact returned error: %v", err)
	}

	heading := strings.Index(pushed.Storage, "<h2>Release</h2>")
	paragraph := strings.Index(pushed.Storage, "<p>Before <strong>deploy</strong>.</p>")
	preserved := strings.Index(pushed.Storage, macro)
	image := strings.Index(pushed.Storage, `ri:filename="diagram.png"`)
	if heading < 0 || paragraph <= heading || preserved <= paragraph || image <= preserved {
		t.Fatalf("semantic content or ordering changed: %s", pushed.Storage)
	}
}

func pushMetadata() Metadata {
	return Metadata{
		SchemaVersion:      SchemaVersion,
		Page:               PageMetadata{ID: "123", SpaceKey: "DOCS", Title: "Deployment", BaseVersion: 7},
		PreservedFragments: map[string]string{"fragment-0001": `<ac:structured-macro ac:name="status" />`},
	}
}

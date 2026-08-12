package content

import (
	"strings"
	"testing"
)

func TestRenderStorageConvertsSupportedNodes(t *testing.T) {
	artifact, err := RenderStorage(testStoragePage(`<h1>Deployment</h1><p>Use <strong>care</strong> and <code>conflux</code>.</p><ul><li>Build</li><li>Push</li></ul>`))
	if err != nil {
		t.Fatalf("RenderStorage returned error: %v", err)
	}

	for _, expected := range []string{"# Deployment", "Use **care** and `conflux`.", "- Build", "- Push"} {
		if !strings.Contains(artifact.Markdown, expected) {
			t.Fatalf("Markdown does not contain %q:\n%s", expected, artifact.Markdown)
		}
	}
	if len(artifact.Metadata.PreservedFragments) != 0 {
		t.Fatalf("supported content was preserved opaquely: %v", artifact.Metadata.PreservedFragments)
	}
}

func TestRenderStoragePreservesUnknownMacroAtOriginalPosition(t *testing.T) {
	macro := `<ac:structured-macro ac:name="status" ac:schema-version="1"><ac:parameter ac:name="title">READY</ac:parameter></ac:structured-macro>`
	artifact, err := RenderStorage(testStoragePage(`<p>Before.</p>` + macro + `<p>After.</p>`))
	if err != nil {
		t.Fatalf("RenderStorage returned error: %v", err)
	}

	marker := `<!-- conflux:preserved id="fragment-0001" -->`
	before := strings.Index(artifact.Markdown, "Before.")
	preserved := strings.Index(artifact.Markdown, marker)
	after := strings.Index(artifact.Markdown, "After.")
	if before < 0 || preserved <= before || after <= preserved {
		t.Fatalf("preserved marker is out of order:\n%s", artifact.Markdown)
	}
	if artifact.Metadata.PreservedFragments["fragment-0001"] != macro {
		t.Fatalf("macro was not preserved verbatim: %q", artifact.Metadata.PreservedFragments["fragment-0001"])
	}
}

func TestRenderStoragePreservesMixedEditableNodeWhole(t *testing.T) {
	paragraph := `<p>Before <ac:structured-macro ac:name="status"><ac:parameter ac:name="title">READY</ac:parameter></ac:structured-macro> after.</p>`
	artifact, err := RenderStorage(testStoragePage(paragraph))
	if err != nil {
		t.Fatalf("RenderStorage returned error: %v", err)
	}
	if artifact.Metadata.PreservedFragments["fragment-0001"] != paragraph {
		t.Fatalf("mixed paragraph was partially converted: %#v", artifact.Metadata.PreservedFragments)
	}
}

func TestRenderStorageCreatesAttachmentDownloadIntent(t *testing.T) {
	page := testStoragePage(`<ac:image><ri:attachment ri:filename="diagram one.png" /></ac:image>`)
	page.AttachmentDirectory = "deployment.attachments"
	page.Attachments = []AttachmentMetadata{{ID: "att-1", Filename: "diagram one.png", MediaType: "image/png", SHA256: "abc"}}

	artifact, err := RenderStorage(page)
	if err != nil {
		t.Fatalf("RenderStorage returned error: %v", err)
	}
	if artifact.Markdown != "![diagram one.png](deployment.attachments/diagram%20one.png)\n" {
		t.Fatalf("unexpected image Markdown: %q", artifact.Markdown)
	}
	if len(artifact.Downloads) != 1 || artifact.Downloads[0].ID != "att-1" {
		t.Fatalf("unexpected downloads: %#v", artifact.Downloads)
	}
}

func TestRenderStorageRejectsUnsafeAttachmentDirectory(t *testing.T) {
	page := testStoragePage(`<p>content</p>`)
	page.AttachmentDirectory = "../attachments"
	if _, err := RenderStorage(page); err == nil || !strings.Contains(err.Error(), "attachment directory") {
		t.Fatalf("error = %v, want attachment directory failure", err)
	}
}

func TestRenderStorageDeduplicatesAttachmentDownloads(t *testing.T) {
	image := `<ac:image><ri:attachment ri:filename="diagram.png" /></ac:image>`
	page := testStoragePage(image + image)
	page.Attachments = []AttachmentMetadata{{ID: "att-1", Filename: "diagram.png", MediaType: "image/png"}}

	artifact, err := RenderStorage(page)
	if err != nil {
		t.Fatalf("RenderStorage returned error: %v", err)
	}
	if len(artifact.Downloads) != 1 {
		t.Fatalf("downloads were not deduplicated: %#v", artifact.Downloads)
	}
}

func TestRenderStorageDiscoversAttachmentInPreservedMacro(t *testing.T) {
	storage := `<ac:structured-macro ac:name="view-file"><ac:parameter ac:name="name"><ri:attachment ri:filename="runbook.pdf" /></ac:parameter></ac:structured-macro>`
	page := testStoragePage(storage)
	page.Attachments = []AttachmentMetadata{{ID: "att-2", Filename: "runbook.pdf", MediaType: "application/pdf"}}

	artifact, err := RenderStorage(page)
	if err != nil {
		t.Fatalf("RenderStorage returned error: %v", err)
	}
	if artifact.Metadata.PreservedFragments["fragment-0001"] != storage {
		t.Fatal("view-file macro was not preserved")
	}
	if len(artifact.Downloads) != 1 || artifact.Downloads[0].Filename != "runbook.pdf" {
		t.Fatalf("attachment was not discovered: %#v", artifact.Downloads)
	}
}

func TestRenderStorageConvertsCodeMacro(t *testing.T) {
	storage := `<ac:structured-macro ac:name="code"><ac:parameter ac:name="language">go</ac:parameter><ac:plain-text-body><![CDATA[fmt.Println("hello")]]></ac:plain-text-body></ac:structured-macro>`
	artifact, err := RenderStorage(testStoragePage(storage))
	if err != nil {
		t.Fatalf("RenderStorage returned error: %v", err)
	}
	if artifact.Markdown != "```go\nfmt.Println(\"hello\")\n```\n" {
		t.Fatalf("unexpected code Markdown: %q", artifact.Markdown)
	}
	if len(artifact.Metadata.PreservedFragments) != 0 {
		t.Fatalf("code macro was preserved instead of converted: %v", artifact.Metadata.PreservedFragments)
	}
}

func TestRenderStoragePreservesCodeMacroWithUnsupportedParameters(t *testing.T) {
	storage := `<ac:structured-macro ac:name="code"><ac:parameter ac:name="title">Example</ac:parameter><ac:plain-text-body><![CDATA[echo hello]]></ac:plain-text-body></ac:structured-macro>`
	artifact, err := RenderStorage(testStoragePage(storage))
	if err != nil {
		t.Fatalf("RenderStorage returned error: %v", err)
	}
	if artifact.Metadata.PreservedFragments["fragment-0001"] != storage {
		t.Fatalf("code macro options were lost: %#v", artifact.Metadata.PreservedFragments)
	}
}

func TestRenderStoragePreservesLayoutsAndUnknownNamespaces(t *testing.T) {
	storage := `<ac:layout><ac:layout-section ac:type="single"><ac:layout-cell><p>Inside layout</p></ac:layout-cell></ac:layout-section></ac:layout>` +
		`<custom:widget xmlns:custom="urn:custom"><custom:value>Keep me</custom:value></custom:widget>`
	artifact, err := RenderStorage(testStoragePage(storage))
	if err != nil {
		t.Fatalf("RenderStorage returned error: %v", err)
	}
	if len(artifact.Metadata.PreservedFragments) != 2 {
		t.Fatalf("preserved fragments = %#v", artifact.Metadata.PreservedFragments)
	}
	if artifact.Metadata.PreservedFragments["fragment-0001"] != storage[:strings.Index(storage, `<custom:widget`)] {
		t.Fatal("layout was not preserved verbatim")
	}
	if artifact.Metadata.PreservedFragments["fragment-0002"] != storage[strings.Index(storage, `<custom:widget`):] {
		t.Fatal("custom namespace was not preserved verbatim")
	}
}

func TestRenderStorageAcceptsNamedHTMLEntities(t *testing.T) {
	artifact, err := RenderStorage(testStoragePage(`<p>Space&nbsp;preserved</p>`))
	if err != nil {
		t.Fatalf("RenderStorage returned error: %v", err)
	}
	if !strings.Contains(artifact.Markdown, "Space") || !strings.Contains(artifact.Markdown, "preserved") {
		t.Fatalf("unexpected Markdown: %q", artifact.Markdown)
	}
}

func TestRenderStorageRejectsUnknownAttachment(t *testing.T) {
	_, err := RenderStorage(testStoragePage(`<ac:image><ri:attachment ri:filename="missing.png" /></ac:image>`))
	if err == nil || !strings.Contains(err.Error(), "unknown attachment") {
		t.Fatalf("error = %v, want unknown attachment", err)
	}
}

func TestRenderStorageRejectsMalformedStorage(t *testing.T) {
	_, err := RenderStorage(testStoragePage(`<p>unclosed`))
	if err == nil || !strings.Contains(err.Error(), "tokenize Confluence storage") {
		t.Fatalf("error = %v, want tokenization failure", err)
	}
}

func TestRenderStorageValidatesPageIdentity(t *testing.T) {
	tests := map[string]func(*StoragePage){
		"id":      func(page *StoragePage) { page.ID = "" },
		"space":   func(page *StoragePage) { page.SpaceKey = "" },
		"title":   func(page *StoragePage) { page.Title = "" },
		"version": func(page *StoragePage) { page.BaseVersion = 0 },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			page := testStoragePage(`<p>content</p>`)
			mutate(&page)
			if _, err := RenderStorage(page); err == nil {
				t.Fatal("RenderStorage unexpectedly succeeded")
			}
		})
	}
}

func testStoragePage(storage string) StoragePage {
	return StoragePage{ID: "12345", SpaceKey: "DOCS", Title: "Deployment", BaseVersion: 17, Storage: storage}
}

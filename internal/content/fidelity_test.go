package content

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"testing"
)

func TestEditableArtifactFidelityFixtures(t *testing.T) {
	attachmentBody := map[string][]byte{
		"diagram.png": []byte("image fixture"),
		"runbook.pdf": []byte("pdf fixture"),
	}
	tests := []struct {
		name                 string
		attachments          []AttachmentMetadata
		wantMarkdown         []string
		wantPreserved        int
		wantDownloads        int
		wantEditableTable    bool
		wantOpaqueTableMacro bool
	}{
		{name: "plain", wantMarkdown: []string{"# Deployment Guide", "**release**", "1. Build", "> Verify production."}},
		{name: "attachments", attachments: fixtureAttachmentMetadata(attachmentBody), wantMarkdown: []string{"page.attachments/diagram.png"}, wantPreserved: 2, wantDownloads: 2},
		{name: "code", wantMarkdown: []string{"```go", `fmt.Println("deploy")`}},
		{name: "macros", wantMarkdown: []string{"Before.", "Between.", "After."}, wantPreserved: 2},
		{name: "layout-namespaces", wantPreserved: 3},
		{name: "tables", wantMarkdown: []string{`conflux:table layout="full-width" width="1200"`, "| Service | Owner |"}, wantPreserved: 1, wantEditableTable: true, wantOpaqueTableMacro: true},
		{name: "jira", wantMarkdown: []string{`[PSS-3369](jira:PSS-3369){conflux-display=inline`}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			storage := readFidelityFixture(t, test.name+".storage.xml")
			pulled, err := RenderStorage(StoragePage{
				ID: "page-123", SpaceKey: "DOCS", Title: "Fixture", BaseVersion: 7,
				Storage: storage, AttachmentDirectory: "page.attachments", Attachments: test.attachments,
			})
			if err != nil {
				t.Fatalf("RenderStorage returned error: %v", err)
			}
			for _, expected := range test.wantMarkdown {
				if !strings.Contains(pulled.Markdown, expected) {
					t.Errorf("Markdown does not contain %q:\n%s", expected, pulled.Markdown)
				}
			}
			if len(pulled.Metadata.PreservedFragments) != test.wantPreserved {
				t.Fatalf("preserved fragments = %d, want %d: %#v", len(pulled.Metadata.PreservedFragments), test.wantPreserved, pulled.Metadata.PreservedFragments)
			}
			if len(pulled.Downloads) != test.wantDownloads {
				t.Fatalf("downloads = %d, want %d: %#v", len(pulled.Downloads), test.wantDownloads, pulled.Downloads)
			}

			localAttachments := fixtureLocalAttachments(test.attachments, attachmentBody)
			pushed, err := RenderArtifact(pulled.Markdown, pulled.Metadata, localAttachments)
			if err != nil {
				t.Fatalf("RenderArtifact returned error: %v", err)
			}
			assertNormalizedStorageEqual(t, storage, pushed.Storage)
			if len(pushed.Uploads) != 0 {
				t.Fatalf("no-edit round trip requested uploads: %#v", pushed.Uploads)
			}
			if test.wantEditableTable && !strings.Contains(pushed.Storage, `data-layout="full-width"`) {
				t.Fatal("editable table layout was not restored")
			}
			if test.wantOpaqueTableMacro && !strings.Contains(pushed.Storage, `ac:name="status"`) {
				t.Fatal("macro-bearing table was not preserved")
			}
		})
	}
}

func readFidelityFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/fidelity/" + name)
	if err != nil {
		t.Fatalf("read fidelity fixture %s: %v", name, err)
	}
	return string(data)
}

func fixtureAttachmentMetadata(bodies map[string][]byte) []AttachmentMetadata {
	filenames := []string{"diagram.png", "runbook.pdf"}
	mediaTypes := []string{"image/png", "application/pdf"}
	attachments := make([]AttachmentMetadata, 0, len(filenames))
	for index, filename := range filenames {
		digest := sha256.Sum256(bodies[filename])
		attachments = append(attachments, AttachmentMetadata{
			ID: "att-" + fmt.Sprint(index+1), Filename: filename, MediaType: mediaTypes[index], SHA256: hex.EncodeToString(digest[:]),
		})
	}
	return attachments
}

func fixtureLocalAttachments(metadata []AttachmentMetadata, bodies map[string][]byte) []LocalAttachment {
	attachments := make([]LocalAttachment, 0, len(metadata))
	for _, attachment := range metadata {
		attachments = append(attachments, LocalAttachment{
			Filename: attachment.Filename, MediaType: attachment.MediaType, Content: bodies[attachment.Filename],
		})
	}
	return attachments
}

func assertNormalizedStorageEqual(t *testing.T, want, got string) {
	t.Helper()
	wantTokens := normalizedStorageTokens(t, want)
	gotTokens := normalizedStorageTokens(t, got)
	if strings.Join(wantTokens, "\n") != strings.Join(gotTokens, "\n") {
		t.Fatalf("storage changed after no-edit round trip\nwant:\n%s\n\ngot:\n%s", strings.Join(wantTokens, "\n"), strings.Join(gotTokens, "\n"))
	}
}

func normalizedStorageTokens(t *testing.T, fragment string) []string {
	t.Helper()
	decoder := storageNodeDecoder(fragment)
	var tokens []string
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				return tokens
			}
			t.Fatalf("parse storage XML: %v\n%s", err, fragment)
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "conflux-root" {
				continue
			}
			attributes := make([]string, 0, len(value.Attr))
			for _, attribute := range value.Attr {
				attributes = append(attributes, normalizedXMLName(attribute.Name)+"="+attribute.Value)
			}
			sort.Strings(attributes)
			tokens = append(tokens, "start:"+normalizedXMLName(value.Name)+":"+strings.Join(attributes, ","))
		case xml.EndElement:
			if value.Name.Local != "conflux-root" {
				tokens = append(tokens, "end:"+normalizedXMLName(value.Name))
			}
		case xml.CharData:
			if text := strings.Join(strings.Fields(string(value)), " "); text != "" {
				tokens = append(tokens, "text:"+text)
			}
		}
	}
}

func normalizedXMLName(name xml.Name) string {
	return name.Space + ":" + name.Local
}

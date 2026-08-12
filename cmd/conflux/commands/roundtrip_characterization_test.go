package commands

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"testing"

	"conflux/internal/confluence"
	confluxmarkdown "conflux/internal/markdown"
)

// TestSupportedStorageRoundTripCharacterization locks the current no-edit
// behavior for storage constructs that both conversion directions understand.
// Future renderers should reuse this fixture while tightening the fidelity
// contract around unsupported Confluence elements.
func TestSupportedStorageRoundTripCharacterization(t *testing.T) {
	storage := readRoundTripFixture(t, "supported.storage.xml")
	page := &confluence.Page{}
	page.Body.Storage.Value = storage

	markdown, err := generatePageOutput(page, "markdown")
	if err != nil {
		t.Fatalf("convert storage to markdown: %v", err)
	}

	rendered := confluxmarkdown.ConvertToConfluenceFormat(markdown)
	assertSemanticXMLEqual(t, storage, rendered)
}

// TestUnsupportedMacroCharacterization records the fidelity gap that motivates
// preservation markers: the current pull conversion loses an unknown macro.
func TestUnsupportedMacroCharacterization(t *testing.T) {
	storage := readRoundTripFixture(t, "unsupported-macro.storage.xml")
	page := &confluence.Page{}
	page.Body.Storage.Value = storage

	markdown, err := generatePageOutput(page, "markdown")
	if err != nil {
		t.Fatalf("convert storage to markdown: %v", err)
	}

	if !strings.Contains(markdown, "READY") {
		t.Fatalf("current behavior did not retain the macro's visible title: %q", markdown)
	}
	if strings.Contains(markdown, "structured-macro") {
		t.Fatalf("current behavior unexpectedly preserved the macro structure: %q", markdown)
	}
	if !strings.Contains(markdown, "Before the macro.") || !strings.Contains(markdown, "After the macro.") {
		t.Fatalf("surrounding editable text was not retained: %q", markdown)
	}
}

func readRoundTripFixture(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile("testdata/roundtrip/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

func assertSemanticXMLEqual(t *testing.T, want, got string) {
	t.Helper()

	wantTokens := canonicalXMLTokens(t, want)
	gotTokens := canonicalXMLTokens(t, got)
	if !bytes.Equal([]byte(strings.Join(wantTokens, "\n")), []byte(strings.Join(gotTokens, "\n"))) {
		t.Fatalf("storage is not semantically equal\nwant:\n%s\n\ngot:\n%s",
			strings.Join(wantTokens, "\n"), strings.Join(gotTokens, "\n"))
	}
}

// canonicalXMLTokens deliberately ignores formatting whitespace and attribute
// order while preserving element order, names, attributes, and visible text.
func canonicalXMLTokens(t *testing.T, fragment string) []string {
	t.Helper()

	decoder := xml.NewDecoder(strings.NewReader("<conflux-root>" + fragment + "</conflux-root>"))
	var tokens []string
	var elementStarts []int
	var elementHasContent []bool
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
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
				attributes = append(attributes, fmt.Sprintf("%s=%q", qualifiedName(attribute.Name), attribute.Value))
			}
			sort.Strings(attributes)
			elementStarts = append(elementStarts, len(tokens))
			elementHasContent = append(elementHasContent, false)
			tokens = append(tokens, "start:"+qualifiedName(value.Name)+":"+strings.Join(attributes, ","))
		case xml.EndElement:
			if value.Name.Local != "conflux-root" {
				last := len(elementStarts) - 1
				start := elementStarts[last]
				hasContent := elementHasContent[last]
				elementStarts = elementStarts[:last]
				elementHasContent = elementHasContent[:last]
				if !hasContent && value.Name.Local == "p" {
					tokens = tokens[:start]
					continue
				}
				tokens = append(tokens, "end:"+qualifiedName(value.Name))
				if len(elementHasContent) > 0 {
					elementHasContent[len(elementHasContent)-1] = true
				}
			}
		case xml.CharData:
			text := strings.Join(strings.Fields(string(value)), " ")
			if text != "" {
				tokens = append(tokens, "text:"+text)
				if len(elementHasContent) > 0 {
					elementHasContent[len(elementHasContent)-1] = true
				}
			}
		}
	}
	return tokens
}

func qualifiedName(name xml.Name) string {
	if name.Space == "" {
		return name.Local
	}
	return name.Space + ":" + name.Local
}

package content

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"

	htmldoc "github.com/JohannesKaufmann/html-to-markdown/v2"
)

type StoragePage struct {
	ID          string
	SpaceKey    string
	Title       string
	BaseVersion int
	Storage     string
	// AttachmentDirectory is the artifact directory name used in Markdown
	// links, for example "deployment.attachments". It defaults to
	// "attachments" for compatibility with legacy exports.
	AttachmentDirectory string
	Attachments         []AttachmentMetadata
}

type AttachmentDownload struct {
	ID        string
	Filename  string
	MediaType string
	SHA256    string
}

type EditableArtifact struct {
	Markdown  string
	Metadata  Metadata
	Downloads []AttachmentDownload
}

type storageNode struct {
	Name xml.Name
	Raw  string
}

func RenderStorage(page StoragePage) (EditableArtifact, error) {
	if strings.TrimSpace(page.ID) == "" {
		return EditableArtifact{}, fmt.Errorf("storage page id is required")
	}
	if strings.TrimSpace(page.SpaceKey) == "" {
		return EditableArtifact{}, fmt.Errorf("storage page space key is required")
	}
	if strings.TrimSpace(page.Title) == "" {
		return EditableArtifact{}, fmt.Errorf("storage page title is required")
	}
	if page.BaseVersion < 1 {
		return EditableArtifact{}, fmt.Errorf("storage page base version must be positive")
	}
	attachmentDirectory := page.AttachmentDirectory
	if attachmentDirectory == "" {
		attachmentDirectory = "attachments"
	}
	if filepath.IsAbs(attachmentDirectory) || filepath.Base(attachmentDirectory) != attachmentDirectory ||
		attachmentDirectory == "." || attachmentDirectory == ".." {
		return EditableArtifact{}, fmt.Errorf("attachment directory must be a relative directory name: %q", attachmentDirectory)
	}

	nodes, err := tokenizeStorage(page.Storage)
	if err != nil {
		return EditableArtifact{}, fmt.Errorf("tokenize Confluence storage: %w", err)
	}

	metadata := Metadata{
		SchemaVersion: SchemaVersion,
		Page: PageMetadata{
			ID:          page.ID,
			SpaceKey:    page.SpaceKey,
			Title:       page.Title,
			BaseVersion: page.BaseVersion,
		},
		PreservedFragments: make(map[string]string),
		Attachments:        append([]AttachmentMetadata(nil), page.Attachments...),
	}
	attachmentsByFilename := make(map[string]AttachmentMetadata, len(page.Attachments))
	for _, attachment := range page.Attachments {
		attachmentsByFilename[attachment.Filename] = attachment
	}

	var markdownParts []string
	var downloads []AttachmentDownload
	fragmentNumber := 0
	for _, node := range nodes {
		if strings.TrimSpace(node.Raw) == "" {
			continue
		}
		for _, filename := range referencedAttachmentFilenames(node.Raw) {
			attachment, exists := attachmentsByFilename[filename]
			if !exists {
				return EditableArtifact{}, fmt.Errorf("confluence storage references unknown attachment %q", filename)
			}
			downloads = appendDownload(downloads, attachment)
		}

		if isAttachmentImage(node) {
			filename, ok := extractAttachmentFilename(node.Raw)
			if !ok {
				return EditableArtifact{}, fmt.Errorf("confluence image is missing an attachment filename")
			}
			markdownParts = append(markdownParts, fmt.Sprintf("![%s](%s)", filename, attachmentMarkdownPath(attachmentDirectory, filename)))
			continue
		}

		if isEditableHTMLNode(node) && !containsConfluenceNamespace(node.Raw) {
			markdown, err := htmldoc.ConvertString(node.Raw)
			if err != nil {
				return EditableArtifact{}, fmt.Errorf("convert supported storage node %s: %w", node.Name.Local, err)
			}
			if trimmed := strings.TrimSpace(markdown); trimmed != "" {
				markdownParts = append(markdownParts, trimmed)
			}
			continue
		}

		if isCodeMacro(node) {
			code, language, ok := parseCodeMacro(node.Raw)
			if ok {
				markdownParts = append(markdownParts, fencedCode(code, language))
				continue
			}
		}

		fragmentNumber++
		fragmentID := fmt.Sprintf("fragment-%04d", fragmentNumber)
		marker, err := PreservationMarker(fragmentID)
		if err != nil {
			return EditableArtifact{}, err
		}
		metadata.PreservedFragments[fragmentID] = node.Raw
		markdownParts = append(markdownParts, marker)
	}

	markdown := strings.Join(markdownParts, "\n\n")
	if markdown != "" {
		markdown += "\n"
	}
	if _, err := ValidateArtifact(markdown, &metadata); err != nil {
		return EditableArtifact{}, fmt.Errorf("validate rendered artifact: %w", err)
	}
	return EditableArtifact{Markdown: markdown, Metadata: metadata, Downloads: downloads}, nil
}

func tokenizeStorage(storage string) ([]storageNode, error) {
	const prefix = `<conflux-root xmlns:ac="urn:conflux:ac" xmlns:ri="urn:conflux:ri">`
	const suffix = `</conflux-root>`
	decoder := xml.NewDecoder(strings.NewReader(prefix + storage + suffix))
	decoder.Strict = false

	depth := 0
	start := -1
	var name xml.Name
	var nodes []storageNode
	for {
		before := decoder.InputOffset()
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		switch value := token.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 {
				start = int(before) - len(prefix)
				name = value.Name
			}
		case xml.EndElement:
			if depth == 2 && start >= 0 {
				end := int(decoder.InputOffset()) - len(prefix)
				if start < 0 || end > len(storage) || start > end {
					return nil, fmt.Errorf("invalid storage token offsets")
				}
				nodes = append(nodes, storageNode{Name: name, Raw: storage[start:end]})
				start = -1
			}
			depth--
		case xml.CharData:
			if depth == 1 && strings.TrimSpace(string(value)) != "" {
				start := int(before) - len(prefix)
				end := int(decoder.InputOffset()) - len(prefix)
				nodes = append(nodes, storageNode{Raw: storage[start:end]})
			}
		case xml.Comment:
			if depth == 1 {
				start := int(before) - len(prefix)
				end := int(decoder.InputOffset()) - len(prefix)
				nodes = append(nodes, storageNode{Raw: storage[start:end]})
			}
		}
	}
	return nodes, nil
}

func isEditableHTMLNode(node storageNode) bool {
	if node.Name.Space != "" {
		return false
	}
	switch strings.ToLower(node.Name.Local) {
	case "h1", "h2", "h3", "h4", "h5", "h6", "p", "ul", "ol", "pre", "blockquote":
		return true
	default:
		return false
	}
}

func containsConfluenceNamespace(raw string) bool {
	return strings.Contains(raw, "<ac:") || strings.Contains(raw, "<ri:")
}

func isAttachmentImage(node storageNode) bool {
	return node.Name.Space == "urn:conflux:ac" && node.Name.Local == "image"
}

func extractAttachmentFilename(raw string) (string, bool) {
	filenames := referencedAttachmentFilenames(raw)
	if len(filenames) == 0 {
		return "", false
	}
	return filenames[0], true
}

func referencedAttachmentFilenames(raw string) []string {
	decoder := storageNodeDecoder(raw)
	var filenames []string
	for {
		token, err := decoder.Token()
		if err != nil {
			return filenames
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Space != "urn:conflux:ri" || start.Name.Local != "attachment" {
			continue
		}
		for _, attribute := range start.Attr {
			if attribute.Name.Space == "urn:conflux:ri" && attribute.Name.Local == "filename" {
				filenames = append(filenames, attribute.Value)
			}
		}
	}
}

func attachmentMarkdownPath(directory, filename string) string {
	return url.PathEscape(directory) + "/" + url.PathEscape(filename)
}

func appendDownload(downloads []AttachmentDownload, attachment AttachmentMetadata) []AttachmentDownload {
	for _, download := range downloads {
		if download.ID == attachment.ID && download.Filename == attachment.Filename {
			return downloads
		}
	}
	return append(downloads, AttachmentDownload(attachment))
}

func isCodeMacro(node storageNode) bool {
	if node.Name.Space != "urn:conflux:ac" || node.Name.Local != "structured-macro" {
		return false
	}
	decoder := storageNodeDecoder(node.Raw)
	for {
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Space != "urn:conflux:ac" || start.Name.Local != "structured-macro" {
			continue
		}
		for _, attribute := range start.Attr {
			if attribute.Name.Space == "urn:conflux:ac" && attribute.Name.Local == "name" && attribute.Value == "code" {
				return true
			}
		}
		return false
	}
}

func parseCodeMacro(raw string) (string, string, bool) {
	decoder := storageNodeDecoder(raw)
	var language, code strings.Builder
	var captureLanguage, captureCode bool
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", "", false
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Space == "urn:conflux:ac" && value.Name.Local == "parameter" {
				parameterName := ""
				for _, attribute := range value.Attr {
					if attribute.Name.Space == "urn:conflux:ac" && attribute.Name.Local == "name" {
						parameterName = attribute.Value
					}
				}
				if parameterName != "language" {
					return "", "", false
				}
				captureLanguage = true
			}
			if value.Name.Space == "urn:conflux:ac" && value.Name.Local == "plain-text-body" {
				captureCode = true
			}
		case xml.EndElement:
			if value.Name.Space == "urn:conflux:ac" && value.Name.Local == "parameter" {
				captureLanguage = false
			}
			if value.Name.Space == "urn:conflux:ac" && value.Name.Local == "plain-text-body" {
				captureCode = false
			}
		case xml.CharData:
			if captureLanguage {
				language.Write(value)
			}
			if captureCode {
				code.Write(value)
			}
		}
	}
	if code.Len() == 0 {
		return "", "", false
	}
	return code.String(), strings.TrimSpace(language.String()), true
}

func storageNodeDecoder(raw string) *xml.Decoder {
	decoder := xml.NewDecoder(strings.NewReader(`<conflux-root xmlns:ac="urn:conflux:ac" xmlns:ri="urn:conflux:ri">` + raw + `</conflux-root>`))
	decoder.Strict = false
	return decoder
}

func fencedCode(code, language string) string {
	fence := "```"
	for strings.Contains(code, fence) {
		fence += "`"
	}
	return fence + language + "\n" + code + "\n" + fence
}

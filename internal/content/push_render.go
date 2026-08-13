package content

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type LocalAttachment struct {
	Filename  string
	MediaType string
	Content   []byte
}

type AttachmentUpload struct {
	Filename  string
	MediaType string
	SHA256    string
	Content   []byte
}

type PushArtifact struct {
	PageID      string
	SpaceKey    string
	Title       string
	BaseVersion int
	Storage     string
	Uploads     []AttachmentUpload
}

var (
	headingLine    = regexp.MustCompile(`^(#{1,6})[ \t]+(.+)$`)
	unorderedLine  = regexp.MustCompile(`^[ \t]*[-*+][ \t]+(.+)$`)
	orderedLine    = regexp.MustCompile(`^[ \t]*[0-9]+\.[ \t]+(.+)$`)
	blockquoteLine = regexp.MustCompile(`^[ \t]*>[ \t]?(.*)$`)
	imageLink      = regexp.MustCompile(`!\[([^]]*)]\(([^)]+)\)`)
	markdownLink   = regexp.MustCompile(`\[([^]]+)]\(([^)]+)\)`)
	inlineCode     = regexp.MustCompile("`([^`]+)`")
	strongText     = regexp.MustCompile(`\*\*([^*]+)\*\*|__([^_]+)__`)
	emphasisText   = regexp.MustCompile(`\*([^*]+)\*|_([^_]+)_`)
)

func RenderArtifact(markdown string, metadata Metadata, localAttachments []LocalAttachment) (PushArtifact, error) {
	validation, err := ValidateArtifact(markdown, &metadata)
	if err != nil {
		return PushArtifact{}, fmt.Errorf("validate push artifact: %w", err)
	}
	if len(validation.Warnings) > 0 {
		return PushArtifact{}, fmt.Errorf("artifact would discard preserved content: %s", strings.Join(validation.Warnings, "; "))
	}
	if err := validateStandaloneMarkers(markdown); err != nil {
		return PushArtifact{}, fmt.Errorf("validate preservation marker placement: %w", err)
	}

	files := make(map[string]LocalAttachment, len(localAttachments))
	for _, attachment := range localAttachments {
		if err := validateAttachmentFilename(attachment.Filename); err != nil {
			return PushArtifact{}, fmt.Errorf("local attachment: %w", err)
		}
		key := strings.ToLower(attachment.Filename)
		if _, exists := files[key]; exists {
			return PushArtifact{}, fmt.Errorf("duplicate local attachment filename %q", attachment.Filename)
		}
		files[key] = attachment
	}

	storage, references, err := markdownToStorage(markdown, metadata.PreservedFragments)
	if err != nil {
		return PushArtifact{}, fmt.Errorf("render artifact Markdown: %w", err)
	}
	remote := make(map[string]AttachmentMetadata, len(metadata.Attachments))
	for _, attachment := range metadata.Attachments {
		remote[strings.ToLower(attachment.Filename)] = attachment
	}

	var uploads []AttachmentUpload
	for _, filename := range references {
		file, exists := files[strings.ToLower(filename)]
		if !exists {
			return PushArtifact{}, fmt.Errorf("referenced attachment %q is missing from the local artifact", filename)
		}
		digest := sha256.Sum256(file.Content)
		hash := hex.EncodeToString(digest[:])
		if existing, exists := remote[strings.ToLower(filename)]; exists && strings.EqualFold(existing.SHA256, hash) {
			continue
		}
		uploads = append(uploads, AttachmentUpload{
			Filename: file.Filename, MediaType: file.MediaType, SHA256: hash, Content: bytes.Clone(file.Content),
		})
	}

	return PushArtifact{
		PageID: metadata.Page.ID, SpaceKey: metadata.Page.SpaceKey, Title: metadata.Page.Title,
		BaseVersion: metadata.Page.BaseVersion, Storage: storage, Uploads: uploads,
	}, nil
}

func markdownToStorage(markdown string, fragments map[string]string) (string, []string, error) {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	var storage strings.Builder
	var references []string
	var paragraph []string
	var listType string
	var codeLanguage string
	var codeLines []string
	inCode := false
	var codeFence byte
	var codeFenceLength int

	closeParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		storage.WriteString("<p>")
		storage.WriteString(convertInline(strings.Join(paragraph, " "), &references))
		storage.WriteString("</p>")
		paragraph = nil
	}
	closeList := func() {
		if listType != "" {
			storage.WriteString("</" + listType + ">")
			listType = ""
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		fence, fenceLength := fenceMarker(trimmed)
		if (!inCode && fence != 0) || (inCode && fence == codeFence && fenceLength >= codeFenceLength) {
			closeParagraph()
			closeList()
			if !inCode {
				inCode = true
				codeFence = fence
				codeFenceLength = fenceLength
				codeLanguage = strings.TrimSpace(trimmed[fenceLength:])
				codeLines = nil
			} else {
				storage.WriteString(`<ac:structured-macro ac:name="code" ac:schema-version="1">`)
				if codeLanguage != "" {
					storage.WriteString(`<ac:parameter ac:name="language">` + html.EscapeString(codeLanguage) + `</ac:parameter>`)
				}
				storage.WriteString(`<ac:plain-text-body><![CDATA[` + escapeCDATA(strings.Join(codeLines, "\n")) + `]]></ac:plain-text-body></ac:structured-macro>`)
				inCode = false
				codeFence = 0
				codeFenceLength = 0
			}
			continue
		}
		if inCode {
			codeLines = append(codeLines, line)
			continue
		}
		if marker := preservationMarker.FindStringSubmatch(trimmed); len(marker) == 2 && marker[0] == trimmed {
			closeParagraph()
			closeList()
			storage.WriteString(fragments[marker[1]])
			continue
		}
		if match := headingLine.FindStringSubmatch(line); match != nil {
			closeParagraph()
			closeList()
			level := strconv.Itoa(len(match[1]))
			storage.WriteString("<h" + level + ">" + convertInline(match[2], &references) + "</h" + level + ">")
			continue
		}
		if match := unorderedLine.FindStringSubmatch(line); match != nil {
			closeParagraph()
			if listType != "ul" {
				closeList()
				storage.WriteString("<ul>")
				listType = "ul"
			}
			storage.WriteString("<li>" + convertInline(match[1], &references) + "</li>")
			continue
		}
		if match := orderedLine.FindStringSubmatch(line); match != nil {
			closeParagraph()
			if listType != "ol" {
				closeList()
				storage.WriteString("<ol>")
				listType = "ol"
			}
			storage.WriteString("<li>" + convertInline(match[1], &references) + "</li>")
			continue
		}
		if match := blockquoteLine.FindStringSubmatch(line); match != nil {
			closeParagraph()
			closeList()
			storage.WriteString("<blockquote><p>" + convertInline(match[1], &references) + "</p></blockquote>")
			continue
		}
		if trimmed == "" {
			closeParagraph()
			closeList()
			continue
		}
		closeList()
		paragraph = append(paragraph, trimmed)
	}
	if inCode {
		return "", nil, fmt.Errorf("markdown contains an unclosed fenced code block")
	}
	closeParagraph()
	closeList()
	return storage.String(), deduplicateStrings(references), nil
}

func convertInline(value string, references *[]string) string {
	escaped := html.EscapeString(value)
	var protected []string
	placeholderPrefix := "CONFLUXPROTECTED"
	for strings.Contains(escaped, placeholderPrefix) {
		placeholderPrefix += "X"
	}
	placeholder := func(index int) string {
		return fmt.Sprintf("%s%dZ", placeholderPrefix, index)
	}
	protect := func(rendered string) string {
		token := placeholder(len(protected))
		protected = append(protected, rendered)
		return token
	}
	escaped = imageLink.ReplaceAllStringFunc(escaped, func(match string) string {
		parts := imageLink.FindStringSubmatch(match)
		filename := attachmentFilename(parts[2])
		if filename == "" {
			return match
		}
		*references = append(*references, filename)
		return protect(`<ac:image ac:alt="` + parts[1] + `"><ri:attachment ri:filename="` + html.EscapeString(filename) + `" /></ac:image>`)
	})
	escaped = markdownLink.ReplaceAllStringFunc(escaped, func(match string) string {
		parts := markdownLink.FindStringSubmatch(match)
		if filename := attachmentFilename(parts[2]); filename != "" {
			*references = append(*references, filename)
			return protect(`<ac:link><ri:attachment ri:filename="` + html.EscapeString(filename) + `" /><ac:link-body>` + formatEmphasis(parts[1]) + `</ac:link-body></ac:link>`)
		}
		return protect(`<a href="` + parts[2] + `">` + formatEmphasis(parts[1]) + `</a>`)
	})
	escaped = inlineCode.ReplaceAllStringFunc(escaped, func(match string) string {
		parts := inlineCode.FindStringSubmatch(match)
		return protect(`<code>` + parts[1] + `</code>`)
	})
	escaped = formatEmphasis(escaped)
	for i, rendered := range protected {
		escaped = strings.ReplaceAll(escaped, placeholder(i), rendered)
	}
	return escaped
}

func formatEmphasis(value string) string {
	value = strongText.ReplaceAllStringFunc(value, func(match string) string {
		parts := strongText.FindStringSubmatch(match)
		return "<strong>" + firstNonEmpty(parts[1], parts[2]) + "</strong>"
	})
	return emphasisText.ReplaceAllStringFunc(value, func(match string) string {
		parts := emphasisText.FindStringSubmatch(match)
		return "<em>" + firstNonEmpty(parts[1], parts[2]) + "</em>"
	})
}

func validateStandaloneMarkers(markdown string) error {
	for _, line := range strings.Split(markdownOutsideCode(markdown), "\n") {
		match := preservationMarker.FindString(strings.TrimSpace(line))
		if match != "" && match != strings.TrimSpace(line) {
			return fmt.Errorf("preservation marker must appear on its own line")
		}
	}
	return nil
}

func attachmentFilename(target string) string {
	decoded, err := url.PathUnescape(strings.TrimSpace(target))
	if err != nil {
		return ""
	}
	clean := filepath.ToSlash(filepath.Clean(decoded))
	if !strings.HasPrefix(clean, "attachments/") && !strings.Contains(clean, ".attachments/") {
		return ""
	}
	return filepath.Base(clean)
}

func escapeCDATA(value string) string { return strings.ReplaceAll(value, "]]>", "]]]]><![CDATA[>") }

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func deduplicateStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}

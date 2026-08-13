package content

import (
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"regexp"
	"strconv"
	"strings"

	htmldoc "github.com/JohannesKaufmann/html-to-markdown/v2"
)

type jiraMacro struct {
	Key      string
	Server   string
	ServerID string
}

var (
	tableDirectivePattern   = regexp.MustCompile(`^<!--\s*conflux:table\s+layout="(default|center|wide|full-width)"(?:\s+width="([0-9]+)")?\s*-->$`)
	tableDirectiveCandidate = regexp.MustCompile(`^<!--\s*conflux:table\b`)
	jiraDirectivePattern    = regexp.MustCompile(`^\[([^]]+)]\(jira:([A-Za-z][A-Za-z0-9]+-[0-9]+)\)\{conflux-display=inline\s+jira-server="([^"]+)"\s+jira-server-id="([^"]+)"}$`)
	tableRowPattern         = regexp.MustCompile(`(?s)<tr\b[^>]*>(.*?)</tr>`)
	tableCellPattern        = regexp.MustCompile(`(?s)<(th|td)\b[^>]*>(.*?)</(?:th|td)>`)
)

func parseJiraMacro(node storageNode) (jiraMacro, bool) {
	if node.Name.Space != "urn:conflux:ac" || node.Name.Local != "structured-macro" || !strings.Contains(node.Raw, `ac:name="jira"`) {
		return jiraMacro{}, false
	}
	decoder := storageNodeDecoder(node.Raw)
	result := jiraMacro{}
	var parameter string
	for {
		token, err := decoder.Token()
		if err != nil {
			return result, err == io.EOF && result.Key != "" && result.Server != "" && result.ServerID != ""
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Space == "urn:conflux:ac" && value.Name.Local == "parameter" {
				for _, attribute := range value.Attr {
					if attribute.Name.Space == "urn:conflux:ac" && attribute.Name.Local == "name" {
						parameter = attribute.Value
					}
				}
			}
		case xml.EndElement:
			if value.Name.Space == "urn:conflux:ac" && value.Name.Local == "parameter" {
				parameter = ""
			}
		case xml.CharData:
			switch parameter {
			case "key":
				result.Key += string(value)
			case "server":
				result.Server += string(value)
			case "serverId":
				result.ServerID += string(value)
			}
		}
	}
}

func jiraMarkdown(jira jiraMacro) string {
	return fmt.Sprintf(`[%s](jira:%s){conflux-display=inline jira-server="%s" jira-server-id="%s"}`,
		jira.Key, jira.Key, jira.Server, jira.ServerID)
}

func jiraStorage(line string) (string, bool) {
	match := jiraDirectivePattern.FindStringSubmatch(line)
	if match == nil {
		return "", false
	}
	return `<ac:structured-macro ac:name="jira" ac:schema-version="1">` +
		`<ac:parameter ac:name="key">` + html.EscapeString(match[2]) + `</ac:parameter>` +
		`<ac:parameter ac:name="serverId">` + html.EscapeString(match[4]) + `</ac:parameter>` +
		`<ac:parameter ac:name="server">` + html.EscapeString(match[3]) + `</ac:parameter>` +
		`</ac:structured-macro>`, true
}

func tableStorageOptions(raw string) (string, int) {
	decoder := storageNodeDecoder(raw)
	for {
		token, err := decoder.Token()
		if err != nil {
			return "default", 0
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "table" {
			continue
		}
		layout := "default"
		width := 0
		for _, attribute := range start.Attr {
			switch attribute.Name.Local {
			case "data-layout":
				layout = attribute.Value
			case "data-table-width":
				width, _ = strconv.Atoi(attribute.Value)
			}
		}
		return layout, width
	}
}

func tableDirective(layout string, width int) string {
	if width > 0 {
		return fmt.Sprintf(`<!-- conflux:table layout="%s" width="%d" -->`, layout, width)
	}
	return fmt.Sprintf(`<!-- conflux:table layout="%s" -->`, layout)
}

func storageTableMarkdown(raw string) (string, error) {
	rows := tableRowPattern.FindAllStringSubmatch(raw, -1)
	if len(rows) == 0 {
		return "", fmt.Errorf("confluence table has no rows")
	}
	var markdown strings.Builder
	columnCount := 0
	for rowIndex, row := range rows {
		cells := tableCellPattern.FindAllStringSubmatch(row[1], -1)
		if rowIndex == 0 {
			columnCount = len(cells)
		}
		if len(cells) == 0 || len(cells) != columnCount {
			return "", fmt.Errorf("confluence table row %d has inconsistent cells", rowIndex+1)
		}
		markdown.WriteString("|")
		for _, cell := range cells {
			value, err := htmldoc.ConvertString(cell[2])
			if err != nil {
				return "", fmt.Errorf("convert Confluence table cell: %w", err)
			}
			value = strings.ReplaceAll(strings.TrimSpace(value), "|", `\|`)
			value = strings.Join(strings.Fields(value), " ")
			markdown.WriteString(" " + value + " |")
		}
		markdown.WriteByte('\n')
		if rowIndex == 0 {
			markdown.WriteString("|")
			for range cells {
				markdown.WriteString(" --- |")
			}
			markdown.WriteByte('\n')
		}
	}
	return markdown.String(), nil
}

func markdownTableToStorage(lines []string, start int, layout, widthValue string, references *[]string) (string, int, error) {
	if start+1 >= len(lines) {
		return "", start, fmt.Errorf("table directive must be followed by a Markdown table")
	}
	header := splitTableRow(lines[start])
	delimiter := splitTableRow(lines[start+1])
	if len(header) == 0 || len(header) != len(delimiter) || !isTableDelimiter(delimiter) {
		return "", start, fmt.Errorf("table directive must be followed immediately by a valid Markdown table")
	}
	width := 0
	if widthValue != "" {
		width, _ = strconv.Atoi(widthValue)
		if width < 1 {
			return "", start, fmt.Errorf("table width must be positive")
		}
	}

	var storage strings.Builder
	storage.WriteString(`<table data-layout="` + layout + `"`)
	if width > 0 {
		storage.WriteString(` data-table-width="` + strconv.Itoa(width) + `"`)
	}
	storage.WriteString(`><tbody><tr>`)
	for _, cell := range header {
		storage.WriteString("<th><p>" + convertInline(cell, references) + "</p></th>")
	}
	storage.WriteString("</tr>")
	last := start + 1
	for index := start + 2; index < len(lines); index++ {
		cells := splitTableRow(lines[index])
		if len(cells) == 0 {
			break
		}
		if len(cells) != len(header) {
			return "", index, fmt.Errorf("table row %d has %d cells; expected %d", index-start, len(cells), len(header))
		}
		storage.WriteString("<tr>")
		for _, cell := range cells {
			storage.WriteString("<td><p>" + convertInline(cell, references) + "</p></td>")
		}
		storage.WriteString("</tr>")
		last = index
	}
	storage.WriteString("</tbody></table>")
	return storage.String(), last, nil
}

func splitTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	if !strings.Contains(trimmed, "|") {
		return nil
	}
	trimmed = strings.TrimPrefix(strings.TrimSuffix(trimmed, "|"), "|")
	parts := strings.Split(trimmed, "|")
	result := make([]string, len(parts))
	for index, part := range parts {
		result[index] = strings.TrimSpace(part)
	}
	return result
}

func isTableDelimiter(cells []string) bool {
	for _, cell := range cells {
		cell = strings.Trim(cell, ":")
		if len(cell) < 3 || strings.Trim(cell, "-") != "" {
			return false
		}
	}
	return true
}

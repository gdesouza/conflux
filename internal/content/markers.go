package content

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	preservationMarker          = regexp.MustCompile(`<!--\s*conflux:preserved\s+id="([A-Za-z0-9._-]+)"\s*-->`)
	preservationMarkerCandidate = regexp.MustCompile(`<!--\s*conflux:preserved\b`)
	preservedFragmentID         = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

type ValidationResult struct {
	Warnings []string
}

func ValidateArtifact(markdown string, metadata *Metadata) (ValidationResult, error) {
	markerSource := markdownOutsideCode(markdown)
	matches := preservationMarker.FindAllStringSubmatch(markerSource, -1)
	if len(preservationMarkerCandidate.FindAllStringIndex(markerSource, -1)) != len(matches) {
		return ValidationResult{}, fmt.Errorf("markdown contains a malformed preservation marker")
	}

	if metadata == nil {
		if len(matches) > 0 {
			return ValidationResult{}, fmt.Errorf("preservation markers require artifact metadata")
		}
		return ValidationResult{}, nil
	}
	if err := metadata.Validate(); err != nil {
		return ValidationResult{}, fmt.Errorf("validate artifact metadata: %w", err)
	}

	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		id := match[1]
		if _, duplicate := seen[id]; duplicate {
			return ValidationResult{}, fmt.Errorf("duplicate preservation marker %q", id)
		}
		seen[id] = struct{}{}
		if _, exists := metadata.PreservedFragments[id]; !exists {
			return ValidationResult{}, fmt.Errorf("preservation marker %q has no metadata fragment", id)
		}
	}

	var unreferenced []string
	for id := range metadata.PreservedFragments {
		if _, exists := seen[id]; !exists {
			unreferenced = append(unreferenced, id)
		}
	}
	sort.Strings(unreferenced)

	result := ValidationResult{}
	for _, id := range unreferenced {
		result.Warnings = append(result.Warnings, fmt.Sprintf("preserved fragment %q is not referenced by the markdown", id))
	}
	return result, nil
}

func markdownOutsideCode(markdown string) string {
	lines := strings.SplitAfter(markdown, "\n")
	var result strings.Builder
	var fence byte
	var fenceLength int

	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		marker, length := fenceMarker(trimmed)
		if fence != 0 {
			result.WriteString(strings.Repeat(" ", len(strings.TrimSuffix(line, "\n"))))
			if strings.HasSuffix(line, "\n") {
				result.WriteByte('\n')
			}
			if marker == fence && length >= fenceLength {
				fence = 0
				fenceLength = 0
			}
			continue
		}
		if marker != 0 {
			fence = marker
			fenceLength = length
			result.WriteString(strings.Repeat(" ", len(strings.TrimSuffix(line, "\n"))))
			if strings.HasSuffix(line, "\n") {
				result.WriteByte('\n')
			}
			continue
		}
		result.WriteString(maskInlineCode(line))
	}
	return result.String()
}

func fenceMarker(line string) (byte, int) {
	if len(line) < 3 || (line[0] != '`' && line[0] != '~') {
		return 0, 0
	}
	marker := line[0]
	length := 0
	for length < len(line) && line[length] == marker {
		length++
	}
	if length < 3 {
		return 0, 0
	}
	return marker, length
}

func maskInlineCode(line string) string {
	masked := []byte(line)
	for i := 0; i < len(line); {
		if line[i] != '`' {
			i++
			continue
		}
		runLength := 1
		for i+runLength < len(line) && line[i+runLength] == '`' {
			runLength++
		}
		closing := strings.Index(line[i+runLength:], strings.Repeat("`", runLength))
		if closing < 0 {
			i += runLength
			continue
		}
		end := i + runLength + closing + runLength
		for j := i; j < end; j++ {
			masked[j] = ' '
		}
		i = end
	}
	return string(masked)
}

func PreservationMarker(id string) (string, error) {
	if !preservedFragmentID.MatchString(id) {
		return "", fmt.Errorf("invalid preserved fragment id %q", id)
	}
	return fmt.Sprintf(`<!-- conflux:preserved id="%s" -->`, id), nil
}

package content

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	preservationMarker  = regexp.MustCompile(`<!--\s*conflux:preserved\s+id="([A-Za-z0-9._-]+)"\s*-->`)
	preservedFragmentID = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

type ValidationResult struct {
	Warnings []string
}

func ValidateArtifact(markdown string, metadata *Metadata) (ValidationResult, error) {
	matches := preservationMarker.FindAllStringSubmatch(markdown, -1)
	markerPrefixCount := strings.Count(markdown, "<!-- conflux:preserved")
	if markerPrefixCount != len(matches) {
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

func PreservationMarker(id string) (string, error) {
	if !preservedFragmentID.MatchString(id) {
		return "", fmt.Errorf("invalid preserved fragment id %q", id)
	}
	return fmt.Sprintf(`<!-- conflux:preserved id="%s" -->`, id), nil
}

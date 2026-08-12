package content

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const SchemaVersion = 1

var ErrMetadataNotFound = errors.New("artifact metadata not found")

type Metadata struct {
	SchemaVersion      int                  `json:"schema_version"`
	Page               PageMetadata         `json:"page"`
	PreservedFragments map[string]string    `json:"preserved_fragments"`
	Attachments        []AttachmentMetadata `json:"attachments"`
}

type PageMetadata struct {
	ID          string `json:"id"`
	SpaceKey    string `json:"space_key"`
	Title       string `json:"title"`
	BaseVersion int    `json:"base_version"`
}

type AttachmentMetadata struct {
	ID        string `json:"id"`
	Filename  string `json:"filename"`
	MediaType string `json:"media_type"`
	SHA256    string `json:"sha256"`
}

func (m Metadata) Validate() error {
	if m.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported metadata schema version %d; supported version is %d", m.SchemaVersion, SchemaVersion)
	}
	if strings.TrimSpace(m.Page.ID) == "" {
		return fmt.Errorf("metadata page id is required")
	}
	if strings.TrimSpace(m.Page.SpaceKey) == "" {
		return fmt.Errorf("metadata page space key is required")
	}
	if strings.TrimSpace(m.Page.Title) == "" {
		return fmt.Errorf("metadata page title is required")
	}
	if m.Page.BaseVersion < 1 {
		return fmt.Errorf("metadata page base version must be positive")
	}

	seenFilenames := make(map[string]struct{}, len(m.Attachments))
	for i, attachment := range m.Attachments {
		if err := validateAttachmentFilename(attachment.Filename); err != nil {
			return fmt.Errorf("attachment %d: %w", i, err)
		}
		key := strings.ToLower(attachment.Filename)
		if _, exists := seenFilenames[key]; exists {
			return fmt.Errorf("duplicate attachment filename %q", attachment.Filename)
		}
		seenFilenames[key] = struct{}{}
	}

	for id, fragment := range m.PreservedFragments {
		if !preservedFragmentID.MatchString(id) {
			return fmt.Errorf("preserved fragment id %q is invalid", id)
		}
		if strings.TrimSpace(fragment) == "" {
			return fmt.Errorf("preserved fragment %q is empty", id)
		}
	}
	return nil
}

func LoadMetadata(path string) (Metadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Metadata{}, fmt.Errorf("%w: %s", ErrMetadataNotFound, path)
		}
		return Metadata{}, fmt.Errorf("read artifact metadata: %w", err)
	}

	if err := rejectDuplicateJSONFields(data); err != nil {
		return Metadata{}, fmt.Errorf("decode artifact metadata: %w", err)
	}

	var metadata Metadata
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&metadata); err != nil {
		return Metadata{}, fmt.Errorf("decode artifact metadata: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Metadata{}, fmt.Errorf("decode artifact metadata: unexpected trailing content")
	}
	if err := metadata.Validate(); err != nil {
		return Metadata{}, fmt.Errorf("validate artifact metadata: %w", err)
	}
	return metadata, nil
}

func rejectDuplicateJSONFields(data []byte) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	if err := inspectJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected trailing content")
		}
		return err
	}
	return nil
}

func inspectJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}

	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			if _, duplicate := seen[key]; duplicate {
				return fmt.Errorf("duplicate JSON field %q", key)
			}
			seen[key] = struct{}{}
			if err := inspectJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err := inspectJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
}

func LoadArtifactMetadata(markdownPath string) (Metadata, error) {
	paths, err := PathsFor(markdownPath)
	if err != nil {
		return Metadata{}, err
	}
	return LoadMetadata(paths.MetadataPath)
}

func SaveArtifactMetadata(markdownPath string, metadata Metadata) error {
	paths, err := PathsFor(markdownPath)
	if err != nil {
		return err
	}
	return SaveMetadata(paths.MetadataPath, metadata)
}

func SaveMetadata(path string, metadata Metadata) error {
	if err := metadata.Validate(); err != nil {
		return fmt.Errorf("validate artifact metadata: %w", err)
	}

	dir := filepath.Dir(filepath.Clean(path))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create artifact metadata directory: %w", err)
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("encode artifact metadata: %w", err)
	}
	data = append(data, '\n')

	temp, err := os.CreateTemp(dir, ".metadata-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary artifact metadata: %w", err)
	}
	tempPath := temp.Name()
	keepTemp := true
	defer func() {
		if keepTemp {
			_ = os.Remove(tempPath)
		}
	}()

	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("secure temporary artifact metadata: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write temporary artifact metadata: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync temporary artifact metadata: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary artifact metadata: %w", err)
	}
	// path is the caller-selected artifact path; tempPath is created in that
	// same directory and cannot cross the artifact seam.
	// #nosec G703 -- replacing the explicitly selected metadata path is intended.
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace artifact metadata: %w", err)
	}
	keepTemp = false
	return nil
}

func validateAttachmentFilename(filename string) error {
	if strings.TrimSpace(filename) == "" {
		return fmt.Errorf("filename is required")
	}
	if filename == metadataFilename {
		return fmt.Errorf("filename %q is reserved", filename)
	}
	if filepath.IsAbs(filename) || filepath.Base(filename) != filename || filename == "." || filename == ".." {
		return fmt.Errorf("filename must not escape the attachments directory: %q", filename)
	}
	return nil
}

package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"conflux/internal/confluence"
	"conflux/internal/content"
)

func pullEditableArtifact(ctx context.Context, outputWriter io.Writer, client confluence.ConfluenceClient, page *confluence.Page, spaceKey, output string, force bool) error {
	paths, err := content.PathsFor(output)
	if err != nil {
		return fmt.Errorf("resolve output artifact: %w", err)
	}
	if !force && (pathExists(paths.MarkdownPath) || pathExists(paths.AttachmentsDir)) {
		return fmt.Errorf("output artifact already exists; use --force to replace it")
	}
	downloader, ok := client.(attachmentDownloader)
	if !ok {
		return fmt.Errorf("confluence adapter does not support attachment downloads")
	}

	attachments, err := client.ListAttachments(page.ID)
	if err != nil {
		return fmt.Errorf("list page attachments: %w", err)
	}
	attachmentMetadata := make([]content.AttachmentMetadata, 0, len(attachments))
	for _, attachment := range attachments {
		attachmentMetadata = append(attachmentMetadata, content.AttachmentMetadata{
			ID: attachment.ID, Filename: attachment.Title, MediaType: attachment.MediaType,
		})
	}
	artifact, err := content.RenderStorage(content.StoragePage{
		ID: page.ID, SpaceKey: spaceKey, Title: page.Title, BaseVersion: page.Version.Number,
		Storage: page.Body.Storage.Value, AttachmentDirectory: filepath.Base(paths.AttachmentsDir),
		Attachments: attachmentMetadata,
	})
	if err != nil {
		return fmt.Errorf("render editable artifact: %w", err)
	}

	if err := os.MkdirAll(paths.MarkdownDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	stageRoot, err := os.MkdirTemp(paths.MarkdownDir, ".conflux-pull-*")
	if err != nil {
		return fmt.Errorf("create pull staging directory: %w", err)
	}
	defer os.RemoveAll(stageRoot)
	stageMarkdown := filepath.Join(stageRoot, filepath.Base(paths.MarkdownPath))
	stageAttachments := filepath.Join(stageRoot, filepath.Base(paths.AttachmentsDir))
	if err := os.Mkdir(stageAttachments, 0o755); err != nil {
		return fmt.Errorf("create staged attachments directory: %w", err)
	}

	for _, download := range artifact.Downloads {
		digest, err := downloadAttachment(ctx, downloader, page.ID, download, stageAttachments)
		if err != nil {
			return err
		}
		for i := range artifact.Metadata.Attachments {
			if artifact.Metadata.Attachments[i].ID == download.ID {
				artifact.Metadata.Attachments[i].SHA256 = digest
			}
		}
	}
	if err := os.WriteFile(stageMarkdown, []byte(artifact.Markdown), 0o600); err != nil {
		return fmt.Errorf("write staged Markdown: %w", err)
	}
	if err := content.SaveMetadata(filepath.Join(stageAttachments, "metadata.json"), artifact.Metadata); err != nil {
		return err
	}
	if err := installPulledArtifact(paths, stageMarkdown, stageAttachments, force); err != nil {
		return err
	}
	fmt.Fprintf(outputWriter, "Pulled page %s to %s\n", page.ID, paths.MarkdownPath)
	return nil
}

func downloadAttachment(ctx context.Context, downloader attachmentDownloader, pageID string, download content.AttachmentDownload, directory string) (string, error) {
	body, err := downloader.DownloadAttachment(ctx, pageID, download.ID)
	if err != nil {
		return "", fmt.Errorf("download attachment %q: %w", download.Filename, err)
	}
	defer body.Close()

	path := filepath.Join(directory, download.Filename)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("create staged attachment %q: %w", download.Filename, err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(file, hash), body)
	closeErr := file.Close()
	if copyErr != nil {
		return "", fmt.Errorf("write attachment %q: %w", download.Filename, copyErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close attachment %q: %w", download.Filename, closeErr)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func installPulledArtifact(paths content.ArtifactPaths, stageMarkdown, stageAttachments string, force bool) error {
	markdownExists := pathExists(paths.MarkdownPath)
	attachmentsExist := pathExists(paths.AttachmentsDir)
	if !force && (markdownExists || attachmentsExist) {
		return fmt.Errorf("output artifact already exists; use --force to replace it")
	}

	backupRoot, err := os.MkdirTemp(paths.MarkdownDir, ".conflux-backup-*")
	if err != nil {
		return fmt.Errorf("create artifact backup directory: %w", err)
	}
	defer os.RemoveAll(backupRoot)
	backupMarkdown := filepath.Join(backupRoot, filepath.Base(paths.MarkdownPath))
	backupAttachments := filepath.Join(backupRoot, filepath.Base(paths.AttachmentsDir))
	if markdownExists {
		if err := os.Rename(paths.MarkdownPath, backupMarkdown); err != nil {
			return fmt.Errorf("back up existing Markdown: %w", err)
		}
	}
	if attachmentsExist {
		if err := os.Rename(paths.AttachmentsDir, backupAttachments); err != nil {
			if markdownExists {
				_ = os.Rename(backupMarkdown, paths.MarkdownPath)
			}
			return fmt.Errorf("back up existing attachments: %w", err)
		}
	}

	rollback := func() {
		_ = os.Remove(paths.MarkdownPath)
		_ = os.RemoveAll(paths.AttachmentsDir)
		if markdownExists {
			_ = os.Rename(backupMarkdown, paths.MarkdownPath)
		}
		if attachmentsExist {
			_ = os.Rename(backupAttachments, paths.AttachmentsDir)
		}
	}
	if err := os.Rename(stageAttachments, paths.AttachmentsDir); err != nil {
		rollback()
		return fmt.Errorf("install attachments directory: %w", err)
	}
	if err := os.Rename(stageMarkdown, paths.MarkdownPath); err != nil {
		rollback()
		return fmt.Errorf("install Markdown: %w", err)
	}
	return nil
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

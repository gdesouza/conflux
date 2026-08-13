package commands

import (
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"conflux/internal/config"
	"conflux/internal/confluence"
	artifactcontent "conflux/internal/content"
	"conflux/internal/markdown"
	"conflux/pkg/logger"
)

var (
	pushFile    string
	pushSpace   string
	pushParent  string
	pushProject string
	pushForce   bool
)

// pushCmd pushes (creates or updates) a single markdown file as a Confluence page
var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push a single markdown file to Confluence",
	Long: `Create or update a Confluence page from a single local markdown file.

Space resolution precedence:
  1. --space flag
  2. --project flag (project's space)
  3. First project in config (implicit default, if space unset)
  4. Top-level confluence.space_key (legacy single-project)

Parent resolution:
  - If --parent looks numeric it is treated as a page ID
  - Otherwise it is resolved as a title in the target space.

If a matching .attachments/metadata.json exists, the recorded page is updated
with optimistic version checks. Otherwise a page with the markdown title is
updated or created.`,
	RunE: runPush,
}

func runPush(cmd *cobra.Command, args []string) error {
	if pushFile == "" {
		return fmt.Errorf("file flag is required for push command")
	}

	info, err := os.Stat(pushFile)
	if err != nil {
		return fmt.Errorf("failed to access file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory; provide a single markdown file", pushFile)
	}
	if strings.ToLower(filepath.Ext(pushFile)) != ".md" {
		return fmt.Errorf("file must have .md extension: %s", pushFile)
	}

	metadata, artifact, err := loadPushArtifact(pushFile)
	if err != nil {
		return err
	}

	log := logger.New(verbose)

	// Load relaxed config similar to list-pages (space can be provided by flags / project)
	cfg, err := config.LoadForListPages(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Project selection if provided
	if pushProject != "" {
		if err := cfg.SelectProject(pushProject); err != nil {
			return fmt.Errorf("failed to select project: %w", err)
		}
		if pushSpace == "" {
			pushSpace = cfg.Confluence.SpaceKey
		}
	} else if pushSpace == "" && cfg.Confluence.SpaceKey == "" && len(cfg.Projects) > 0 {
		// Apply default project if nothing specified
		cfg.ApplyDefaultProject()
		pushSpace = cfg.Confluence.SpaceKey
	}
	if artifact {
		if pushSpace != "" && pushSpace != metadata.Page.SpaceKey {
			return fmt.Errorf("artifact belongs to space %q, not %q", metadata.Page.SpaceKey, pushSpace)
		}
		pushSpace = metadata.Page.SpaceKey
	}

	if pushSpace == "" {
		return fmt.Errorf("space flag or --project required for push command")
	}

	client := newConfluenceClient(cfg.Confluence.BaseURL, cfg.Confluence.Username, cfg.Confluence.APIToken, log)
	if artifact {
		return pushEditableArtifact(client, pushFile, metadata, pushForce)
	}

	// Parse markdown file
	doc, err := markdown.ParseFile(pushFile)
	if err != nil {
		return fmt.Errorf("failed to parse markdown file: %w", err)
	}
	log.Debug("Parsed markdown file: title=%s", doc.Title)

	// Convert markdown -> Confluence storage format (initial pass without attachments/mermaid images)
	content := markdown.ConvertToConfluenceFormat(doc.Content)

	// Resolve parent ID if provided
	var parentID string
	if pushParent != "" {
		if isNumeric(pushParent) { // treat as ID
			parentID = pushParent
			log.Debug("Using numeric parent page ID: %s", parentID)
		} else {
			log.Debug("Resolving parent by title: %s", pushParent)
			parentPage, err := client.FindPageByTitle(pushSpace, pushParent)
			if err != nil {
				return fmt.Errorf("failed to resolve parent page '%s': %w", pushParent, err)
			}
			if parentPage == nil {
				return fmt.Errorf("parent page '%s' not found in space '%s'", pushParent, pushSpace)
			}
			parentID = parentPage.ID
		}
	}

	// Determine if page exists already (lookup by title)
	existing, err := client.FindPageByTitle(pushSpace, doc.Title)
	if err != nil {
		return fmt.Errorf("failed to search for existing page: %w", err)
	}

	var page *confluence.Page
	if existing != nil {
		log.Debug("Updating existing page ID=%s title=%s", existing.ID, existing.Title)
		page, err = client.UpdatePage(existing.ID, doc.Title, content)
		if err != nil {
			return fmt.Errorf("failed to update page: %w", err)
		}
		fmt.Printf("Updated page '%s' (ID: %s) in space '%s'\n", page.Title, page.ID, pushSpace)
	} else {
		if parentID != "" {
			log.Debug("Creating new page with parent %s", parentID)
			page, err = client.CreatePageWithParent(pushSpace, doc.Title, content, parentID)
		} else {
			log.Debug("Creating new root page in space %s", pushSpace)
			page, err = client.CreatePage(pushSpace, doc.Title, content)
		}
		if err != nil {
			return fmt.Errorf("failed to create page: %w", err)
		}
		fmt.Printf("Created page '%s' (ID: %s) in space '%s'\n", page.Title, page.ID, pushSpace)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(pushCmd)

	pushCmd.Flags().StringVarP(&pushFile, "file", "f", "", "Path to local markdown file (required)")
	pushCmd.Flags().StringVarP(&pushSpace, "space", "s", "", "Confluence space key (can be inferred from --project)")
	pushCmd.Flags().StringVarP(&pushParent, "parent", "p", "", "Optional parent page title or ID")
	pushCmd.Flags().StringVarP(&pushProject, "project", "P", "", "Project name defined in config to infer space")
	pushCmd.Flags().BoolVar(&pushForce, "force", false, "Overwrite an artifact page even when its remote version has changed")

	if err := pushCmd.MarkFlagRequired("file"); err != nil {
		panic(fmt.Sprintf("Failed to mark file flag as required: %v", err))
	}
}

func loadPushArtifact(markdownPath string) (artifactcontent.Metadata, bool, error) {
	metadata, err := artifactcontent.LoadArtifactMetadata(markdownPath)
	if err == nil {
		return metadata, true, nil
	}
	if errors.Is(err, artifactcontent.ErrMetadataNotFound) {
		return artifactcontent.Metadata{}, false, nil
	}
	return artifactcontent.Metadata{}, false, fmt.Errorf("load editable artifact: %w", err)
}

type artifactPushClient interface {
	GetPage(pageID string) (*confluence.Page, error)
	UpdatePageAtVersion(pageID, title, content string, baseVersion int) (*confluence.Page, error)
	UploadAttachment(pageID, filePath string) (*confluence.Attachment, error)
	UploadAttachmentVersion(pageID, attachmentID, filePath string) (*confluence.Attachment, error)
	ListAttachments(pageID string) ([]confluence.Attachment, error)
}

func pushEditableArtifact(client confluence.ConfluenceClient, markdownPath string, metadata artifactcontent.Metadata, force bool) error {
	pusher, ok := client.(artifactPushClient)
	if !ok {
		return fmt.Errorf("confluence adapter does not support safe artifact pushes")
	}
	markdownBody, err := os.ReadFile(markdownPath)
	if err != nil {
		return fmt.Errorf("read artifact Markdown: %w", err)
	}
	paths, err := artifactcontent.PathsFor(markdownPath)
	if err != nil {
		return fmt.Errorf("resolve artifact paths: %w", err)
	}
	attachments, attachmentPaths, err := readLocalAttachments(paths.AttachmentsDir)
	if err != nil {
		return err
	}
	rendered, err := artifactcontent.RenderArtifact(string(markdownBody), metadata, attachments)
	if err != nil {
		return fmt.Errorf("render editable artifact: %w", err)
	}

	remote, err := pusher.GetPage(rendered.PageID)
	if err != nil {
		return fmt.Errorf("get current page: %w", err)
	}
	if remote == nil {
		return fmt.Errorf("page %s was not found", rendered.PageID)
	}
	updateBase := rendered.BaseVersion
	if remote.Version.Number != rendered.BaseVersion {
		if !force {
			return fmt.Errorf("page %s changed in Confluence (local base version %d, remote version %d); pull again or use --force", rendered.PageID, rendered.BaseVersion, remote.Version.Number)
		}
		updateBase = remote.Version.Number
	}

	remoteAttachmentIDs, err := resolveRemoteAttachmentIDs(pusher, rendered.PageID, metadata, rendered.Uploads)
	if err != nil {
		return err
	}
	page, err := pusher.UpdatePageAtVersion(rendered.PageID, rendered.Title, rendered.Storage, updateBase)
	if err != nil {
		return fmt.Errorf("update page at version %d: %w", updateBase, err)
	}
	if page == nil || page.Version.Number < 1 {
		return fmt.Errorf("update page returned no valid version")
	}

	updatedMetadata := metadata
	updatedMetadata.Page.BaseVersion = page.Version.Number
	for _, upload := range rendered.Uploads {
		path := attachmentPaths[strings.ToLower(upload.Filename)]
		attachmentID := remoteAttachmentIDs[strings.ToLower(upload.Filename)]
		var attachment *confluence.Attachment
		var uploadErr error
		if attachmentID == "" {
			attachment, uploadErr = pusher.UploadAttachment(rendered.PageID, path)
		} else {
			attachment, uploadErr = pusher.UploadAttachmentVersion(rendered.PageID, attachmentID, path)
		}
		if uploadErr != nil {
			if saveErr := artifactcontent.SaveArtifactMetadata(markdownPath, updatedMetadata); saveErr != nil {
				return fmt.Errorf("upload attachment %q: %v; preserve updated page version in metadata: %w", upload.Filename, uploadErr, saveErr)
			}
			return fmt.Errorf("upload attachment %q: %w", upload.Filename, uploadErr)
		}
		mergeAttachmentMetadata(&updatedMetadata, upload, attachment)
	}
	if err := artifactcontent.SaveArtifactMetadata(markdownPath, updatedMetadata); err != nil {
		return fmt.Errorf("save updated artifact metadata: %w", err)
	}
	fmt.Printf("Updated page '%s' (ID: %s) in space '%s'\n", page.Title, page.ID, rendered.SpaceKey)
	return nil
}

func resolveRemoteAttachmentIDs(client artifactPushClient, pageID string, metadata artifactcontent.Metadata, uploads []artifactcontent.AttachmentUpload) (map[string]string, error) {
	ids := make(map[string]string, len(uploads))
	needsRemoteLookup := false
	for _, upload := range uploads {
		key := strings.ToLower(upload.Filename)
		ids[key] = attachmentIDFor(metadata, upload.Filename)
		if ids[key] == "" {
			needsRemoteLookup = true
		}
	}
	if !needsRemoteLookup {
		return ids, nil
	}
	attachments, err := client.ListAttachments(pageID)
	if err != nil {
		return nil, fmt.Errorf("list current page attachments: %w", err)
	}
	for _, attachment := range attachments {
		key := strings.ToLower(attachment.Title)
		if _, requested := ids[key]; requested && ids[key] == "" {
			ids[key] = attachment.ID
		}
	}
	return ids, nil
}

func attachmentIDFor(metadata artifactcontent.Metadata, filename string) string {
	for _, attachment := range metadata.Attachments {
		if strings.EqualFold(attachment.Filename, filename) {
			return attachment.ID
		}
	}
	return ""
}

func readLocalAttachments(directory string) ([]artifactcontent.LocalAttachment, map[string]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, nil, fmt.Errorf("read artifact attachments: %w", err)
	}
	attachments := make([]artifactcontent.LocalAttachment, 0, len(entries))
	paths := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.Name() == "metadata.json" {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return nil, nil, fmt.Errorf("inspect attachment %q: %w", entry.Name(), infoErr)
		}
		if !info.Mode().IsRegular() {
			return nil, nil, fmt.Errorf("attachment %q is not a regular file", entry.Name())
		}
		path := filepath.Join(directory, entry.Name())
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, nil, fmt.Errorf("read attachment %q: %w", entry.Name(), readErr)
		}
		attachments = append(attachments, artifactcontent.LocalAttachment{
			Filename: entry.Name(), MediaType: mime.TypeByExtension(filepath.Ext(entry.Name())), Content: body,
		})
		paths[strings.ToLower(entry.Name())] = path
	}
	return attachments, paths, nil
}

func mergeAttachmentMetadata(metadata *artifactcontent.Metadata, upload artifactcontent.AttachmentUpload, remote *confluence.Attachment) {
	for index := range metadata.Attachments {
		if strings.EqualFold(metadata.Attachments[index].Filename, upload.Filename) {
			metadata.Attachments[index].SHA256 = upload.SHA256
			metadata.Attachments[index].MediaType = upload.MediaType
			if remote != nil && remote.ID != "" {
				metadata.Attachments[index].ID = remote.ID
			}
			return
		}
	}
	attachment := artifactcontent.AttachmentMetadata{Filename: upload.Filename, MediaType: upload.MediaType, SHA256: upload.SHA256}
	if remote != nil {
		attachment.ID = remote.ID
	}
	metadata.Attachments = append(metadata.Attachments, attachment)
}

package commands

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	htmldoc "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/spf13/cobra"

	"conflux/internal/config"
	"conflux/internal/confluence"
	"conflux/pkg/logger"
)

// preprocessConfluenceImages replaces <ac:image><ri:attachment ... /></ac:image> with markdown image syntax
func preprocessConfluenceImages(html string) string {
	// Improved regex to match <ac:image ...><ri:attachment ri:filename="..." ... /></ac:image>
	imgRe := regexp.MustCompile(`(?s)<ac:image[^>]*>\s*<ri:attachment[^>]*ri:filename=["']([^"']+)["'][^>]*/?>\s*</ac:image>`) // strict for single attachment
	return imgRe.ReplaceAllStringFunc(html, func(match string) string {
		filenameMatch := imgRe.FindStringSubmatch(match)
		var filename string
		if len(filenameMatch) > 1 {
			filename = filenameMatch[1]
		} else {
			// Fallback: manual search for ri:filename="..."
			idx := strings.Index(match, `ri:filename="`)
			if idx != -1 {
				start := idx + len(`ri:filename="`)
				end := strings.Index(match[start:], `"`)
				if end != -1 {
					filename = match[start : start+end]
				}
			}
		}
		if filename != "" {
			// URL-encode spaces
			link := url.PathEscape(filename)
			return fmt.Sprintf("![%s](attachments/%s)", filename, link)
		}
		return ""
	})
}

var (
	getPageSpace     string
	getPageIDOrTitle string
	getPageFormat    string
	getPageProject   string
)

// getPageCmd returns the raw page storage content for a page
var getPageCmd = &cobra.Command{
	Use:   "get-page",
	Short: "Return the contents of a Confluence page",
	Long: `Fetch the storage-format content of a Confluence page by ID or title.

You must provide either:
  - a space key via --space, or
  - a project via --project (space inferred from project)

Then specify either a numeric page ID or a page title with --page.`,
	Example: `  conflux get-page -space DOCS -page 123456789
  conflux get-page -space DOCS -page "My Page Title"`,
	RunE: runGetPage,
}

func runGetPage(cmd *cobra.Command, args []string) error {
	if getPageIDOrTitle == "" {
		return fmt.Errorf("page flag is required for get-page command")
	}

	// Validate format
	switch getPageFormat {
	case "", "storage", "html", "markdown":
		// ok (empty treated as storage)
	default:
		return fmt.Errorf("unsupported format: %s", getPageFormat)
	}

	log := logger.New(verbose)

	cfg, err := config.LoadForListPages(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Project selection if provided
	if getPageProject != "" {
		if err := cfg.SelectProject(getPageProject); err != nil {
			return fmt.Errorf("failed to select project: %w", err)
		}
		if getPageSpace == "" {
			getPageSpace = cfg.Confluence.SpaceKey
		}
	}
	if getPageSpace == "" {
		return fmt.Errorf("space flag or --project required for get-page command")
	}

	client := newConfluenceClient(cfg.Confluence.BaseURL, cfg.Confluence.Username, cfg.Confluence.APIToken, log)

	var page *confluence.Page

	// Try by ID if input looks numeric
	if isNumeric(getPageIDOrTitle) {
		page, err = client.GetPage(getPageIDOrTitle)
		if err != nil {
			log.Debug("failed to get page by ID: %v", err)
			page = nil
		}
	}

	// If not found by ID, try by title
	if page == nil {
		page, err = client.FindPageByTitle(getPageSpace, getPageIDOrTitle)
		if err != nil {
			return fmt.Errorf("failed to find page by title: %w", err)
		}
	}

	if page == nil {
		return fmt.Errorf("page '%s' not found in space '%s'", getPageIDOrTitle, getPageSpace)
	}

	// Print header then the requested format
	fmt.Printf("# %s (ID: %s)\n\n", page.Title, page.ID)

	format := getPageFormat
	if format == "" {
		format = "storage"
	}

	content, err := generatePageOutput(page, format)
	if err != nil {
		return err
	}

	// Download attachments and update markdown
	attachments, err := client.ListAttachments(page.ID)
	if err != nil {
		log.Debug("failed to list attachments: %v", err)
		// Continue even if no attachments
	}

	attachmentDir := "attachments"
	if len(attachments) > 0 {
		if err := os.MkdirAll(attachmentDir, 0755); err != nil {
			return fmt.Errorf("failed to create attachment directory: %w", err)
		}
	}

	log.Debug("Found %d attachments for page %s", len(attachments), page.ID)
	for _, att := range attachments {
		log.Debug("Attachment: ID=%s Title=%s MediaType=%s Download=%s", att.ID, att.Title, att.MediaType, att.Links.Download)
		downloadURL, err := client.GetAttachmentDownloadURL(page.ID, att.ID)
		if err != nil {
			log.Debug("failed to get download URL for attachment %s: %v", att.Title, err)
			continue
		}
		localPath := fmt.Sprintf("%s/%s", attachmentDir, att.Title)
		req, err := http.NewRequest("GET", downloadURL, nil)
		if err != nil {
			log.Debug("Failed to create request for attachment %s: %v", att.Title, err)
			continue
		}
		req.SetBasicAuth(cfg.Confluence.Username, cfg.Confluence.APIToken)
		realClient, ok := client.(*confluence.Client)
		if !ok {
			log.Debug("Failed to access underlying HTTP client for attachment download")
			continue
		}
		resp, err := realClient.DoAuthenticatedRequest(req)
		if err != nil {
			log.Debug("Failed to download attachment %s: %v", att.Title, err)
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Debug("Failed to download attachment %s: status %d, body: %s", att.Title, resp.StatusCode, string(body))
			continue
		}
		log.Debug("Saving attachment to %s", localPath)
		f, err := os.Create(localPath)
		if err != nil {
			log.Debug("Failed to create local file for attachment %s: %v", att.Title, err)
			continue
		}
		_, err = io.Copy(f, resp.Body)
		f.Close()
		if err != nil {
			log.Debug("Failed to save attachment %s: %v", att.Title, err)
			continue
		}
		// Replace inline markdown references
		if strings.HasPrefix(att.MediaType, "image/") {
			// Image macro replacement now handles inline links; no further replacement needed
			// (If you need to support legacy or non-macro images, add logic here)
		} else {
			// Replace all file references
			content = strings.ReplaceAll(content, fmt.Sprintf("[%s]", att.Title), fmt.Sprintf("[%s](%s)", att.Title, localPath))
			content = strings.ReplaceAll(content, fmt.Sprintf("[]( %s)", att.Title), fmt.Sprintf("[%s](%s)", att.Title, localPath))
			content = strings.ReplaceAll(content, fmt.Sprintf("[]( %s)", localPath), fmt.Sprintf("[%s](%s)", att.Title, localPath))
		}
	}

	fmt.Println(content)
	return nil
}

func preprocessConfluenceMacros(htmlContent string) string {
	// Handle TOC macro
	tocRe := regexp.MustCompile(`(?s)<ac:structured-macro ac:name="toc".*?/>`)
	htmlContent = tocRe.ReplaceAllString(htmlContent, "") // Remove TOC macro

	// Handle Info and Note macros
	infoNoteRe := regexp.MustCompile(`(?s)<ac:structured-macro ac:name="(info|note)".*?><ac:rich-text-body>(.*?)</ac:rich-text-body></ac:structured-macro>`)
	htmlContent = infoNoteRe.ReplaceAllStringFunc(htmlContent, func(match string) string {
		// Extract content from ac:rich-text-body
		contentRe := regexp.MustCompile(`(?s)<ac:rich-text-body>(.*?)</ac:rich-text-body>`)
		contentMatch := contentRe.FindStringSubmatch(match)
		content := ""
		if len(contentMatch) > 1 {
			content = contentMatch[1]
		}
		// Un-escape &gt;
		content = strings.ReplaceAll(content, "&gt;", ">")
		// Convert to blockquote
		return "\n> " + strings.ReplaceAll(content, "\n", "\n> ") + "\n"
	})

	// Handle Inline Comment Marker macro (remove it)
	inlineCommentRe := regexp.MustCompile(`(?s)<ac:inline-comment-marker.*?>.*?</ac:inline-comment-marker>`)
	htmlContent = inlineCommentRe.ReplaceAllString(htmlContent, "")

	// Handle View File macro
	viewFileRe := regexp.MustCompile(`(?s)<ac:structured-macro ac:name="view-file".*?><ac:parameter ac:name="name"><ri:attachment ri:filename="(.*?)".*? /></ac:parameter></ac:structured-macro>`)
	htmlContent = viewFileRe.ReplaceAllStringFunc(htmlContent, func(match string) string {
		filenameRe := regexp.MustCompile(`ri:filename="(.*?)"`)
		filenameMatch := filenameRe.FindStringSubmatch(match)
		filename := ""
		if len(filenameMatch) > 1 {
			filename = filenameMatch[1]
		}
		// Create a markdown link to the attachment
		return fmt.Sprintf("[%s](attachments/%s)", filename, url.PathEscape(filename))
	})

	// Handle Code macro
	codeRe := regexp.MustCompile(`(?s)<ac:structured-macro ac:name="code".*?>(.*?)</ac:structured-macro>`)
	htmlContent = codeRe.ReplaceAllStringFunc(htmlContent, func(match string) string {
		// Extract language
		langRe := regexp.MustCompile(`<ac:parameter ac:name="language">(.*?)</ac:parameter>`)
		langMatch := langRe.FindStringSubmatch(match)
		language := ""
		if len(langMatch) > 1 {
			language = ` class="language-` + langMatch[1] + `"`
		}

		// Extract code
		cdataRe := regexp.MustCompile(`(?s)<!\[CDATA\[(.*?)]]>`)
		cdataMatch := cdataRe.FindStringSubmatch(match)
		code := ""
		if len(cdataMatch) > 1 {
			code = html.EscapeString(cdataMatch[1])
		}

		return "<pre><code" + language + ">" + code + "</code></pre>"
	})

	return htmlContent
}

// generatePageOutput returns the page content in the requested format.
// It does not include the header line with title/ID.
func generatePageOutput(page *confluence.Page, format string) (string, error) {
	switch format {
	case "storage":
		return page.Body.Storage.Value, nil
	case "html":
		if page.Body.View.Value != "" {
			return page.Body.View.Value, nil
		}
		return page.Body.Storage.Value, nil
	case "markdown":
		var htmlContent string
		if page.Body.View.Value != "" {
			htmlContent = page.Body.View.Value
		} else {
			htmlContent = page.Body.Storage.Value
		}
		// Preprocess Confluence image macros to markdown image syntax
		htmlContent = preprocessConfluenceImages(htmlContent)
		// Preprocess other Confluence macros
		htmlContent = preprocessConfluenceMacros(htmlContent)

		markdown, err := htmldoc.ConvertString(htmlContent)
		if err != nil {
			return htmlContent, nil // fallback to raw HTML on conversion errors
		}
		// Patch: unescape image syntax if needed
		patched := strings.ReplaceAll(string(markdown), "!\\\"[", "![")

		// New patch: unescape underscores in image URLs
		imgUrlRe := regexp.MustCompile(`!\[(.*?)\]\((.*?)\)`)
		patched = imgUrlRe.ReplaceAllStringFunc(patched, func(match string) string {
			parts := imgUrlRe.FindStringSubmatch(match)
			altText := parts[1]
			url := parts[2]
			unescapedUrl := strings.ReplaceAll(url, "\\_", "_")
			return fmt.Sprintf("![%s](%s)", altText, unescapedUrl)
		})

		// New patch: unescape &gt; in blockquotes
		patched = strings.ReplaceAll(patched, "&gt;", ">")

		return patched, nil
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

func init() {
	rootCmd.AddCommand(getPageCmd)

	getPageCmd.Flags().StringVarP(&getPageSpace, "space", "s", "", "Confluence space key (can be inferred from --project)")
	getPageCmd.Flags().StringVarP(&getPageIDOrTitle, "page", "p", "", "Page title or ID to fetch (required)")
	getPageCmd.Flags().StringVarP(&getPageFormat, "format", "f", "storage", "Output format: storage|html|markdown")
	getPageCmd.Flags().StringVarP(&getPageProject, "project", "P", "", "Project name defined in config to infer space")

	if err := getPageCmd.MarkFlagRequired("page"); err != nil {
		panic(fmt.Sprintf("Failed to mark page flag as required: %v", err))
	}
}

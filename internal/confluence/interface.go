package confluence

// ConfluenceClient defines the interface for Confluence operations
type ConfluenceClient interface {
	CreatePage(spaceKey, title, content string) (*Page, error)
	CreatePageWithParent(spaceKey, title, content, parentID string) (*Page, error)
	UpdatePage(pageID, title, content string) (*Page, error)
	FindPageByTitle(spaceKey, title string) (*Page, error)
	GetPage(pageID string) (*Page, error)
	UploadAttachment(pageID, filePath string) (*Attachment, error)
	GetPageHierarchy(spaceKey, parentPageTitle string) ([]PageInfo, error)
	GetPageAncestors(pageID string) ([]PageInfo, error)
	GetChildPages(pageID string) ([]PageInfo, error)
	ListAttachments(pageID string) ([]Attachment, error)
	GetAttachmentDownloadURL(pageID, attachmentID string) (string, error)
}

// Ensure Client implements the interface
var _ ConfluenceClient = (*Client)(nil)

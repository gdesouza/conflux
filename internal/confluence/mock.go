package confluence

// MockClient is an in-memory implementation of ConfluenceClient for tests.
type MockClient struct {
	Pages            map[string]*Page        // pageID -> Page
	PagesByTitle     map[string]*Page        // spaceKey:title -> Page
	Children         map[string][]PageInfo   // pageID -> children
	Ancestors        map[string][]PageInfo   // pageID -> ancestors chain
	SpaceHierarchies map[string][]PageInfo   // spaceKey -> root pages (fully nested)
	Attachments      map[string][]Attachment // pageID -> attachments
	CreateCalls      []string                // titles created (for assertions)
	UpdateCalls      []string                // titles updated
	LastUploadedFile string
	FailFindByTitle  bool
}

func NewMockClient() *MockClient {
	return &MockClient{
		Pages:            make(map[string]*Page),
		PagesByTitle:     make(map[string]*Page),
		Children:         make(map[string][]PageInfo),
		Ancestors:        make(map[string][]PageInfo),
		SpaceHierarchies: make(map[string][]PageInfo),
		Attachments:      make(map[string][]Attachment),
	}
}

func (m *MockClient) key(spaceKey, title string) string { return spaceKey + ":" + title }

func (m *MockClient) CreatePage(spaceKey, title, content string) (*Page, error) {
	p := &Page{ID: title + "-id", Title: title}
	p.Body.Storage.Value = content
	m.Pages[p.ID] = p
	m.PagesByTitle[m.key(spaceKey, title)] = p
	m.CreateCalls = append(m.CreateCalls, title)
	return p, nil
}

func (m *MockClient) CreatePageWithParent(spaceKey, title, content, parentID string) (*Page, error) {
	return m.CreatePage(spaceKey, title, content)
}

func (m *MockClient) UpdatePage(pageID, title, content string) (*Page, error) {
	if p, ok := m.Pages[pageID]; ok {
		p.Title = title
		p.Body.Storage.Value = content
		m.UpdateCalls = append(m.UpdateCalls, title)
		return p, nil
	}
	return nil, nil
}

func (m *MockClient) FindPageByTitle(spaceKey, title string) (*Page, error) {
	if m.FailFindByTitle {
		return nil, nil
	}
	return m.PagesByTitle[m.key(spaceKey, title)], nil
}

func (m *MockClient) GetPage(pageID string) (*Page, error) {
	return m.Pages[pageID], nil
}

func (m *MockClient) UploadAttachment(pageID, filePath string) (*Attachment, error) {
	att := Attachment{ID: "att-" + filePath, Title: filePath}
	m.Attachments[pageID] = append(m.Attachments[pageID], att)
	m.LastUploadedFile = filePath
	return &att, nil
}

func (m *MockClient) GetPageHierarchy(spaceKey, parentPageTitle string) ([]PageInfo, error) {
	if parentPageTitle != "" {
		// find parent in hierarchy and return its children if available
		roots := m.SpaceHierarchies[spaceKey]
		var stack []PageInfo
		stack = append(stack, roots...)
		for len(stack) > 0 {
			cur := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if cur.Title == parentPageTitle {
				return cur.Children, nil
			}
			stack = append(stack, cur.Children...)
		}
		return []PageInfo{}, nil
	}
	return m.SpaceHierarchies[spaceKey], nil
}

func (m *MockClient) GetPageAncestors(pageID string) ([]PageInfo, error) {
	return m.Ancestors[pageID], nil
}

func (m *MockClient) GetChildPages(pageID string) ([]PageInfo, error) {
	return m.Children[pageID], nil
}

func (m *MockClient) ListAttachments(pageID string) ([]Attachment, error) {
	return m.Attachments[pageID], nil
}

func (m *MockClient) GetAttachmentDownloadURL(pageID, attachmentID string) (string, error) {
	// Return a dummy local path for testing
	return "attachments/" + attachmentID, nil
}

var _ ConfluenceClient = (*MockClient)(nil)

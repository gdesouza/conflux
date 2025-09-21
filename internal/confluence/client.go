package confluence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"conflux/pkg/logger"
)

type Client struct {
	baseURL  string
	username string
	apiToken string
	client   *http.Client
	logger   *logger.Logger
}

type Page struct {
	ID    string `json:"id,omitempty"`
	Title string `json:"title"`
	Body  struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
	} `json:"body,omitempty"`
	Space struct {
		Key string `json:"key"`
	} `json:"space,omitempty"`
	Version struct {
		Number int `json:"number"`
	} `json:"version,omitempty"`
}

type PageInfo struct {
	ID       string     `json:"id"`
	Title    string     `json:"title"`
	Children []PageInfo `json:"children,omitempty"`
}

func New(baseURL, username, apiToken string) *Client {
	return &Client{
		baseURL:  baseURL,
		username: username,
		apiToken: apiToken,
		client:   &http.Client{},
	}
}

func NewClient(baseURL, username, apiToken string, log *logger.Logger) *Client {
	return &Client{
		baseURL:  baseURL,
		username: username,
		apiToken: apiToken,
		client:   &http.Client{},
		logger:   log,
	}
}

func (c *Client) CreatePage(spaceKey, title, content string) (*Page, error) {
	page := map[string]interface{}{
		"type":  "page",
		"title": title,
		"space": map[string]string{"key": spaceKey},
		"body": map[string]interface{}{
			"storage": map[string]interface{}{
				"value":          content,
				"representation": "storage",
			},
		},
	}

	data, err := json.Marshal(page)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal page data: %w", err)
	}

	// Debug: Log directory pages with children macro content
	if c.logger != nil && len(content) > 0 && (containsChildrenMacro(content) || containsDirectoryKeywords(content)) {
		c.logger.Debug("Creating directory page '%s' with children macro content", title)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/rest/api/content", bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, body)
	}

	var result Page
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (c *Client) CreatePageWithParent(spaceKey, title, content, parentID string) (*Page, error) {
	page := map[string]interface{}{
		"type":  "page",
		"title": title,
		"space": map[string]string{"key": spaceKey},
		"body": map[string]interface{}{
			"storage": map[string]interface{}{
				"value":          content,
				"representation": "storage",
			},
		},
	}

	// Add parent relationship if parentID is provided
	if parentID != "" {
		page["ancestors"] = []map[string]string{
			{"id": parentID},
		}
	}

	data, err := json.Marshal(page)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal page data: %w", err)
	}

	// Debug: Log directory pages with children macro content
	if c.logger != nil && len(content) > 0 && (containsChildrenMacro(content) || containsDirectoryKeywords(content)) {
		c.logger.Debug("Creating directory page '%s' with parent '%s' and children macro content", title, parentID)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/rest/api/content", bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, body)
	}

	var result Page
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (c *Client) UpdatePage(pageID, title, content string) (*Page, error) {
	// Debug: Log directory page updates
	if c.logger != nil && len(content) > 0 && (containsChildrenMacro(content) || containsDirectoryKeywords(content)) {
		c.logger.Debug("Updating directory page '%s' with children macro content", title)
	}

	// First, get the current page to retrieve its version
	currentPage, err := c.GetPage(pageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get current page version: %w", err)
	}

	// Increment the version number
	newVersion := currentPage.Version.Number + 1

	page := map[string]interface{}{
		"id":    pageID,
		"type":  "page",
		"title": title,
		"body": map[string]interface{}{
			"storage": map[string]interface{}{
				"value":          content,
				"representation": "storage",
			},
		},
		"version": map[string]interface{}{
			"number": newVersion,
		},
	}

	data, err := json.Marshal(page)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal page data: %w", err)
	}

	req, err := http.NewRequest("PUT", c.baseURL+"/rest/api/content/"+pageID, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, body)
	}

	var result Page
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (c *Client) GetPage(pageID string) (*Page, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/rest/api/content/"+pageID+"?expand=version", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.apiToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, body)
	}

	var result Page
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (c *Client) FindPageByTitle(spaceKey, title string) (*Page, error) {
	params := url.Values{}
	params.Add("spaceKey", spaceKey)
	params.Add("title", title)
	params.Add("expand", "body.storage")

	req, err := http.NewRequest("GET", c.baseURL+"/rest/api/content?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.apiToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var result struct {
		Results []Page `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Results) == 0 {
		return nil, nil
	}

	return &result.Results[0], nil
}

func (c *Client) GetPageHierarchy(spaceKey, parentPageTitle string) ([]PageInfo, error) {
	var pages []PageInfo
	var err error

	if parentPageTitle != "" {
		// Find the parent page first
		parentPage, err := c.FindPageByTitle(spaceKey, parentPageTitle)
		if err != nil {
			return nil, fmt.Errorf("failed to find parent page '%s': %w", parentPageTitle, err)
		}
		if parentPage == nil {
			return nil, fmt.Errorf("parent page '%s' not found in space '%s'", parentPageTitle, spaceKey)
		}

		// Get children of the parent page
		pages, err = c.getChildPages(parentPage.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get child pages: %w", err)
		}
	} else {
		// Get all pages in the space
		pages, err = c.getAllPagesInSpace(spaceKey)
		if err != nil {
			return nil, fmt.Errorf("failed to get pages in space: %w", err)
		}
	}

	return pages, nil
}

func (c *Client) getAllPagesInSpace(spaceKey string) ([]PageInfo, error) {
	// Get all pages and build proper hierarchy
	allPages, err := c.getAllPagesWithParents(spaceKey)
	if err != nil {
		return nil, err
	}

	// Build the tree by finding root pages and their children
	return c.buildPageTree(allPages), nil
}

// getAllPagesWithParents gets all pages in a space with parent information
func (c *Client) getAllPagesWithParents(spaceKey string) (map[string]PageInfo, error) {
	params := url.Values{}
	params.Add("spaceKey", spaceKey)
	params.Add("type", "page")
	params.Add("limit", "1000")
	params.Add("expand", "ancestors")

	req, err := http.NewRequest("GET", c.baseURL+"/rest/api/content?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.apiToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Results []struct {
			ID        string `json:"id"`
			Title     string `json:"title"`
			Ancestors []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"ancestors"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	pages := make(map[string]PageInfo)
	parentChildMap := make(map[string][]string) // parentID -> []childID

	for _, page := range result.Results {
		pageInfo := PageInfo{
			ID:       page.ID,
			Title:    page.Title,
			Children: []PageInfo{}, // Initialize empty, will be populated later
		}
		pages[page.ID] = pageInfo

		// Determine parent-child relationships
		var parentID string
		if len(page.Ancestors) > 0 {
			// The immediate parent is the last ancestor
			parentID = page.Ancestors[len(page.Ancestors)-1].ID
		}

		if parentID != "" {
			parentChildMap[parentID] = append(parentChildMap[parentID], page.ID)
		}
	}

	// Now build the children relationships
	for parentID, childIDs := range parentChildMap {
		if parent, exists := pages[parentID]; exists {
			for _, childID := range childIDs {
				if child, exists := pages[childID]; exists {
					parent.Children = append(parent.Children, child)
				}
			}
			pages[parentID] = parent
		}
	}

	return pages, nil
}

// buildPageTree builds the tree structure by identifying root pages
func (c *Client) buildPageTree(allPages map[string]PageInfo) []PageInfo {
	// First pass: identify all pages that are children of other pages
	childPages := make(map[string]bool)
	for _, page := range allPages {
		for _, child := range page.Children {
			childPages[child.ID] = true
		}
	}

	// Second pass: root pages are those that are not children of any other page
	var rootPages []PageInfo
	for _, page := range allPages {
		if !childPages[page.ID] {
			rootPages = append(rootPages, page)
		}
	}

	return rootPages
}

func (c *Client) getChildPages(pageID string) ([]PageInfo, error) {
	params := url.Values{}
	params.Add("expand", "children.page")

	req, err := http.NewRequest("GET", c.baseURL+"/rest/api/content/"+pageID+"/child/page?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.apiToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Results []PageInfo `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Recursively get children for each page
	for i := range result.Results {
		children, err := c.getChildPages(result.Results[i].ID)
		if err != nil {
			c.logger.Info("Warning: failed to get children for page '%s': %v", result.Results[i].Title, err)
			continue
		}
		result.Results[i].Children = children
	}

	return result.Results, nil
}

// Helper functions for debug logging
func containsChildrenMacro(content string) bool {
	return strings.Contains(content, "ac:structured-macro ac:name=\"children\"") ||
		strings.Contains(content, "ac:structured-macro ac:name='children'") ||
		strings.Contains(content, "children")
}

func containsDirectoryKeywords(content string) bool {
	content = strings.ToLower(content)
	return strings.Contains(content, "directory page") ||
		strings.Contains(content, "automatically created to organize") ||
		strings.Contains(content, "automatically created") ||
		strings.Contains(content, "documentation for") ||
		strings.Contains(content, "directory") ||
		strings.Contains(content, "organize")
}

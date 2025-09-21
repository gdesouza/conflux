package confluence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

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
	ID       string `json:"id,omitempty"`
	Title    string `json:"title"`
	Content  string `json:"body,omitempty"`
	SpaceKey string `json:"space,omitempty"`
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
			"number": 2,
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
	params := url.Values{}
	params.Add("spaceKey", spaceKey)
	params.Add("type", "page")
	params.Add("limit", "100")
	params.Add("expand", "children.page")

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
			ID       string `json:"id"`
			Title    string `json:"title"`
			Children struct {
				Page struct {
					Results []PageInfo `json:"results"`
				} `json:"page"`
			} `json:"children"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var pages []PageInfo
	for _, page := range result.Results {
		pageInfo := PageInfo{
			ID:       page.ID,
			Title:    page.Title,
			Children: page.Children.Page.Results,
		}
		pages = append(pages, pageInfo)
	}

	return pages, nil
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

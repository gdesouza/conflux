package confluence

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"conflux/pkg/logger"
)

// MockHTTPClient allows for testing HTTP requests
type mockHTTPClient struct {
	responses map[string]*http.Response
	requests  []*http.Request
}

// Implement the http.RoundTripper interface to be compatible with http.Client
func (m *mockHTTPClient) RoundTrip(req *http.Request) (*http.Response, error) {
	m.requests = append(m.requests, req)

	// Try to find a matching response using the full URL first
	if response, exists := m.responses[fmt.Sprintf("%s %s", req.Method, req.URL.String())]; exists {
		return response, nil
	}

	// Fallback to checking just the path
	if response, exists := m.responses[fmt.Sprintf("%s %s", req.Method, req.URL.Path)]; exists {
		return response, nil
	}

	// Also check for partial path matches for the API paths
	for storedKey, response := range m.responses {
		if strings.Contains(storedKey, req.URL.Path) && strings.HasPrefix(storedKey, req.Method) {
			return response, nil
		}
	}

	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader("Not found")),
	}, nil
}

func newMockHTTPClient() *mockHTTPClient {
	return &mockHTTPClient{
		responses: make(map[string]*http.Response),
		requests:  make([]*http.Request, 0),
	}
}

func (m *mockHTTPClient) addResponse(method, path string, statusCode int, body interface{}) {
	var bodyReader io.Reader
	if body != nil {
		if str, ok := body.(string); ok {
			bodyReader = strings.NewReader(str)
		} else {
			data, _ := json.Marshal(body)
			bodyReader = bytes.NewReader(data)
		}
	} else {
		bodyReader = strings.NewReader("")
	}

	response := &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bodyReader),
		Header:     make(http.Header),
	}
	response.Header.Set("Content-Type", "application/json")

	key := fmt.Sprintf("%s %s", method, path)
	m.responses[key] = response
}

func (m *mockHTTPClient) getLastRequest() *http.Request {
	if len(m.requests) == 0 {
		return nil
	}
	return m.requests[len(m.requests)-1]
}

func (m *mockHTTPClient) getRequestCount() int {
	return len(m.requests)
}

func createTestClient() (*Client, *mockHTTPClient) {
	mockTransport := newMockHTTPClient()
	httpClient := &http.Client{Transport: mockTransport}
	logger := logger.New(false) // Use false for non-verbose mode

	client := &Client{
		baseURL:  "https://test.atlassian.net/wiki",
		username: "test@example.com",
		apiToken: "test-token",
		client:   httpClient,
		logger:   logger,
	}

	return client, mockTransport
}

func TestNew(t *testing.T) {
	client := New("https://test.atlassian.net/wiki", "test@example.com", "test-token")

	if client.baseURL != "https://test.atlassian.net/wiki" {
		t.Errorf("Expected baseURL to be 'https://test.atlassian.net/wiki', got '%s'", client.baseURL)
	}

	if client.username != "test@example.com" {
		t.Errorf("Expected username to be 'test@example.com', got '%s'", client.username)
	}

	if client.apiToken != "test-token" {
		t.Errorf("Expected apiToken to be 'test-token', got '%s'", client.apiToken)
	}

	if client.client == nil {
		t.Error("Expected HTTP client to be initialized")
	}
}

func TestNewClient(t *testing.T) {
	logger := logger.New(false) // Use false for non-verbose mode
	client := NewClient("https://test.atlassian.net/wiki", "test@example.com", "test-token", logger)

	if client.logger != logger {
		t.Error("Expected logger to be set")
	}
}

func TestPageUpdateForbiddenError(t *testing.T) {
	err := &PageUpdateForbiddenError{
		PageID: "123456",
		Title:  "Test Page",
		Msg:    "Access denied",
	}

	if err.Error() != "Access denied" {
		t.Errorf("Expected error message 'Access denied', got '%s'", err.Error())
	}
}

func TestIsPageUpdateForbidden(t *testing.T) {
	// Test with PageUpdateForbiddenError
	forbiddenErr := &PageUpdateForbiddenError{
		PageID: "123456",
		Title:  "Test Page",
		Msg:    "Access denied",
	}

	if !IsPageUpdateForbidden(forbiddenErr) {
		t.Error("Expected IsPageUpdateForbidden to return true for PageUpdateForbiddenError")
	}

	// Test with regular error
	regularErr := fmt.Errorf("regular error")
	if IsPageUpdateForbidden(regularErr) {
		t.Error("Expected IsPageUpdateForbidden to return false for regular error")
	}
}

func TestCreatePage(t *testing.T) {
	client, mockTransport := createTestClient()

	expectedPage := Page{
		ID:    "123456",
		Title: "Test Page",
	}

	mockTransport.addResponse("POST", "/wiki/rest/api/content", http.StatusOK, expectedPage)

	page, err := client.CreatePage("TEST", "Test Page", "<p>Test content</p>")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if page.ID != "123456" {
		t.Errorf("Expected page ID '123456', got '%s'", page.ID)
	}

	if page.Title != "Test Page" {
		t.Errorf("Expected page title 'Test Page', got '%s'", page.Title)
	}

	// Verify request was made correctly
	lastReq := mockTransport.getLastRequest()
	if lastReq.Method != "POST" {
		t.Errorf("Expected POST request, got %s", lastReq.Method)
	}

	if !strings.Contains(lastReq.URL.Path, "/rest/api/content") {
		t.Errorf("Expected URL to contain '/rest/api/content', got '%s'", lastReq.URL.Path)
	}

	// Check basic auth
	username, password, ok := lastReq.BasicAuth()
	if !ok {
		t.Error("Expected basic auth to be set")
	}
	if username != "test@example.com" || password != "test-token" {
		t.Error("Expected correct basic auth credentials")
	}
}

func TestCreatePageWithParent(t *testing.T) {
	client, mockTransport := createTestClient()

	expectedPage := Page{
		ID:    "123456",
		Title: "Child Page",
	}

	mockTransport.addResponse("POST", "/wiki/rest/api/content", http.StatusOK, expectedPage)

	page, err := client.CreatePageWithParent("TEST", "Child Page", "<p>Child content</p>", "parent123")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if page.ID != "123456" {
		t.Errorf("Expected page ID '123456', got '%s'", page.ID)
	}

	if page.Title != "Child Page" {
		t.Errorf("Expected page title 'Child Page', got '%s'", page.Title)
	}
}

func TestCreatePageAPIError(t *testing.T) {
	client, mockTransport := createTestClient()

	mockTransport.addResponse("POST", "/wiki/rest/api/content", http.StatusBadRequest, "Bad request")

	page, err := client.CreatePage("TEST", "Test Page", "<p>Test content</p>")

	if err == nil {
		t.Fatal("Expected error for API failure")
	}

	if page != nil {
		t.Error("Expected nil page on error")
	}

	if !strings.Contains(err.Error(), "API request failed with status 400") {
		t.Errorf("Expected error message about status 400, got: %v", err)
	}
}

func TestUpdatePage(t *testing.T) {
	client, mockTransport := createTestClient()

	// Mock getting current page for version
	currentPage := Page{
		ID:    "123456",
		Title: "Test Page",
		Version: struct {
			Number int `json:"number"`
		}{Number: 1},
	}
	mockTransport.addResponse("GET", "/wiki/rest/api/content/123456", http.StatusOK, currentPage)

	// Mock update response
	updatedPage := Page{
		ID:    "123456",
		Title: "Updated Page",
		Version: struct {
			Number int `json:"number"`
		}{Number: 2},
	}
	mockTransport.addResponse("PUT", "/wiki/rest/api/content/123456", http.StatusOK, updatedPage)

	page, err := client.UpdatePage("123456", "Updated Page", "<p>Updated content</p>")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if page.Title != "Updated Page" {
		t.Errorf("Expected page title 'Updated Page', got '%s'", page.Title)
	}

	// Should have made 2 requests: GET for current version, PUT for update
	if mockTransport.getRequestCount() != 2 {
		t.Errorf("Expected 2 requests, got %d", mockTransport.getRequestCount())
	}
}

func TestUpdatePageForbidden(t *testing.T) {
	client, mockTransport := createTestClient()

	// Mock getting current page for version
	currentPage := Page{
		ID:    "123456",
		Title: "Test Page",
		Version: struct {
			Number int `json:"number"`
		}{Number: 1},
	}
	mockTransport.addResponse("GET", "/wiki/rest/api/content/123456", http.StatusOK, currentPage)

	// Mock forbidden response
	mockTransport.addResponse("PUT", "/wiki/rest/api/content/123456", http.StatusForbidden, "Page is archived")

	page, err := client.UpdatePage("123456", "Updated Page", "<p>Updated content</p>")

	if err == nil {
		t.Fatal("Expected forbidden error")
	}

	if page != nil {
		t.Error("Expected nil page on forbidden error")
	}

	// Verify it's the correct error type
	if !IsPageUpdateForbidden(err) {
		t.Error("Expected PageUpdateForbiddenError")
	}

	var forbiddenErr *PageUpdateForbiddenError
	if !errors.As(err, &forbiddenErr) {
		t.Error("Expected error to be PageUpdateForbiddenError")
	} else {
		if forbiddenErr.PageID != "123456" {
			t.Errorf("Expected PageID '123456', got '%s'", forbiddenErr.PageID)
		}
		if forbiddenErr.Title != "Updated Page" {
			t.Errorf("Expected Title 'Updated Page', got '%s'", forbiddenErr.Title)
		}
	}
}

func TestGetPage(t *testing.T) {
	client, mockTransport := createTestClient()

	expectedPage := Page{
		ID:    "123456",
		Title: "Test Page",
		Version: struct {
			Number int `json:"number"`
		}{Number: 1},
	}

	mockTransport.addResponse("GET", "/wiki/rest/api/content/123456", http.StatusOK, expectedPage)

	page, err := client.GetPage("123456")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if page.ID != "123456" {
		t.Errorf("Expected page ID '123456', got '%s'", page.ID)
	}

	if page.Title != "Test Page" {
		t.Errorf("Expected page title 'Test Page', got '%s'", page.Title)
	}

	if page.Version.Number != 1 {
		t.Errorf("Expected version number 1, got %d", page.Version.Number)
	}
}

func TestFindPageByTitle(t *testing.T) {
	client, mockTransport := createTestClient()

	searchResult := struct {
		Results []Page `json:"results"`
	}{
		Results: []Page{
			{
				ID:    "123456",
				Title: "Test Page",
			},
		},
	}

	// The FindPageByTitle method includes query parameters, so we need to match the full path with them
	mockTransport.addResponse("GET", "/wiki/rest/api/content", http.StatusOK, searchResult)

	page, err := client.FindPageByTitle("TEST", "Test Page")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if page.ID != "123456" {
		t.Errorf("Expected page ID '123456', got '%s'", page.ID)
	}

	if page.Title != "Test Page" {
		t.Errorf("Expected page title 'Test Page', got '%s'", page.Title)
	}
}

func TestFindPageByTitleNotFound(t *testing.T) {
	client, mockTransport := createTestClient()

	searchResult := struct {
		Results []Page `json:"results"`
	}{
		Results: []Page{},
	}

	mockTransport.addResponse("GET", "/wiki/rest/api/content", http.StatusOK, searchResult)

	page, err := client.FindPageByTitle("TEST", "Nonexistent Page")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if page != nil {
		t.Error("Expected nil page when not found")
	}
}

func TestContainsChildrenMacro(t *testing.T) {
	testCases := []struct {
		content  string
		expected bool
	}{
		{"<ac:structured-macro ac:name=\"children\">", true},
		{"<ac:structured-macro ac:name='children'>", true},
		{"Some content with children macro", true},
		{"Regular content without macro", false},
		{"", false},
	}

	for _, tc := range testCases {
		result := containsChildrenMacro(tc.content)
		if result != tc.expected {
			t.Errorf("containsChildrenMacro(%q) = %v, expected %v", tc.content, result, tc.expected)
		}
	}
}

func TestContainsDirectoryKeywords(t *testing.T) {
	testCases := []struct {
		content  string
		expected bool
	}{
		{"This is a directory page", true},
		{"Automatically created to organize", true},
		{"AUTOMATICALLY CREATED", true},
		{"Documentation for the API", true},
		{"Directory structure", true},
		{"Let's organize this", true},
		{"Regular page content", false},
		{"", false},
	}

	for _, tc := range testCases {
		result := containsDirectoryKeywords(tc.content)
		if result != tc.expected {
			t.Errorf("containsDirectoryKeywords(%q) = %v, expected %v", tc.content, result, tc.expected)
		}
	}
}

// Test error handling and edge cases

func TestCreatePageMarshalError(t *testing.T) {
	// This test is harder to simulate since json.Marshal rarely fails
	// with simple data structures, but we test the path exists
	client, _ := createTestClient()

	// Test with extremely long title that might cause issues
	longTitle := strings.Repeat("a", 1000000) // Very long title

	page, err := client.CreatePage("TEST", longTitle, "<p>Test</p>")

	// The request might succeed or fail depending on server limits
	// We just ensure no panic occurs
	if err != nil && page != nil {
		t.Error("If error occurs, page should be nil")
	}
}

func TestUpdatePageGetCurrentPageError(t *testing.T) {
	client, mockTransport := createTestClient()

	// Mock error getting current page
	mockTransport.addResponse("GET", "/wiki/rest/api/content/123456", http.StatusNotFound, "Page not found")

	page, err := client.UpdatePage("123456", "Updated Page", "<p>Updated content</p>")

	if err == nil {
		t.Fatal("Expected error when getting current page fails")
	}

	if page != nil {
		t.Error("Expected nil page on error")
	}

	if !strings.Contains(err.Error(), "failed to get current page version") {
		t.Errorf("Expected error about getting current page version, got: %v", err)
	}
}

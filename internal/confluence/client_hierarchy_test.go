package confluence

import (
	"net/http"
	"testing"
)

func TestGetPageHierarchyWithParent(t *testing.T) {
	client, mockTransport := createTestClient()

	// Mock finding the parent page
	parentPage := Page{
		ID:    "123",
		Title: "Parent Page",
	}
	mockTransport.addResponse("GET", "/wiki/rest/api/content?expand=body.storage&spaceKey=TEST&title=Parent+Page", http.StatusOK, struct {
		Results []Page `json:"results"`
	}{
		Results: []Page{parentPage},
	})

	// Mock getting child pages
	childPages := []PageInfo{
		{ID: "456", Title: "Child 1"},
		{ID: "789", Title: "Child 2"},
	}
	mockTransport.addResponse("GET", "/wiki/rest/api/content/123/child/page?expand=children.page", http.StatusOK, struct {
		Results []PageInfo `json:"results"`
	}{
		Results: childPages,
	})
	mockTransport.addResponse("GET", "/wiki/rest/api/content/456/child/page?expand=children.page", http.StatusOK, struct {
		Results []PageInfo `json:"results"`
	}{
		Results: []PageInfo{},
	})
	mockTransport.addResponse("GET", "/wiki/rest/api/content/789/child/page?expand=children.page", http.StatusOK, struct {
		Results []PageInfo `json:"results"`
	}{
		Results: []PageInfo{},
	})

	pages, err := client.GetPageHierarchy("TEST", "Parent Page")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(pages) != 2 {
		t.Fatalf("Expected 2 pages, got %d", len(pages))
	}

	if pages[0].Title != "Child 1" {
		t.Errorf("Expected first page to be 'Child 1', got '%s'", pages[0].Title)
	}
}

func TestGetPageHierarchyNoParent(t *testing.T) {
	client, mockTransport := createTestClient()

	// Mock getting all pages in space
	allPagesResponse := struct {
		Results []struct {
			ID        string `json:"id"`
			Title     string `json:"title"`
			Ancestors []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"ancestors"`
		} `json:"results"`
	}{
		Results: []struct {
			ID        string `json:"id"`
			Title     string `json:"title"`
			Ancestors []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"ancestors"`
		}{
			{ID: "1", Title: "Root 1", Ancestors: []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			}{}},
			{ID: "2", Title: "Child 1.1", Ancestors: []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			}{{ID: "1", Title: "Root 1"}}},
			{ID: "3", Title: "Root 2", Ancestors: []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			}{}},
		},
	}
	mockTransport.addResponse("GET", "/wiki/rest/api/content?expand=ancestors&limit=1000&spaceKey=TEST&type=page", http.StatusOK, allPagesResponse)

	pages, err := client.GetPageHierarchy("TEST", "")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(pages) != 2 {
		t.Fatalf("Expected 2 root pages, got %d", len(pages))
	}

	if pages[0].Title != "Root 1" && pages[1].Title != "Root 1" {
		t.Errorf("Expected to find 'Root 1'")
	}

	if pages[0].Title != "Root 2" && pages[1].Title != "Root 2" {
		t.Errorf("Expected to find 'Root 2'")
	}
}

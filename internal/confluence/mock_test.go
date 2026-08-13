package confluence

import "testing"

func TestMockClientUpdatePageReturnsErrorForUnknownPage(t *testing.T) {
	mock := NewMockClient()
	page, err := mock.UpdatePage("missing", "Page", "body")
	if err == nil || page != nil {
		t.Fatalf("page=%v error=%v, want not-found error", page, err)
	}
}

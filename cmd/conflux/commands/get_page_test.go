package commands

import (
	"strings"
	"testing"

	"conflux/internal/confluence"
)

// helper to build a Page quickly
func testPage(storageHTML, viewHTML string) *confluence.Page {
	p := &confluence.Page{}
	p.Body.Storage.Value = storageHTML
	p.Body.View.Value = viewHTML
	return p
}

func TestGeneratePageOutput_Storage(t *testing.T) {
	p := testPage("<p>Storage Content</p>", "")
	out, err := generatePageOutput(p, "storage")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "<p>Storage Content</p>" {
		pfx := out
		if len(pfx) > 50 {
			pfx = pfx[:50]
		}
		t.Fatalf("expected storage html, got: %q", pfx)
	}
}

func TestGeneratePageOutput_HTMLPrefersView(t *testing.T) {
	p := testPage("<p>Storage</p>", "<h1>View</h1>")
	out, err := generatePageOutput(p, "html")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "<h1>View</h1>" {
		t.Fatalf("expected view html, got %q", out)
	}
}

func TestGeneratePageOutput_HTMLFallsBackToStorage(t *testing.T) {
	p := testPage("<div>Only Storage</div>", "")
	out, err := generatePageOutput(p, "html")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "<div>Only Storage</div>" {
		t.Fatalf("expected storage html fallback, got %q", out)
	}
}

func TestGeneratePageOutput_MarkdownUsesViewWhenAvailable(t *testing.T) {
	p := testPage("<p>Storage</p>", "<h2>Title</h2><p>Body</p>")
	out, err := generatePageOutput(p, "markdown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Title") || !strings.Contains(out, "Body") {
		t.Fatalf("expected converted markdown containing Title and Body, got: %q", out)
	}
}

func TestGeneratePageOutput_MarkdownFallsBackToStorageIfNoView(t *testing.T) {
	p := testPage("<p>Only Storage</p>", "")
	out, err := generatePageOutput(p, "markdown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Only Storage") {
		t.Fatalf("expected markdown containing storage content, got %q", out)
	}
}

func TestGeneratePageOutput_Unsupported(t *testing.T) {
	p := testPage("<p>X</p>", "")
	_, err := generatePageOutput(p, "unknown")
	if err == nil {
		t.Fatalf("expected error for unsupported format")
	}
}

func TestIsNumeric(t *testing.T) {
	cases := map[string]bool{
		"123": true,
		"000": true,
		"12a": false,
		"":    false,
		"-10": true, // negative still parses as int; treated numeric
	}
	for in, expected := range cases {
		got := isNumeric(in)
		if got != expected {
			t.Fatalf("isNumeric(%q)=%v expected %v", in, got, expected)
		}
	}
}

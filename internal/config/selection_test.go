package config

import "testing"

func TestSelectProject(t *testing.T) {
	cfg := &Config{
		Confluence: ConfluenceConfig{BaseURL: "https://example", Username: "u", APIToken: "t"},
		Projects: []ProjectConfig{
			{Name: "alpha", SpaceKey: "ALPHA", Local: LocalConfig{MarkdownDir: "./alpha"}},
			{Name: "beta", SpaceKey: "BETA", Local: LocalConfig{MarkdownDir: "./beta", Exclude: []string{"README.md"}}},
		},
	}

	if err := cfg.SelectProject("beta"); err != nil {
		t.Fatalf("expected to select beta: %v", err)
	}
	if cfg.Confluence.SpaceKey != "BETA" {
		t.Fatalf("expected space key BETA got %s", cfg.Confluence.SpaceKey)
	}
	if cfg.Local.MarkdownDir != "./beta" {
		t.Fatalf("expected local markdown ./beta got %s", cfg.Local.MarkdownDir)
	}
	if len(cfg.Local.Exclude) != 1 || cfg.Local.Exclude[0] != "README.md" {
		t.Fatalf("expected exclude preserved, got %#v", cfg.Local.Exclude)
	}

	if err := cfg.SelectProject("missing"); err == nil {
		t.Fatalf("expected error selecting missing project")
	}
}

func TestApplyDefaultProject(t *testing.T) {
	cfg := &Config{
		Confluence: ConfluenceConfig{BaseURL: "https://example", Username: "u", APIToken: "t"},
		Projects: []ProjectConfig{
			{Name: "alpha", SpaceKey: "ALPHA", Local: LocalConfig{MarkdownDir: "./alpha"}},
			{Name: "beta", SpaceKey: "BETA", Local: LocalConfig{MarkdownDir: "./beta"}},
		},
	}
	applied := cfg.ApplyDefaultProject()
	if !applied {
		t.Fatalf("expected default project applied")
	}
	if cfg.Confluence.SpaceKey != "ALPHA" || cfg.Local.MarkdownDir != "./alpha" {
		t.Fatalf("expected first project fields applied; got space=%s dir=%s", cfg.Confluence.SpaceKey, cfg.Local.MarkdownDir)
	}

	// no projects case
	empty := &Config{Confluence: ConfluenceConfig{BaseURL: "https://e", Username: "u", APIToken: "t"}}
	if empty.ApplyDefaultProject() {
		t.Fatalf("expected false when no projects present")
	}
}

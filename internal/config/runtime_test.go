package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRuntimePrecedence(t *testing.T) {
	t.Setenv("CONFLUX_SPACE_KEY", "ENV")
	source := &Config{
		Confluence: ConfluenceConfig{BaseURL: "https://example", Username: "user", APIToken: "token", SpaceKey: "CONFIG"},
		Projects: []ProjectConfig{
			{Name: "alpha", SpaceKey: "ALPHA"},
			{Name: "beta", SpaceKey: "BETA"},
		},
	}
	tests := []struct {
		name, space, profile, wantSpace, wantProfile, wantSource string
	}{
		{name: "space flag", space: "FLAG", profile: "beta", wantSpace: "FLAG", wantProfile: "beta", wantSource: "--space"},
		{name: "profile", profile: "beta", wantSpace: "BETA", wantProfile: "beta", wantSource: "--project"},
		{name: "environment", wantSpace: "ENV", wantSource: "CONFLUX_SPACE_KEY"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runtime, err := ResolveRuntime(source, RuntimeOptions{SpaceKey: test.space, Profile: test.profile})
			if err != nil {
				t.Fatal(err)
			}
			if runtime.Confluence.SpaceKey != test.wantSpace || runtime.Profile != test.wantProfile || runtime.SpaceSource != test.wantSource {
				t.Fatalf("runtime=%#v", runtime)
			}
		})
	}
}

func TestResolveRuntimeConfiguredAndDefaultProfileFallbacks(t *testing.T) {
	t.Setenv("CONFLUX_SPACE_KEY", "")
	configured := &Config{Confluence: ConfluenceConfig{SpaceKey: "CONFIG"}, Projects: []ProjectConfig{{Name: "alpha", SpaceKey: "ALPHA"}}}
	runtime, err := ResolveRuntime(configured, RuntimeOptions{})
	if err != nil || runtime.Confluence.SpaceKey != "CONFIG" || runtime.SpaceSource != "confluence.space_key" {
		t.Fatalf("configured fallback=%#v error=%v", runtime, err)
	}

	profilesOnly := &Config{Projects: []ProjectConfig{{Name: "alpha", SpaceKey: "ALPHA"}}}
	runtime, err = ResolveRuntime(profilesOnly, RuntimeOptions{})
	if err != nil || runtime.Confluence.SpaceKey != "ALPHA" || runtime.Profile != "alpha" {
		t.Fatalf("profile fallback=%#v error=%v", runtime, err)
	}

	if _, err := ResolveRuntime(&Config{}, RuntimeOptions{}); err == nil {
		t.Fatal("missing space unexpectedly resolved")
	}
	if _, err := ResolveRuntime(nil, RuntimeOptions{}); err == nil {
		t.Fatal("nil config unexpectedly resolved")
	}
}

func TestResolveRuntimeDoesNotMutateSource(t *testing.T) {
	t.Setenv("CONFLUX_SPACE_KEY", "")
	source := &Config{
		Confluence: ConfluenceConfig{SpaceKey: "CONFIG"},
	}
	runtime, err := ResolveRuntime(source, RuntimeOptions{SpaceKey: "FLAG"})
	if err != nil {
		t.Fatal(err)
	}
	runtime.Confluence.SpaceKey = "changed"
	if source.Confluence.SpaceKey != "CONFIG" {
		t.Fatalf("source was mutated: %#v", source)
	}
}

func TestLoadRuntimeAppliesCredentialEnvironment(t *testing.T) {
	t.Setenv("CONFLUX_BASE_URL", "https://env.example")
	t.Setenv("CONFLUX_USERNAME", "env-user")
	t.Setenv("CONFLUX_API_TOKEN", "env-token")
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("confluence: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadRuntime(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Confluence.BaseURL != "https://env.example" || cfg.Confluence.Username != "env-user" || cfg.Confluence.APIToken != "env-token" {
		t.Fatalf("environment was not applied: %#v", cfg.Confluence)
	}
}

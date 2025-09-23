package version

import (
	"runtime"
	"strings"
	"testing"
)

func TestGet(t *testing.T) {
	buildInfo := Get()

	// Test that all fields are populated
	if buildInfo.Version == "" {
		t.Error("Expected Version to be populated")
	}

	// GitCommit and BuildDate may be empty in dev builds, so we just check they're strings
	if buildInfo.GoVersion == "" {
		t.Error("Expected GoVersion to be populated")
	}

	if buildInfo.Platform == "" {
		t.Error("Expected Platform to be populated")
	}

	// Verify GoVersion starts with "go" (e.g., "go1.21.0")
	if !strings.HasPrefix(buildInfo.GoVersion, "go") {
		t.Errorf("Expected GoVersion to start with 'go', got: %s", buildInfo.GoVersion)
	}

	// Verify Platform format (e.g., "linux/amd64", "darwin/arm64")
	if !strings.Contains(buildInfo.Platform, "/") {
		t.Errorf("Expected Platform to contain '/', got: %s", buildInfo.Platform)
	}

	// Verify Platform matches runtime values
	expectedPlatform := runtime.GOOS + "/" + runtime.GOARCH
	if buildInfo.Platform != expectedPlatform {
		t.Errorf("Expected Platform '%s', got '%s'", expectedPlatform, buildInfo.Platform)
	}

	// Verify GoVersion matches runtime
	if buildInfo.GoVersion != runtime.Version() {
		t.Errorf("Expected GoVersion '%s', got '%s'", runtime.Version(), buildInfo.GoVersion)
	}
}

func TestGetConsistency(t *testing.T) {
	// Calling Get() multiple times should return same values
	buildInfo1 := Get()
	buildInfo2 := Get()

	if buildInfo1.Version != buildInfo2.Version {
		t.Error("Expected consistent Version across calls")
	}

	if buildInfo1.GitCommit != buildInfo2.GitCommit {
		t.Error("Expected consistent GitCommit across calls")
	}

	if buildInfo1.BuildDate != buildInfo2.BuildDate {
		t.Error("Expected consistent BuildDate across calls")
	}

	if buildInfo1.GoVersion != buildInfo2.GoVersion {
		t.Error("Expected consistent GoVersion across calls")
	}

	if buildInfo1.Platform != buildInfo2.Platform {
		t.Error("Expected consistent Platform across calls")
	}
}

func TestBuildInfoString(t *testing.T) {
	tests := []struct {
		name      string
		buildInfo BuildInfo
		expected  []string // strings that should be present in output
	}{
		{
			name: "complete build info",
			buildInfo: BuildInfo{
				Version:   "1.0.0",
				GitCommit: "abc123",
				BuildDate: "2023-01-01",
				GoVersion: "go1.21.0",
				Platform:  "linux/amd64",
			},
			expected: []string{
				"conflux version 1.0.0",
				"(abc123)",
				"built on 2023-01-01",
				"go1.21.0",
				"linux/amd64",
			},
		},
		{
			name: "minimal build info (dev)",
			buildInfo: BuildInfo{
				Version:   "dev",
				GitCommit: "",
				BuildDate: "",
				GoVersion: "go1.21.0",
				Platform:  "darwin/arm64",
			},
			expected: []string{
				"conflux version dev",
				"go1.21.0",
				"darwin/arm64",
			},
		},
		{
			name: "with commit but no build date",
			buildInfo: BuildInfo{
				Version:   "v0.1.0",
				GitCommit: "def456",
				BuildDate: "",
				GoVersion: "go1.20.5",
				Platform:  "windows/amd64",
			},
			expected: []string{
				"conflux version v0.1.0",
				"(def456)",
				"go1.20.5",
				"windows/amd64",
			},
		},
		{
			name: "with build date but no commit",
			buildInfo: BuildInfo{
				Version:   "v2.0.0",
				GitCommit: "",
				BuildDate: "2023-12-25",
				GoVersion: "go1.21.5",
				Platform:  "linux/arm64",
			},
			expected: []string{
				"conflux version v2.0.0",
				"built on 2023-12-25",
				"go1.21.5",
				"linux/arm64",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.buildInfo.String()

			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected result to contain '%s', got: %s", expected, result)
				}
			}

			// Verify it starts with "conflux version"
			if !strings.HasPrefix(result, "conflux version") {
				t.Errorf("Expected result to start with 'conflux version', got: %s", result)
			}
		})
	}
}

func TestBuildInfoStringFormat(t *testing.T) {
	buildInfo := BuildInfo{
		Version:   "1.2.3",
		GitCommit: "abcd1234",
		BuildDate: "2023-06-15",
		GoVersion: "go1.20.0",
		Platform:  "linux/amd64",
	}

	result := buildInfo.String()

	// Test the exact format
	expected := "conflux version 1.2.3 (abcd1234) built on 2023-06-15 go1.20.0 linux/amd64"
	if result != expected {
		t.Errorf("Expected exact format:\n%s\nGot:\n%s", expected, result)
	}
}

func TestBuildInfoStringNoOptionalFields(t *testing.T) {
	buildInfo := BuildInfo{
		Version:   "dev",
		GitCommit: "",
		BuildDate: "",
		GoVersion: "go1.21.0",
		Platform:  "darwin/amd64",
	}

	result := buildInfo.String()

	// Should not contain commit or build date info
	if strings.Contains(result, "(") || strings.Contains(result, ")") {
		t.Errorf("Expected no commit info when GitCommit is empty, got: %s", result)
	}

	if strings.Contains(result, "built on") {
		t.Errorf("Expected no build date info when BuildDate is empty, got: %s", result)
	}

	// Should still contain version, go version, and platform
	expected := "conflux version dev go1.21.0 darwin/amd64"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestDefaultValues(t *testing.T) {
	// Test that the package-level variables have sensible defaults
	if Version == "" {
		t.Error("Expected Version to have a default value")
	}

	// GitCommit and BuildDate can be empty strings by default
	// This is fine for development builds

	// Test that the default Version is "dev" (as set in the package)
	if Version != "dev" {
		t.Logf("Note: Version is '%s', expected 'dev' for development builds", Version)
	}
}

func TestBuildInfoFields(t *testing.T) {
	buildInfo := BuildInfo{
		Version:   "test-version",
		GitCommit: "test-commit",
		BuildDate: "test-date",
		GoVersion: "test-go-version",
		Platform:  "test-platform",
	}

	// Verify all fields are properly set
	if buildInfo.Version != "test-version" {
		t.Errorf("Expected Version 'test-version', got '%s'", buildInfo.Version)
	}

	if buildInfo.GitCommit != "test-commit" {
		t.Errorf("Expected GitCommit 'test-commit', got '%s'", buildInfo.GitCommit)
	}

	if buildInfo.BuildDate != "test-date" {
		t.Errorf("Expected BuildDate 'test-date', got '%s'", buildInfo.BuildDate)
	}

	if buildInfo.GoVersion != "test-go-version" {
		t.Errorf("Expected GoVersion 'test-go-version', got '%s'", buildInfo.GoVersion)
	}

	if buildInfo.Platform != "test-platform" {
		t.Errorf("Expected Platform 'test-platform', got '%s'", buildInfo.Platform)
	}
}

func TestEmptyBuildInfo(t *testing.T) {
	buildInfo := BuildInfo{}
	result := buildInfo.String()

	// Should handle empty values gracefully
	if !strings.Contains(result, "conflux version") {
		t.Errorf("Expected to contain 'conflux version' even with empty values, got: %s", result)
	}

	// Should not contain parentheses or "built on" with empty values
	if strings.Contains(result, "(") || strings.Contains(result, ")") {
		t.Errorf("Expected no parentheses with empty GitCommit, got: %s", result)
	}

	if strings.Contains(result, "built on") {
		t.Errorf("Expected no 'built on' with empty BuildDate, got: %s", result)
	}
}

// Benchmark tests
func BenchmarkGet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Get()
	}
}

func BenchmarkBuildInfoString(b *testing.B) {
	buildInfo := BuildInfo{
		Version:   "1.0.0",
		GitCommit: "abc123",
		BuildDate: "2023-01-01",
		GoVersion: "go1.21.0",
		Platform:  "linux/amd64",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buildInfo.String()
	}
}

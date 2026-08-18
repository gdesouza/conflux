package config

import (
	"fmt"
	"os"
	"strings"
)

type RuntimeOptions struct {
	SpaceKey string
	Profile  string
}

// RuntimeConfig is a command-ready value detached from the parsed Config.
// Resolving it never mutates the source configuration.
type RuntimeConfig struct {
	Confluence  ConfluenceConfig
	Profile     string
	SpaceSource string
}

func ResolveRuntime(source *Config, options RuntimeOptions) (RuntimeConfig, error) {
	if source == nil {
		return RuntimeConfig{}, fmt.Errorf("config is required")
	}
	runtime := RuntimeConfig{
		Confluence: source.Confluence,
	}

	spaceFlag := strings.TrimSpace(options.SpaceKey)
	profileName := strings.TrimSpace(options.Profile)
	if spaceFlag != "" {
		runtime.Confluence.SpaceKey = spaceFlag
		runtime.SpaceSource = "--space"
		if profileName != "" {
			profile, err := findProject(source.Projects, profileName)
			if err != nil {
				return RuntimeConfig{}, err
			}
			runtime.Profile = profile.Name
		}
		return runtime, nil
	}
	if profileName != "" {
		profile, err := findProject(source.Projects, profileName)
		if err != nil {
			return RuntimeConfig{}, err
		}
		runtime.Profile = profile.Name
		runtime.Confluence.SpaceKey = profile.SpaceKey
		runtime.SpaceSource = "--project"
		return runtime, nil
	}
	if environmentSpace := strings.TrimSpace(os.Getenv("CONFLUX_SPACE_KEY")); environmentSpace != "" {
		runtime.Confluence.SpaceKey = environmentSpace
		runtime.SpaceSource = "CONFLUX_SPACE_KEY"
		return runtime, nil
	}
	if strings.TrimSpace(source.Confluence.SpaceKey) != "" {
		runtime.SpaceSource = "confluence.space_key"
		return runtime, nil
	}
	if len(source.Projects) > 0 {
		profile := source.Projects[0]
		runtime.Profile = profile.Name
		runtime.Confluence.SpaceKey = profile.SpaceKey
		runtime.SpaceSource = "default project " + profile.Name
		return runtime, nil
	}
	return RuntimeConfig{}, fmt.Errorf("space key is required: use --space, --project, CONFLUX_SPACE_KEY, or configure confluence.space_key")
}

func findProject(projects []ProjectConfig, name string) (ProjectConfig, error) {
	for _, project := range projects {
		if project.Name == name {
			return project, nil
		}
	}
	return ProjectConfig{}, fmt.Errorf("project %q not found", name)
}

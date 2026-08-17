package commands

import (
	"fmt"

	"conflux/internal/config"
)

func resolveRuntimeConfig(spaceKey, profile string) (config.RuntimeConfig, error) {
	cfg, err := config.LoadRuntime(configFile)
	if err != nil {
		return config.RuntimeConfig{}, fmt.Errorf("failed to load config: %w", err)
	}
	runtime, err := config.ResolveRuntime(cfg, config.RuntimeOptions{SpaceKey: spaceKey, Profile: profile})
	if err != nil {
		return config.RuntimeConfig{}, fmt.Errorf("failed to resolve runtime config: %w", err)
	}
	return runtime, nil
}

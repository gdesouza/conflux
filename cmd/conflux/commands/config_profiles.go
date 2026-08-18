package commands

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"conflux/internal/config"
)

func runConfigProfiles(cmd *cobra.Command, _ []string) error {
	cfg, err := config.LoadRuntime(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if len(cfg.Projects) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No profiles configured.")
		return nil
	}
	profiles := append([]config.ProjectConfig(nil), cfg.Projects...)
	sort.SliceStable(profiles, func(i, j int) bool { return profiles[i].Name < profiles[j].Name })
	fmt.Fprintln(cmd.OutOrStdout(), "Configured Profiles:")
	fmt.Fprintln(cmd.OutOrStdout())
	for _, profile := range profiles {
		defaultMarker := ""
		if cfg.Projects[0].Name == profile.Name {
			defaultMarker = " (default)"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "- %s%s\n  space: %s\n", profile.Name, defaultMarker, profile.SpaceKey)
	}
	return nil
}

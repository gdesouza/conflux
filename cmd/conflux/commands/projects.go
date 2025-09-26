package commands

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"conflux/internal/config"
)

var (
	projectsShowRaw bool
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List configured documentation projects",
	Long: `List projects defined in the configuration file. Shows the project name,
associated Confluence space key, and markdown directory. The first project is the
implicit default when none is specified with --project in other commands.`,
	RunE: runProjects,
}

func runProjects(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Projects) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No projects defined (using legacy single-project configuration).")
		return nil
	}

	// Stable order by name except keep original index for default indicator (index 0)
	projects := make([]config.ProjectConfig, len(cfg.Projects))
	copy(projects, cfg.Projects)
	sort.SliceStable(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })

	fmt.Fprintln(cmd.OutOrStdout(), "Configured Projects:")
	fmt.Fprintln(cmd.OutOrStdout())
	for _, p := range projects {
		defaultMarker := ""
		if cfg.Projects[0].Name == p.Name { // original first remains default
			defaultMarker = " (default)"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "- %s%s\n  space: %s\n  docs:  %s\n", p.Name, defaultMarker, p.SpaceKey, p.Local.MarkdownDir)
		if len(p.Local.Exclude) > 0 && projectsShowRaw {
			fmt.Fprintf(cmd.OutOrStdout(), "  exclude: %v\n", p.Local.Exclude)
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(projectsCmd)
	projectsCmd.Flags().BoolVar(&projectsShowRaw, "show-exclude", false, "Show exclude patterns for each project")
}

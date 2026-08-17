package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"conflux/internal/config"
)

var (
	cfgSets           []string
	cfgAddProjects    []string
	cfgRemoveProjects []string
	cfgYes            bool
	cfgPrint          bool
	cfgNonInteractive bool
)

var cfgCmd = &cobra.Command{
	Use:   "config",
	Short: "Create or edit the configuration file interactively or via flags",
	Long: `Interactively create or edit the Conflux configuration file (config.yaml by default).

Features:
- Interactive prompts for Confluence and named space profiles
- Apply key=value overrides via --set
- Add profiles via --add-project (e.g. --add-project "name=docs,space_key=DOCS")
- Remove profiles via --remove-project <name>
- Non-interactive scripting with --non-interactive --yes --set ...
- Print resulting YAML with --print instead of writing
`,
	RunE: runConfig,
}

var cfgProfilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "List configured profiles and their effective defaults",
	RunE:  runConfigProfiles,
}

func init() {
	rootCmd.AddCommand(cfgCmd)
	cfgCmd.AddCommand(cfgProfilesCmd)
	cfgCmd.Flags().StringArrayVar(&cfgSets, "set", nil, "Set a config field using dotted path (e.g. confluence.base_url=http://example)")
	cfgCmd.Flags().StringArrayVar(&cfgAddProjects, "add-project", nil, "Add a named space profile (e.g. \"name=docs,space_key=DOCS\")")
	cfgCmd.Flags().StringArrayVar(&cfgRemoveProjects, "remove-project", nil, "Remove an existing profile by name (repeatable)")
	cfgCmd.Flags().BoolVar(&cfgYes, "yes", false, "Automatically confirm saving changes")
	cfgCmd.Flags().BoolVar(&cfgPrint, "print", false, "Print resulting YAML instead of writing to file")
	cfgCmd.Flags().BoolVar(&cfgNonInteractive, "non-interactive", false, "Disable interactive prompts (use with --set / --add-project)")
}

func runConfig(cmd *cobra.Command, args []string) error {
	path := configFile
	cfg, existed, err := loadOrInitConfig(path)
	if err != nil {
		return err
	}

	// Apply flag mutations first (non-interactive layer)
	if err := applySetOperations(cfg, cfgSets); err != nil {
		return err
	}
	if err := applyAddProjects(cfg, cfgAddProjects); err != nil {
		return err
	}
	if err := applyRemoveProjects(cfg, cfgRemoveProjects); err != nil {
		return err
	}

	interactive := !cfgNonInteractive && len(args) == 0
	if interactive {
		if err := interactiveEdit(cfg, existed); err != nil {
			return err
		}
	}

	// Validate final config by round-trip through loader
	if err := validateConfig(cfg); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	outYAML, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	if cfgPrint {
		cmd.Print(string(outYAML))
		return nil
	}

	if !cfgYes && interactive {
		confirm := false
		prompt := &survey.Confirm{Message: "Save configuration to " + path + "?", Default: true}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}
		if !confirm {
			fmt.Println("Aborted (no changes saved).")
			return nil
		}
	}

	if err := writeConfigFile(path, outYAML); err != nil {
		return err
	}
	cmd.Printf("Configuration saved to %s\n", path)
	return nil
}

func loadOrInitConfig(path string) (*config.Config, bool, error) {
	resolved := config.ResolveConfigPath(path)
	if fileExists(resolved) {
		cfg, err := config.Load(resolved)
		if err != nil {
			return nil, true, err
		}
		return cfg, true, nil
	}
	return &config.Config{}, false, nil
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func writeConfigFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func applySetOperations(cfg *config.Config, sets []string) error {
	for _, s := range sets {
		parts := strings.SplitN(s, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid --set value '%s' (expected key=value)", s)
		}
		key := parts[0]
		val := parts[1]
		if err := setField(cfg, key, val); err != nil {
			return fmt.Errorf("set %s: %w", key, err)
		}
	}
	return nil
}

func setField(cfg *config.Config, key, value string) error {
	switch key {
	case "confluence.base_url":
		cfg.Confluence.BaseURL = value
	case "confluence.username":
		cfg.Confluence.Username = value
	case "confluence.api_token":
		cfg.Confluence.APIToken = value
	case "confluence.space_key":
		cfg.Confluence.SpaceKey = value
	default:
		return fmt.Errorf("unsupported key '%s'", key)
	}
	return nil
}

func applyAddProjects(cfg *config.Config, defs []string) error {
	for _, d := range defs {
		pConf, err := parseProjectDefinition(d)
		if err != nil {
			return fmt.Errorf("add-project: %w", err)
		}
		replaced := false
		for i, ex := range cfg.Projects {
			if ex.Name == pConf.Name {
				cfg.Projects[i] = pConf
				replaced = true
				break
			}
		}
		if !replaced {
			cfg.Projects = append(cfg.Projects, pConf)
		}
	}
	return nil
}

func parseProjectDefinition(d string) (config.ProjectConfig, error) {
	parts := strings.Split(d, ",")
	var p config.ProjectConfig
	for _, kv := range parts {
		pair := strings.SplitN(kv, "=", 2)
		if len(pair) != 2 {
			return p, fmt.Errorf("invalid key=value pair: %s", kv)
		}
		switch pair[0] {
		case "name":
			p.Name = pair[1]
		case "space_key":
			p.SpaceKey = pair[1]
		default:
			return p, fmt.Errorf("unknown project field: %s", pair[0])
		}
	}
	if p.Name == "" || p.SpaceKey == "" {
		return p, errors.New("project requires name and space_key")
	}
	return p, nil
}

func applyRemoveProjects(cfg *config.Config, names []string) error {
	for _, name := range names {
		found := false
		for i, p := range cfg.Projects {
			if p.Name == name {
				cfg.Projects = append(cfg.Projects[:i], cfg.Projects[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("remove-project: project '%s' not found", name)
		}
	}
	return nil
}

func validateConfig(c *config.Config) error {
	// Marshal and re-load using existing validation logic
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp("", "conflux-validate-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if _, err := config.Load(tmp.Name()); err != nil {
		return err
	}
	return nil
}

// Interactive editing -------------------------------------------------------

func interactiveEdit(cfg *config.Config, existed bool) error {
	fmt.Println("Interactive configuration editor. Press Enter to accept defaults.")
	if existed {
		fmt.Println("Loaded existing configuration. You can modify sections.")
	}

	// Confluence section
	if err := promptConfluence(cfg); err != nil {
		return err
	}

	// Projects section (optional)
	if err := promptProjects(cfg); err != nil {
		return err
	}

	return nil
}

func promptConfluence(cfg *config.Config) error {
	qs := []*survey.Question{
		{Name: "base_url", Prompt: &survey.Input{Message: "Confluence Base URL", Default: cfg.Confluence.BaseURL}},
		{Name: "username", Prompt: &survey.Input{Message: "Confluence Username", Default: cfg.Confluence.Username}},
		{Name: "api_token", Prompt: &survey.Password{Message: "Confluence API Token (leave blank to keep)"}},
		{Name: "space_key", Prompt: &survey.Input{Message: "Default Space Key (leave blank if using projects)", Default: cfg.Confluence.SpaceKey}},
	}
	answers := struct {
		BaseURL  string `survey:"base_url"`
		Username string `survey:"username"`
		APIToken string `survey:"api_token"`
		SpaceKey string `survey:"space_key"`
	}{}
	if err := survey.Ask(qs, &answers); err != nil {
		return err
	}
	cfg.Confluence.BaseURL = answers.BaseURL
	cfg.Confluence.Username = answers.Username
	if answers.APIToken != "" { // keep existing if blank
		cfg.Confluence.APIToken = answers.APIToken
	}
	cfg.Confluence.SpaceKey = answers.SpaceKey
	return nil
}

func promptProjects(cfg *config.Config) error {
	addMore := true
	for addMore {
		var want bool
		msg := "Add or edit a project? (current: " + fmt.Sprintf("%d", len(cfg.Projects)) + ")"
		if err := survey.AskOne(&survey.Confirm{Message: msg, Default: false}, &want); err != nil {
			return err
		}
		if !want {
			break
		}

		// Gather fields
		var name, spaceKey string
		if err := survey.AskOne(&survey.Input{Message: "Project Name"}, &name, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
		if err := survey.AskOne(&survey.Input{Message: "Space Key"}, &spaceKey, survey.WithValidator(survey.Required)); err != nil {
			return err
		}

		p := config.ProjectConfig{Name: name, SpaceKey: spaceKey}
		// Replace if exists
		replaced := false
		for i, existing := range cfg.Projects {
			if existing.Name == name {
				cfg.Projects[i] = p
				replaced = true
				break
			}
		}
		if !replaced {
			cfg.Projects = append(cfg.Projects, p)
		}
	}
	return nil
}

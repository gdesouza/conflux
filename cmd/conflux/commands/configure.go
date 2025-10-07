package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"conflux/internal/config"
)

var (
	configureSets           []string
	configureAddProjects    []string
	configureRemoveProjects []string
	configureYes            bool
	configurePrint          bool
	configureNonInteractive bool
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Create or edit the configuration file interactively or via flags",
	Long: `Interactively create or edit the Conflux configuration file (config.yaml by default).

Features:
- Interactive prompts for Confluence, Projects, Mermaid, and Images sections
- Apply key=value overrides via --set
- Add projects via --add-project (e.g. --add-project "name=docs,space_key=DOCS,markdown_dir=./docs")
- Remove projects via --remove-project <name>
- Non-interactive scripting with --non-interactive --yes --set ...
- Print resulting YAML with --print instead of writing
`,
	RunE: runConfigure,
}

func init() {
	rootCmd.AddCommand(configureCmd)
	configureCmd.Flags().StringArrayVar(&configureSets, "set", nil, "Set a config field using dotted path (e.g. confluence.base_url=http://example)")
	configureCmd.Flags().StringArrayVar(&configureAddProjects, "add-project", nil, "Add a project definition (e.g. \"name=docs,space_key=DOCS,markdown_dir=./docs\")")
	configureCmd.Flags().StringArrayVar(&configureRemoveProjects, "remove-project", nil, "Remove an existing project by name (repeatable)")
	configureCmd.Flags().BoolVar(&configureYes, "yes", false, "Automatically confirm saving changes")
	configureCmd.Flags().BoolVar(&configurePrint, "print", false, "Print resulting YAML instead of writing to file")
	configureCmd.Flags().BoolVar(&configureNonInteractive, "non-interactive", false, "Disable interactive prompts (use with --set / --add-project)")
}

func runConfigure(cmd *cobra.Command, args []string) error {
	path := configFile
	cfg, existed, err := loadOrInitConfig(path)
	if err != nil {
		return err
	}

	// Apply flag mutations first (non-interactive layer)
	if err := applySetOperations(cfg, configureSets); err != nil {
		return err
	}
	if err := applyAddProjects(cfg, configureAddProjects); err != nil {
		return err
	}
	if err := applyRemoveProjects(cfg, configureRemoveProjects); err != nil {
		return err
	}

	interactive := !configureNonInteractive && len(args) == 0
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

	if configurePrint {
		cmd.Print(string(outYAML))
		return nil
	}

	if !configureYes && interactive {
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
	case "local.markdown_dir":
		cfg.Local.MarkdownDir = value
	case "local.exclude":
		cfg.Local.Exclude = splitList(value)
	case "mermaid.mode":
		cfg.Mermaid.Mode = value
	case "mermaid.format":
		cfg.Mermaid.Format = value
	case "mermaid.cli_path":
		cfg.Mermaid.CLIPath = value
	case "mermaid.theme":
		cfg.Mermaid.Theme = value
	case "mermaid.width":
		w, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		cfg.Mermaid.Width = w
	case "mermaid.height":
		h, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		cfg.Mermaid.Height = h
	case "mermaid.scale":
		s, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		cfg.Mermaid.Scale = s
	case "images.supported_formats":
		cfg.Images.SupportedFormats = splitList(value)
	case "images.max_file_size":
		m, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		cfg.Images.MaxFileSize = m
	case "images.resize_large":
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		cfg.Images.ResizeLarge = b
	case "images.max_width":
		mw, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		cfg.Images.MaxWidth = mw
	case "images.max_height":
		mh, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		cfg.Images.MaxHeight = mh
	default:
		return fmt.Errorf("unsupported key '%s'", key)
	}
	return nil
}

func splitList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

func parseBool(s string) (bool, error) {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false, err
	}
	return b, nil
}

func applyAddProjects(cfg *config.Config, defs []string) error {
	for _, d := range defs {
		pConf, err := parseProjectDefinition(d)
		if err != nil {
			return err
		}
		// Replace if same name exists
		replaced := false
		for i, existing := range cfg.Projects {
			if existing.Name == pConf.Name {
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

func applyRemoveProjects(cfg *config.Config, names []string) error {
	if len(names) == 0 {
		return nil
	}
	remove := map[string]bool{}
	for _, n := range names {
		remove[n] = true
	}
	var filtered []config.ProjectConfig
	for _, p := range cfg.Projects {
		if !remove[p.Name] {
			filtered = append(filtered, p)
		}
	}
	cfg.Projects = filtered
	return nil
}

func parseProjectDefinition(def string) (config.ProjectConfig, error) {
	pc := config.ProjectConfig{}
	items := strings.Split(def, ",")
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		kv := strings.SplitN(item, "=", 2)
		if len(kv) != 2 {
			return pc, fmt.Errorf("invalid project token '%s' (expected key=value)", item)
		}
		k, v := kv[0], kv[1]
		switch k {
		case "name":
			pc.Name = v
		case "space_key":
			pc.SpaceKey = v
		case "markdown_dir":
			pc.Local.MarkdownDir = v
		case "exclude":
			pc.Local.Exclude = splitList(v)
		default:
			return pc, fmt.Errorf("unknown project field '%s'", k)
		}
	}
	if pc.Name == "" || pc.SpaceKey == "" || pc.Local.MarkdownDir == "" {
		return pc, errors.New("project requires name, space_key, markdown_dir")
	}
	return pc, nil
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

	// Mermaid
	if err := promptMermaid(cfg); err != nil {
		return err
	}

	// Images
	if err := promptImages(cfg); err != nil {
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
		var name, spaceKey, dir, exclude string
		if err := survey.AskOne(&survey.Input{Message: "Project Name"}, &name, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
		if err := survey.AskOne(&survey.Input{Message: "Space Key"}, &spaceKey, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
		if err := survey.AskOne(&survey.Input{Message: "Markdown Dir"}, &dir, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
		if err := survey.AskOne(&survey.Input{Message: "Exclude (comma list, optional)"}, &exclude); err != nil {
			return err
		}

		p := config.ProjectConfig{Name: name, SpaceKey: spaceKey, Local: config.LocalConfig{MarkdownDir: dir, Exclude: splitList(exclude)}}
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

func promptMermaid(cfg *config.Config) error {
	var edit bool
	if err := survey.AskOne(&survey.Confirm{Message: "Edit Mermaid settings?", Default: false}, &edit); err != nil {
		return err
	}
	if !edit {
		return nil
	}
	themes := []string{"default", "dark", "forest", "neutral"}
	mode := cfg.Mermaid.Mode
	if mode == "" {
		mode = "convert-to-image"
	}
	format := cfg.Mermaid.Format
	if format == "" {
		format = "svg"
	}
	qs := []*survey.Question{
		{Name: "mode", Prompt: &survey.Select{Message: "Mermaid Mode", Options: []string{"preserve", "convert-to-image"}, Default: mode}},
		{Name: "format", Prompt: &survey.Select{Message: "Mermaid Output Format", Options: []string{"svg", "png", "pdf"}, Default: format}},
		{Name: "cli_path", Prompt: &survey.Input{Message: "Mermaid CLI Path", Default: cfg.Mermaid.CLIPath}},
		{Name: "theme", Prompt: &survey.Select{Message: "Theme", Options: themes, Default: firstNonEmpty(cfg.Mermaid.Theme, "default")}},
		{Name: "width", Prompt: &survey.Input{Message: "Width", Default: intToStringOr(cfg.Mermaid.Width, 1200)}},
		{Name: "height", Prompt: &survey.Input{Message: "Height", Default: intToStringOr(cfg.Mermaid.Height, 800)}},
		{Name: "scale", Prompt: &survey.Input{Message: "Scale", Default: floatToStringOr(cfg.Mermaid.Scale, 2.0)}},
	}
	answers := struct {
		Mode   string `survey:"mode"`
		Format string `survey:"format"`
		CLI    string `survey:"cli_path"`
		Theme  string `survey:"theme"`
		Width  string `survey:"width"`
		Height string `survey:"height"`
		Scale  string `survey:"scale"`
	}{}
	if err := survey.Ask(qs, &answers); err != nil {
		return err
	}
	cfg.Mermaid.Mode = answers.Mode
	cfg.Mermaid.Format = answers.Format
	cfg.Mermaid.CLIPath = answers.CLI
	cfg.Mermaid.Theme = answers.Theme
	if v, err := strconv.Atoi(answers.Width); err == nil {
		cfg.Mermaid.Width = v
	}
	if v, err := strconv.Atoi(answers.Height); err == nil {
		cfg.Mermaid.Height = v
	}
	if v, err := strconv.ParseFloat(answers.Scale, 64); err == nil {
		cfg.Mermaid.Scale = v
	}
	return nil
}

func promptImages(cfg *config.Config) error {
	var edit bool
	if err := survey.AskOne(&survey.Confirm{Message: "Edit Image settings?", Default: false}, &edit); err != nil {
		return err
	}
	if !edit {
		return nil
	}
	qs := []*survey.Question{
		{Name: "supported", Prompt: &survey.Input{Message: "Supported Formats (comma)", Default: strings.Join(cfg.Images.SupportedFormats, ",")}},
		{Name: "max_file_size", Prompt: &survey.Input{Message: "Max File Size (bytes)", Default: int64ToStringOr(cfg.Images.MaxFileSize, 10*1024*1024)}},
		{Name: "resize_large", Prompt: &survey.Input{Message: "Resize Large Images (true/false)", Default: fmt.Sprintf("%v", cfg.Images.ResizeLarge)}},
		{Name: "max_width", Prompt: &survey.Input{Message: "Max Width", Default: intToStringOr(cfg.Images.MaxWidth, 1200)}},
		{Name: "max_height", Prompt: &survey.Input{Message: "Max Height", Default: intToStringOr(cfg.Images.MaxHeight, 800)}},
	}
	answers := struct {
		Supported   string `survey:"supported"`
		MaxFileSize string `survey:"max_file_size"`
		Resize      string `survey:"resize_large"`
		MaxWidth    string `survey:"max_width"`
		MaxHeight   string `survey:"max_height"`
	}{}
	if err := survey.Ask(qs, &answers); err != nil {
		return err
	}
	cfg.Images.SupportedFormats = splitList(answers.Supported)
	if v, err := strconv.ParseInt(answers.MaxFileSize, 10, 64); err == nil {
		cfg.Images.MaxFileSize = v
	}
	if v, err := strconv.ParseBool(answers.Resize); err == nil {
		cfg.Images.ResizeLarge = v
	}
	if v, err := strconv.Atoi(answers.MaxWidth); err == nil {
		cfg.Images.MaxWidth = v
	}
	if v, err := strconv.Atoi(answers.MaxHeight); err == nil {
		cfg.Images.MaxHeight = v
	}
	return nil
}

// Utility helpers -----------------------------------------------------------

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func intToStringOr(v int, fallback int) string {
	if v == 0 {
		return fmt.Sprintf("%d", fallback)
	}
	return fmt.Sprintf("%d", v)
}

func int64ToStringOr(v int64, fallback int64) string {
	if v == 0 {
		return fmt.Sprintf("%d", fallback)
	}
	return fmt.Sprintf("%d", v)
}

func floatToStringOr(v float64, fallback float64) string {
	if v == 0 {
		return fmt.Sprintf("%.2f", fallback)
	}
	return fmt.Sprintf("%.2f", v)
}

# Conflux

A command-line tool to synchronize local markdown files to Confluence spaces.

## Features

- Sync markdown files to Confluence pages
- Create new pages or update existing ones
- Dry-run mode for testing
- Configurable file exclusions
- Verbose logging

## Installation

### From Source

```bash
# Build locally
make build

# Install to /usr/local/bin (requires sudo)
make install

# Uninstall
make uninstall
```

### Manual Build

```bash
go build -o conflux ./cmd/conflux
```

## Configuration

Create a `config.yaml` file:

```yaml
confluence:
  base_url: "https://yourcompany.atlassian.net/wiki"
  username: "your.email@company.com" 
  api_token: "your-api-token-here"
  space_key: "DOCS"

local:
  markdown_dir: "./docs"
  exclude:
    - "README.md"
    - "*.tmp.md"
```

## Usage

```bash
# Basic usage - sync current directory with default config
conflux

# Specify documents directory via CLI (recommended)
conflux -docs ./documentation

# Use custom config file
conflux -config /path/to/config.yaml

# Specify documents directory and custom config
conflux -docs /path/to/markdown -config my-config.yaml

# Dry run (no changes made)
conflux -dry-run -verbose

# Complex example
conflux -docs ./my-docs -config prod-config.yaml -dry-run -verbose

# Show help
conflux -help
```

### CLI Flags

- `-docs` - Path to markdown documents directory (default: current directory)
- `-config` - Path to configuration file (default: "config.yaml") 
- `-dry-run` - Preview changes without syncing to Confluence
- `-verbose` - Enable detailed logging output
- `-help` - Show usage information

**Note**: The `-docs` flag will override any `markdown_dir` specified in your config file, making it easy to work with different document directories.

## Getting a Confluence API Token

1. Go to https://id.atlassian.com/manage/api-tokens
2. Click "Create API token"
3. Give it a name and copy the generated token
4. Use your email and the token for authentication

## License

MIT
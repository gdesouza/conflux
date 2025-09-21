# Conflux

A command-line tool to synchronize local markdown files to Confluence spaces.

## Features

- **Sync markdown files to Confluence pages** - Convert and upload your local documentation
- **Automatic directory page creation** - Creates organized parent pages with children macros for folder structures
- **Smart page hierarchy** - Maintains your local directory structure in Confluence
- **Create new pages or update existing ones** - Handles both new content and updates seamlessly
- **Children macro integration** - Automatically lists child pages in directory pages
- **Dry-run mode for testing** - Preview changes before making them
- **Configurable file exclusions** - Skip files you don't want to sync
- **Verbose logging** - Detailed output for debugging and monitoring
- **Proper page versioning** - Handles Confluence page version management automatically

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

## How It Works

### Directory Structure Mapping

Conflux automatically creates a hierarchical structure in Confluence that mirrors your local directory organization:

```
docs/
├── README.md                    → "Docs" page (directory page)
├── getting-started.md           → "Getting Started" page
├── api/
│   ├── authentication.md       → "Api" page (directory page)
│   └── endpoints.md            → "Authentication" & "Endpoints" pages
└── tutorials/
    ├── basic-usage.md          → "Tutorials" page (directory page)
    └── advanced-features.md    → "Basic Usage" & "Advanced Features" pages
```

### Automatic Directory Pages

For each directory containing markdown files, Conflux creates a corresponding "directory page" in Confluence that:

- **Serves as a parent page** for all files in that directory
- **Automatically lists child pages** using Confluence's children macro
- **Updates dynamically** when child pages are added, removed, or modified
- **Maintains proper hierarchy** with parent-child relationships
- **Includes attribution** with a link back to this project

Example directory page content:
```
# Api Documentation

This section contains documentation for api. The pages below are automatically 
listed and updated whenever child pages are added or modified.

## Contents
[Children macro - automatically shows: Authentication, Endpoints]

*This page was automatically created by Conflux to organize documentation hierarchy.*
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

### Sync Command (Default)

```bash
# Basic usage - sync current directory with default config
conflux

# Specify documents directory via CLI (recommended)
conflux -docs ./documentation

# Use custom config file
conflux -config /path/to/config.yaml

# Specify documents directory and custom config
conflux sync -docs /path/to/markdown -config my-config.yaml

# Dry run (no changes made)
conflux sync -dry-run -verbose

# Complex example
conflux sync -docs ./my-docs -config prod-config.yaml -dry-run -verbose
```

### List Pages Command

```bash
# List all pages in a space
conflux list-pages -space DOCS

# List pages under a specific parent page
conflux list-pages -space DOCS -parent "API Documentation"

# Use custom config file
conflux list-pages -config prod-config.yaml -space TEAM -verbose
```

### CLI Commands

- `sync` - Sync local markdown files to Confluence (default command)
- `list-pages` - List page hierarchy from a Confluence space

### CLI Flags

**Global Flags:**
- `-config` - Path to configuration file (default: "config.yaml") 
- `-verbose` - Enable detailed logging output
- `-help` - Show usage information

**Sync Command Flags:**
- `-docs` - Path to markdown documents directory (default: current directory)
- `-dry-run` - Preview changes without syncing to Confluence

**List-Pages Command Flags:**
- `-space` - Confluence space key (required)
- `-parent` - Parent page title to start hierarchy from (optional)

**Note**: The `-docs` flag will override any `markdown_dir` specified in your config file, making it easy to work with different document directories.

## Troubleshooting

### Directory Pages Not Updating

If you notice that directory pages aren't showing updated children macro content:

1. **Delete and recreate**: In earlier versions, directory pages weren't updated automatically. Delete the directory pages in Confluence and run the sync again.
2. **Check permissions**: Ensure your API token has permission to update pages in the space.
3. **Use dry-run**: Test with `conflux sync -dry-run -verbose` to see what changes would be made.

### Children Macro Not Working

The children macro requires:
- **Proper parent-child relationships** - Conflux automatically sets these up
- **Valid Confluence space** - Make sure your space exists and is accessible
- **Appropriate permissions** - Your API token needs page creation/update rights

### Debug Output

Use the verbose flag (`-v` or `-verbose`) to see detailed information about:
- Which pages are being created or updated
- Directory page content generation
- API requests and responses
- Children macro detection and processing

```bash
# Example with full debug output
conflux sync -docs ./documentation -config prod.yaml -dry-run -verbose
```

## Getting a Confluence API Token

1. Go to https://id.atlassian.com/manage/api-tokens
2. Click "Create API token"
3. Give it a name and copy the generated token
4. Use your email and the token for authentication

## Recent Improvements

### v1.1.0 (Latest)
- **✅ Fixed children macro rendering** - Directory pages now properly display child page lists
- **✅ Enhanced directory page updates** - Existing directory pages are now updated with new content
- **✅ Simplified children macro** - Improved compatibility with Confluence Cloud
- **✅ Better error handling** - More robust page version management and API error handling
- **✅ Enhanced debug logging** - Comprehensive debugging output for troubleshooting
- **✅ Project attribution** - Directory pages now include a link back to this GitHub repository

### Key Bug Fixes
- Directory pages are now properly updated when they already exist (previously they were skipped)
- Children macro uses optimized parameters for better Confluence Cloud compatibility
- Fixed logger initialization issues that prevented debug output
- Improved Storage Format XML structure for Confluence API compatibility

## License

MIT
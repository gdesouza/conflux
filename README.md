# Conflux

A command-line tool to synchronize local markdown files to Confluence spaces with Mermaid.js diagram support.

## Features

- **Sync markdown files to Confluence pages** - Convert and upload your local documentation
- **Image attachment support** - Automatically upload and reference images from your markdown files
- **Mermaid.js diagram support** - Automatically convert or preserve mermaid diagrams in your documentation
- **Automatic directory page creation** - Creates organized parent pages with children macros for folder structures
- **Smart page hierarchy** - Maintains your local directory structure in Confluence
- **Create new pages or update existing ones** - Handles both new content and updates seamlessly
- **Children macro integration** - Automatically lists child pages in directory pages
- **Dry-run mode for testing** - Preview changes before making them
- **Configurable file exclusions** - Skip files you don't want to sync
- **Verbose logging** - Detailed output for debugging and monitoring
- **Proper page versioning** - Handles Confluence page version management automatically

## Mermaid.js Diagram Support

Conflux supports automatic processing of Mermaid.js diagrams in your markdown files. When mermaid code blocks are detected, you can choose to either preserve them as syntax-highlighted code blocks in Confluence or convert them to images.

### Setup

1. **Install Mermaid CLI** (for image conversion):
   ```bash
   npm install -g @mermaid-js/mermaid-cli
   ```

2. **Configure mermaid support** in your `config.yaml`:
   ```yaml
   mermaid:
     mode: "convert-to-image"  # Options: "preserve" or "convert-to-image"
     format: "png"             # Options: "png", "svg", "pdf"
     theme: "default"          # Options: "default", "dark", "forest", "neutral"
   ```

### Processing Modes

**Preserve Mode** (`mode: "preserve"`):
- Keeps mermaid diagrams as syntax-highlighted code blocks in Confluence
- No external dependencies required
- Diagrams remain editable in Confluence

**Convert-to-Image Mode** (`mode: "convert-to-image"`):
- Converts mermaid diagrams to images (PNG, SVG, or PDF)
- Images are uploaded as Confluence attachments
- Requires `@mermaid-js/mermaid-cli` to be installed
- Provides better visual presentation

### Example Usage

Create a markdown file with mermaid diagrams:

````markdown
# System Architecture

```mermaid
graph TD
    A[User] --> B[Frontend]
    B --> C[API Gateway]
    C --> D[Backend Service]
    D --> E[Database]
```

## Process Flow

```mermaid
sequenceDiagram
    participant U as User
    participant F as Frontend
    participant B as Backend
    U->>F: Login Request
    F->>B: Authenticate
    B-->>F: Token
    F-->>U: Success
```
````

When synced to Confluence:
- **Preserve mode**: Diagrams appear as formatted code blocks
- **Convert-to-image mode**: Diagrams are rendered as images and embedded in the page

### Dependency Checking

Conflux automatically checks for mermaid CLI availability:
- If `mmdc` is not found and mode is "convert-to-image", it falls back to "preserve" mode
- Use `conflux sync -verbose` to see dependency check results
- Graceful fallback ensures sync operations continue even if CLI is unavailable

## Image Attachment Support

Conflux automatically detects and uploads image files referenced in your markdown documentation, making them available as Confluence page attachments.

### Supported Image Formats

By default, Conflux supports the following image formats:
- **PNG** - Portable Network Graphics
- **JPG/JPEG** - Joint Photographic Experts Group
- **GIF** - Graphics Interchange Format
- **SVG** - Scalable Vector Graphics
- **WEBP** - Modern web-optimized format

### How It Works

1. **Automatic Detection**: Conflux scans your markdown for image references using standard markdown syntax: `![alt text](path/to/image.png)`
2. **Path Resolution**: Both relative and absolute image paths are supported
3. **Validation**: Images are checked for existence, file size limits, and supported formats
4. **Upload**: Valid images are uploaded as Confluence page attachments
5. **Reference Replacement**: Markdown image syntax is replaced with Confluence image macros

### Configuration

Configure image processing in your `config.yaml`:

```yaml
images:
  supported_formats: ["png", "jpg", "jpeg", "gif", "svg", "webp"]
  max_file_size: 10485760  # 10MB in bytes
  resize_large: false      # Future feature for image resizing
  max_width: 1200          # Max width for future resizing feature
  max_height: 800          # Max height for future resizing feature
```

### Example Usage

Create a markdown file with image references:

```markdown
# Project Architecture

Here's our system architecture:

![System Architecture](./images/architecture.png)

## Component Diagram

![Component Relationships](../diagrams/components.svg)

## Screenshots

![Application Screenshot](./screenshots/main-ui.jpg)
```

When synced to Confluence:
- Images are uploaded as page attachments
- Markdown image syntax is replaced with Confluence image macros
- Images display properly in Confluence pages
- Alt text is preserved for accessibility

### Error Handling

Conflux handles image processing errors gracefully:
- **Missing files**: Logs warnings but continues sync operation
- **Unsupported formats**: Skips invalid images and reports in logs
- **Size limits**: Reports files that exceed configured limits
- **Upload failures**: Continues with other images and page content

Use `conflux sync -verbose` to see detailed image processing information.

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

# Optional: Image attachment support
images:
  supported_formats: ["png", "jpg", "jpeg", "gif", "svg", "webp"]
  max_file_size: 10485760  # 10MB in bytes
  resize_large: false      # Future feature for image resizing
  max_width: 1200          # Max width for future resizing feature
  max_height: 800          # Max height for future resizing feature

# Optional: Mermaid.js diagram support
mermaid:
  mode: "convert-to-image"  # or "preserve"
  format: "png"             # png, svg, or pdf (for convert-to-image mode)
  cli_path: "mmdc"          # path to mermaid CLI (optional, uses 'mmdc' by default)
  theme: "default"          # mermaid theme: default, dark, forest, neutral
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

### Images Not Uploading or Displaying

If images in your markdown aren't being uploaded or displayed correctly:

1. **Check file paths**: Ensure image paths in your markdown are correct relative to the markdown file
   ```markdown
   ![Image](./images/diagram.png)  # Relative to markdown file location
   ![Image](/absolute/path/image.png)  # Absolute path
   ```

2. **Verify file formats**: Ensure images use supported formats
   - Supported: PNG, JPG, JPEG, GIF, SVG, WEBP
   - Check configuration in `config.yaml` under `images.supported_formats`

3. **Check file sizes**: Large files may be rejected
   - Default limit: 10MB
   - Configure in `config.yaml`: `images.max_file_size`

4. **Review permissions**: Ensure your API token can upload attachments
   - Confluence admin permissions may be required for file uploads

5. **Use verbose logging**: See detailed image processing information
   ```bash
   conflux sync -verbose -dry-run  # See what images are detected
   ```

### Mermaid Diagrams Not Converting

If mermaid diagrams aren't being converted to images:

1. **Check CLI installation**: Ensure `@mermaid-js/mermaid-cli` is installed globally
   ```bash
   npm install -g @mermaid-js/mermaid-cli
   mmdc --version  # Should show version number
   ```

2. **Verify configuration**: Check your `config.yaml` has correct mermaid settings
   ```yaml
   mermaid:
     mode: "convert-to-image"
     format: "png"
   ```

3. **Check dependencies**: Use verbose mode to see dependency check results
   ```bash
   conflux sync -verbose -dry-run
   ```

4. **Fallback behavior**: If CLI is unavailable, Conflux automatically falls back to preserve mode

### Debug Output

Use the verbose flag (`-v` or `-verbose`) to see detailed information about:
- Which pages are being created or updated
- Directory page content generation
- API requests and responses
- Children macro detection and processing
- Mermaid diagram detection and conversion
- Dependency checks and fallback decisions

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

### v1.3.0 (Latest)
- **✅ Image attachment support** - Automatically detect and upload images referenced in markdown files
  - **Automatic detection**: Finds `![alt](image.png)` syntax in markdown content
  - **Multiple formats**: Support for PNG, JPG, JPEG, GIF, SVG, and WEBP images
  - **Path resolution**: Handles both relative and absolute image paths
  - **File validation**: Checks for file existence, size limits, and supported formats
  - **Graceful error handling**: Continues sync operation even when some images fail
- **✅ Enhanced markdown processing** - Extended parser to handle image references alongside mermaid diagrams
- **✅ Configurable image processing** - File size limits, supported formats, and future resizing options
- **✅ Improved sync logic** - Post-processing now handles both images and mermaid diagrams efficiently
- **✅ Comprehensive image validation** - Built-in checks for file existence, formats, and size limits

### v1.2.0
- **✅ Mermaid.js diagram support** - Automatically process mermaid diagrams with two modes:
  - **Preserve mode**: Keep diagrams as syntax-highlighted code blocks
  - **Convert-to-image mode**: Convert to PNG/SVG/PDF and upload as attachments
- **✅ Enhanced markdown processing** - Extended parser to detect and handle mermaid code blocks
- **✅ Confluence attachment support** - Added API methods for uploading and managing attachments
- **✅ Dependency checking** - Automatic detection of mermaid CLI availability with graceful fallbacks
- **✅ Configurable mermaid themes** - Support for default, dark, forest, and neutral themes
- **✅ Multiple output formats** - PNG, SVG, and PDF support for converted diagrams
- **✅ Security improvements** - Upgraded from MD5 to SHA256 for file hashing

### v1.1.0
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
- Enhanced security with SHA256 hashing instead of MD5
- Improved error handling and temp file cleanup in mermaid processing

## License

MIT
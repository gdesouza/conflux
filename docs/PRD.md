# Product Requirements Document: Conflux

## Executive Summary

**Product Name:** Conflux  
**Version:** 1.1.0+  
**Product Type:** Command-line Confluence page editing tool

Conflux is a CLI for loss-preserving Confluence page editing. Its primary workflow pulls one page into Markdown plus paired metadata and attachments, permits focused local edits, and pushes the artifact back without silently discarding unsupported Confluence content. Tree synchronization is legacy functionality scheduled for deprecation and removal.

## Product Vision

Enable teams to edit specific parts of Confluence pages using familiar Markdown while preserving page meaning, unsupported native content, attachment identity, and concurrent-edit safety.

## Target Users

**Primary Users:**
- Technical writers managing documentation workflows
- Development teams using markdown for documentation  
- DevOps engineers implementing documentation automation
- Product managers maintaining project documentation

**Secondary Users:**
- QA teams needing documentation validation workflows
- Engineering managers overseeing documentation standards

## Core Functionality

### 1. Page Artifact Workflow

**Primary Features:**
- **Paired artifacts**: `<name>.md` plus `<name>.attachments/metadata.json` and referenced files
- **Loss-preserving pull**: Unsupported storage nodes become opaque fragments referenced by embedded markers
- **Deterministic rendering**: Conversion is pure and testable without Confluence credentials
- **Attachment intents**: Renderers describe downloads and uploads without performing network operations
- **Conflict-safe push target**: Existing-page artifacts carry a base version so remote edits can be rejected before mutation

**Supported Markdown Elements:**
- Headers (H1-H4)
- Code blocks with syntax highlighting
- Unordered and ordered lists  
- Inline formatting (bold, italic, code)
- Paragraphs with proper spacing
- Links and attachment-backed images
- Blockquotes and tables
- Jira inline previews through explicit Conflux directives

**Artifact Safety Rules:**
- Preservation markers must resolve uniquely to metadata fragments.
- Unsupported metadata schema versions fail explicitly.
- Attachment paths cannot escape the paired attachments directory.
- `metadata.json` is never uploaded as a Confluence attachment.
- Remote attachments are never deleted implicitly.
- A stale page version must be rejected unless the user explicitly overrides it.

### 2. Hierarchical Organization System

**Directory Structure Mapping:**
- **Automatic parent-child relationships**: Maps local directory structure to Confluence page hierarchy
- **Directory page creation**: Generates organizational pages for folders containing markdown files
- **Children macro integration**: Automatically lists child pages with dynamic updates
- **Multi-level nesting**: Supports unlimited directory depth

**Directory Page Features:**
- Auto-generated descriptive content
- Confluence children macro for dynamic content listing
- Attribution links to project source
- Automatic updates when structure changes

### 3. CLI Interface & Commands

**Core Commands:**
- `pull`: Export storage/HTML/Markdown to stdout or write an editable artifact with `--output`
- `push`: Create/update a page; artifact-aware conflict-safe orchestration is the next integration phase
- `pages`: List and inspect Confluence pages
- `config`: Create and inspect configuration
- `sync`: Legacy synchronization command, to be deprecated and removed
- `projects`: Legacy profile-listing command; named profiles remain useful to page commands
- `version`: Show detailed build and version information

**Global Flags:**
- `--config/-c`: Specify configuration file path
- `--verbose/-v`: Enable detailed logging output

**Sync Command Options:**
- `--docs/-d`: Override markdown directory path
- `--dry-run`: Preview changes without execution
- `--space/-s`: Override Confluence space key

**List-Pages Command Options:**  
- `--space/-s`: Confluence space key (required)
- `--parent/-p`: Filter by parent page title

### 4. Configuration Management

**Configuration File Support (YAML):**
```yaml
confluence:
  base_url: "https://company.atlassian.net/wiki"
  username: "user@company.com" 
  api_token: "api-token"
  space_key: "DOCS"

local:
  markdown_dir: "./docs"
  exclude:
    - "README.md"
    - "*.tmp.md"
```

**Configuration Features:**
- Flexible validation for different command contexts
- CLI override capabilities
- File exclusion patterns with glob support
- Environment-specific configurations

### 5. Confluence Integration

**API Capabilities:**
- **Page Operations**: Create, read, update operations via REST API
- **Space Management**: List pages and manage space hierarchies  
- **Authentication**: API token-based authentication
- **Error Handling**: Comprehensive API error handling and retry logic

**Page Management Features:**
- Title-based page lookup and deduplication
- Parent-child relationship management
- Version number tracking and incrementing
- Storage format XML generation

### 6. Visual Feedback & Reporting

**Dry-Run Visualization:**
- Tree-structured preview with status icons
- Page status indicators (new, changed, up-to-date)
- Directory structure representation
- Color-coded status messaging

**Page Hierarchy Display:**
- Visual tree formatting with Unicode characters
- Icon-based page type identification (📁 folders, 📄 pages)
- Parent-child relationship visualization
- Confluence page ID display

### 7. Development & Build System

**Build Infrastructure:**
- Makefile-based build system with version injection
- Git-based version management with semantic versioning
- Cross-platform build support (Go-based)
- Installation and distribution tools

**Quality Assurance:**
- golangci-lint integration with comprehensive linter set
- Structured logging with configurable verbosity
- Error propagation with context wrapping

## Technical Architecture

**Core Components:**
- **CLI Framework**: Cobra-based command structure
- **Configuration Layer**: YAML-based configuration with validation
- **Markdown Parser**: Custom parser with Confluence format conversion
- **Confluence Client**: HTTP REST API client with authentication
- **Content Module**: Pure storage-to-artifact and artifact-to-storage rendering, validation, markers, and attachment intents
- **Sync Engine**: Hierarchical synchronization logic
- **Logger**: Structured logging with multiple severity levels

**Primary Data Flow:**
1. Resolve immutable configuration and selected profile.
2. Fetch page storage, version, and attachment metadata through the Confluence adapter.
3. Render Markdown, metadata, preservation markers, and attachment download intents.
4. Stage and atomically install the paired local artifact.
5. Read and validate the edited artifact before any remote mutation.
6. Render Confluence storage and attachment upload intents.
7. Compare the remote page version, upload changed attachments, update the page, then atomically advance local metadata.

## Success Metrics

**Operational Metrics:**
- File processing speed (files per second)
- API call efficiency (requests per sync operation)
- Error rate and recovery success
- Configuration validation accuracy

**User Experience Metrics:**
- Setup time from installation to first sync
- Documentation hierarchy accuracy
- Sync operation success rate
- User-reported synchronization issues

## Dependencies & Requirements

**Runtime Requirements:**
- Go 1.24.4+ for builds
- Network connectivity to Confluence instance
- Valid Confluence API token with appropriate permissions

**API Dependencies:**
- Confluence REST API v2
- YAML configuration parsing (gopkg.in/yaml.v3)
- CLI framework (github.com/spf13/cobra)

## Deployment & Distribution

**Installation Methods:**
- Source compilation with Make
- Manual Go build process
- Binary distribution via system package managers

**Configuration Requirements:**
- Confluence instance URL and credentials
- Local markdown directory specification
- Space key identification
- Optional file exclusion patterns

## Future Enhancement Opportunities

**Potential Features:**
- Editable representations for additional Confluence macros and layouts
- Smart Link card and embed appearances after tenant storage fixtures are captured
- Three-way merging after version conflicts
- Webhook-based automatic synchronization
- Multi-space synchronization support
- Template-based page generation
- Content conflict resolution strategies
- Integration with version control systems

**Technical Improvements:**
- Incremental synchronization based on file modification times
- Parallel processing for large document sets
- Configuration validation and testing utilities
- Enhanced error recovery and rollback capabilities

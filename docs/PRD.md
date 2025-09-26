# Product Requirements Document: Conflux

## Executive Summary

**Product Name:** Conflux  
**Version:** 1.1.0+  
**Product Type:** Command-line documentation synchronization tool  

Conflux is a specialized CLI tool designed to bridge the gap between local markdown documentation and Confluence knowledge management systems. It automatically converts, uploads, and maintains hierarchical documentation structures while preserving the natural organization of local file systems.

## Product Vision

Enable development teams to maintain their documentation in familiar markdown format locally while automatically keeping their Confluence spaces up-to-date with proper hierarchical organization and cross-references.

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

### 1. Document Synchronization Engine

**Primary Features:**
- **Markdown to Confluence conversion**: Converts markdown files to Confluence Storage Format XML
- **Bidirectional page management**: Creates new pages and updates existing ones
- **Content versioning**: Handles Confluence page version management automatically
- **Batch processing**: Processes multiple files efficiently with proper error handling

**Supported Markdown Elements:**
- Headers (H1-H4)
- Code blocks with syntax highlighting
- Unordered and ordered lists  
- Inline formatting (bold, italic, code)
- Paragraphs with proper spacing

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
- `sync` (default): Synchronize local markdown files to Confluence
- `list-pages`: Display Confluence space hierarchy with visual formatting
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
- **Sync Engine**: Hierarchical synchronization logic
- **Logger**: Structured logging with multiple severity levels

**Data Flow:**
1. Configuration loading and validation
2. Markdown file discovery and parsing  
3. Directory structure analysis
4. Confluence API connectivity verification
5. Hierarchical page creation (directories first)
6. Document synchronization with parent relationships
7. Status reporting and error handling

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
- Support for additional markdown elements (tables, images, links)
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

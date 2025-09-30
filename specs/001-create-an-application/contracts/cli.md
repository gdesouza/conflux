# CLI Contracts: Confluence Sync Application

## `conflux init`

Initializes a new project.

**Arguments**:
- `--name`: The name of the project.
- `--path`: The local path to the project.
- `--space`: The Confluence space to sync to.
- `--parent-page-id`: The ID of the parent page in Confluence.

## `conflux sync`

Synchronizes the local project with Confluence.

**Arguments**:
- `--project`: The name of the project to sync.

## `conflux download`

Downloads a page from Confluence.

**Arguments**:
- `--project`: The name of the project.
- `--page-id`: The ID of the page to download.

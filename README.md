# Conflux

Conflux is a command-line tool for safely editing Confluence pages through local Markdown. Its primary workflow is pull, edit, and push, with optimistic conflict protection and round-trip preservation for supported Confluence structures.

## Build

Requires Go 1.24 or later.

```sh
make build
make test
make lint
```

The binary is written to `bin/conflux`.

## Configuration

Create `config.yaml` in the working directory:

```yaml
confluence:
  base_url: https://example.atlassian.net
  username: you@example.com
  api_token: your-api-token
  space_key: DOCS
```

Credentials can instead be supplied through `CONFLUX_BASE_URL`, `CONFLUX_USERNAME`, and `CONFLUX_API_TOKEN`. A space can be selected with `--space`, `CONFLUX_SPACE_KEY`, `confluence.space_key`, or a named profile:

```yaml
confluence:
  base_url: https://example.atlassian.net
  username: you@example.com
  api_token: your-api-token
projects:
  - name: docs
    space_key: DOCS
  - name: engineering
    space_key: ENG
```

Although the configuration key remains `projects` for compatibility, these entries are only named space profiles. List them with `conflux config profiles` and select one with `--project`.

The `config` command can create or update the file non-interactively:

```sh
conflux config --non-interactive --yes \
  --set confluence.base_url=https://example.atlassian.net \
  --set confluence.username=you@example.com \
  --set confluence.api_token=your-api-token \
  --add-project name=docs,space_key=DOCS
```

## Pull, edit, push

Pull an editable artifact by providing an output Markdown file:

```sh
conflux pull --space DOCS --page 5911379971 --output page.md
```

This creates a paired artifact:

```text
page.md
page.attachments/
  metadata.json
  diagram.png
```

Edit `page.md`, then push it:

```sh
conflux push --file page.md
```

`metadata.json` records page identity, version, preserved structures, and attachment state. Push checks the remote version before updating. If the page changed remotely, pull it again and reapply the local edit; use `--force` only when intentionally overwriting the newer version.

Moving or renaming the Markdown file and its matching `.attachments` directory together is supported.

### Supported round-trip controls

Conflux embeds explicit HTML comments in pulled Markdown where Markdown alone cannot represent Confluence semantics. Keep these markers intact.

Editable Jira inline previews use this syntax:

```markdown
[PSS-3369](jira:PSS-3369){conflux-display=inline jira-server="System Jira" jira-server-id="4a67abd8-f396-3524-919a-398ffb606bf7"}
```

The Jira key may be edited; retain the server name and ID from the pulled page. Ordinary URL links stay ordinary Markdown links. Card and embed Jira appearances are not yet represented as editable Markdown and are preserved opaquely rather than guessed.

Table layout uses a directive immediately before the Markdown table:

```markdown
<!-- conflux:table layout="full-width" width="1200" -->
| Service | Owner |
| --- | --- |
| API | Platform |
```

Supported layouts are `default`, `center`, `wide`, and `full-width`; width is an optional positive integer. The directive must remain directly adjacent to a structurally valid table.

Unsupported macros and structures are preserved as opaque marked regions. Editing inside an opaque region is intentionally restricted; surrounding Markdown remains editable.

## Other commands

```sh
# List a space
conflux pages --space DOCS

# Inspect one page
conflux pages show --space DOCS --page "API Reference"

# Download content to stdout
conflux pull --space DOCS --page 5911379971 --format markdown

# Create or update a standalone Markdown page without artifact metadata
conflux push --space DOCS --file new-page.md

# Create below a parent ID or title
conflux push --space DOCS --file new-page.md --parent "Documentation"
```

Run `conflux <command> --help` for all flags.

## Workflow boundaries

Conflux operates on one page at a time. Directory synchronization, local caches, rename detection, and the former `sync` and `projects` commands have been removed. Attachments used by editable artifacts are managed through the page-specific `.attachments` directory.

## License

See [LICENSE](LICENSE).

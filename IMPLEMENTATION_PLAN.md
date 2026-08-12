# Implementation Plan: Loss-Preserving Confluence Page Editing

## Goal

Refocus Conflux on reliable page-level tooling. A user can pull a Confluence page, edit the generated Markdown, and push it back while preserving the rendered meaning of content that Conflux does not yet understand.

The primary workflow is:

```bash
conflux pull --page 12345 --output deployment.md
# edit deployment.md
conflux push deployment.md
```

The resulting local artifact is:

```text
deployment.md
deployment.attachments/
├── metadata.json
├── diagram.png
└── screenshot.jpg
```

## Product Scope

### Commands

| Command | Direction |
|---|---|
| `config` | Keep and improve configuration resolution. |
| `pages` | Keep; improve after the page-editing workflow is stable. |
| `projects` | Deprecate the standalone listing command, but retain named configuration profiles used by `pull` and `push`. |
| `pull` | Make it produce a loss-preserving editable artifact. |
| `push` | Make it consume that artifact safely and preserve unsupported content. |
| `sync` | Deprecate, freeze, and later remove with its cache, rename detection, and prompts. |

### Initial fidelity contract

Pulling a page and pushing it without edits must preserve its Confluence rendering semantically. Byte-for-byte storage XML equality is not required because Confluence may normalize storage markup.

Initially supported editable content:

- Headings
- Paragraphs
- Emphasis and inline code
- Ordered and unordered lists
- Fenced code blocks
- Links
- Images backed by page attachments

Unsupported Confluence elements are retained as opaque storage fragments referenced by embedded Markdown markers:

```markdown
<!-- conflux:preserved id="fragment-0001" -->
```

Unsupported elements are not silently converted, discarded, or rewritten.

## Architectural Direction

The implementation is organized around three deep modules:

```text
pull command
    │
    ├── Confluence adapter ── fetch page and attachments
    │
    └── Content module ───── storage XML → editable artifact
                                  │
                                  ▼
                       Markdown + metadata + files
                                  │
                                  ▼
push command
    │
    ├── Content module ───── editable artifact → storage XML + attachment intents
    │
    └── Confluence adapter ── check version, upload changes, update page
```

The command modules coordinate the workflow but do not parse storage XML, convert Markdown, or perform raw HTTP requests.

### Content module

Create `internal/content` as the owner of round-trip conversion and artifact validation. It must not depend on the Confluence client, HTTP, terminal I/O, or command globals.

Its conceptual inputs and outputs are value types:

```go
type PulledPage struct {
    ID       string
    SpaceKey string
    Title    string
    Version  int
    Storage  string
}

type EditableArtifact struct {
    Markdown  string
    Metadata  Metadata
    Downloads []AttachmentDownload
}

type PushArtifact struct {
    PageID      string
    BaseVersion int
    Title       string
    Storage     string
    Uploads     []AttachmentUpload
}
```

Exact names may change during implementation, but the seam must retain these properties:

- Conversion is deterministic and testable without network access.
- Unsupported storage fragments are preserved verbatim in metadata.
- Attachment operations are described as intents rather than executed by the renderer.
- Artifact validation happens before remote mutation.

### Confluence adapter

Keep `internal/confluence` as the only module that understands Confluence endpoints and authentication. Deepen it around shared transport policy:

- Injected `http.Client` with a finite timeout
- Context-aware requests
- Central authentication and common headers
- Typed errors for status and version conflicts
- Bounded error-body reads
- Pagination in one implementation
- Typed attachment upload and download operations
- No exported raw authenticated-request escape hatch

Consumer-owned interfaces should remain narrow. `pull` needs page reading and attachment downloading; `push` needs page reading, attachment inspection/upload, and page update.

## Artifact Contract

### Paths

For a Markdown file `<directory>/<name>.md`:

```text
Metadata:    <directory>/<name>.attachments/metadata.json
Attachments: <directory>/<name>.attachments/<filename>
```

`metadata.json` is a local control file and must never be uploaded as a Confluence attachment.

### Metadata schema

The first schema should contain at least:

```json
{
  "schema_version": 1,
  "page": {
    "id": "12345",
    "space_key": "DOCS",
    "title": "Deployment",
    "base_version": 17
  },
  "preserved_fragments": {
    "fragment-0001": "<ac:structured-macro>...</ac:structured-macro>"
  },
  "attachments": [
    {
      "id": "67890",
      "filename": "diagram.png",
      "media_type": "image/png",
      "sha256": "..."
    }
  ]
}
```

Rules:

- `schema_version` is mandatory and unsupported versions fail explicitly.
- Page ID, space key, and base version are mandatory for safe update.
- Fragment IDs are unique and stable within the artifact.
- Every preservation marker in Markdown must resolve to exactly one metadata fragment.
- Unreferenced preserved fragments produce a warning initially; strict rejection can be added later.
- Attachment paths must remain inside the matching attachments directory.
- Duplicate attachment filenames fail validation.
- Metadata is written atomically with restrictive file permissions.

## Delivery Phases

### Phase 0 — Lock behavior with characterization tests

Before restructuring production code:

- Add fixtures for representative Confluence storage XML.
- Capture current Markdown conversion behavior for supported constructs.
- Add command tests for current `pull` and `push` configuration precedence.
- Add a no-edit semantic round-trip test harness.
- Define canonical comparison that ignores insignificant XML normalization while preserving element order, attributes, macro bodies, and visible text.

Exit criteria:

- Existing behavior is recorded.
- Round-trip fixtures can express known fidelity gaps.
- `make test`, `make build`, and `make lint` pass.

### Phase 1 — Define and validate the editable artifact

- Add metadata types and schema-version validation under `internal/content`.
- Implement artifact path derivation from the Markdown filename.
- Implement preservation marker parsing and validation.
- Implement atomic metadata read/write.
- Reject path traversal and metadata/Markdown page mismatches.
- Add unit tests for missing, malformed, mismatched, and unsupported metadata.

Exit criteria:

- Given `deployment.md`, the module deterministically resolves `deployment.attachments/metadata.json`.
- Missing metadata is allowed only when the Markdown contains no preservation markers and the user is intentionally creating a new page.
- A malformed or incomplete existing-page artifact cannot reach the network adapter.

### Phase 2 — Build pure storage-to-artifact rendering

- Replace regex-only preservation of complex storage markup with structural XML tokenization/parsing.
- Convert supported storage elements to Markdown.
- Store unsupported elements as opaque fragments and emit markers at their original positions.
- Discover attachment references without downloading them.
- Preserve ordering between editable and opaque content.
- Keep the existing conversion entry points temporarily as compatibility adapters.

Exit criteria:

- Conversion tests require no HTTP client.
- Unsupported macros, layouts, and unknown namespaced elements survive a no-edit round trip.
- The content module returns download intents rather than performing downloads.

### Phase 3 — Deepen the Confluence adapter

- Introduce an injected HTTP client and request context.
- Consolidate request construction, authentication, error decoding, and JSON decoding.
- Add typed page-version conflict and not-found errors.
- Implement attachment download through the adapter.
- Implement attachment listing with pagination.
- Remove `DoAuthenticatedRequest` from command usage.
- Add `httptest.Server` coverage for success, pagination, timeouts, malformed responses, and typed errors.

Exit criteria:

- Commands contain no `net/http` calls.
- Every request has a deadline or inherited context cancellation.
- The adapter is testable without Confluence credentials.

### Phase 4 — Rebuild `pull` around the artifact

- Add/standardize `--output <page.md>`.
- Resolve page by ID or title using the selected configuration profile.
- Fetch storage content and attachment metadata through the adapter.
- Render the editable artifact through `internal/content`.
- Download only referenced attachments into `<name>.attachments/`.
- Write Markdown, attachments, and metadata using a staged temporary directory so partial pulls do not replace a valid artifact.
- Refuse destructive overwrites unless explicitly authorized.
- Keep stdout formats as an explicit export mode if still useful, separate from editable artifact output.

Exit criteria:

- Pull produces the agreed filesystem layout.
- `metadata.json` is not hidden and is never confused with an attachment.
- Interrupted or failed pulls do not leave a partially valid artifact.

### Phase 5 — Build artifact-to-storage rendering

- Read Markdown and matching metadata.
- Reinsert preserved fragments at marker positions.
- Convert supported Markdown to Confluence storage XML.
- Calculate hashes for local attachment files.
- Produce upload intents for new or changed files.
- Preserve unchanged remote attachment references.
- Treat a removed local attachment as an error when Markdown still references it.
- Do not produce automatic remote deletion intents.

Exit criteria:

- Rendering is pure and returns storage plus attachment intents.
- A no-edit artifact reproduces semantically equivalent storage.
- Marker deletion or corruption cannot silently discard a preserved fragment.

### Phase 6 — Rebuild `push` with conflict safety

- Detect whether the input is an existing-page artifact or a new standalone Markdown page.
- For an existing artifact, fetch the current page and compare its version with `base_version`.
- Refuse a stale update by default with an actionable error.
- Support `--force` only as an explicit override.
- Upload only new or changed attachments.
- Render final attachment references and update the page.
- Write the returned page version and attachment identities back to metadata atomically after success.
- Preserve metadata unchanged after a failed update.

Exit criteria:

- Remote edits since pull cannot be overwritten accidentally.
- A successful push advances `base_version` locally.
- Repeating push without local changes causes no attachment uploads.
- Remote attachments are never deleted implicitly.

### Phase 7 — End-to-end fidelity suite

Add fixture-driven tests for:

- Plain supported Markdown
- Images and non-image attachments
- Code macros
- Unknown structured macros
- Nested macros
- Layouts and tables retained opaquely
- Namespaced XML attributes
- Multiple preserved fragments between editable sections
- Deleted and duplicated markers
- Renamed local Markdown plus matching attachments directory
- Missing attachment files
- Remote version conflict
- No-edit pull/push round trip

Where possible, compare normalized storage XML and additionally assert all visible text, attachment references, and preserved fragments.

Exit criteria:

- The supported fidelity contract is executable as tests.
- Known unsupported editable semantics are documented but losslessly preserved.
- Total project coverage does not fall below the current 65% baseline; new core modules target at least 80% statement coverage.

### Phase 8 — Configuration and command cleanup

- Replace duplicated config loaders with one parse/default/validate path.
- Resolve an immutable runtime configuration from profile, command flags, and environment variables.
- Document precedence consistently across `pull`, `push`, and `pages`.
- Retain `--project` as a profile selector unless renamed through a separate compatibility decision.
- Mark the standalone `projects` command deprecated and point users to `config` profile inspection.
- Mark `sync` deprecated and freeze its feature set.

Exit criteria:

- `pull` and `push` resolve the same profile and space in the same way.
- Deprecation warnings identify replacements and removal timing.
- No new content or transport behavior depends on `internal/sync`.

### Phase 9 — Remove synchronization

Perform this only after the deprecation policy has been released and accepted:

- Remove `sync` command registration and flags.
- Remove `internal/sync`, sync metadata cache, rename detection, and interactive prompts.
- Remove sync-only configuration fields after migration support is no longer needed.
- Remove dead compatibility conversion paths exposed only to sync.
- Update README, PRD, examples, CI fixtures, and release notes.

Exit criteria:

- The binary contains only the focused page tooling commands.
- `rg` finds no live imports of `internal/sync`.
- Build, tests, lint, and documented pull/edit/push smoke test pass.

## Safety and Failure Policy

- Never discard unsupported Confluence content silently.
- Never overwrite a page whose version changed without `--force`.
- Never delete remote attachments automatically.
- Never upload `metadata.json`.
- Never write attachment files outside the derived attachments directory.
- Never update local metadata until the remote operation succeeds.
- Prefer an explicit error over a degraded conversion when fidelity is uncertain.

## Compatibility and Deprecation

Deprecation should occur in two steps:

1. Emit warnings and update documentation while commands still work.
2. Remove commands in a clearly announced breaking release.

`projects` command deprecation does not remove named profiles. The profile capability remains necessary for `pull`, `push`, and `pages` unless configuration design later replaces it with an equivalent concept.

`sync` should receive no architectural refactor beyond fixes required during its deprecation window. New content and Confluence adapter modules may be called through compatibility adapters, but their design must be driven by `pull` and `push`.

## Deferred Work

- Three-way merging after a version conflict
- Partial/section-level remote edits
- Automatic remote attachment deletion
- Editable native representations for every Confluence macro
- Page trees containing multiple local Markdown documents
- Byte-for-byte storage XML preservation
- Bulk pull/push workflows

These can be added after the no-edit semantic round trip and conflict-safe single-page workflow are reliable.

## Release Gate

The first production-ready release of the new workflow requires:

- `pull --output page.md` produces Markdown, matching attachments directory, and `metadata.json`.
- Immediate `push page.md` preserves the page rendering on the supported fixture corpus.
- Unsupported constructs survive through preservation markers.
- Changed remote page versions stop push by default.
- New and changed local attachments upload correctly; unchanged ones do not.
- All tests, build, and lint pass.
- User documentation explains artifact pairing, renaming, conflicts, markers, and fidelity limits.

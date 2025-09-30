# Tasks: Confluence Sync Application

**Input**: Design documents from `/specs/001-create-an-application/`
**Prerequisites**: plan.md (required), research.md, data-model.md, contracts/

## Phase 3.1: Setup
- [ ] T001 Create project structure per implementation plan in `cmd/conflux` and `internal`
- [ ] T002 Initialize Go project with spf13/cobra dependencies
- [ ] T003 [P] Configure linting and formatting tools

## Phase 3.2: Tests First (TDD) ⚠️ MUST COMPLETE BEFORE 3.3
**CRITICAL: These tests MUST be written and MUST FAIL before ANY implementation**
- [ ] T004 [P] Integration test for `conflux init` command in `cmd/conflux/commands/init_test.go`
- [ ] T005 [P] Integration test for `conflux sync` command in `cmd/conflux/commands/sync_test.go`
- [ ] T006 [P] Integration test for `conflux download` command in `cmd/conflux/commands/download_test.go`

## Phase 3.3: Core Implementation (ONLY after tests are failing)
- [ ] T007 [P] Configuration model in `internal/config/config.go`
- [ ] T008 [P] Project model in `internal/config/config.go`
- [ ] T009 [P] Page model in `internal/confluence/client.go`
- [ ] T010 [P] Attachment model in `internal/confluence/client.go`
- [ ] T011 [P] `init` command in `cmd/conflux/commands/init.go`
- [ ] T012 [P] `sync` command in `cmd/conflux/commands/sync.go`
- [ ] T013 [P] `download` command in `cmd/conflux/commands/download.go`
- [ ] T014 [P] Markdown to Confluence conversion logic in `internal/markdown/parser.go`
- [ ] T015 [P] Mermaid to image conversion logic in `internal/mermaid/processor.go`

## Phase 3.4: Integration
- [ ] T016 Connect to Confluence API in `internal/confluence/client.go`
- [ ] T017 Implement page versioning check in `internal/sync/syncer.go`

## Phase 3.5: Polish
- [ ] T018 [P] Unit tests for `internal/config/config.go`
- [ ] T019 [P] Unit tests for `internal/markdown/parser.go`
- [ ] T020 [P] Unit tests for `internal/mermaid/processor.go`
- [ ] T021 [P] Update `README.md` with usage instructions

## Phase 3.6: Quality Gates
- [ ] T022 Code Quality: Ensure all new code adheres to the established coding style guides.
- [ ] T023 Testing Standards: Verify that all new features are accompanied by unit and integration tests and that code coverage is at least 80%.
- [ ] T024 Performance Requirements: Ensure that all new code has been benchmarked and meets performance targets.

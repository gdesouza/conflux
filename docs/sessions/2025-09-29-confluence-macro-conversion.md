# Session: Confluence Macro Conversion
**Date**: 2025-09-29
**Duration**: ~X hours
**Participants**: User, Gemini

## Objectives
- Implement conversion for various Confluence macros to markdown format.
- Identify and fix issues with existing macro conversions.

## Key Decisions
- Used regex-based preprocessing to convert Confluence macros to HTML tags that `html-to-markdown` can handle, or directly to markdown.
- Implemented specific patches after `html-to-markdown` conversion to fix issues like escaped underscores in image URLs and `&gt;` in blockquotes.
- Decided to ignore `inline-comment-marker` macros for now.

## Implementation Summary
- Modified `cmd/conflux/commands/get_page.go` to include a `preprocessConfluenceMacros` function.
- This function handles `toc`, `info`, `note`, `inline-comment-marker`, `view-file`, and `code` macros.
- Added post-processing patches in `generatePageOutput` for image URLs and blockquotes.

## Technical Details

### Modified Components
- `cmd/conflux/commands/get_page.go`: Added `preprocessConfluenceMacros` function and modified `generatePageOutput`.

### Files Modified/Created
- `cmd/conflux/commands/get_page.go` - Added `preprocessConfluenceMacros` function, modified `generatePageOutput` to call it and added post-processing patches.
- `docs/sessions/2025-09-29-confluence-macro-conversion.md` - This session summary.

## Tests Added
- Manual verification by running `conflux get-page` with `--format markdown` on various pages.

## Configuration Changes
None.

## Documentation Updates
None.

## Lessons Learned
- Parsing complex HTML/XML with regex can be challenging and fragile.
- Interacting with external libraries like `html-to-markdown` requires careful handling of input and output to avoid unexpected escaping or formatting issues.
- Debugging environment-specific issues (like the `undefined` errors with `goquery`) can be time-consuming and may require workarounds.

## Known Issues/TODOs
- The `inline-comment-marker` macro is currently ignored.
- The `collapse` parameter for code blocks is not handled.
- The regex-based approach might be fragile and could break with future changes in Confluence's storage format. A more robust HTML parsing library would be preferable if the environment issues are resolved.

## Next Steps
- Consider implementing handling for `inline-comment-marker` and `collapse` parameters.
- Explore alternative HTML parsing methods if the `goquery` issues can be resolved.

## Related Commits
- (Pending) Commit for Confluence macro conversion.
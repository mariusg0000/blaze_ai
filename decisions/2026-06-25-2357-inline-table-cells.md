# Session Decision Summary: Inline Table Cell Rendering

Date: 2026-06-25 23:57
Base commit: 243098c

## Context
The console renderer was displaying Markdown pipe tables, but inline markers inside table cells such as `**Main**` and inline code were being shown raw. The user confirmed the issue came from the table-rendering branch, not from the general bold renderer.

## Changes Made
- Updated the table-row rendering path in `internal/console/console.go` so each table cell now passes through `renderInline()` before being joined for display.
- Left the existing table-row layout intact; only the cell content rendering changed.

## Decisions And Rationale
- The smallest safe fix was to reuse the existing inline Markdown renderer for table cells instead of introducing a new table-specific formatter.
- This preserves the current console behavior while removing raw Markdown markers from table content.

## Implementation Approach
- In the `isTableRow` branch, the code now maps `renderInline()` across all cells returned by `splitTableRow()`.
- The build was validated after the change.

## Files Included
- `internal/console/console.go`: render inline Markdown inside pipe-table cells.
- `decisions/2026-06-25-2357-inline-table-cells.md`: this summary.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.

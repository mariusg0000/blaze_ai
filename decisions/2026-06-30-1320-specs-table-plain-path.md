# Session Decision Summary: specs table plain path

Date: 2026-06-30 13:20
Base commit: 27a8e58

## Context
The generated `specs.md` index table was using Markdown links with identical display text and URL (`[specs/01-architecture.md](specs/01-architecture.md)`). Redundant and noisy.

## Changes Made
- Updated `skills/specs-manager.md` template to use bare relative paths (`specs/01-architecture.md`) instead of Markdown links.

## Files Included
- `skills/specs-manager.md`: template fix
- `decisions/2026-06-30-1320-specs-table-plain-path.md`: this summary

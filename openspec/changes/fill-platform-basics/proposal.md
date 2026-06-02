## Why

GCFeed already has product, architecture, engineering, UI/UX, optimization, and module documents, but the information was spread across overlapping files. This change establishes a clean documentation baseline and records the long-lived platform rules in OpenSpec.

## What Changes

- Consolidate README into a project entry with startup commands and documentation map.
- Make `docs/product.md` the single product scope and feature status source.
- Merge engineering standards into `docs/engineering.md`.
- Rename the performance topic to `docs/optimization.md`.
- Normalize module documents under `docs/modules/`.
- Add `openspec/project.md` and a platform-basics capability spec.

## Capabilities

### New Capabilities

- `platform-basics`: Project-level documentation, engineering, module, and OpenSpec baseline for future GCFeed changes.

### Modified Capabilities

None.

## Impact

- Affected documentation: `README.md`, `docs/product.md`, `docs/quickread.md`, `docs/uiux.md`, `docs/modules/*.md`, `docs/engineering.md`, `docs/optimization.md`.
- Removed duplicate documentation: `docs/sdd.md`, `docs/modules/standard.md`, `docs/system optimization.md`.
- Affected OpenSpec files: `openspec/project.md`, `openspec/changes/fill-platform-basics/*`.
- No application runtime behavior changes.

## Context

The project had valuable documentation, but responsibilities overlapped. README contained detailed product tables, `docs/sdd.md` and `docs/modules/standard.md` both described engineering rules, and the performance topic used a filename with a space. OpenSpec also had a change directory without concrete artifacts.

## Goals / Non-Goals

**Goals:**

- Establish a small set of clear documentation entry points.
- Preserve existing product, architecture, engineering, UI, and module knowledge.
- Make module documentation easier to scan and extend.
- Add an OpenSpec project context and a baseline capability spec.
- Keep the change limited to documentation and OpenSpec artifacts.

**Non-Goals:**

- Implement new backend or frontend functionality.
- Change API behavior.
- Change database schema.
- Commit or push repository changes.

## Decisions

- README is the project entry and links to focused documents.
- Product status lives in `docs/product.md`.
- Engineering standards live in `docs/engineering.md`.
- Feed performance and stability content lives in `docs/optimization.md`.
- Module docs use the same section pattern: responsibilities, APIs, data tables, business rules, tests, frontend touchpoints.
- OpenSpec `platform-basics` captures the documentation and engineering baseline as a long-lived capability.

## Risks / Trade-offs

- Some older detailed prose was compressed to reduce duplication.
- The renamed optimization and engineering files require links to be updated.
- Planned modules remain specification-level until implementation changes add code and tests.

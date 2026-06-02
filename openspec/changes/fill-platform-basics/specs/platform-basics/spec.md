## ADDED Requirements

### Requirement: Documentation Entry Points

GCFeed SHALL maintain a concise project documentation map that separates project startup, product scope, architecture, engineering standards, optimization guidance, UI/UX, and module design.

#### Scenario: Reader starts from README

- **WHEN** a reader opens `README.md`
- **THEN** they can find startup commands and links to the focused documentation entry points

#### Scenario: Reader checks feature status

- **WHEN** a reader needs current product scope or P0/P1 status
- **THEN** `docs/product.md` provides the module map and feature status

### Requirement: Engineering Baseline

GCFeed SHALL keep backend, frontend, API, persistence, testing, and documentation conventions in `docs/engineering.md`.

#### Scenario: Developer adds a backend module

- **WHEN** a developer adds a backend module
- **THEN** they can follow the Domain, Application, Infrastructure, Interfaces layering rules in `docs/engineering.md`

#### Scenario: Developer verifies a change

- **WHEN** a developer completes a relevant code or documentation change
- **THEN** the project provides verification commands for OpenSpec, Go tests, and Web build

### Requirement: Module Documentation Template

GCFeed module documents SHALL follow a common structure covering responsibilities, APIs, data tables, business rules, testing guidance, and frontend touchpoints.

#### Scenario: Reader opens a module document

- **WHEN** a reader opens a document under `docs/modules/`
- **THEN** they can scan the module responsibilities, API surface, persistence model, rules, tests, and frontend integration points

### Requirement: OpenSpec Project Baseline

GCFeed SHALL include OpenSpec project context and a platform-basics capability for future spec-driven changes.

#### Scenario: Agent prepares an OpenSpec change

- **WHEN** an agent or developer prepares a new OpenSpec change
- **THEN** `openspec/project.md` describes the project purpose, stack, architecture, documentation map, conventions, and verification commands

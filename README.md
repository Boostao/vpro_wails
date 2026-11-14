# VPro 2.0 Desktop (`vpro_wails`)

Modern cross-platform Biogeoclimatic Ecosystem Classification (BEC) plot, classification, 3D geospatial, diagnostic synthesis, and phytosociological reporting workstation built with **Go 1.25+**, **Wails v3**, **Svelte 5**, **SQLite (`VP08`, `WAL`)**, and vendored **`libmdb` (`mdbtools` 1.0.1)** — eliminating Microsoft Access runtime constraints across Windows, macOS, and Linux.

## Authoritative Architecture & Specification ([`okf/`](./okf/00_index.md))

All domain specifications, historical forensics, parity matrices, and architecture blueprints live in-tree under [`okf/`](./okf/00_index.md):

- **Master Manifest:** [`okf/00_index.md`](./okf/00_index.md)
- **System Architecture (`4`-Layer Go DAG & `12` Wails Services):** [`okf/02_present_state/system_architecture.md`](./okf/02_present_state/system_architecture.md)
- **Capability & Triage Matrix (`v2.0.1` + `vpro` `v2.0.0`):** [`okf/02_present_state/capability_matrix.md`](./okf/02_present_state/capability_matrix.md)
- **Canonical `VP08` Data Dictionary & 2026 Standards:** [`okf/02_present_state/data_dictionary.md`](./okf/02_present_state/data_dictionary.md)
- **Architecture, Automation & Delivery Blueprints:** [`okf/03_future_architecture/`](./okf/03_future_architecture/)
  - [`transition_and_adoption.md`](./okf/03_future_architecture/transition_and_adoption.md) — Zero-friction UX continuity, universal datasheet grid, dual-pane tree, `545 MB` `BECMaster` qualification
  - [`analytics_and_governance.md`](./okf/03_future_architecture/analytics_and_governance.md) — In-app MapLibre GL JS v6 + WebGPU + offline `PMTiles`, GeoPackage, official TurboVeg 3 SQLite + `Plot_package` XML, R `.rds`, Cloud `BECMaster`
  - [`intelligent_automation_and_abstractions.md`](./okf/03_future_architecture/intelligent_automation_and_abstractions.md) — `Auto-1`..`Auto-7` (`LMH25` rules, reactive BEC Diagnostics, 3-way merge, `LumpSpec`, R sidecar `climr -> aSMR`)
  - [`documentation_help_and_release_portal.md`](./okf/03_future_architecture/documentation_help_and_release_portal.md) — Single-source offline `F1` `LMH25` contextual help, 4 spotlight tours, BC Gov Design System portal
  - [`target_stack_recommendation.md`](./okf/03_future_architecture/target_stack_recommendation.md) — Federated Go/Wails/Svelte 5 + headless `vpro` R SDK + Cloud Postgres/Quarto rationale
- **Citations & Decision Register:** [`okf/04_evidence_and_sources/citations_index.md`](./okf/04_evidence_and_sources/citations_index.md) · [`okf/04_evidence_and_sources/decisions_and_gaps.md`](./okf/04_evidence_and_sources/decisions_and_gaps.md)

*(Historical October 2026 slice-by-slice parity ledgers are archived under [`archive/oct2026-parity-ledgers/`](./archive/oct2026-parity-ledgers/) for provenance reference only.)*

## Build, Test & Development

### Prerequisites
Ensure Node `22.12+`, Go `1.25+` (`CGO_ENABLED=1`), and `wails3` (`v3.0.0-beta.26`) are on `PATH` (plus GTK4/WebKitGTK 6.0 dev packages on Linux or MinGW/w64devkit on Windows).

```bash
# Development mode (native Wails window + Vite on port 9245)
wails3 dev

# Regenerate TypeScript interface bindings (MUST include -i)
wails3 generate bindings -ts -i

# Default SQLite-only desktop builds
wails3 task linux:build EXTRA_TAGS=sqliteonly
wails3 task windows:build:sqliteonly
```

### Validation Suites
```bash
# Frontend unit tests, typecheck & Vite build
(cd frontend && npm test && npm run check && npm run build)

# Backend Go tests (SQLite-only profile)
go test -tags sqliteonly ./...
```

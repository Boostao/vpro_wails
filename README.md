# VPRO 2.0 Desktop (`vpro_wails`)

Modern cross-platform Biogeoclimatic Ecosystem Classification (BEC) plot, classification, geospatial, and phytosociological reporting workstation built with **Go 1.25**, **Wails v3**, **Svelte 5**, **SQLite (`VP08`)**, and vendored **`libmdb` (`mdbtools` 1.0.1)** — eliminating Microsoft Access runtime constraints.

## Authoritative Architecture & Specification (`okf/`)

All domain specifications, historical forensics, parity matrices, and target-state blueprints live in the **Operational Knowledge Framework**:

- **Master Manifest:** `C:\Users\BrunoTremblay\Work\VPRO_ACCESS\okf\00_index.md`
- **Capability & Triage Matrix:** `C:\Users\BrunoTremblay\Work\VPRO_ACCESS\okf\02_present_state\capability_matrix.md`
- **Canonical `VP08` Data Dictionary:** `C:\Users\BrunoTremblay\Work\VPRO_ACCESS\okf\02_present_state\data_dictionary.md`
- **Future Architecture & Blueprints:** `C:\Users\BrunoTremblay\Work\VPRO_ACCESS\okf\03_future_architecture\`
  - `transition_and_adoption.md` — Zero-friction UX continuity, universal datasheet grid, dual-pane tree
  - `analytics_and_governance.md` — In-app MapLibre GL JS v6 + WebGPU + offline `PMTiles`, GeoPackage, TurboVeg 3 + XML, R `.rds`
  - `intelligent_automation_and_abstractions.md` — `LMH25 (2023)` `rSMR/rSNR` rules, reactive Diagnostic synthesis, 3-way merge
  - `documentation_help_and_release_portal.md` — Offline `F1` `LMH25` contextual help, guided tours, BC Gov Design System portal
  - `target_stack_recommendation.md` — Federated Go/Wails/Svelte 5 + headless `vpro`/`BEC_R` architecture

*(Historical October 2026 slice-by-slice parity ledgers are archived under `archive/oct2026-parity-ledgers/` for provenance reference only.)*

## Build, Test & Development

### Prerequisites (Windows Host)
Ensure Node `22.12+` (`C:\Users\BrunoTremblay\tools\node-v22.14.0-win-x64`), Go `1.25+`, MinGW/w64devkit (`C:\Users\BrunoTremblay\tools\w64devkit\bin`), and `wails3` (`v3.0.0-beta.26`) lead on `PATH`.

```powershell
# Development mode (native Wails window + Vite on port 9245)
wails3 dev

# Regenerate TypeScript interface bindings (MUST include -i)
wails3 generate bindings -ts -i

# Default SQLite-only desktop build
wails3 task windows:build:sqliteonly
```

### Validation Suites
```powershell
# Frontend unit tests, typecheck & Vite build
Push-Location frontend
npm.cmd test
npm.cmd run check
npm.cmd run build
Pop-Location

# Backend Go tests (SQLite-only profile)
go test -tags sqliteonly ./...
```

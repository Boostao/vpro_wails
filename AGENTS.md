# VPRO 2.0 Engineering & Multi-Agent Instructions (`vpro_wails`)

## 1. Authoritative Specification & Product Direction

- **Single Source of Truth:** Read `~/Work/VPRO_ACCESS/okf/00_index.md` first, together with:
  - `okf/02_present_state/capability_matrix.md` & `data_dictionary.md`
  - `okf/03_future_architecture/transition_and_adoption.md`
  - `okf/03_future_architecture/target_stack_recommendation.md`
  - `okf/03_future_architecture/analytics_and_governance.md` (In-App MapLibre v6 + WebGPU + `PMTiles`, GeoPackage, TurboVeg 3 + XML, R `.rds`)
  - `okf/03_future_architecture/intelligent_automation_and_abstractions.md` (`Auto-1`..`Auto-7`)
  - `okf/03_future_architecture/documentation_help_and_release_portal.md` (Offline `F1` `LMH25` help, guided tours, BC Gov Design System portal)
  - `okf/04_evidence_and_sources/decisions_and_gaps.md`
- **Historical Archive Only:** Superseded October 2026 slice-by-slice parity ledgers (`MIGRATION_PLAN.md`, `WINDOWS_HANDOFF.md`, `CLIENT_SCOPE.md`, `FEATURE_CAPABILITIES.md`, `*_CONTRACT.md`) are archived in `archive/oct2026-parity-ledgers/` strictly for historical lookup. **They do NOT govern scope, feature gating, or work organization.**
- **Feature Graduation & Anti-Drift Rules:**
  - Graduate validated workflows to **default-ON** in standard `-tags sqliteonly` builds (`PRAGMA journal_mode = WAL`). Do **not** introduce new `VITE_*` or `VPRO_*` feature-flag sprawl; replace per-slice env flags with a clean `config.yml` `release_channel: stable | experimental` setting.
  - Follow Will MacKenzie's verified `VPro2.0_Functions` triage (`Red = Remove`, `Orange = Deferred`, `Green/Unmarked = Priority Retain/Modify`): remove Red-marked over-builds (`PC-ORD Break Report` and standalone `CHARS` forms/menus) and expose CHARS (`Cover5a..5c`, `SnowCoverregime`, `ActiveLayerDepth`) via a single `"Show extended shrub layers"` toggle on standard FS882 / 2-Page / SIVI forms.
  - Consolidate the flat root Go package and 86 granular Wails bindings toward cohesive domain packages/services (~12 services) while preserving existing behavior and test coverage.
  - Product identity is **`VPro`** (*VENUS — Vegetation and Environment NexUS — Professional*, **B.C. Ministry of Forests, Research Branch — Biogeoclimatic Ecosystem Classification (BEC) Program**), never `"My Company"`, `"My Product"`, `"Wails Edition"`, or `vpro-wails.app`.
  - Release packages are **unsigned for Phase 1** (relying on static `release-manifest.json` + `SHA256SUMS.txt` verification); `VTrees` / standalone Mensuration, CWD, and WHA remain **out of scope** for Phase 1.

### 1.1 Surrounding Workspace & Sibling Asset Map (`~/Work/`)

**IMPORTANT DISCOVERY RULE:** Workspace `glob` and `grep` tools default solely to `~/Work/vpro_wails`. Always pass explicit absolute `paths` (expand `~` to `/home/$USER`) when inspecting the sibling directories below, and **always include relevant sibling paths in subagent prompts**. Never mutate legacy `.accdb`/`.mdb`/`.zip` source archives (inspect read-only or copy to a temporary directory first):

1. **Authoritative Knowledge Repository (`okf/`):** `~/Work/VPRO_ACCESS/okf/` (start at `00_index.md`; 17 Markdown files across `01_history_and_evolution/`, `02_present_state/`, `03_future_architecture/`, `04_evidence_and_sources/`).
2. **Legacy Manuals, Triage, Emails & `VPro64` Exports:** `~/Work/VPRO_ACCESS/`
   - Field Manuals: `LMH25_2023_web.pdf` (2023 ed.) & `Lmh25_2015.pdf` (2015 ed.), `VProXP_Userguide.pdf`
   - Client Scope & Triage: `VPro2.0_Functions to remove-retain-modify.pdf` (`.docx`), `_BEC_data_system_fs1a_schedule_of_services_v2 (13).md`, `VproMailExchange/` (106 emails incl. executed contract `RE26OCF433` in `0072`), `SCREENCAPS/` (21 VPro 64 PNGs)
   - Decompiled Access Forms/VBA/Tables: `VPro64_forAI/`, `VPro64/`, `VLists/`, `VUser/`, `VMetaData/`, `Diagnostic10.xla`
3. **545 MB Production Scale Dataset (`BECMaster`):** `~/Work/VPRO_ACCESS_1/VPro64/`
   - `BECMaster_fixing.accdb` (`71,567` plots, `1,651,842` veg rows, `597,277` audit rows, `BECMaster_fixing_edatopic` `57,023` rows, `spatial_overlay_BGCv13_4/8`), `Haida_Gwaii_2019.accdb`, `Manning_TEM_Vpro19.accdb`
4. **Fresh 2026 Official `BECWeb` Master Standards:** `~/Work/becweb_standards_2025_2026/`
   - `BECdb_v13_2026.zip` (`BECdb_v13_2026.accdb` `2026-05-28` Version 13 + `Current_BGCunits_v13.csv` + `Current_Unique_SiteUnits_v13.csv`)
   - `BCPlantTaxa2025_access.zip` (`BCPlantTaxonomy.accdb` `2026-02-09` Version 19 `USysAllSpecs`) + `Species_Readme_Feb_2025_Ver19.docx` + `Tree_codes_2021.docx`
5. **Staged BC Gov Spatial Layers:** `~/Work/spatial_assets/`
   - `bc_ecosections_epsg4326.geojson` (`139` polygons), `bc_nr_districts_epsg4326.geojson` (`23` polygons), plus live unauthenticated BC OpenMaps WFS `pub:WHSE_FOREST_VEGETATION.BEC_BIOGEOCLIMATIC_POLY` (`17,870` BGC polygons)
6. **Cloned MoF & BCGov R / Quarto / Vector-Tile Reference Repos:**
   - `~/Work/MoF-Skeena-Research/`: `BEC_R_working` (`_functions/*.R`), `BEC_Classification_Analysis`, `BEC_Unit_Reporting` (`unit.report.qmd`), `aSMR-x-rSMR` (`CMD -> aSMR`), `EdatopicGrid_Spp_Distribution`, `BEC_Clean_BECdb`, `BEC_soils_fixing`, `BGC_Postgres`, `CCISS_VectorTiles`
   - `~/Work/bcgov_ref/`: `BEC_ReportR`, `cciss-docs` (Quarto book reference)
7. **Sibling Headless R Package & MDB Reader:**
   - `~/Work/vpro/` (`master` headless R/DuckDB/SQLite package + git commits `68b7d8b` `R/mod_becweb_map.R` / `scripts/publish_dataset.R` and `fde9833` `bcgov/climr`)
   - `~/Work/go-mdbtools-windows-readiness/` (optional CGO `libmdb` upstream hardening reference; copy it when needed)

---

## 2. Local Linux Toolchain & Build Commands

Fedora toolchain (ensure `~/go/bin` and `~/.local/bin` are on `PATH`):

- **Node 22:** `node` and `npm` from the system `PATH`.
- **Go & CLIs:** `go` plus `~/go/bin` (`wails3` v3.0.0-beta.26, `task`, `nfpm`) and `~/.local/bin` (`pmtiles`, `tippecanoe`).
- **GitHub & Quarto CLIs:** `gh` and `quarto` on `PATH`.
- **CGO & Windows x64:** native `gcc`, `x86_64-w64-mingw32-gcc`, and `makensis`; Wails defaults to GTK4 and WebKitGTK 6.0 development packages, with GTK3 and WebKitGTK 4.1 required for the legacy `gtk3` build tag.
- **R Runtime:** `Rscript` on `PATH` with the headless `vpro` and `climr` packages.

Key commands:
- **Regenerate Wails TypeScript bindings:** Always pass **`-ts -i`** (`wails3 generate bindings -ts -i`) to preserve interface-style nullable-array contracts (`-ts` alone emits classes and breaks frontend callers).
- **Default Desktop Tests (SQLite canonical):** `go test -tags sqliteonly ./...`; use `wails3 task linux:build EXTRA_TAGS=sqliteonly` for a native Linux build and `wails3 task windows:build:sqliteonly` for the Windows target when its toolchain is available.
- **Frontend Validation:** In `frontend/`, run `npm test`, `npm run check`, and `npm run build`.
- **CGO Rebuild Note:** After changing C headers/sources nested under an amalgamating CGO file (`internal/accessreader`), force a fresh compile with `-a` (`-count=1` alone can reuse cached C objects). For full-repo `-race` runs on Windows, pass `-timeout 45m` (or `-timeout 150m` across all packages).

---

## 3. Multi-Agent Parallel Orchestration

- Track multi-wave execution in the session SQLite `todos` and `todo_deps` tables and continue autonomously across waves until all deliverables are verified.
- **Parallel Subagents Encouraged with Worktree Isolation:** Fan out independent tracks concurrently using `general-purpose` subagents, giving **each mutating subagent its own isolated `git worktree`** (`../vpro_wails.worktrees/waveN-<track>`) and strictly disjoint file ownership (especially around `frontend/bindings/` and shared root state).
- Use `task` subagents to run verbose Go/Node/`svelte-check` suites and `code-review` / `rubber-duck` subagents as read-only quality gates before merging each wave into `main`.

---

## 4. Preserved Technical & Domain Invariants (Must Not Regress)

1. **Canonical SQLite Family & Context Isolation (`sqlitecontext.go`):**
   - Preserve the 7-database SQLite family (`<Project>.db`, `VPro64.db`, `VLists.db`, `VUser.db`, `VMetaData.db`, `VMessageBoard.db`, `VPics.db`), `_table_metadata` (`'VP08'`), and `config.yml` (with lossless legacy JSON migration). Embedded reference `.db` files are read-models, not replacements for `VLists.db` / `VUser.db`.
   - Build SQLite `file:` URIs from resolved absolute paths with proper `net/url` escaping (`#` in raw paths can truncate `mode=ro`!).
   - On the pinned `:memory:` read coordinator, use caller-owned read snapshot cleanup (`beginReadSnapshot`) so cancelled queries never drop `ATTACH`ed databases or `TEMP` `USys*` views.
   - Scoped operations already holding the context's operation read lease must reuse it when borrowing pools rather than recursively re-acquiring `RWMutex.RLock` (prevents deadlock behind a queued context switch).
   - Every context switch mints a new 128-bit `contextID`; reject stale requests deterministically.
2. **Data Types, Validation & Access Calibration:**
   - **Mutations + `<Project>_Audit` + `__VPRO_*` Identity Reservations** share a single atomic SQLite transaction (`BEGIN IMMEDIATE`). Never reuse deleted child IDs.
   - **Access `BOOLEAN`:** Normalize reads explicitly and preserve `true = -1`, `false = 0` in SQLite storage and audit rows.
   - **IEEE-754 `SINGLE` (`float32`) Cover Narrowing:** Narrow `float64` inputs to `float32` *before* evaluating `<100 Or Is Null` (`99.999996` narrows to `99.99999237060547` and passes; `99.999999` narrows to `100` and rejects).
   - **ACE Fixed-Decimal Rounding:** Format `DOUBLE` to 15 significant decimal digits before half-away rounding (`1.005 -> 1.01` at 2 decimals; reuse the calibrated decimal formatter in Plot Profiling and reports).
   - **UTF-16 Bounds & Malformed Text:** Enforce Access field lengths in UTF-16 code units (`PlotNumber <= 7`, `Species <= 8`, `Horizon <= 8`, `Texture <= 4`, `Colour <= 14`). Reject malformed UTF-8 (`CAST(X'FF' AS TEXT)`) and duplicate/raw-surrogate JSON keys prior to Go JSON repair.
   - **Historical Field Tolerance:** Omit unchanged historical out-of-rule columns on `UPDATE` so legacy plots can be edited without failing on untouched fields.
   - **Literal Scoped Plot Lookup:** Use `typeof(col)='text'` and `CAST(col AS BLOB)=CAST(? AS BLOB)` so SQLite numeric affinity never coerces leading-zero plot numbers (e.g., `'00337'`).
   - **Species Lifeform Widths:** `VLists.USysAllSpecs.Lifeform` is `INT32` (`LONG`) while `VUser.USysUserSpp.LifeForm` is `INT16` (`INTEGER`); preserve explicit projection rules and `NotInList` synonym (`OldCode -> Code`) vs. personal species decisions.
   - **Soil Horizon Ordering & `LMH25` Depth Sign:** Upland `L/F/H` humus horizons measure upward from `0 cm` (`UpperDepth > LowerDepth`, ordered `UpperDepth DESC`); wetland `Of/Om/Oh` and mineral `A/B/C/R/W` measure downward (`UpperDepth < LowerDepth`, ordered `UpperDepth ASC`).
3. **Frontend Svelte 5, Wails IPC & Excelize Guards:**
   - Detach Svelte 5 deep-reactive proxies with `$state.snapshot(...)` (or store immutable DTOs in `$state.raw`) before passing to Wails bindings or `structuredClone`.
   - Draft validation errors must survive tab remounts and block Save/Lock/context-switch/window-close (`CloseService` cryptographic nonce handshake `"vpro:close-request"` $\rightarrow$ `ConfirmClose` / `CancelClose`). Wait for explicit `CancelClose` acknowledgement in lifecycle tests before mutating barriers.
   - Use `strings.EqualFold` (matching `excelize/v2`) when checking worksheet name collisions, and validate case-folded joined grouping keys *before* threshold filtering in reports.
   - With `gopkg.in/yaml.v3`, keep `type configValues = map[string]any` as a type alias so nested map assertions stay `map[string]any`.
- After awaited client-side result validation, recheck the request generation
  before assigning UI state. Large XML validation must yield so Cancel/remount/
  close guards can run; test a held validation yield in actual Wails, release it
  after remount and prove the old result cannot populate the new panel.
- Frontend mocks must match the production wire, not just their validator.
  Share a strict-Go round-trip fixture for boundaries such as metadata columns
  (`declaredType`, not `type`). Go tests can resolve test-only package helpers;
  also compile production and inspect binding-generation warnings.
- Large Wails requests use UTF-8 byte chunks; a native fetch guard that examines
  only string bodies can miss a successful Save. Reassemble chunks by their
  upload ID and hold the final actual response. If capture was missed, inspect
  owned physical history/data first and never replay the completed mutation.
- Native COM wrappers must keep HWND/state outside native interface memory and
  pass the receiver plus every argument; a compiling public binding can still
  crash in Release, as go-common-file-dialog v1.2.0 did.
  Interpret HRESULT as signed32, classify normal Cancel only from Show's
  ERROR_CANCELLED, and remember that IUnknown.Release returns a reference count,
  not HRESULT. Balance successful STA initialization after releasing interface/
  path resources; fake-dialog tests do not replace actual owned native Cancel
  and literal-selection acceptance.

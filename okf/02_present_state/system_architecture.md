# Present State System Architecture (`vpro_wails` & `vpro`)

> **Navigation:** [Master Index](../00_index.md) | **Present State:** [Domain Model](./domain_model.md) · [Core Workflows](./core_workflows.md) · `System Architecture` · [Capability Matrix](./capability_matrix.md) · [Data Dictionary](./data_dictionary.md) | **Related:** [Legacy Architecture](../01_history_and_evolution/legacy_architecture.md) · [Target Stack Recommendation](../03_future_architecture/target_stack_recommendation.md)

---

## 1. Dual-Track Ecosystem Topology

Following the October–November 2026 architectural realignment and multi-wave qualification (`VproMailExchange` `0017`, `2026-10-02`; `archive/oct2026-parity-ledgers/CLIENT_SCOPE.md`), VPro's production codebase operates as **two complementary, schema-compatible tracks** sharing an identical 7-database SQLite (`WAL` mode) family and `config.yml` contract:

1. **Primary Interactive Desktop Track — `boostao/vpro_wails` (`v2.0.1` @ `266921e`):** Cross-platform desktop workstation built on **Go `1.25.0`** (CI built with Go `1.26`), **Wails v3 (`v3.0.0-beta.26`)**, **Svelte 5 (`^5.46.4`, Runes mode)**, **MapLibre GL JS (`^6.18.0`) + `deck.gl` (`^9.4.1`) / `luma.gl` WebGPU + `pmtiles` (`^4.5.0`)**, **Tailwind CSS v4 (`^4.3.3`)**, **CGO SQLite (`github.com/mattn/go-sqlite3` `v1.14.52`, `-tags sqliteonly` default)**, and an embedded C amalgamation of **`mdbtools` `1.0.1` (`libmdb`)** compiled across `windows`, `linux`, and `darwin` (`go.mod:1-15`, `frontend/package.json:63-91`, `internal/accessreader/reader.go:1-16`).
2. **Headless R Statistical & Companion SDK Track — `boostao/vpro` (`v2.0.0` @ `fae33d2`):** Clean-slate companion R package (`21` `R/*.R` files, `R >= 4.4.0`, zero-note `R CMD check`, qualified on 71,567-plot `BECMaster`) executing concurrent read-only SQLite WAL queries alongside the running desktop app via `RSQLite`/`data.table`/optional `duckdb`, providing community matrices, multivariate BEC clustering, `sf` GeoPackage export, `LMH25 (2023)` pedon validation, `climr` normals & Ministry `aSMR` modelling, dual edatopic grids, BEC diagnostics, guarded candidate `*_SU` write-back, and Quarto BEC ecosystem guides (`../vpro/DESCRIPTION:1-45`).

```mermaid
flowchart TB
    subgraph Shared["Shared Local Persistence Contract"]
        CFG["config.yml (release_channel: stable | experimental)<br/>(Replaces HKCU\...\VPro64 Registry & Legacy JSON)"]
        FAM["7-DB Canonical SQLite Family<br/>VPro64.db, VLists.db (2026), VUser.db,<br/>VMetaData.db, VMessageBoard.db, VPics.db"]
        PROJ["User Project SQLite Files (<Project>.db, WAL Mode)<br/>8 VP08 Core Tables + *_SU / *_Hierarchy / *_Lump / *_Profile"]
        MDB["Legacy MS Access (.mdb / .accdb) & VENUS 5.0<br/>VP05, VP06, VP07, VP08 + 545 MB BECMaster"]
    end

    subgraph Desktop["Primary Desktop App: boostao/vpro_wails v2.0.1 (4-Layer Go DAG + Wails v3)"]
        UI["Svelte 5 Runes Frontend (Vite 8 + Tailwind v4)<br/>24 Views · 118 Components · MapLibre v6 + WebGPU · F1 Help"]
        IPC["Wails v3 Typed IPC Bridge (-ts -i)<br/>Strictly 12 Domain Services + Close Nonce Handshake"]
        CTX["Dual-Connection SQLiteContext (internal/vprocore)<br/>1) Pinned :memory: RO Coordinator (beginReadSnapshot)<br/>2) Lazy rw+WAL c.projectDB Writer (FK=ON, busy_timeout=5000)"]
        CAT["2026 Embedded SHA-256 Read-Model Catalogues<br/>BECdb v13, BCPlantTaxonomy v19, LMH25 (2023), bc-overview.pmtiles"]
        CGO["internal/accessreader + accessimport (Win/Linux/macOS)<br/>Vendored libmdb 1.0.1 + Orphan Quarantine"]
        EXP["Interchange & Exporters (internal/interchange)<br/>TurboVeg 3 SQLite+XML, .gpkg, .rds, .xlsx, .pdf, .kml, .zip"]
    end

    subgraph HeadlessR["Headless R Companion SDK & Sidecar: boostao/vpro v2.0.0"]
        RSIDE["internal/rsidecar Out-of-Process Rscript Bridge"]
        RAPI["vpro 2.0.0 R SDK (R >= 4.4 + RSQLite WAL / DuckDB)<br/>climr -> aSMR, vegclust, Dual Edatopic Grids & Quarto Guides"]
    end

    UI <-->|"JSON IPC ($state.snapshot)"| IPC
    IPC --> CTX
    IPC --> CAT
    IPC --> CGO
    IPC --> EXP
    IPC <-->|"Quoted Rscript CLI"| RSIDE
    RSIDE --> RAPI
    CTX <--> CFG
    CTX -->|"mode=ro (net/url escaped)"| FAM
    CTX <-->|"RO Snapshots + RW WAL Tx"| PROJ
    CGO -->|"Streaming Read + Orphan Quarantine"| MDB
    CGO -->|"Atomic os.Link VP08 .db"| PROJ
    RAPI <-->|"Concurrent WAL RO + Guarded *_SU"| PROJ
    RAPI --> FAM
```

---

## 2. Verified Technology Stack (`vpro_wails` `v2.0.1`)

| Architectural Layer | Component / Module | Exact Version | Source Evidence |
|---|---|---|---|
| **Backend Language** | Go (`github.com/boostao/vpro-wails`, `CGO_ENABLED=1`, `-tags sqliteonly` default) | `go 1.25.0` (`1.26` in CI) | `go.mod:1-3`, `.github/workflows/release.yml:52` |
| **Desktop Shell & IPC** | Wails v3 (`github.com/wailsapp/wails/v3` + `@wailsio/runtime`, `wails3 generate bindings -ts -i`) | `v3.0.0-beta.26` | `go.mod:10`, `frontend/package.json:74` |
| **Primary SQL Engine** | CGO SQLite3 (`github.com/mattn/go-sqlite3`, `PRAGMA journal_mode = WAL`) | `v1.14.52` | `go.mod:9`, `internal/vprocore/sqlitecontext.go` |
| **Optional Analytical Engine** | CGO DuckDB (`github.com/duckdb/duckdb-go/v2`, omitted in standard `-tags sqliteonly` desktop builds) | `v2.10505.0` | `go.mod:6`, `build/windows/Taskfile.yml` |
| **Access Binary Reader** | Internal CGO `accessreader` wrapping vendored `mdbtools` (`libmdb`) on `linux \|\| windows \|\| darwin` | `1.0.1` (`-fvisibility=hidden`, Win `-static`) | `internal/accessreader/reader.go:1-16` |
| **Windows Native Shell/COM** | `github.com/go-ole/go-ole` (`IFileDialog` / `IShellItem` COM syscalls, balanced STA) | `v1.3.0` | `go.mod:7`, `internal/pictures/` |
| **Document & Interchange** | `excelize/v2` (`.xlsx`), `go-pdf/fpdf` + `LiberationSans` (`.pdf`), `golang.org/x/text`, `gopkg.in/yaml.v3` | `v2.10.1`, `v0.9.0`, `v0.39.0`, `v3.0.1` | `go.mod:8,11,14,15`, `resources/fonts/` |
| **Frontend & 3D GIS** | Svelte 5 (`^5.46.4`), `maplibre-gl` (`^6.18.0`), `@deck.gl/*` (`^9.4.1`), `@luma.gl/webgpu` (`^9.4.2`), `pmtiles` (`^4.5.0`) | Svelte 5 + MapLibre 6.18 | `frontend/package.json` |
| **CSS, Fonts & Icons** | Tailwind CSS v4 (`@tailwindcss/vite`), `@lucide/svelte`, IBM Plex Sans & Mono (`BC Sans` WOFF2 in `site/`) | `^4.3.3`, `^1.48.0`, `^5.3.0` | `frontend/package.json`, `site/fonts/` |
| **Bundler & Test Suites** | Vite 8 (`^8.0.5`), `svelte-check` (`^4.4.8`), Node 22 single-pass `frontend/scripts/run-tests.mjs` (`190` `.test.cjs` suites in `frontend/tests/`), Go test (`*_test.go`) | `^8.0.5` / Node `22` | `frontend/package.json`, `frontend/scripts/run-tests.mjs` |

---

## 3. Backend Go 4-Layer Acyclic Architecture (`431` Production `.go` Files)

### 3.1 Startup Lifecycle, Non-Destructive Recovery & Guarded Window Close
Unlike legacy `VPro64.accdb`—which silently resets or crashes when linked `.accdb` paths move (`VPro64_forAI/Modules/V7mdlSplash.txt:70-137`)—`vpro_wails/main.go` (`240` LOC) enforces deterministic startup, build-tier (`dev` | `alpha` | `release`), and shutdown contracts:

1. **Startup & Recovery Mode (`main.go:96-186`, `internal/lifecycle/startupservice.go`, `internal/lifecycle/buildinfo.go`):** Resolves `%APPDATA%\vpro\config.yml` and `%LOCALAPPDATA%\vpro\`, verifies SHA-256 hashes of all 5 support databases (`databasefamily.go:19-61`) and 8 embedded reference `.db` catalogues, and opens `SQLiteProjectService`. If `config.yml` or a configured external `.db` fails validation, `StartupService` enters non-destructive recovery mode so the user can relocate `.db` paths or materialize `Sample.db` **without overwriting `config.yml` without backup**.
2. **Cryptographic Nonce Window-Close Handshake (`closeservice.go:15-133`, `main.go:70-76,207-235`):** Prevents data loss from dirty forms or in-flight workbook publications when the OS window close button is clicked:

```mermaid
sequenceDiagram
    actor User
    participant OS as Native Window (Wails v3)
    participant CS as Go CloseService
    participant App as Svelte 5 App.svelte / CloseConfirm

    User->>OS: Click Window [X] or Quit
    OS->>CS: WindowClosing / ShouldQuit event
    CS->>OS: event.Cancel() (block immediate exit)
    CS->>CS: Mint 128-bit random prefix + monotonic nonce
    CS-->>App: Emit "vpro:close-request" (nonce)
    alt Clean state (no dirty editors or active publications)
        App->>CS: ConfirmClose(nonce)
        CS->>OS: Application.Quit()
    else Unsaved drafts or in-flight export
        App->>User: Render CloseConfirm.svelte modal
        alt User chooses "Discard & Close"
            User->>App: Confirm discard
            App->>CS: ConfirmClose(nonce)
            CS->>OS: Application.Quit()
        else User chooses "Stay & Save"
            User->>App: Cancel close
            App->>CS: CancelClose(nonce)
        end
    end
```

### 3.2 Dual-Connection `sqliteContext` & Stale-Tab Isolation (`sqlitecontext.go`)
Legacy Access rewrote shared `QueryDefs` on disk (`SetCurrentProject`), making multi-window/multi-user operation unsafe. `vpro_wails` replaces this with an isolated dual-connection design (`vpro_wails/sqlitecontext.go:56-444`, `vpro_wails/sqliteprojectservice.go:108-159`):

* **Connection 1 — Pinned `:memory:` Read Coordinator (`MaxOpenConns(1)`):**
  * Attaches `VPro64.db`, `VLists.db`, `VUser.db`, `VMetaData.db`, `VMessageBoard.db`, active `<Project>.db`, and optional `<SU>.db`, `<Hierarchy>.db`, `<Profile>.db` using SQLite URI **`mode=ro`**, verifying `os.SameFile` before and after `ATTACH DATABASE` to prevent TOCTOU path swapping (`sqlitecontext.go:95-248`).
  * Validates exact column counts, indexes, and `_table_metadata` (`'VP08'`) across all 8 project tables (`sqlitecontext.go:332-360`).
  * Synthesizes temporary `:memory:` SQL views (`Filtered_Env`, `USysEnv`, `USysVeg`, `USysVegA..D`, `USysHumus`, `USysMineral`, `USysOther`, `USysAuditTrail`, `USysMetadata`, `MasterSiteUnitList`, `USysSU`, `USysHierarchy`) (`sqlitecontext.go:363-444`).
* **Connection 2 — Lazy Read-Write Project Handle (`c.projectDB`):**
  * Opened on demand with `mode=rw&_foreign_keys=on&_busy_timeout=5000` (`MaxOpenConns(2)`) solely for atomic `BEGIN IMMEDIATE ... COMMIT` transactions writing `<Project>_Env`, `<Project>_Admin`, child tables, `<Project>_Audit`, and `__VPRO_*` receipt tables (`sqlitecontext.go:56-75`).
* **128-Bit `contextID` Epoch Fencing (`contextservice.go:70-100`):** Every project/SU/hierarchy switch mints a new 128-bit hex `contextID`. Every mutation and scoped query checks the caller's `contextID`, deterministically rejecting late IPC calls from stale UI views.

### 3.3 4-Layer Acyclic Package Decomposition & 12 Registered Wails Services
Wave 1C–1D consolidated the earlier 86 flat root Wails services into a strict **4-layer acyclic `internal/*` DAG** enforced by automated architectural tests (`internal/integration/architecture_dag_test.go`), registering **strictly 12 cohesive domain services** in `main.go:187-200` (`domainservices_*.go`):

* **Layer 0 — Embedded Assets & Foundations:** `resources` (`resources/embed.go` exporting embedded `embed.FS` / `[]byte` assets + `RepoRoot()`), `internal/sqlutil`, `internal/accessreader` (`linux || windows || darwin`), `internal/accessimport`, `internal/accessinventory`, `internal/accesslayout`, `internal/accesscompare`, `internal/fs882layout`, `internal/listcatalog`, `internal/reports/subzonematrix`.
* **Layer 1 — Core Context, Reference Catalogues & Shared Engines:** `internal/references` (`BECService`, `ReferenceService`, `Site/Soil/Parent/Geology/Region/Quality` code services), `internal/vprocore` (`SQLiteContext` with `beginReadSnapshot`, `DesktopConfig`, `report_preferences_handler.go`, 7-DB `DatabaseFamily` seeding `VPics.db` on cold boot, `ProjectService`, `ContextService`, `PlotService`), `internal/sourceform`, `internal/reports/reportcore`.
* **Layer 2 — Independent Domain Packages (zero sibling imports):** `internal/lifecycle`, `internal/plotentry`, `internal/sivi`, `internal/twopage`, `internal/datasheet`, `internal/classification`, `internal/lumping`, `internal/diagnostics`, `internal/lmh25rules`, `internal/projectops`, `internal/rsidecar`, `internal/validation`, `internal/vegreport`, `internal/envreport`, `internal/spatial`, `internal/pictures`, `internal/interchange`, `internal/cloudsync` (plus cross-domain `internal/integration`).
* **Layer 3 — Thin Wails Host (`package main` — 12 Bound Services):**

| # | Registered Wails Service (`main.go:187-200`) | Backing Domain Package(s) & Core Responsibilities |
|---:|---|---|
| `1` | **`LifecycleService`** | Startup activation/recovery hints, cryptographic close nonce (`CloseService`), `USysUserLog` review |
| `2` | **`ContextService`** | Active `Project`/`SU`/`Hierarchy` switching, 128-bit `contextID` fencing, `New/SaveAs/Splinter/Compare/Merge` (`internal/projectops`), scoped plot lookup |
| `3` | **`PlotEntryService`** | 6-tab FS882, Two-Page (`internal/twopage`), FS1333 SIVI (`internal/sivi`), inline CHARS toggle, `LMH25` live pedon/edatopic diagnostics (`internal/lmh25rules`), `ProjectMetadata` presets & auto-derivation |
| `4` | **`DatasheetService`** | Universal editable spreadsheet grid (`_Env`, `_Admin`, `_Veg`, `_Humus`, `_Mineral`, `_Other`, `_SU`), global search, TSV paste (`internal/datasheet`) |
| `5` | **`ClassificationService`** | Dual-pane `SU`/`Hierarchy` tree explorer, `Env <-> SU` sync, Plot Profiling, `Combine Species` (`*_Lump` + `LumpSpec`), reactive BEC Diagnostics (`internal/diagnostics`), R Sidecar (`internal/rsidecar`) |
| `6` | **`ValidationService`** | Batch `USysTableOfLists` picklist & numeric bounds compliance gate, 1:N FK integrity, `SpeciesCodeCheck`, transactional `OptimizeVeg` |
| `7` | **`ImportExportService`** | Access `VP05`–`VP08` (`545 MB` `BECMaster` streaming + orphan quarantine), `VENUS 5.0`, **official `TurboVeg 3` 31-table SQLite & `Plot_package` XML**, `.gpkg`, `.rds`, Long/Wide CSV & `.zip`, `User Species`, Label PDF |
| `8` | **`VegetationReportService`** | Summary & Long Vegetation previews and `.xlsx` workbooks, SiteUnit suffix exclusions, Lifeform & Species Attributes summaries |
| `9` | **`EnvironmentReportService`** | Long Environment, Site Unit Summary (mean/quartiles + qualitative), and Subzone Matrix previews & `.xlsx` workbooks |
| `10` | **`SpatialService`** | MapLibre v6 + `PMTiles` byte-range streaming, NAD83 `UTM <-> DD` geodetic QA, FlatGeobuf BGC/Ecosection/District auto-stamp, AOI $\rightarrow$ `*_SU`, KML 2.1 |
| `11` | **`PictureService`** | Cold-boot `VPics.db` seeding, plot photo browser, metadata editing, native OS file selection, creation/deletion/restoration |
| `12` | **`HelpService`** | Offline `F1` `LMH25 (2023)` contextual help (`resources/help_bundle.json`), 4 guided tours, shortcuts, `What's New`, safe `Reset Workspace to Sample`, `About & Standards Stamps` |

### 3.4 Native Access `.mdb`/`.accdb` CGO Ingestion Pipeline (`accessreader` & `accessimport`)
`vpro_wails` eliminates runtime dependence on Microsoft Access, Windows ACE/Jet ODBC drivers, or PowerShell COM by embedding a cross-platform fork of **MDB Tools `1.0.1` (`libmdb`)** compiled directly via CGO (`internal/accessreader/reader.go:1-40`):

```mermaid
flowchart LR
    File[".mdb / .accdb File<br/>(VP05..VP08 + 545 MB BECMaster)"] -->|"Read-Only Stream<br/>(Win/Linux/macOS)"| LibMDB["CGO internal/accessreader<br/>Vendored libmdb 1.0.1 C<br/>(-fvisibility=hidden, Win -static)"]
    LibMDB -->|"Sticky C Error Guards<br/>Nanosecond DATETIME"| Inspect["InspectAccessDatabase()<br/>Detect *_Env + Version Tag<br/>Count Rows & Check Eligibility"]
    Inspect --> Stage["internal/accessimport<br/>Streaming SQLite Staging<br/>VP05/06/07 -> VP08 + Orphan Quarantine"]
    Stage -->|"SHA-256 Source Check<br/>+ Companion Tables"| Validate["inspectFileContext()<br/>Verify 8 VP08 Tables,<br/>Indexes & Foreign Keys"]
    Validate -->|"Atomic Publish"| Dest["Canonical <Target>.db (WAL)<br/>+ Optional Context Switch"]
```

* **Key Hardening Guarantees (`internal/interchange/accessprojectimport.go`):**
  * Rejects embedded `U+0000` NUL truncation, corrupt Jet3/Jet4 memo/OLE page chains, and lossy character conversions via sticky C error codes (`GO_MDB_ERROR_*`).
  * Compiles cleanly across **Windows (`amd64`/`arm64`, `-static` MinGW linking with zero non-inbox PE DLLs), Linux (`-fvisibility=hidden` isolating vendored `fakeglib` from GTK4), and macOS Universal (`darwin`)**.
  * Upgrades **`VP05`**, **`VP06`**, and **`VP07`** to canonical `VP08`, streams **`545 MB` `BECMaster_fixing.accdb` (`71,567` plots, `1.65M` veg rows)** with automatic orphan-row quarantine so `PRAGMA foreign_keys = ON` succeeds cleanly, and imports companion `*_SU`, `*_Hierarchy`, `*_Lump`, and `*_Profile` tables.

---

## 4. Frontend Svelte 5 Architecture (`118` `.svelte` Components, `24` Views)

### 4.1 Reactive State, Session Machines & Bundle Splitting
* **Svelte 5 Runes + Class Session Machines:** Uses Svelte 5 Runes (`$state`, `$derived`, `$effect`, `$props`) paired with stateful TypeScript session classes cached across view switches so unsaved drafts and preview snapshots survive tab navigation and block context-switch/close barriers (`41a09f8`, `d630301`).
* **Proxy Detachment & Generation-Fenced Async Reads:** Before calling Wails v3 bindings, Svelte 5 deep proxies are stripped with `$state.snapshot(...)`, while `ReadRequests` (`readRequests.ts`) and post-await generation checks prevent stale async results from populating remounted views.
* **Vendor & Sub-Editor Code Splitting (`e9fdb9d`):** Report workbenches, FS882 sub-editors, and MapLibre/WebGPU vendor chunks are lazy-loaded via dynamic `import(...)` with Vite worker asset bundling (`16960a8`).

### 4.2 UI Component & View Hierarchy (`24` Active Views)

```mermaid
flowchart TD
    App["App.svelte (Root Shell & 24-View Router)"]
    Nav["Navigation.svelte (Home · Plots · 3D Map · Forms · Reports · Interchange · Help)"]
    Side["Left Sidebar (#project-context)<br/>Project / SU / Hierarchy Selectors · New/Attach .db · Import .mdb/.accdb · Reveal in Explorer"]
    Help["HelpDrawer.svelte (F1 LMH25) + OnboardingTour.svelte + CloseConfirm.svelte"]

    App --> Nav
    App --> Side
    App --> Help

    subgraph Editors["Data Entry, Datasheet & Classification Views"]
        FS882["FS882Form.svelte (6-Tab + SIVI + Inline CHARS Toggle)<br/>Site · Veg · VegOther · Soils · Other · Audit · siviParent"]
        TwoPage["TwoPageEntryEditor.svelte (Full Soil/Veg CRUD & Restore)"]
        Grid["UniversalDatasheetGrid.svelte (_Env/_Admin/_Veg/_Humus/_Mineral/_Other/_SU)"]
        ProjAdm["ProjectAdminWorkbench.svelte (New/SaveAs/Splinter/Compare/3-Way Merge/QA)"]
        ClassEx["ClassificationExplorer.svelte (Dual-Pane SU & Hierarchy Tree) + LumpEditor.svelte"]
    end

    subgraph Synthesis["3D Geospatial, Diagnostics, R Sidecar, Reports & Interchange"]
        Map3D["GeospatialWorkbench.svelte (MapLibre v6 + WebGPU + Offline PMTiles + AOI->SU)"]
        Diag["ClassificationSynthesis.svelte (Live Differential d/dd/cd/c/ic + Outlier Queue)"]
        RSide["RSidecarWorkbench.svelte (climr -> aSMR + Dual Edatopic + vegclust + Quarto)"]
        VegEnv["Long/Summary Veg & Env (.xlsx) · Lifeform · SubzoneMatrix · PlotLabel (.pdf)"]
        Hub["InterchangeHub.svelte (VENUS 5, TurboVeg 3 SQLite+XML, .gpkg, .rds, CSV .zip, KML)"]
    end

    App --> Editors
    App --> Synthesis
```

### 4.3 Graduated Release-Channel Configuration (`config.yml`)
Wave 1A (`c4cbd1a`) graduated all validated desktop workflows to **default-ON** in standard `-tags sqliteonly` builds, replacing the historical 79 `VITE_*` / 62 `VPRO_*` demo environment flags (`archive/oct2026-parity-ledgers/`) with a clean `release_channel: stable | experimental` setting in `config.yml` and pruning client-retired `CHARS` standalone forms and `PC-ORD Break Report` menus. Offline extraction tools under `cmd/` (`cmd/standards2026`, `cmd/accessinventory`, `cmd/fs882layout`, `cmd/*codefixture`) compile 2026 `BECWeb` standards and Access twip geometry into deterministic embedded assets under `resources/`.

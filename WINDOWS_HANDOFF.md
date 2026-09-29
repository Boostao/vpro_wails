# Windows VM continuation

## Start here

Read `AGENTS.md` and the migration ledger in `README.md`. This is a read-only Go/Wails/Svelte prototype, not a completed migration. Keep SQLite canonical and do not enable writes on production projects.

Clone `boostao/vpro_wails` beside `vpro` and `VPRO_ACCESS` if those references are available. Their paths are conventions, not requirements: locate the actual Access front end, linked backends, SaveAsText exports, and R source before running probes. Do not copy Linux absolute paths into Windows configuration.

The `go.mod` module currently uses `github.com/boostao/vpro-wails` (hyphen); the GitHub repository requested for this handoff is `boostao/vpro_wails` (underscore). Local Wails builds do not require a module rename. Generated binding paths follow the module path; do not rename it piecemeal.

## Current implementation

- Local project discovery, VP08 family validation, saved selection, Sample fallback, and bad-file diagnostics.
- Read-only project plots require Env/Admin membership; Sample has 52 plots.
- Same-file SU selection filters Env/SU/Admin membership; Sample has 51 plots. None or a project switch clears SU. Master SUs are disabled without authorization.
- Hierarchy selection is independent and stored by name plus source file. Sample has 43 nodes. Selection survives project switches; a missing source deactivates it on restart.
- No FS882 drafts, save/discard, child writes, audit restoration, Access import, or reports. Disabled menus must stay disabled until their actual workflows are implemented.
- `coordinator.go` is an optional runtime experiment, but DuckDB is currently a compiled dependency. No network extension install occurs at startup. Its test skips when the matching cached `sqlite_scanner` is absent; Windows extension packaging is unverified.

## Native verification

Use isolated `VPRO_DATA_DIR` and `VPRO_CONFIG_DIR` directories. Both are supported by the app; do not point them at real user data for experiments. Install the pinned Wails CLI from `go.mod`/`README.md`, Go, Node/npm, WebView2, and a compatible Windows C/C++ toolchain. SQLite and DuckDB use CGO: do not accept a CGO-disabled build as a functioning database app.

From PowerShell after setting the toolchain PATH:

```powershell
$env:CGO_ENABLED = '1'
$env:VPRO_DATA_DIR = Join-Path $PWD 'data'
$env:VPRO_CONFIG_DIR = Join-Path $PWD 'config'
Push-Location frontend
npm ci
npm run check
npm run build
Pop-Location
go test -race ./...
wails3 build CGO_ENABLED=1
wails3 dev
```

If the race runtime or DuckDB linking fails, record the compiler/architecture and exact failure; do not claim Windows support from Linux results. `build/config.yml` explicitly passes `CGO_ENABLED=1` to the dev child build. The Windows task defaults CGO to 0 unless explicitly supplied, so use the production command above rather than plain `wails3 build`. Windows native build and native binding interactions have not been verified in the Linux session.

Verify the native Wails window, not just `http://127.0.0.1:9245/`: Sample 52 plots, SU Sample 51, None 52, hierarchy Sample 43 nodes, restart persistence, project-switch SU reset and hierarchy retention. Browser preview cannot prove Go binding behavior.

## Access oracle: next task

The active R module `inst/app/modules/mod_fs882_6x4.R` identifies `FS882-6x4XL` as its Access source. Do not silently substitute `FS882-6x4`: the variants differ. The Linux session could not reach the VM (127.0.0.1:2222 refused).

Work only on disposable copies of BOTH the Access front end and all linked backends. Confirm the copies' actual linked-table paths before any edit; copying only the front end does not isolate data. Isolate registry settings or snapshot and restore them, because VPRO selection is persisted outside the database.

| Probe | Required evidence | Status |
| --- | --- | --- |
| Project selection and SU None | `V7mdlSetCurrent.SetCurrentProject`, main-menu Change/Click/GotFocus, `USysEnv` SQL and stored selection | Source traced; native Access observation needed |
| SU selection | `V7mdlShortCutToolBarCmds.SetCurrentSu`, Env/SU/Admin filter, blank/orphan handling | Source traced; native Access observation needed |
| Independent hierarchy selection | Source event, selected table and stored identity across project switch | R precedent only; Access observation needed |
| XL new plot | PlotNumber AfterUpdate/LostFocus, transient StartDate, Env/Admin/audit rows after each commit point | Missing |
| XL Save / Undo | Dirty record before and after explicit save, focus change, lock, cancel and selection change | Missing |
| Collision / failure | Duplicate key, failed Admin insert, first child insert, error and partial data effects | Missing |
| Audit and child grids | Exact XL subform sources, ID defaults, audit table labels, vegetation/soil/Other effects | Missing |

For each run record exact source form/procedure, fixture identity, inputs, observed row deltas and failure cases. Retain scripts and sanitized observations in the repository; keep full database copies and potentially identifying data under ignored `evidence/private/`. Source and R tests are precedents, not proof of Access runtime parity.

Linux `/tmp/vpro_parity/wails-fs882/` contained a non-XL spec and scaffold for comparison, plus a checkpoint. Those files do not travel with this repository. No XL spec/scaffold was generated: the skill generator script was absent. Regenerate on disposable export copies if tooling exists, or create a reviewed XL event/source contract from targeted reads. Never write generated artifacts into canonical `VPRO_ACCESS`.

## Work ownership and order

1. Main agent controls contracts, integration and native verification.
2. Read-only evidence agent traces one workflow; a Go implementation agent owns exclusive backend files; a Svelte agent owns exclusive frontend files after the service contract is agreed.
3. Finish activation diagnostics/external paths and FS882 draft/write contracts, then configuration/reference databases, import and reports. Never equate visible menu items with completed parity.
4. The separate `go-mdbtools` agent owns the embedded reader. Do not modify or duplicate its implementation. Request a stable API, supported Windows toolchain, null/binary/type behavior, and fixture results before integrating a transactional import boundary.

Linux baseline: Go race suite, frontend typecheck/build, and Wails production build passed. New selector interactions were not tested in the native window. Windows builds, Access probes and end-to-end write parity remain unverified.
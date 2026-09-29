# VPRO Desktop Migration

This is an **early, read-only migration slice**, not a replacement for the R/Shiny application. It uses Wails v3 beta.26, Go, Svelte 5, Tailwind CSS 4, and SQLite. Do not use it to edit production project data.

For the Windows Access-oracle continuation, start with [WINDOWS_HANDOFF.md](WINDOWS_HANDOFF.md). It records the current contracts, isolated native checks, pending probes, and separate MDBTools ownership.

## Run

Install Go, Node.js, npm, the Linux GTK4/WebKitGTK development dependencies (or the equivalent for your platform), and the Wails v3 CLI:

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.26
export PATH="$(go env GOPATH)/bin:$PATH"
wails3 dev
```

The native window connects to Vite at `http://127.0.0.1:9245/`. That URL in a regular browser is **UI preview only**; Go bindings work in the desktop window. For a release binary, use `wails3 build` and run `bin/vpro-wails`. Verification: `go test -race ./...` and `cd frontend && npm run check && npm run build`.

On first launch the packaged `Sample.db` is copied without replacing existing data to the VPRO user data `projects` directory. On Linux this defaults to `$HOME/.local/share/vpro/projects` unless `XDG_DATA_HOME` is set; set `VPRO_DATA_DIR` to use an isolated location. The desktop app stores its current project in `desktop-selection.json` under the platform user config directory (`$XDG_CONFIG_HOME/vpro` or `$HOME/.config/vpro` on Linux, `%APPDATA%/vpro` on Windows, and `~/Library/Application Support/vpro` on macOS). Set `VPRO_CONFIG_DIR` to override it. If no config selection exists, startup reads the former data-directory selection without deleting it; the next selection saves to the config directory. It does not yet synchronize that selection to the R application's `config.yml` or attach reference databases. Only complete VP08 SQLite project families in the data directory are selectable. The backend opens their data read-only.

## Migration Ledger

| Source behavior | Desktop status | Evidence / next boundary |
| --- | --- | --- |
| `inst/app/ui.R` navbar and sidebar | Partial | Home, project navigation, plot browser, and a read-only hierarchy browser work in `frontend/src/App.svelte`. Active Forms, Reports and Help destinations are listed but disabled until implemented. |
| `R/install.R`, `R/project-open.R`, `R/project.R` | Partial | Bundled Sample install, eight-table VP08 check, saved selection, paged Env/Admin plot read in `projectservice.go`. A missing saved project now falls back to Sample and persists the recovery; unreadable unrelated `.db` files are reported in the app without blocking startup. A corrupt Sample still blocks startup. Opening arbitrary files and historical Access projects are absent. |
| `R/db-connection.R`, `R/project-context.R`, `R/su-context.R`, `R/hierarchy-context.R` | Partial | Same-file read-only SU selection persists and filters the plot browser to Env-SU-Admin plot membership (Sample: 51 plots); None restores the full view, and a project switch clears SU. Master tables are disabled without authorization. Hierarchies in the projects directory can be selected independently by name and file, browsed read-only, and recovered or deactivated on startup; project switches retain them. External SU/hierarchy paths, full diagnostics, and write paths are missing. `coordinator.go` remains an optional in-memory DuckDB experiment; extension packaging is unverified. Keep SQLite as canonical storage. |
| `inst/app/modules/mod_fs882_6x4.R`, `R/plot-*.R` | Missing | No draft/save/discard, child CRUD, audit restoration, or write transaction; browser is read-only. The active R module targets Access `FS882-6x4XL`, not the non-XL variant. A parity checklist is retained under `/tmp/vpro_parity/wails-fs882/`; the Win11 VM probe was unavailable (SSH port 2222 refused), so write behavior remains unverified and disabled. |
| `inst/app/modules/mod_import.R`, `R/access-archive.R` | Missing | Access `.mdb/.accdb` conversion and CSV/ZIP/Excel import require separately validated native implementations. |
| `inst/app/modules/mod_reporting.R`, `inst/app/reports/` | Missing | Quarto rendering, Excel export, report preview and template parity. |
| `inst/app/modules/mod_becweb_map.R` | Missing | Leaflet map, filtering, map resources and offline tile policy. |
| Other active modules and shell workflows | Missing | Inventory every active event/dependency before porting; do not treat commented-out server code as active. |

Current tests cover the sample's VP08 metadata, project/SU/hierarchy selection and restart, an orphan Env row, master-SU denial, 52 project plots, 51 SU-filtered plots, 43 hierarchy nodes, paging bounds, name validation, non-overwrite behavior, and all 11 DuckDB view row counts against R activation. The DuckDB test skips on a machine without a cached extension; no extension is downloaded during startup or tests. Before allowing writes, port the R package's transaction and authorization rules and validate against the Access oracle and package tests. A green Wails build does not establish feature parity.

### Activation parity checklist

| Control or event | Desktop status | Next check |
| --- | --- | --- |
| Select a complete VP08 project and reopen it on startup | Implemented for projects discovered in the user data directory | Confirm selection and failure recovery in the native window. |
| Saved project missing or unreadable | Implemented: recover Sample and persist its selection | Check external project paths when file-open support is added. |
| Unrelated project file unreadable | Implemented: retain usable projects and expose a diagnostic | Add file-management UX before import is enabled. |
| Project activation clears the active SU | Implemented for the same-file read-only selector | Verify Access focus/change timing on a disposable VM copy. |
| Project activation retains the active hierarchy | Implemented for discovered local SQLite hierarchy tables | Confirm selection side effects in the Access VM. |
| Restore an SU's Env-SU-Admin plot filter | Implemented for SUs in the selected project file | Add external SU paths, full diagnostics, and source parity for orphan Env/Admin rows. |
| Directly activate a master SU | Missing: disabled, with no authorization UI | Design an explicit authorization contract before enabling it. |
| Restore an independent hierarchy selection | Implemented for local files with source identity and missing-source deactivation | Add external file attachment, tree diagnostics, and Access VM checks. |

### Navigation inventory

The active Shiny navigation in `inst/app/ui.R` defines Home, Forms (FS882, SIVI, Metadata, Combine Species, Herbarium, Colour-theme, User setup, User log), Reports (long/summary vegetation and environment, subzone matrix, hierarchy diagram, plot label, plot locations file, Google Earth), and Help (What's New). The desktop menu exposes these destinations for orientation; Home and the separately exposed read-only Plots and Hierarchy browsers work. Disabled entries do not open placeholder workflows or modify data.

The Shiny Data and References menus, most Help entries, and the later Modules/Sync/Administration block are commented out in that UI definition. They remain migration candidates, not active Shiny navigation or implemented desktop functionality. Their underlying Access behavior still needs assessment before deciding what belongs in the desktop app. The active What's New action reads and updates `tblWhatsNew` in a separate VPro64 SQLite database; it cannot be replaced by static release notes without changing its behavior.

## Next Milestones

1. Extend project/SU/hierarchy activation to external SQLite paths and full diagnostic/authorization parity, keeping canonical data in SQLite. Local read-only selection and startup recovery are available. Identify workflows that genuinely need cross-database joins before making the in-memory DuckDB coordinator mandatory; if needed, provision `sqlite_scanner` offline on each target platform. Only the locally cached Linux 1.5.5 extension has been verified so far.
2. Port remaining R configuration and reference-database attachment, including the VPro64 What's New store. Project selection now uses a platform config directory with legacy-file fallback, but is not synchronized with R `config.yml`. Make selection broadcasts and unsaved-draft handling explicit before opening multiple Wails windows.
3. Port the FS882 plot editor and the transactional `R/plot-*.R` operations, including audit and child records; compare fixtures and failure cases with the Access and R oracles before enabling writes.
4. Validate a native `.mdb`/`.accdb` conversion path against `mdbr` output, then port CSV/ZIP/Excel IO, Quarto/report templates, maps, and remaining active modules. No verified Go Access parser is selected yet.
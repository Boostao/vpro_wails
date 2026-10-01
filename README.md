# VPRO Desktop

VPRO is being migrated from Access to Go, Wails v3 beta.26, Svelte 5 and SQLite.
This is an **incomplete, experimental application**, not a production replacement.
Do not use its write workflows on production projects.

## Current capabilities

| Workflow | Status |
| --- | --- |
| Database/context/configuration foundation | **F1-F3 implemented and native-verified.** Shared default-init/runtime YAML, retained legacy JSON migration, preserved support family, SQLite-owned external contexts and draft-safe switching are active. Administrative writes and multiwindow coordination remain unavailable. |
| Project installation, discovery and saved selection | Bundled Sample remains byte-identical to R's SQLite Sample, including tables/indexes/views/`_table_metadata`. Non-overwrite family installation, VP08 compatibility, managed discovery, explicit external attachment and YAML path restoration work. Attachment does not copy or convert files. |
| Working units and hierarchies | Read-only selection/filtering and hierarchy browsing support separate external files. Verified Working Unit editing includes per-user mode preferences; Master authorization editing and bulk actions remain incomplete. |
| FS882 storage and source relationships | All 98 parent columns and 69 nonidentity XL child bindings plus five legacy extras are mapped. Extracted geometry is retained as evidence; presentation preserves source containment, labels, bindings and embedded-form links. |
| FS882 responsive presentation | Site, Soil/Terrain, Vegetation, Veg Other and Other use labelled semantic groups and readable 40px controls. Groups reflow with the available width, child tables scroll locally, and project context can collapse. Actual native sizing, resizing, drafts, validation and close recovery are verified. |
| FS882 ordinary parent editing | **77/98 parent fields are verified writable**, including coordinates, BEC, Working Unit, quality, substrate, disturbance/exposure, Region/Ecosection, Soil classification, Bedrock and 21 additional terrain/classification codes. Remaining controls stay unavailable until their workflows are verified. |
| FS882 children and height | Transactional child CRUD/storage and bounded existing-row height drafts work. Species selection, height insertion/deletion, broader child editing, pictures and calculations remain incomplete. |
| Audit and lifecycle | Data/audit transactions, bounded selective restoration, Undo, and native window/context Save/Discard/Cancel are verified, including hidden-invalid and height drafts, failed save/config publication and stale-context rejection. Cover restoration, broader child dirty-state propagation and multiwindow coordination remain incomplete. |
| Soil classification | Two independent nullable four-UTF-16-unit editors default on. Native selection/manual entry, NULL, Undo, Lock, hidden validation, atomic rollback/retry and actual window-close recovery are verified, alongside frozen catalogue browsing. |
| Bedrock classification | Three independent nullable four-UTF-16-unit editors default on, using the frozen87-row catalogue. Native full-item/raw entry, NULL, hidden validation, rollback/retry, Lock and clean close pass. Source effective properties remain unmeasured; this is an explicit safer adaptation, not exact Access input-mechanism parity. |
| Ordinary terrain/classification codes | 21 independent nullable editors default on: Realm, coarse-fragment lithology, surface/subsurface terrain, flooding, humus, hydrogeology, rooting type/particle size and water source. Field-specific UTF-16 bounds, per-list failures/Retry, draft-bound review, hidden validation, rollback and native close recovery are verified. SoilDrainage remains unavailable pending its distinct strict membership policy. |
| Import/export, reporting, maps and administration | Not complete. Unimplemented navigation remains disabled. |

The field count is a secondary coverage measure, not proof of complete form or
application parity. SQLite is canonical; the DuckDB coordinator remains an optional
experiment, with offline extension packaging unresolved.
The active Go application does not use DuckDB as its data layer. The foundation
uses connection-owned SQLite attachments/TEMP views, without an offline
extension dependency. See the compact F1-F3 gates in [MIGRATION_PLAN.md](MIGRATION_PLAN.md).

The active bootstrap owns a pinned readonly SQLite coordinator; verified writers
use a context-owned project pool and separate transactions against the selected
project file. Plot reads and writes borrow that same pool instead of opening and
closing it per call. Its two-connection bound, per-connection foreign keys and
five-second busy policy are tested; switch/shutdown close the pool after leases
complete. ContextService binds
editor reads/writes to an immutable context identity and blocks switching while an
operation is running. Candidate validation and YAML persistence precede publication;
failure retains the previous context. Legacy unscoped mutation/switch APIs cannot
bypass this protection in the active application.
Bundled originals live separately under `resources/database-family`, preserving
the derived editor read models. Support files remain readonly in this foundation.

Per-project files, original table names and `_table_metadata` descriptions are
architectural invariants. Descriptions preserve native Access table-object metadata,
not just compatibility versions. Editor-specific frozen catalogues and the current
derived `Species`/`Lists` database are read models, not replacements for `VPro64`,
`VLists`, `VUser`, `VMetaData`, `VMessageBoard` or project storage.

Region/Site/Soil/Bedrock/ordinary parent codes reuse shared Unicode, reference grouping
and draft-review helpers; physical write/restore guards are shared where policies match. Field-specific limits,
Exposure membership, source geometry and failure behavior remain explicit.

FS882 keeps the app's green/gold and IBM Plex styling rather than reproducing
Access's cramped pixel sizes. Routine guidance and reference definitions start
collapsed below the fields; warnings, loading, errors and required review remain
visible above them. The native window prefers 1400 x 900 within the available
work area, with a 560 x 520 minimum capped for smaller screens.

## Run and validate

Install Go, Node/npm, the platform Wails prerequisites and the pinned Wails CLI:

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.26
wails3 dev
```

The native development window uses Vite at `http://127.0.0.1:9245/`. A regular
browser is **preview only** and cannot establish Go-binding behavior.
Build a release with `wails3 build`; the Windows executable is under `bin`.

```sh
go test -race ./...
cd frontend
npm test
npm run check
npm run build
```

On Windows use `npm.cmd` and the local tool paths in
[WINDOWS_HANDOFF.md](WINDOWS_HANDOFF.md). Use focused tests while implementing;
run the complete race suite and frontend validation at integration boundaries.

## Data and safety

- Set `VPRO_DATA_DIR` and `VPRO_CONFIG_DIR` to isolated disposable directories for
  testing. Sample is copied without replacing existing files.
- The active application installs bundled `config.init.yml` defaults into runtime
  `config.yml` in the platform user config directory. A shared owner serializes
  selection, coordinate, Working Unit, audit strength and user updates; unknown
  and inactive R settings retain their values/types.
- Existing `desktop-selection.json`, `coordinate-settings.json` and
  `working-unit-settings.json` are validated and imported together, retaining the
  files and recording source hashes. Explicit YAML conflicts, changed legacy
  files and malformed input fail diagnostically; completed imports are not
  replayed. Selection keeps config-directory-first/data-root fallback.
- Failed YAML replacement preserves prior bytes and effective settings; strength
  is restricted to0-3. Fresh installs use R's `Admin`; existing Go installations
  retain `User` unless explicitly configured. The measured no-SU initialization
  to Master remains unchanged. Machine-specific Access paths are not imported.
- Discovery/browsing use read-only connections; experimental writes use separate
  transactional connections.
- Missing family files install under `<dataDir>/database-family` without replacing
  existing files. `Desktop.DatabasePaths` can override `VPro64`, `VLists`, `VUser`,
  `VMetaData` and `VMessageBoard` with explicit absolute paths. Seed hashes establish
  provenance, not validity of existing user data.
- Project, SU and hierarchy paths are independently retained in YAML. Same-name
  projects show full paths; attachment enables the existing verified writers against
  the selected file, not a copy. Use disposable files only.
- Invalid configuration or unavailable selected paths open a visible recovery window
  with editors disabled and configuration retained. Correct the reported YAML/path
  and restart; automatic repair, conversion and production write approval are absent.
- Unchanged historical invalid values must survive unrelated saves. New values
  and restoration targets are validated without silent repair or truncation.
- Deliberate departures from Access defects include atomic data/history, explicit
  completion instead of silent code rewriting, and no phantom option audits or
  automatic deletion of height-only vegetation during restoration.

## Experimental switches

`VITE_SOIL_CODES_REFERENCE=true` enables the read-only Soil catalogue viewer.
`VITE_SOIL_CODES_EDITING=false` disables the verified Soil editors; editing defaults
on. The separate reference viewer defaults off and is suppressed while editing.
`VITE_GEOLOGY_CODES_EDITING=false` disables the verified Bedrock editors; they
default on. `VITE_PARENT_CODES_EDITING=false` disables the 21 additional ordinary
parent-code editors; they default on. Coarse-fragment lithology uses TEXT12,
not Bedrock's TEXT4; humus phase uses TEXT50 and flooding frequency TEXT7.
SoilDrainage's explicit source LimitToList is not part of this permissive batch.
`VITE_EXTERNAL_PROJECTS=false` hides the verified external attachment entry point;
it defaults on. It does not disable restoration of valid saved external contexts.

Soil deliberately preserves raw casing and uses explicit full-item selection,
not Access's implicit completion. It rejects overlength input without truncation,
keeps data/history atomic and omits phantom checkbox audits. Source case-only
`ca` against stored `CA` remained `CA`; its exact cause and physical astral,
overlength, Lock-child and failed-save behavior remain unmeasured. Native desktop
proof is not a claim that the full installed Access baseline passes.

Native inspection is off by default. For disposable debugging only,
`VPRO_WEBVIEW_DEBUG_PORT` accepts a loopback port from 1024 to 65535. Use one isolated
config/WebView profile per simultaneous instance; never expose the inspector.

Session-owned PlotService pooling is implemented and verified. Repeated warm
header reads reuse one idle connection; a bounded backend benchmark measured
0.38-0.40ms/read versus3.39-3.69ms for the retained per-operation adapter on this
machine. This is not a frontend latency or contention guarantee. Native warm
reads, same-name project switching, stale rejection and close preserve fixture
data/config/supports; process handles plateau after warmup.
Shared catalogue verification caching and end-to-end read cancellation remain
planned; catalogue lookups still rehash/revalidate. See C1-C5 in
[MIGRATION_PLAN.md](MIGRATION_PLAN.md) for bounded performance/cleanup gates,
including incremental packages and optional typed catalogue transport consolidation.

## Migration references

- [MIGRATION_PLAN.md](MIGRATION_PLAN.md): one ordered, workflow-based backlog.
- [WINDOWS_HANDOFF.md](WINDOWS_HANDOFF.md): current local state, paths and next gate.
- [Access contract](docs/FS882-6x4XL-access-contract.md): detailed source and measured
  behavior, including limitations and intentional adaptations.
- [AGENTS.md](AGENTS.md): source-first development and bounded verification rules.

Historical continuation logs and build receipts remain in ignored
`evidence/private`; they are evidence, not the current work plan.

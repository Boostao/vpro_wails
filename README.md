# VPRO Desktop

Access-to-Go/Wails v3 beta.26/Svelte5 migration with SQLite canonical.
**Incomplete, experimental; not a production Access replacement.**
Use disposable projects for writes. The companion R package is planned as
UI-free core automation, not a second data-entry UI.

## Current capabilities

| Workflow | Delivered boundary | Availability/gaps |
| --- | --- | --- |
| Database/context/configuration | Original SQLite family, YAML defaults/runtime and retained JSON migration; non-overwrite installation, external attachment/compatibility, offline TEMP views, draft-safe switching and visible recovery | Active; conversion/administration/multiwindow not complete |
| Project selection and browsing | Managed/external project paths, Working Unit/SU filtering and read-only hierarchy browsing | Attachment is not copying/conversion or full SU/hierarchy editing |
| FS882 parent |98/98 mapped parent fields native-verified writable, responsive source labels/groups/links and shared draft/audit/Lock/close behavior | Bounded baseline, not all forms/events/pictures/projection |
| FS882 children | Other8/Humus12/Mineral18 CRUD; Veg Other12 attributes, Collected, species decisions,11 cover/total fields and6 heights across source grids | Default-on editors; remaining calculations/effective source defaults and full restoration incomplete |
| Guarded vegetation lifecycle | Reviewed full-row deletion/reserved IDs; creation with explicit species/numeric/NULL decisions; independent personal definitions | Opt-in adaptations; no guessed defaults, invented U/X membership or hidden VUser write |
| Standards checking | Whole-project/selected-WU reviewed species-code replacements; exact source membership and all physical rows | Opt-in; broader standards/environment compliance remains open |
| Project metadata | Review/all-field drafts, blank/master-template creation and dedicated typed provenance restoration | Opt-in, distinct project/master/user storage; generic metadata restore unavailable |
| Profiles and navigation | Ordered preview, rule edit/create/delete, explicit result navigation, independent selection/writer ownership, usable blank files/tables and reviewed Save as SU | Opt-in; arbitrary-file administration, broad history restoration and some criterion semantics open |
| Find/Enter | Source-bound explicit Enter navigation and exact scoped Find with safe Open lifecycle | Opt-in adaptations; no Access Find-dialog emulation |
| Env Into SU / SU Into Env | Independent reviews of all matching selected-SU plots; owned-project file only; atomic typed provenance, drift/trigger rollback/retry and history replay guards | Independently opt-in; separate-file SU and personal-definition writes unavailable |
| Long Environment | Owned read-only native preview: exact72 source rows, selected-SU membership, transposed per-plot values and explicit orphan/name diagnostics | Independently opt-in; external SU reads supported; preview-only title edits, file export/Excel/summary unavailable |
| Other forms/reports/interchange | Navigation inventory and original resources provide precedents | FS1333/SIVI/two-page/CHARS equivalence, usable client-priority reports/locations and complete import/export not delivered |

Current report-preview acceptance: full Go race (root942.439s),276 frontend
tests, check0/0, opt-in/default production builds, five native cases and a
zero-write default proof. Reverse-transfer predecessor is `96dca8d`.
Native acceptance used disposable data/configuration and restored original bytes.
Feature implementation/native verification does not mean a default-off workflow
is enabled or its Access execution parity is complete.

## Architecture and safety

- Retain VPro64/VLists/VUser/VMetaData/VMessageBoard and per-project physical
  table naming. Bundled [Sample](resources/Sample.db) is byte-identical to the
  R SQLite seed, including tables/indexes/views/`_table_metadata`; SHA256
  `e63f0c2a051761701bdad3c81bcfde4067ab322883c7e4ac84a3541ae8578ad8`.
  Descriptions are native Access table-object metadata, not only version flags.
  Derived editor catalogues are read models, not replacement databases.
- Missing support files install under `<dataDir>/database-family` without
  replacing existing files. `Desktop.DatabasePaths` retains explicit absolute
  family overrides. Project/SU/hierarchy selections keep independent paths;
  selected existing files are not silently copied or converted.
- `config.init.yml` initializes persistent `config.yml`. Shared preference
  ownership serializes selection/coordinate/Working Unit/audit/user changes.
  Unknown/inactive settings retain values/types. Three old JSON files are
  validated/imported together, retained and hash-recorded; conflicts, changed
  inputs and malformed data fail explicitly. Failed replacement preserves
  prior bytes/effective settings. Obsolete Access machine paths are not imported.
- SQLite owns readonly attachments/TEMP views and separate transactional writers.
  Immutable context IDs, file observations, leases and draft-safe switching
  prevent stale calls. DuckDB remains optional; offline startup needs no
  unprovisioned extension.
- Errors/proposals retain editor/original identity through tab remounts and block
  Save/Lock/native close/context/unrelated writes. Data and audits/provenance
  share transactions; cancellation, drift, collisions, rollback and retry are
  tested. Known commits retire proposals before refresh; do not replay failures
  explicitly reported as already committed.
- Preserve raw text, NULL/empty, duplicate reference metadata, BOOLEAN true=-1,
  UTF-16 bounds and unchanged historical invalid values. Do not trim, recase,
  infer defaults, silently repair raw JSON Unicode or enforce new field-manual
  recommendations on historical projects. No phantom audits/destructive restore.
- Configuration/path failures open visible recovery with editors disabled.
  Correct the reported YAML/path and restart; automatic repair and production
  write approval are absent. Set `VPRO_DATA_DIR`/`VPRO_CONFIG_DIR` to disposable
  directories for validation.

## Workflow boundaries worth retaining

Species changes preserve alias precedence and explicit replace/keep/personal-code
decisions; non-ASCII exact selection is allowed without claiming general Access
collation parity. C/C-height NULL Cover6 removes view membership, not the record;
A/A-height missing-cover notices do not infer covers or delete height-only rows.
Source creation does not save personal definitions implicitly.

Metadata's supplemental typed history is a desktop adaptation: old plaintext
audits cannot establish historical SQLite storage classes. Dedicated restoration
retains provenance and checks the final row after all pruning. Source bulk
environment/SU SQL bypasses form audits; new atomic technical transfer history
does not claim Access bulk-audit parity.

Forward Env Into SU copies Admin.UserSiteUnit through original Env/Admin links
into SiteUnit TEXT255, over the whole selected SU rather than only current plot/
profile navigation. Matching duplicate physical links reject. The SU must be the
owned project file. Reviewed NULL and empty text remain distinct; unrelated
tables/schema/history are independently checked before commit.

Reverse SU Into Env copies selected SU.SiteUnit into Admin.UserSiteUnit TEXT100.
Uniquely matching master then personal definitions supply short/long names
TEXT50/100; personal definitions override master, missing definitions preserve
names, and NULL/empty codes stay distinct. Overlength assignments, ambiguous
definitions and changes to locked/unverifiably unlocked plots reject. These
guards, technical history and selected-SU names instead of source hard-coded
`Sample_SU` are explicit desktop adaptations, not blanket Access execution parity.

The Long Environment preview preserves67 original Env/Admin fields and five heading
rows from `V7mdlReportsEnv.EnvReport`, including labels/punctuation. It groups by
selected SU, not Admin.UserSiteUnit or profile navigation, and adds no quality
filter or summary calculations. Numeric zero, empty text, NULL and historical
storage remain typed; missing Env/Admin retain membership with blank projections.
Deterministic raw ordering, duplicate-membership counts and explicit conflicting/
unsupported name diagnostics replace arbitrary `First()` selection. Reference
candidate IDs, NULL/empty names and duplicates remain distinguishable. Owned
read-only transactions use the original physical project/SU/reference tables,
including external selected-SU files, with cancellation and file/context identity
checks. Navigation reuses Save/Discard/Cancel; native close is guarded during
reads. The configured YAML `ReportOptions.LEReportTitle` is preserved and read
without fallback on malformed configuration. Title edits affect this preview
only: source registry-title persistence remains an explicit adaptation/gap.
The preview writes no data, audits or configuration and does not launch
Access/Excel, create export files or implement summary calculations.

## Feature gates

These are build-time Vite flags, not user permissions or hidden backend grants.
Keep independent gates; do not turn on unavailable workflows as part of cleanup.
Set default-on flags to `false` for read-only opt-out; set default-off flags to
`true` only for their verified experimental surfaces.

| Default | Flags |
| --- | --- |
| On: parent groups | `VITE_COORDINATE_EDITING`, `VITE_BEC_EDITING`, `VITE_WORKING_UNIT_EDITING`, `VITE_MASTER_BEC_EDITING`, `VITE_QUALITY_EDITING`, `VITE_SUBSTRATE_EDITING`, `VITE_SITE_CODES_EDITING`, `VITE_REGION_CODES_EDITING`, `VITE_SOIL_CODES_EDITING`, `VITE_GEOLOGY_CODES_EDITING`, `VITE_PARENT_CODES_EDITING`, `VITE_ORDINARY_PARENT_EDITING`, `VITE_PARENT_FLAGS_EDITING`, `VITE_SOIL_DRAINAGE_EDITING`, `VITE_ADDITIONAL_PARENT_EDITING` |
| On: child groups | `VITE_OTHER_EDITING`, `VITE_SOIL_CHILD_EDITING`, `VITE_VEGETATION_ATTRIBUTE_EDITING`, `VITE_VEGETATION_COLLECTED_EDITING`, `VITE_VEGETATION_SPECIES_EDITING`, `VITE_HEIGHT_EDITING`, `VITE_VEGETATION_NUMBER_EDITING` |
| On: bounded infrastructure | `VITE_EXTERNAL_PROJECTS`, `VITE_AUDIT_RESTORE` |
| Off: lifecycle/checking | `VITE_VEGETATION_CREATE_EDITING`, `VITE_VEGETATION_DELETE_EDITING`, `VITE_PERSONAL_SPECIES_EDITING`, `VITE_SPECIES_CODE_CHECK_EDITING` |
| Off: metadata | `VITE_PROJECT_METADATA_EDITING`, `VITE_PROJECT_METADATA_CREATION`, `VITE_PROJECT_METADATA_TEMPLATE_CREATION`, `VITE_PROJECT_METADATA_RESTORE` |
| Off: navigation/transfer | `VITE_SOURCE_ENTER_NAVIGATION`, `VITE_SOURCE_PLOT_FIND`, `VITE_SOURCE_ENV_SU_TRANSFER`, `VITE_SOURCE_SU_ENV_TRANSFER` |
| Off: profile rules/results | `VITE_PROJECT_PLOT_PROFILE_REVIEW`, `VITE_PROJECT_PLOT_PROFILE_RUN`, `VITE_PROJECT_PLOT_PROFILE_EDITING`, `VITE_PROJECT_PLOT_PROFILE_CREATION`, `VITE_PROJECT_PLOT_PROFILE_DELETION`, `VITE_PROJECT_PLOT_PROFILE_FILTERING`, `VITE_PROJECT_PLOT_PROFILE_SAVE_SU`, `VITE_PROJECT_PLOT_PROFILE_SAVE_SU_PROJECT` |
| Off: stored profile ownership | `VITE_PLOT_PROFILE_SELECTION`, `VITE_PLOT_PROFILE_WRITE_OWNERSHIP`, `VITE_PLOT_PROFILE_FILE_CREATION`, `VITE_PLOT_PROFILE_TABLE_CREATION` |
| Off: read-only catalogue | `VITE_SOIL_CODES_REFERENCE` |
| Off: read-only reports | `VITE_LONG_ENVIRONMENT_REPORT` |

`VITE_HEIGHT_EDITING=false` also disables numeric vegetation editing.
Master edits additionally require source authorization; reference membership,
availability and physical validity remain separate checks. Profile sub-actions
require their parent workflow/selected ownership, not just a flag.

## Run and validate

Install Go, Node/npm and platform Wails prerequisites:

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.26
wails3 dev
```

The native development window uses Vite at `http://127.0.0.1:9245/`.
A browser preview cannot prove Go bindings/lifecycle. Release: `wails3 build`.
Windows executable/tool paths are in [WINDOWS_HANDOFF.md](WINDOWS_HANDOFF.md).

```sh
go test -race -timeout 20m ./...
cd frontend
npm test
npm run check
npm run build
```

On Windows use `npm.cmd`. Focused tests during implementation; full validation
at integration. New frontend tests must be added to explicit package selectors.
Use native Wails with disposable data/config for binding/lifecycle acceptance.

## References and next work

- [Migration plan](MIGRATION_PLAN.md): ordered deliverables and bounded next round.
- [Client scope/domain index](docs/CLIENT_SCOPE.md): page-cited scope decisions,
  uncertainties, companion R boundary and help/manual reference locators.
- [Access contract](docs/FS882-6x4XL-access-contract.md): source/measured behavior
  and explicit adaptations; not proof of all workflows.
- [Development instructions](AGENTS.md): source-first safety and work budget.
- [Release workflow](.github/workflows/release.yml) /
  [download portal](docs/index.html): distribution surfaces, not replacement
  acceptance for every platform.

Next: shared CHARS extended-shrub behavior, then FS1333/two-page
implementation and priority vegetation reports/locations. Static form/report
contracts alone do not prove native execution parity. The historical agreement's
cloud/public-map/BECWeb/publication tracks remain visible but separately scoped.
Private native evidence, original documentation snapshots and closed-build hash
maps are retained outside app assets; no client PDFs or temporary tools are bundled.

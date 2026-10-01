# VPRO migration plan

Updated 2026-10-01. Finish usable workflows, not isolated fields or ever-larger
oracle suites. This plan is the single ordered backlog; current machine state
belongs in [WINDOWS_HANDOFF.md](WINDOWS_HANDOFF.md).

## Working model

1. Read Access exports first: bindings, constraints, event procedures, logical layout and
   parent/child relationships. Classify equivalent controls as a batch.
2. Reuse a measured shared mechanism only when source/event behavior matches.
   Probe effective defaults, visual behavior and genuinely different outcomes.
3. Implement shared domain logic and fast table-driven boundary tests. Keep
   workflows with distinct semantics, such as BEC cascades or Working Unit
   preferences, explicit rather than forcing them into a universal editor.
4. Verify the integrated workflow in Wails on disposable data. Enable only the
   verified surface and record unresolved differences explicitly.
5. Update one current-state checklist. Do not append execution history to the
   README, handoff or plan.

One primary agent owns integration. Delegate at most one substantial, independent
implementation/evidence task at a time; no agent launches further agents or
broadcasts coordination messages. Native Access has one exclusive owner.

## Ordered deliverables

| Order | Deliverable | Exit gate |
| --- | --- | --- |
| 1 | Stabilize the existing implementation (complete) | Focused tests, full Go race, all134 frontend tests, check and production build pass as of2026-10-01. No new feature branch while current validation debt remains. |
| 2 | Finish the staged Soil workflow (complete) | Shared source audit/Lock paths reused. Native Wails proved selection/manual entry, NULL, Undo, Lock, hidden validation, rollback/retry and actual-close lifecycle. Default-on assets match the tested payload byte-for-byte; explicit opt-out builds. Coverage is53/98 with source gaps/adaptations documented. |
| 3 | Consolidate ordinary code editing (complete) | Shared nullable Unicode/reference grouping/suggestions/draft acknowledgement and Region/Soil write/restore guards preserve public APIs, source positions and field-specific policies. All117 frontend tests/check/build, focused Go/full race and actual Wails rollback/close regression pass. New groups reuse definitions/fixtures, not copied validators. |
| 4 | Integrate database family, context and configuration (complete) | F1-F3 pass: retained JSON-to-YAML migration, preserved SQLite family/descriptions, active external-path ownership/compatibility and offline TEMP views, native safe switching/reopen/recovery. Final full race,137 frontend tests/check/build and default-on native delivery pass. |
| 4a | Bounded backend ownership/performance follow-up (C1-C2 complete; C3 next) | Context-owned plot pooling and shared immutable catalogue snapshots pass full race/frontend/native gates. Warm lookups avoid repeated full hashing/profile scans; corruption, replacement and forced Retry preserve metadata/drafts/data. C3 cancellation follows affected retrieval paths; C4-C5 structural cleanup is incremental, not a new expansion-blocking rewrite. Preserve sealed baselines. |
| 5 | Complete remaining FS882 parent workflows (checkpointed at77/98) | Responsive presentation and21 ordinary-code editors are native-verified and default-on. After the foundation gate, continue remaining21 columns by source/event semantics. Classify controls as verified, intentionally adapted, read-only or blocked; cover calculations, coordinates, notes and pictures explicitly. |
| 6 | Complete FS882 child workflows | Vegetation/species/cover/height, Humus, Mineral and Other support their actual add/edit/delete/validation/filtering behavior. Test identity ownership, NULL, audit, rollback and parent/context lifecycle. Storage mapping alone is not completion. |
| 7 | Complete project and administrative domain workflows | Build on the foundation rather than postponing it: project creation/conversion, SU/hierarchy authorization and editing, user/reference/metadata writes and bulk operations. Add multiwindow behavior only when coordination is verified. |
| 8 | Implement data interchange | SQLite/CSV/Excel/archive boundaries first as supported by the source workflows. Access reading belongs to the separate go-mdbtools effort and enters only through fixture-tested import adapters. Validate roundtrip, collision, cancellation and rollback. |
| 9 | Implement reports, maps and remaining active modules | Source-driven templates, preview/export, filters, offline assets and administration. Inventory all active entry points; keep placeholders disabled. |
| 10 | Replacement acceptance | Every active source workflow has a tested desktop equivalent or explicit agreed exclusion. Verify installation, upgrades, representative projects, data preservation, recovery, performance and offline operation in the native app. |

## Database/context/configuration foundation

Architecture decision,2026-10-01: preserve the R database family and default-init/
persistent-runtime YAML model; use SQLite for the active Go context. R supplies
architectural precedents, not Access parity proof. This section defines planned
integration. F1-F3 are implemented, native-verified and promoted. This completes
the bounded foundation, not project administration or the full migration.

### Evidence and current boundaries

- Read-only inspection confirms `resources/Sample.db` is byte-identical to
  [R Sample](../../vpro/inst/extdata/projects/Sample.db), SHA256
  `e63f0c2a051761701bdad3c81bcfde4067ab322883c7e4ac84a3541ae8578ad8`.
  It contains15 tables,31 named indexes and17 views, including `_table_metadata`.
  Preserve the full file/schema, not just the eight core project tables.
- `_table_metadata.description` represents native Access table-object Description
  properties. Go currently uses the Env description for VP08 compatibility;
  this is one consumer, not the table's entire purpose. Preserve all entries,
  extra columns, NULL/empty distinctions and any duplicates. Do not synthesize
  descriptions where a support database currently has no metadata table.
- Go discovers managed databases under local `projects` and supports explicit
  external project/SU/hierarchy files. The active app
  persists selection/coordinate/Working Unit/audit/user in shared `config.yml`,
  with validated, retained, hash-recorded migration of the three legacy JSON
  files. Context transitions use the native-close Save/Discard/Cancel lifecycle;
  hidden-invalid and height drafts block Save, and successful old-context writes
  are not replayed when selection persistence fails.
- Go's `resources/vlists.db` contains derived `Species` and `Lists` tables, not
  the canonical `VLists` family. Specialized editor catalogues are frozen
  validation fixtures/read models, not replacements for system/reference/user/
  metadata databases. Their verified behavior must survive the integration.
- The optional Go DuckDB coordinator builds some temporary project views but is
  not wired as the active application data layer. Its sqlite_scanner extension
  must already be provisioned; offline startup must not require it.
- F2 is active in application startup/selection/reads and resolved writers:
  `databasefamily.go` preserves all five original support files under a separate
  `database-family` directory (no collision with the derived `vlists.db`).
  `sqlitecontext.go` prepares an independently owned, pinned in-memory SQLite
  coordinator with readonly file attachments and TEMP views. Fast tests verify
  external paths with quotes/spaces/Unicode/URI delimiters, shared physical files,
  family identity, source vegetation predicates, SU authorization, failed
  candidates, cancellation, metadata NULL/empty/duplicates/extra columns and
  owned close. Native F3 proof establishes Save/Discard/Cancel, rejected save,
  locked-config publication/retry, stale writes, separate external SU/hierarchy,
  readonly restart and visible missing-path recovery. Support files remain readonly.
  External attachment defaults on with an explicit presentation opt-out.
- Inspected precedents: [default YAML](../../vpro/inst/config.init.yml),
  [configuration](../../vpro/R/config.R), [installation](../../vpro/R/install.R),
  [connections](../../vpro/R/db-connection.R),
  [context/ownership/views](../../vpro/R/project-context.R),
  [guided project opening](../../vpro/R/project-open.R),
  [startup](../../vpro/R/startup.R) and
  [reference views/descriptions](../../vpro/R/startup-reference-views.R).
  R's guided opening copies a selected file into managed storage; its context
  also accepts explicit external paths. Keep these two operations distinct.
- Access sources: [support links](../../VPRO_ACCESS/VPro64_forAI/Modules/V7mdlSplash.txt),
  [project/version attachment](../../VPRO_ACCESS/VPro64_forAI/Modules/V7mdlAttachProjects.txt),
  [current query rewrites](../../VPRO_ACCESS/VPro64_forAI/Modules/V7mdlSetCurrent.txt),
  [registry properties](../../VPRO_ACCESS/VPro64_forAI/Modules/clsVProReg.txt),
  [form preferences](../../VPRO_ACCESS/VPro64_forAI/Modules/clsFormInfo.txt) and
  [Working Unit option mapping](../../VPRO_ACCESS/VPro64_forAI/Modules/V7mdlTableOfLists.txt).
  Reuse existing runtime evidence for measured preferences; only probe genuinely
  unresolved switch/startup behavior on disposable copies.

### Preserved storage and Go coordination

| Role | Preserved database | Ownership and initial access |
| --- | --- | --- |
| System/templates | `VPro64.db` | Preserve original tables/templates; readonly for foundation. Do not turn the system database into the project writer. |
| Reference | `VLists.db` | Canonical species, lists, Master units, templates and table descriptions; readonly. Preserve original names, including `USysAllSpecs`/`USysTableOfLists`. |
| User | `VUser.db` | User species/units/preferences and other existing tables retained; readonly until each mutable workflow has its own authorization/audit contract. |
| Metadata | `VMetaData.db` | Project metadata/codes, distinct from `_table_metadata` descriptions; readonly in this scope. |
| Messages | `VMessageBoard.db` | Retain tables/triggers; message workflows remain disabled. |
| Projects | Separate project SQLite files | Original `<Project>_Env/Admin/Audit/Veg/Humus/Mineral/Other/Metadata` names plus all other objects retained. Only existing verified writers enabled. |

1. Install missing bundled family files into the disposable/app data root without
   overwrite, using sealed originals. Existing files always win; incompatibility
   produces a diagnostic, not replacement by a seed/catalogue. Record seed hashes
   separately from legitimate mutable user data; hashes are not schema/version gates.
2. A single context owner resolves explicit paths and owns project/reference/user/
   metadata/message handles. Identity is canonical full path plus project family,
   not basename alone. Several families can share one SQLite file; duplicate
   project names at different paths require explicit disambiguation, not shadowing.
   Track handle ownership and release only owned, no-longer-used connections.
3. External SQLite attachment inspects an existing file readonly without creating
   it, renaming tables, changing descriptions or automatically copying it. Managed
   import/copy is a separate explicit operation. Preserve stored paths, including
   spaces/Unicode; reject missing files and alias/path collisions explicitly.
   Live write validation remains confined to disposable external copies.
4. Validate the complete eight-table VP08 family and required schema, reading
   Env Description without overwriting it. Unknown/missing/ambiguous descriptions,
   malformed files and incomplete families are diagnostic failures; VP05-07
   require a separate conversion workflow. Inspect support schemas independently.
5. Use a pinned SQLite connection for readonly attachments and connection-local
   TEMP views (`USysEnv`, `USysVeg*`, `USysHumus`, `USysMineral`, `USysOther`,
   `USysMetadata`, `USysAuditTrail` and source-required reference/SU/hierarchy views).
   SQLite TEMP views can span attached databases; persistent project views cannot
   be rewritten for session context. Keep aliases internal/quoted and preserve
   physical names. Do not create attachment/view state on arbitrary pooled handles.
6. Keep existing project writes on a separately owned SQLite transaction against
   the resolved project path: parent, children and their audits remain atomic in
   the same file. This foundation adds no distributed support/project writes.
   Bind each write and async result to a context generation; never let a delayed
   save or stale response redirect into the newly selected project.
7. Prepare and validate a candidate context before publishing it. Require editor
   Save/Discard/Cancel (including hidden errors and height drafts) before switching;
   block during active operations. Save targets the old context; failed Save or
   Cancel leaves its draft, handles and selection intact. Publish only after
   candidate views and selection persistence succeed; on failure keep the old
   context and release the candidate. Startup recovery is explicit/diagnostic,
   not silent preference deletion or arbitrary project substitution.

DuckDB remains an opt-in analytics adapter only if a concrete query workload
needs it. The foundation's offline path uses SQLite without downloads/extensions.
Do not port Access's destructive query rewrites, implicit schema repair or R's
drop-old-views-before-success order as desktop requirements.

### YAML model and legacy preference migration

- Reuse the R `config.init.yml` section/key vocabulary as the bundled immutable
  default, with a documented desktop overlay only where necessary. Install
  writable `config.yml` under the resolved user configuration directory once;
  never overwrite it on startup or upgrade. `ProjectPath` is the selected
  backend file, not the data root; `SUPath`/`HierarchyPath` may be separate files.
- One Go configuration owner validates and serializes updates across services.
  Preserve unknown/unavailable sections and scalar types; validate active keys,
  duplicate keys, malformed YAML and schema-version incompatibility explicitly.
  Do not coerce all Access booleans/strings using a global truthiness rule.
  Environment overrides select disposable data/config roots, not persisted paths.
- Write same-directory temporary files, sync/close and atomically replace only
  after validation. Unlike the R delete-then-rename implementation, never remove
  the last valid configuration before the replacement commits. Failed writes
  retain the complete old file and in-memory state. Record migration/schema
  version and source hashes in a desktop-specific YAML section.
- Import `desktop-selection.json`, `coordinate-settings.json` and
  `working-unit-settings.json` explicitly and idempotently. Keep the existing
  config-directory-first/data-root fallback for legacy selection. If YAML is
  absent, apply validated JSON values over defaults; if YAML exists, fill only
  missing keys. Equal values are harmless; conflicting explicit YAML/JSON values
  produce a migration diagnostic for resolution, never a silent winner/deletion.
  Retain legacy files and report unknown fields, malformed values and unavailable
  paths. Commit the combined migration once; a partial failure must not consume
  some preferences and lose the others. Retry must not reapply completed imports.

| Source setting | YAML target/policy | Current Go status |
| --- | --- | --- |
| Project/SU/hierarchy JSON; Access `Current.CurrProject/CurrPlotlist/CurrHierarchy`, paths | Preserve these R keys and explicit file identity; validate on activation, retain unavailable selections with diagnostics. | YAML migration and managed/external project/SU/hierarchy restoration and native switching verified. |
| Coordinate JSON `dd/dm/dms`; Access `Current.CoordMethod` | `1=dd`, `2=dm`, `3=dms`, as exported `ChangeCoordMethod` defines. Display preference must not edit axes. | Shared YAML migration/persistence and pure conversion verified. |
| Working Unit JSON `env/master/su`; Access `Current.AssignedSuSource` | `1=env`, `2=master`, `3=su`; preserve the measured no-SU initialization behavior. | Shared YAML migration/persistence/fallback verified, including native no-SU initialization to Master. |
| Access `Audit.AuditStrength`, `Current.User` | Validate strength0-3 and preserve user identity before a mutation; configuration changes create no plot audits. | Shared YAML persistence verified; fresh installs use `Admin`, existing Go installations retain `User` unless explicit YAML overrides. Failed setters retain effective audit state. |
| `Current.ProjectIdSource`, form choice, report/export/filter/theme/message/update settings | Retain source-key inventory/defaults as inactive until the corresponding workflow is implemented/tested. | Not a blanket declaration of working preferences. |
| Install/program/executable/picture paths, old window coordinates, cached reference versions and scratch registry state | Do not blindly import machine-specific paths, obsolete state or permissions. Resolve current support paths explicitly; descriptions come from the databases. Optional registry import is an allowlisted, explicit readonly input, never registry writes or startup scraping. | Not implemented; outside automatic preference migration. |

### Compact implementation stages and acceptance gates

| Stage | Bounded scope | Required acceptance tests |
| --- | --- | --- |
| F1: YAML/preferences (complete) | Shared config owner/default-init/runtime YAML; migrate the three JSON files and wire only existing selection/coordinate/Working Unit plus audit/user settings. No new settings UI or database-family rewrite. | Focused tests/full race, regenerated bindings, all134 frontend tests/check/build pass. Actual Wails imported all three retained JSON files, preserved unknown/inactive scalar distinctions, rejected three setters against a Windows-locked config without changing bytes/effective modes/audit identity, retried, rejected strength4 and reopened without replay. All15 tables checked,31 historical audits unchanged, exactly3 intended audits with preserved/raw user identities. Evidence: `evidence/private/native-yaml`. |
| F2: SQLite family/context | Non-overwrite family installation; readonly role/schema/description inspection; managed and external SQLite identities; pinned TEMP project/reference views; existing services resolve through that context rather than catalogue replacements. | Existing files/descriptions/extra objects preserved; multiple families per file; path/alias collisions; spaces/Unicode; missing/unversioned/incomplete/VP05-07 inputs fail without mutation; support-schema diagnostics; native-source view rows/filters/NULL behavior; no persistent schema changes; missing DuckDB extension/network disabled does not prevent SQLite startup; attachments disappear on owned close. |
| F3: Native switching | Bind the existing editor/close lifecycle to project/SU/hierarchy changes and startup restoration; stage/publish/rollback the candidate context. No multiwindow/admin expansion. | Actual Wails Save/Discard/Cancel; hidden-invalid and height drafts; failed save and failed config persistence preserve old context/drafts; stale async/write generation rejected; external path reopen, missing-path recovery diagnostics, readonly support coordination, all-table/audit/description preservation. |

Use one small disposable database-family/config fixture, two distinguishable
projects and an external-path copy, plus injected failure cases in fast tests.
Seal schema/table-description snapshots and source hashes before/after; use compact
data/audit deltas and representative native evidence. Run focused Go tests first,
full race and frontend tests/check/build at integration, then actual Wails before
promoting a candidate. The prior77/98 executable is archived with its seal; the promoted foundation
retains77/98 editor coverage. No R/Access/production data writes.
Installing a family does not enable its unavailable domain workflows.

## Backend peer-review follow-up

Reviewed against the integrated F1-F3 implementation,2026-10-01. These are
bounded improvements. C1 pooling is implemented/verified; checksum caching and
end-to-end cancellation remain planned. Preserve sealed baselines and keep this work
bounded; do not reopen the foundation as a general framework rewrite.

### Confirmed findings and design decisions

- [PlotService.getActiveDB](plotservice.go) now borrows the context's project pool;
  header/child/height/audit callers release the borrow, not the pool. Scoped
  snapshots retain the outer operation lease; legacy active-app reads acquire a
  lease for their borrow without recursive locking. The retained non-context test
  adapter still owns/closes its per-operation pool explicitly.
- [sqliteContext](sqlitecontext.go) already owns the pinned readonly coordinator;
  project-list/hierarchy reads use it. Plot detail/child/audit queries intentionally
  retain physical project semantics through the same context-owned project pool:
  replacing them with SU-filtered TEMP views would change their existing behavior.
  Unify **ownership and access contracts**, not every operation
  onto one SQL connection: retain readonly support attachments/TEMP views and a
  separate, session-owned project writer. Data and audits stay in one project
  transaction; no support writes, distributed transaction or journal-mode change.
- Parent/Geology/Soil share [listcataloguestore.go](listcataloguestore.go) and
  [internal/listcatalog](internal/listcatalog), but each lookup still hashes the
  entire file and reruns full profile validation. Catalogue services already have
  long-lived readonly handles. Reuse the existing shared store/profile seams,
  rather than implementing a new cache independently in each service.
- Wails beta.26 supports a first `context.Context` parameter and generated
  CancellablePromise calls. Its runtime creates a call-specific cancellation
  context; adding a parameter alone does not cancel calls on tab changes.
  Current scoped methods lack the injected parameter, SQL paths use background
  contexts/non-context methods, and editor invalidation drops stale results without
  stopping their queries. Propagate cancellation through the active ContextService
  transport and explicitly cancel superseded frontend read promises.
- Thin, typed dictionary services have related implementations but different
  profiles, method shapes, bounds and membership policies. Reusing shared internals
  is preferable to immediately replacing them with an untyped universal API.
  Root-file count alone is not a reason for a package or binding rewrite.

### Compact cleanup gates

| Gate | Bounded implementation | Acceptance and preservation |
| --- | --- | --- |
| C1: Session-owned database access (complete) | Context owns a lazily initialized project pool; all plot read/write callers release borrows instead of closing it. Filtered browse/hierarchy reads remain on the readonly coordinator; detail/child/audit queries retain physical project scope on its owned pool, avoiding an unverified filter change. Legacy test adapters remain explicit. | Serial100 warm reads/capability calls reuse one idle connection; concurrent24x10 reads remain within two connections. Both connections retain foreign_keys=1/busy_timeout=5000. Failed publication retains the original pool; switch/shutdown retire it after leases; unscoped read borrows block switch and release once. Pooled audit failure rolls back data/history and retry adds exactly one audit. Full race root357.549s and focused pool tests2.304s pass. Native500 reads, managed/external switch, stale rejection and clean close preserve all fixture data/supports and final YAML; handles plateau442/446/446/446 over400 reads. Three100-read backend runs: pooled0.38-0.40ms versus retained adapter3.39-3.69ms. Existing F3 Save/Discard/Cancel UI is unchanged and its proven domain paths are reused; cancellation propagation remains C3, not claimed here. |
| C2: Shared catalogue verification cache (complete) | Parent/Geology/Soil/Region/Site share verified immutable snapshots. Mount hashes/validates once; warm calls clone metadata without SQL/file reads. Per-service locks serialize lookup/reload/close. Temporary readonly SQL handles close before publication; changed identity/size/modtime and Windows ChangeTime require full verification. Explicit UI Retry forces ReloadCatalogue before fetching. | Full Go race root338.652s;139 frontend tests/check0/0/build; actual bindings15 services/104 methods/33 models. Tests prove zero extra warm hashes/scans/bytes across100 all-list iterations, pointer isolation, replacement/removal, failure/restoration, concurrent successful lookup/reload before close and Windows same-size/restored-modtime corruption. Three100-call runs: warm54-63us versus forced full file/profile verification3.09-3.15ms (backend only). Native160 warm reads/eight forced reloads preserve typed DTOs; corruption/remount/failed Retry/atomic restoration/successful Retry retain the invalid draft and reject Save. All fixture bytes/audits/config/supports unchanged. If a host preserves every observed attribute, warm returns only the old verified snapshot; forced Retry detects corruption and invalidates it. Non-Windows uses identity/size/modtime only; no cryptographic freshness claim. |
| C3: End-to-end retrieval cancellation | Add injected contexts first to active scoped reads, project/hierarchy reads and catalogue reads; pass them through QueryContext/QueryRowContext and helper interfaces. Retain/cancel generated read promises on supersession/unmount, with cancellation-aware lease acquisition where waiting is possible. | Native Wails proves explicit promise cancellation reaches SQLite, closes rows/releases leases and leaves data/audits untouched; subsequent valid reads still succeed and stale results remain rejected. Regenerate real bindings and test argument/model compatibility, remount and context switching. Expected cancelled reads are handled deliberately, not surfaced as permanent draft errors or swallowed as success. Do not automatically cancel Save/Lock mid-commit or add implicit write retries; mutation/switch cancellation requires a separate atomic rollback/commit contract. |
| C4: Incremental package seams | Between validated milestones, extract shared catalogue storage first, then self-contained config/coordinator/domain logic into internal packages as useful. Keep main for composition/service registration and thin Wails adapters; preserve exported DTOs and existing helpers during migration. | One cohesive extraction per checkpoint, no dependency cycles or global singleton state. Preserve public bindings, localization/errors, embedded asset ownership and legacy adapters; focused tests then full race at integration. No simultaneous package move, API replacement and behavior change. Existing internal/listcatalog is prior art, not something to recreate. |
| C5: Optional catalogue transport consolidation | After shared internals/policies are stable, evaluate a typed CatalogService with allowlisted domain/list identifiers; retain existing services as compatibility adapters until callers migrate. Implement only if it measurably reduces transport/maintenance duplication. | Unsupported domains/lists fail explicitly; no arbitrary SQL/file selection. Preserve provenance, nullable/duplicate metadata, per-field UTF-16 bounds, strict versus permissive membership, independent failures/Retry and typed return shapes. Real bindings/frontend/native lookup regressions pass; frozen catalogues remain read models, never replacements for the canonical database family. |

C1-C2 are complete. C3a scoped editor retrieval is implemented and promoted:
eight ContextService read methods receive injected context without changing JS
arguments; header/schema/child/audit queries use context-aware SQL. FS882 retains
read promises and explicitly cancels on supersession/unmount; writes are unchanged.
Tests cover pre-cancelled calls, queued operation leases, SQL pool waits, SQLite
interruption/release/recovery and actual SDK promises. Full race root348.272s,
142 frontend tests/check0/0/build; real bindings remain15/104/33. Native cancellation
of an intentionally expensive query on an isolated copy releases the switch lease
in0.328s and the next read is identical. The synthetic schema is removed and exact
project bytes restored after owned close; audits/config/catalogues/supports match.
beta.26's delayed acknowledgement is handled only for an owned explicitly
cancelled promise with a RuntimeError whose message is exactly `context canceled`;
other rejected reads remain errors, including unrelated cancelled promises.
C3 remains open for browse/project/hierarchy/reference/catalogue reads and their
frontend ownership, plus any remaining coordinator/metadata lock waits.
wire C3 along affected retrieval paths without coupling it to an all-service rewrite.
C4/C5 fit between subsequent validated workflows and do not block parent migration
merely to reduce file/service counts. Run focused Go tests/benchmarks first, full
race at integration, frontend tests/check/build for binding/lifecycle changes and
bounded actual Wails checks on disposable fixtures. Record before/after measurements
and pool/hash counts; do not invent latency targets without a measured baseline.

## Responsive presentation direction

- Retain Svelte5 and Tailwind4, using CSS Grid for responsive field placement;
  Bootstrap is supported by Wails but is not needed to reproduce the Shiny layout.
- Use `C:\Users\BrunoTremblay\Work\vpro\inst\app\modules\mod_fs882_6x4.R`
  as a presentation precedent: nested12-column groups, related fields together,
  labels attached to controls, groups stacking in reading order when narrow.
- Preserve the app's green/gold palette and typography. Give fields readable
  dimensions and spacing; keep advanced reference details compact/collapsible.
  Horizontal scrolling belongs to wide child tables, not the entire parent form.
- Size the native window within the available work area; a larger initial window
  supplements, not replaces, reflow. Verify wide/maximized, intermediate and narrow
  native sizes, labels/focus order and unchanged Save/Undo/Lock/error/close behavior.
- Keep raw exported geometry/relationships intact as source evidence. Render one
  live input per field rather than separate hidden desktop/mobile copies.
- Presentation verification is complete: all130 frontend tests/check/build,
  full Go race, actual1400x900 startup, thirteen native layout checks and eight
  save/validation/rollback/close scenarios. Routine guidance now sits collapsed
  below the fields; safety feedback and required review remain visible.

## Native verification budget

- One representative valid, NULL, rejected and rollback path per shared mechanism;
  add field-specific cases only for differing source events, constraints or UX.
- Keep exhaustive Unicode/input/alias/collision combinations in fast tests.
- Spend at most 20 minutes or two attempts on a newly broken oracle mechanism.
  Stop, retain a compact failure record and identify the actual blocker. Do not
  generate another large suite or reopen Access repeatedly for static facts.
- Unknown source behavior must remain unknown. A deliberate safer desktop behavior
  is acceptable only when documented and tested, not passed off as exact parity.
- The original installed-reference baseline remains incomplete. The proven
  startup-suppressed FS882 closure is a separate boundary, not a reason to repair
  canonical Access or rebootstrap every ordinary field.

## Retention and reporting

- Keep one current baseline executable and one current verification candidate.
  Archive closed superseded builds with hashes; do not leave a build per field.
- Use one active disposable Access fixture per workflow. Keep source seals,
  compact data/audit differences, failure text and representative owned visuals.
  Do not make full per-case core copies unless necessary for reproducibility.
- Keep existing historical evidence intact until a manifest proves which copies
  can be removed. Cleanup must not affect canonical data, live profiles or shared
  caches. Extra disk space removes the immediate capacity blocker; stop routine
  compression/grant churn.
- Report completed workflows, unresolved blockers and intentional adaptations.
  Field counts, row/cell counts and code size are supporting metrics.

## Immediate next session

Deliverables1-3, Bedrock, responsive FS882 presentation and the 21-field ordinary
parent-code batch are complete. The exact native-tested default-on payload is
promoted; coverage77/98. Its ten native cases compared all15 tables, preserved31
old audits and added exactly24 intended audits, including real rollback/close
retry and per-list checksum failure/Retry. All134 frontend tests/check/build,
default/opt-out builds and full Go race pass.

F1-F3 are complete and promoted. Final Go race root361.871s,137 frontend tests,
check0/0, default/opt-out builds and actual default-on Wails delivery pass.
Native context evidence preserves15 managed/15 external/17 choices tables,
31 historical audits per project copy, exactly one intended managed and one
external audit, all five support files and the custom user file. Runtime YAML and
all fixture bytes remain unchanged on final readonly delivery.
Current bindings after C2:15 services/104 methods/33 models. Recovery retains configuration and
requires explicit correction/restart; it is not automatic repair.
No pending writes or owned native process need continuation.

C1 pooling and C2 verification caching are complete and promoted. C2 native proof
preserves all fixture bytes through warm reads, corruption, remount, failed Retry,
atomic restoration and successful Retry without resetting invalid drafts.
Next complete the remaining C3 browse/reference/catalogue cancellation on the
affected retrieval paths, then classify and implement the remaining ordinary surveyor/text/
depth/cover/note fields from the already sealed source checklist. Do not repeat
reference capture or field-by-field audit matrices. Remaining21: AirPhotoNum, BECSiteUnit, EnteredBy,
HumusThickness, Photo, RootRestrictingDepth, RootingDepth, SeepageDepth,
SoilDrainage, SoilNotes, SoilSurveyor, SpeciesListComplete, StrataCoverHerb,
StrataCoverMoss, StrataCoverShrub, StrataCoverTree, UpdatedFromCards, VegNotes,
VegSurveyor, XCoord and YCoord. Keep Master BEC authorization, BIT/NULL, X/Y,
pictures and SoilDrainage strict membership/reference availability as distinct
workflows. The original42-field checklist is frozen evidence, not current coverage.

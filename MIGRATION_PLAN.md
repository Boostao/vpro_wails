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
| 4a | Bounded backend ownership/performance follow-up (C1-C3 complete) | Context-owned pooling, verified catalogue snapshots and active retrieval cancellation pass full race/frontend/native gates. Warm lookups avoid repeated hashing/scans; corruption, replacement and Retry preserve metadata/drafts/data. C4-C5 structural cleanup is incremental, not a new expansion-blocking rewrite. Preserve sealed baselines and resume parent workflows. |
| 5 | Complete bounded FS882 parent field editing (complete98/98) | All98 mapped parent fields have a native-verified writable baseline, including source-authorized BEC Master, strict SoilDrainage and final X/Y/Photo scalars. This is not full form/application parity; picture management, projection, bulk/reverse copying and remaining child/calculation events stay separately scoped and unavailable until verified. |
| 6 | Complete FS882 child workflows (in progress; bounded source vegetation workflows verified) | Other8, Humus12 and Mineral18 cells/create/delete, Veg Other12 attributes, Collected, species decisions and cover/height drafts are native-verified. Guarded creation/deletion, independent personal definitions and named-scope parent code checking are verified opt-in adaptations. Finish remaining active navigation, parent metadata/profiling and calculation/event behavior. Test identity ownership, NULL, audit, rollback and parent/context lifecycle. Storage mapping alone is not completion. |
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
bounded improvements. C1 pooling and C2 checksum caching are implemented/verified;
C3's planned active retrieval scope is delivered. Preserve sealed baselines and keep this work
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
| C3: End-to-end retrieval cancellation (complete) | Injected contexts reach scoped, browse/hierarchy/reference, catalogue and state/discovery SQL/helpers/lease waits. Generated read promises are retained/cancelled on supersession/unmount. Persistent preference initialization and mutation/switch/close handshakes remain deliberately outside automatic cancellation. | C3a-e focused/full race/frontend/native gates pass, with identical subsequent reads and preserved data/audits/config/supports. Real bindings remain compatible. Expected owned cancellation acknowledgements are handled, while corruption and cleanup failures remain visible. No automatic write retries or mid-commit cancellation. |
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
C3b browse/hierarchy/reference is implemented and promoted: injected contexts
reach the pinned coordinator and all four reference SQL readers; context-aware
lock acquisition also covers coordinator/pool and scoped snapshot waits. App
browse/hierarchy and FS882 reference/species ownership cancels supersession/unmount
without stale results or mutation cancellation. Focused race3.235s, full
race339.024s,143 frontend tests/check0/0/build; actual bindings remain15/104/33.
Native generated browse and species promises interrupt expensive SQLite queries;
switch release0.375s, subsequent header/browse/species/list metadata identical,
no unhandled errors. Synthetic project/reference schemas are removed and exact
original file bytes restored after owned close; all fixture files match.
C3c shared Parent/Geology/Soil/Region/Site retrieval is implemented and promoted:
injected contexts reach queued exclusive locks, profile/row SQL and snapshot cloning.
Six field/reference consumers share catalogue promise ownership; refresh/dispose
cancel reads without late state publication or mutation cancellation. Focused race
4.518s/full race346.630s;144 frontend tests/check0/0/build; bindings remain15/104/33.
Actual Wails proves five generated reload cancellations, unchanged recovery,
supersession/disposal and remount/failed Retry/restored Retry retaining invalid
drafts. All database/audit/config/support bytes match after owned close.
Immediate short-scan native cancellation does not prove mid-SQL interruption;
queued-lock deadlines are proven by tests. Synchronous file reads are checked
before/after, not promised interruptible. Non-cancellation corruption rejections
remain visible even when their promise was cancelled; the failed overly strict
native assertion and exact-byte cleanup are retained.
C3d BEC/Quality/Working Unit catalogue retrieval is implemented and promoted.
Injected contexts cover catalogue locks/pool waits and dynamic Working Unit
project/SU transactions/schema/choices. Cancelled iteration returns no partial rows.
BEC preserves fulfilled zone caching and replaces pending generations without a
late cancelled rejection clearing the new cache. Quality and Working Unit choices
own supersession/disposal. Working Unit preference initialization and setters are
not pure reads and remain uncancelled; no implicit preference retries.
Focused race19.966s/full race335.572s;146 frontend tests/check0/0/build; bindings15/104/33.
Native expensive BEC/Quality/Master/environment queries all interrupt and recover
identical metadata within3s. Sixteen-unit Quality input (source bound15+1) survives
remount/rejected Save; all fixture bytes restored after owned close. The primary
probe incorrectly used7 characters as invalid; corrected UI phase only, without
repeating completed SQL phases. Evidence/cleanup receipts remain in native-read-cancel.
C3e state/discovery is implemented and promoted. Injected contexts reach readonly
connection opening/Ping, project descriptions, SU policy/schema and hierarchy
inspection. Cancellation aborts discovery, never a successful partial state or
non-Sample diagnostic. App refresh generations cancel reads and guard publication
before context switch; switch/initialization commits stay uncancelled.
Wrapped query cancellations retain the exact SDK acknowledgement message; distinct
corruption or temporary-handle close failures are not suppressed.
Focused race9.785s/full race336.269s;147 frontend tests/check0/0/build; bindings15/104/33.
Native expensive metadata SQL interruption/recovery0.063s, identical state and
visible app refresh; all project/audit/config/support bytes restored after owned
close. C3's planned active retrieval scope is complete. C4/C5 remain optional,
nonblocking cleanup; the full application migration is not complete.
Wire C3 along affected retrieval paths without coupling it to an all-service rewrite.
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
parent-code batch are complete. Its ten native cases compared all15 tables, preserved31
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
C3 active retrieval cancellation is complete and promoted. The fourteen-field
ordinary surveyor/text/depth/cover/note batch is also complete and promoted;
coverage is91/98. Source text bounds, signed16-bit Integer and finite Single
physical domains reuse existing transaction/audit/Lock and historical-omission
mechanics. Memo/raw text is preserved; no guessed positivity, percent balancing,
Single rounding or memo short limit was added. VegNotes Tab goes to Terrain/Soils.
Effective omitted input constraints remain unknown: these are source-bound/adapted
policies, not measured exact Access input parity.
Focused race7.861s/full race371.176s;153 frontend tests/check0/0/build;
real bindings15/104/33. One disposable native fixture verified all fourteen visible
labelled controls, two invalid numeric scopes surviving remount and refusing
Save/Lock/close, Undo, atomic cross-Env/Admin audit failure/retry, precise raw values,
Lock/Unlock, memo NULL/Undo and the Tab event. All15 tables were compared;
historical audits and support/config bytes were preserved. Exactly fourteen parent
fields changed and fifteen intended audit rows were added.
Current proof/visuals/promotion: evidence/private/native-ordinary-parent.

SpeciesListComplete and UpdatedFromCards are now separately native-verified and
default-on; coverage is93/98. Source CheckBox bindings have no individual events.
Shared controls explicitly display NULL and offer Clear; omitted TripleState is not
inferred. Existing storage/audit normalization preserves true=-1, false=0 and NULL,
without phantom option history. Unchanged historical true representations are omitted
from unrelated assignments. SourcePage now dispatches owned checkbox editors before
readonly fallbacks; unavailable partners remain disabled.
Focused race18.920s/full race370.621s;155 frontend tests/check0/0/default+optout builds;
bindings15/104/33. One native fixture checked all15 tables,32 historical audits,
exactly six intended flag audits, cross-Env/Admin rollback/retry, remount, Undo and Lock.
Default assets are byte-identical to the native accepted opt-in payload.
Evidence/private/native-parent-flags retains proof, visual, closed failed dispatcher
probe and sealed fallback. No production/source data changes or Access instance.

SoilDrainage is now separately native-verified/default-on; coverage is94/98.
Its explicit source LimitToList/TEXT5 uses exact canonical full-item membership,
without silent recasing/completion/trimming. The input mechanism is adapted,
not measured Access auto-expansion parity. Fourteen reference rows preserve the
empty Item metadata; thirteen nonempty Items are selectable. New/changed values
and restoration targets require verified references; unchanged historical values
and NULL do not. The owned ParentCodeService/error follows immutable scoped writers.
Focused regression185.126s, drainage5.143s/full race373.627s;157 frontend tests,
check0/0/default+optout builds; bindings15/104/33 unchanged. One native fixture
verified strict invalid values, hidden remount/Save/Lock/close refusal, Undo,
rollback/retry, corrupt-reference refusal, explicit Retry and NULL. All15 tables,
32 historical audits and support/config bytes are preserved; exactly two new
drainage audits. Default assets exactly match the accepted native payload.
Evidence/private/native-drainage retains proof/visual/promotion; no Access instance
or production/source writes.

BEC Master is now separately native-verified/default-on for the source-authorized
configured user; coverage is95/98, not95 unrestricted fields for every user.
Static Form_Load unlocks Will MacKenzie; BeforeUpdate contains only commented code.
Desktop case-insensitive/no-trim identity comparison and backend write/restore
restriction are explicit adaptations, not claims of active Access BeforeUpdate
authorization or a new authentication boundary. Physical Admin.TEXT100/raw Unicode,
historical assignment omission, NULL and exact partner preservation apply.
The existing Master catalogue/Working Unit lookup is reused, with independent
Master acknowledgement and scoped cancellable policy reads. Reference-busy controls
are disabled; the closed failed loading probe/unchanged fixture seal is retained.
Full race385.848s;159 frontend tests/check0/0/default+optout builds;
bindings15/106/33. One native fixture verifies raw100/NULL, hidden remount/Save/
Lock/close refusal, audit rollback/retry, unauthorized readonly and direct scoped
backend refusal. All15 tables/32 old audits/support bytes are preserved; exactly
two Master audits; deliberate disposable user preference roundtrip restored exactly.
Default assets match accepted native payload. Evidence/private/native-master-bec
retains proof, two representative visuals, cleanup and sealed drainage fallback.
No Access instance or production/source writes.

The final XCoord/YCoord/Photo scalar batch is native-verified/default-on:
bounded parent-field coverage is98/98. Static source corrects the earlier assumption
that these three textboxes necessarily require projection or picture-manager events.
They are bound/unlocked with no individual events; Env.XCoord/YCoord are nullable
Single, Env.Photo is TEXT50. Shared ordinary validation/session/restoration and
historical-omission behavior is reused, with a separate opt-out. No invented
geographic range, rounding, coordinate conversion, path completion or picture
attachment occurs. The source picture-manager button remains disabled.
Focused regression13.180s/full race473.062s;161 frontend tests/check0/0/default+
optout builds; bindings15/106/33 unchanged. Native two-error hidden remount,
one-error correction, Save/Lock/close refusal, raw50/full-precision Singles,
atomic audit rollback/retry, NULL/Undo and Lock preserve all15 tables/32 old audits
and support/config bytes, with exactly six scalar audits. Canonical scientific
audit text was verified without replaying a completed Save. Default assets match
the accepted native payload; evidence/private/native-final-scalars retains proof/
visual/promotion and the sealed Master fallback. No Access or production/source writes.

Next: complete the actual FS882 child add/edit/delete/filter/calculation workflows.
Picture management, projections and reverse/bulk actions remain independent gaps;
98 writable mapped fields are not complete form/application parity. Reuse sealed
source evidence and shared lifecycle tests, not field-by-field audit matrices.

### Child checkpoint: Other

Static source read first: SubOtherXL/UsysOther/Sample_Other definitions, parent
PlotNumber links and Form_BeforeUpdate's ID-bound AuditTrail call. Five textboxes
and three checkboxes have no individual update/calculation events. Reuse original
DTO/storage mapping, boolean normalization, identity allocation/reservation,
transactional audits and source renderer; do not add editor catalogue databases.

All eight fields are native-verified/default-on with a separate read-only opt-out.
TEXT50/TEXT255 and raw JSON Unicode bounds, NULL/empty physical distinction,
unchanged historical assignment omission and fresh restoration-alias guards
protect Save/Update and selective restoration. Explicit persistent drafts replace
blur saves; exact expected cell values prevent stale full-row replacement.
Hidden-invalid Save/Lock/close refusal, one-error correction, Undo/Cancel,
atomic multirow audit rollback/retry, raw UTF-16 limits, true=-1/false=0/NULL,
create, cancelled/accepted delete and nonreused deleted identities pass.
The existing create dialog initializes two fields; optional cells remain editable
after creation. Report committed/failed-refresh state and disable editing so
neither patches nor creates are replayed. Height and Other drafts mutually gate
unrelated child/header/audit mutations; context transitions reuse native close.

Native evidence/private/native-other seals12 scenarios, all15 original tables/
32 historical audits,16 new audits and the expected identity ledger; support/
config bytes and canonical sources remain unchanged. Remount/Undo readiness and
hosted native confirmations required bounded harness continuations, not repeated
writes or app behavior changes. Final production delivery independently rejects
Unicode-folded malformed header JSON and preserves all fixture bytes. Default
assets match native opt-in assets exactly;168 frontend tests/check0/0/default+
optout builds; bindings15/108/36. Focused integration15.970s, initial full race
495.442s and final-source full race418.460s pass. Final native-verified production
payload5975b991... is promoted; sealed parent fallback retained.

Humus12 and Mineral18 source-bound fields now have complete physical domains,
source depth ordering, nullable canonical suggestions and persistent drafts.
Vegetation species/membership/cover/calculation events remain the next distinct
workflow, not incidental text editing. Storage-bound coverage alone never enables
unmigrated columns, pictures or bulk operations.
Preserve the exported ordering difference: Humus UpperDepth DESC, Mineral
UpperDepth ASC. This ordering is implemented/native-verified, including visible
rows after tab remount; ID breaks equal-depth ties deterministically as a desktop
adaptation. Missing UpperDepth retains ID ordering without advertising the field.
Focused child/context tests7.074s and full Go race408.049s pass. Readonly Wails
evidence/private/native-soil-order preserves all15 original tables, audit history
and project/support/configuration bytes with zero new audits; exact tested
payload643bed0a... is promoted, with the accepted Other fallback sealed.
All110 rows in the nine soil suggestion groups match the static Windows-1252
VLists export for nonempty metadata and normalized BOOLEAN values. This comparison
does not prove NULL/empty-string equivalence; retain both distinctions in transport.
FecalAbundance intentionally uses MycelAbundance suggestions;
RootsAbundance/RootsSize are textboxes, not invented restricted combos.
Humus12/Mineral18 physical save/restoration guards are implemented and
native-verified: nullable TEXT bounds in UTF-16 units, unrestricted valid Unicode
MEMO, Single finite/range and signed16 Integer checks. Share Other's physical
text validator and existing Single/Integer mechanisms; do not inherit parent
code membership/nonempty rules or invent pH, percentage, depth or total limits.
Every source text token is checked before JSON decoder repair, including
duplicate/case-folded keys. Explicit Unicode-folded NULL remains supplied.
Unchanged historical invalid values are omitted from assignments; new invalid
values and fresh restoration targets are rejected across accepted aliases.
Shared fixtures now use source-sized text, preserving typed roundtrip, identity,
audit thresholds, NULL and rollback assertions rather than relaxing guards.
Focused integration74.994s/full Go race432.476s;168 frontend tests/check0/0;
actual scoped Wails rejects
24 requests without mutations and proves historical omission/audit rollback/retry,
with exactly two intended RootsSize edits/audits. Evidence/private/native-soil-domain
retains one disposable fixture and the sealed ordering fallback; exact accepted
payload5d69931d... is archived as the domain fallback with owner/inspector closed.

Complete soil editor delivery is native-verified and enabled by default, with
`VITE_SOIL_CHILD_EDITING=false` as an explicit read-only opt-out. All30 source
fields have one labelled live control, persistent raw numeric/text/memo drafts,
independent errors and expected-value patches. Other and soil reuse one atomic
transaction writer; soil batches span both tables and multiple rows, including
audit, ownership, stale-cell and physical-domain checks. Save/Undo/Cancel/Lock,
native close, context switching and unrelated mutations share the draft gates.
Committed writes clear drafts before refresh; refresh failures explicitly report
committed status and disable capabilities rather than inviting replay.

Suggestions come from the pinned readonly SQLite-family USysTableOfLists view
through cancellable scoped operations. All10 nullable metadata fields and
duplicates are retained, BOOLEAN reads normalized, and missing groups fail
explicitly. Reload/Retry retains drafts. No derived editor catalogue replaces the
database family. Raw-case/nonmembership acceptance remains an explicit desktop
policy, not proof of omitted Access effective combo properties. Clearing text
submits NULL; memo newlines are retained. Creation retains the bounded dialog
with optional fields editable afterwards. Deleted-ID reservations and audit
behavior remain desktop safety adaptations.

Focused Go integration1.853s, core race456.021s and final race428.787s pass;
173 frontend tests/check0/0/default+opt-out builds and bindings15/111/40 pass.
Actual Wails core evidence verifies12 cases/39 intended audits: all30 fields,
cross-table/multirow Save and NULL, persistent hidden errors, remount/Undo,
create, owned cancelled/accepted delete, ID nonreuse and Lock without phantom
history. Final default-on delivery adds exactly two independent-client audits
and proves stale rejection, audit rollback and reference failure/Retry retain
drafts, plus malformed patch Unicode rejection. All15 original tables/32 old
audits, fixture schema/ledger and support/config/external bytes are preserved
except the explicitly expected core changes. Owned core PID11148 and final
PID6780 exited; inspector9392 is closed. Evidence/private/native-soil-editor
holds the completed fixture, accepted core86032440... and domain fallback.
Exact final native-tested payload96697f18... is promoted. Do not replay completed
proofs or treat completed soil cells as vegetation/calculation acceptance.
The original42-field checklist is frozen evidence, not current coverage.

Vegetation physical boundaries are now delivered for all36 mapped fields:
Species TEXT8 required, Layer TEXT2, Collected TEXT1,21 Single fields,
LL/PV signed32 Long and ten signed16 Integers. Raw text tokens are rejected
before JSON repair. Create, explicit Update, compatibility Save and fresh
restoration aliases share the physical guards; unchanged historical assignments
are omitted. Existing height-grid cover `<100 Or Is Null` policy remains
separate from physical Single bounds; it is not invented for other covers.
Cover restoration remains explicitly unavailable. The old precision fixture now
seeds Cover10 overflow as historical data rather than asking a fresh writer to
create invalid storage, and tests omission with a forbidding trigger.

Focused coupled child/height/restoration tests39.230s and full race498.416s pass.
Actual scoped Wails rejects75 exact-reason requests, including create/update,
raw Unicode, fresh restoration aliases and audit rollback. Only two intended
DC edits/audits occur; data returns to the baseline. All15 original tables,
56 fixture/historical audits, ledger and support/configuration bytes remain
preserved. The injected audit trigger is removed; original triggers and the
planned historical guard remain. A no-write visual timeout is sealed from the
exact two-write checkpoint without replaying Save. Owned PID5860 exited;
inspector9392 closed. Native-tested payload115ad838... is promoted, with
soil editor96697f18... sealed under evidence/private/native-vegetation-domain.
Frontend assets/bindings are unchanged. This is protection, not species,
vegetation draft, attribute, creation or calculation workflow acceptance.

Static executable-event evidence also matters for the next workflow:
USysVegA/C/D membership is based on non-NULL covers, including zero.
SubVegA/D BeforeUpdate exits before the duplicate-species merge code;
UpdateMetadataSppList jumps immediately to MyExit. Do not implement those
unreachable writes as parity requirements. Species NotInList, provincial/user
catalogue choices and Collected's NULL/C/V click cycle remain distinct events
to migrate, not ordinary unrestricted text editing.

USysVegOtherXL's12 existing-row numeric attributes are now native-verified and
default-on; `VITE_VEGETATION_ATTRIBUTE_EDITING=false` is the read-only opt-out.
They reuse the shared expected-value transactional child writer, with explicit
nullable values/expected maps shaped like the proven height transport. Only
these12 properties are allowlisted; species, covers, identity changes and row
creation/deletion remain unavailable here. Signed32 LL/PV and signed16 attributes
retain raw integer errors and original expected NULL independently across rows
and tab remounts. Save/Undo/Cancel/Lock/native close/context and unrelated writers
share the draft gates; committed-refresh failure cannot invite replay.

Ten canonical-family suggestion groups return145 full nullable/duplicate
definitions through the shared cancellable pinned SQLite reader. Both Cultural
controls deliberately use Cultural1; AF is free numeric entry. PV follows
source ORDER BY Item, with ItemOrder as a deterministic equal-item tie adaptation;
the other groups use ItemOrder. Source list-name literal casing is resolved to
canonical family names without changing stored values or metadata. Do not read
obsolete ignored root-level legacy seed files as the active database family.
Reload/Retry retains drafts and disables editing/Save on unavailable references.
No per-field events, LimitToList override or BeforeUpdate audit procedure are
exported for this form. Optional raw numeric-code acceptance is an explicit
desktop policy, not omitted-property parity proof; transactional attribute audits
are a desktop safety adaptation, not a claim of source attribute audit events.

Focused shared backend1.763s/full race504.457s;177 frontend tests/check0/0/
opt-in/default/opt-out builds pass; bindings15/114/41. Actual Wails14 cases/
18 intended audits verify all12 fields plus a second row, hidden independent
errors and lifecycle refusal, remount/Undo, NULL, stale rejection, audit rollback/
retained-draft retry and reference failure/Retry. All15 original tables/32 old
audits are preserved apart from exact planned edits; no identity ledger is
allocated by patches. Support/config/external bytes are restored and the injected
trigger is removed. Existing owned ID0 is preserved, not replaced during fixture
preparation. Core PID14488/default-delivery PID3260 exited; inspector9392 closed.
Default frontend assets are byte-identical to accepted opt-in assets and exactly
embedded in the same eb3a009b... executable; no untested rebuild or redundant
integration suite is needed. Final default-on readonly Wails delivery verifies
all12 live labelled controls/145 reference definitions with every fixture byte
unchanged and zero audits. Evidence/private/native-vegetation-attributes retains
the accepted core and115ad838... fallback; exact eb3a009b... is promoted.
Collected_Click is now default-on across SubVegAXL_BC/CXL/DXL/AhtXL/ChtXL;
`VITE_VEGETATION_COLLECTED_EDITING=false` is the read-only opt-out. All five
exported events agree: NULL -> C -> V -> NULL; other values do nothing, rather
than clear to NULL. Readonly native DAO on a disposable exact source copy
confirms ASCII case/fullwidth C/V database equivalence, not accent equivalence.
Those explicit variants are recognized only on an intentional click; unchanged
values are never recased or repaired. General locale collation is not invented.
The attempted DAO StrComp expression is unsupported and not event parity proof.

Explicit signed32 identity, expected nullable text and bounded click-count
transport reuse the shared child patch/audit transaction. Three clicks returning
to the original value create no audit; a multi-click Save audits only its final
value. Historical noncycle values remain untouched. Legacy VegRecord omits NULL
Collected; the draft adapter retains that contract without conflating empty text.
Drafts persist across all cover/height views and tabs and gate other editing,
Save/Undo/Cancel/Lock/native close/context. Failed Save retains drafts; committed
refresh failure reports the commit and disables editing.

Focused Go0.651s/full race493.062s;180 frontend tests/check0/0/default+opt-out
builds; bindings15/116/42 pass. Actual Wails13 cases/8 intended audits prove
shared five-grid rendering, roundtrip/history no-ops, multirow/NULL/fullwidth
events, remount/Undo/Cancel/Lock/close/context, stale rejection and complete
second-row audit rollback/retry. All15 original tables/32 historical audits,
schema and support/config/external bytes are preserved except exact planned edits.
The initial nullable-transport rejection and modal/startup harness continuations
were sealed before any writes; no completed writes were replayed.
Final default readonly delivery verifies five controls with every fixture byte
unchanged and zero audits. Default assets match accepted opt-in assets exactly;
exact156aeb46... is promoted, not a later untested rebuild. Core PID13356/default
PID15216 exited; inspector9392 closed. Evidence/private/native-vegetation-collected
retains accepted/rejected executables and eb3a009b... fallback.
Continue species/cover/height creation/deletion and calculations next.

The species batch starts with source-driven, read-only canonical references,
not the lossy derived Species browse DTO. All five combos explicitly export
LimitToList and share three list classes: A/A-height lifeforms1-4, C/C-height
5-8/12, D1/2/9-11; source U/X types are compared case-insensitively while returned
metadata retains its literals. USysAllSpecies's five-field UNION yields547/
4742/3319 rows from VLists/VUser, excluding S/s and preserving NULL/empty values.
Old-code lookup retains every matching definition, including two distinct
ACAROSPO replacements;217 duplicate OldCode groups forbid silently guessing the
first mapping. Scoped cancellable readers, raw lookup transport and immutable
client adapter are implemented/tested. The reference-preparation full race passed
(root502.078s). Existing-row exact-list selection now has opt-in persistent drafts
across all five grids. The shared writer verifies source-row membership inside
the project transaction; canonical membership reads use the context-owned readonly
family connection, not the project-only writer pool. Mutation and audit remain
one transaction. Native8 cases/3 intended audits verify lists/ambiguous aliases,
Undo, raw overlength/hidden error remounts, Save/Lock/native close guards,
correction, wrong-list rejection, audit rollback/retained retry and raw/stale
transport. All15 tables/32 old audits and support/config bytes are preserved
apart from intended species assignments. Exact-case list selection is a bounded
desktop adaptation; source NotInList decisions and general Access text comparison
are not claimed. Preparation was published as f286d74 after final full race
(root472.744s); its exact default4eb0b8af... remains archived.

Explicit source decisions now extend the same drafts: master OldCode matches
take precedence, every replacement definition remains selectable, keep retains
the entered code, and an existing personal Code is offered only without a master
alias. Review is cancellable and guarded by form/entered/original identity;
decisions survive remounts, Reload and failed Save. Save independently revalidates
the reference identities and source event value, with mutation/audit rollback.
Source UCase is bounded to ASCII for these decisions; non-ASCII event conversion
is explicitly rejected, while exact literal non-ASCII list selection is allowed.
Cover1/6/7 source focus transfers are adapted: mutually gated drafts postpone
unrelated cover editing until Save rather than focusing disabled controls.
Unknown-code personal-list creation stays disabled. It requires a separately
bounded writable VUser/project coordination contract, not editor fixtures or
implicit catalogue mutation. USysAddSpp uses ordinary bound record saves; its
Update button explicitly says "not working" and exits.

Focused Go1.887s,184 frontend tests/check0/0, generated bindings15/121/46 and36
scoped adapter operations pass. Native9 cases/5 intended audits preserve all15
tables/35 prior audits and support/config bytes; default delivery verifies five
enabled labelled controls, no implicit draft, every fixture byte unchanged and
zero new audits. Default assets equal the accepted opt-in assets byte-for-byte.
Exact60575340... is archived/promoted after final integration race passed
(root517.356s). Species is now
default-on with `VITE_VEGETATION_SPECIES_EDITING=false` read-only opt-out;
unrestricted source-grid edits cannot bypass it. Complete remaining cover/height
creation/deletion/calculation workflows while keeping personal creation disabled.

### Existing-row source cover/height drafts

Static active SubVegAXL_BC/CXL/DXL/AhtXL/ChtXL exports confirm eleven cover/total
fields use `<100 Or Is Null` and CC masks; the six height fields have no individual
validation rule or conversion events. Reuse proven strict lexical/finite Single
parsing, preserving float64 precision, negatives, NULL and unchanged historical
values rather than truncating to the mask. TotalA/B remain manual source fields,
not invented automatic summation.

All25 source controls now share one persistent numeric session across five grids.
Each changed field retains its original source form as well as expected value;
the shared child transaction checks the field allowlist and original source-row
predicate before mutation/audit. Zero keeps a row in a source view; clearing its
last applicable cover can remove it from that view without deleting its identity.
Hidden raw errors block Save/Lock/close/context and unrelated workflows. Failed
Save retains the whole batch; changed cover/height partners audit only once.
Legacy height backend transport remains compatible, but active source controls
use the strict source-aware endpoint.

This workflow defaults on; `VITE_VEGETATION_NUMBER_EDITING=false` or retained
`VITE_HEIGHT_EDITING=false` makes source numeric controls read-only while leaving
Species/Collected available. The legacy experimental grid is a read-only preview,
not a generic write bypass. Unverified vegetation creation/deletion controls are
disabled pending their separate source-driven workflow.

Focused coupled Go4.194s,185 frontend tests/check0/0/default+opt-out builds and
bindings15/123/47 with37 scoped operations pass. Native core6 cases/4 audits verify
all25 controls, Undo, hidden Cover9=100/close/Lock guards, shared partner drafts,
multirow rollback/retry, source/stale/domain rejection and NULL Cover6 view removal.
Default delivery verifies25 enabled controls/read-only preview plus one new
Cover2 audit; opt-out verifies25 disabled controls, retained Species/Collected,
zero audits and every fixture byte unchanged. All15 tables/40 prior audits and
support/config bytes are retained except the five intended assignments/audits.
Core2f8d74bd... and default30199c86... are sealed under the existing private species
fixture; completed proofs were not replayed. Default assets are restored exactly
to the accepted default candidate. Final race passes in vegetation-numbers-
integration (root479.695s); exact default30199c86... is promoted without rebuild.
### Guarded source vegetation deletion checkpoint

The existing shared child writer now supports a checked deletion boundary, not a
second mutation implementation. Scoped readonly review captures source form, ID,
species, every physical column and a typed raw-byte fingerprint including rowid,
NULL/empty distinctions and hidden values. Unsupported BLOBs and undisplayable
species reject explicitly. Confirmation revalidates source membership and the
whole snapshot within the reservation/deletion/full-field audit transaction.
Known Flag BOOLEAN audits normalize true=-1. Deleted identities remain reserved.

The opt-in UI (`VITE_VEGETATION_DELETE_EDITING=true`) owns a persistent review,
explicit Confirm/Cancel and cancellable readonly lookup. Ordinary Save and
Save-and-close never confirm deletion. Undo/Cancel reload current rows without
writes; failed mutation retains review; committed refresh failure disables editing
and reports committed state. Remount, parent Lock/context and unrelated writers
share the existing lifecycle gates. Generic preview deletion remains unavailable.

Focused deletion/child/context Go0.933s,187 frontend tests, check0/0, opt-in/default
builds, bindings15/126/49 and39 scoped operations pass. Full integration
`go test -race ./...` passes (root488.272s). Native coref2b6c939... verifies eight
cases/sealing checks, one intended row deletion and exactly five field audits:
Species, three covers and hidden Flag. All15 original tables/45 historical audits
and support/config bytes are preserved except that deletion/history and the
standard shared identity-ledger table/index/reservation. Temporary trigger/schema
changes are restored; seven completed scenarios were sealed without replaying writes.
Defaultd8d51c7f... verifies25 existing numeric controls plus Species/Collected,
readonly scoped review, disabled source deletion and every fixture byte unchanged.
Owned core/default PID9676/1244 exited; inspector9392 closed.

Access exports omit AllowDeletions/AllowAdditions and deletion events. The bounded
disposable installed-core property probe reached a startup relocation dialog
despite AutomationSecurity=3; it was not acknowledged and no source form opened.
Only verified owned Access PID6008 was stopped. Canonical core bytes and the
pre-dialog registry Location are unchanged. Effective properties remain unknown,
so deletion stays default-disabled; do not repeat this broken oracle mechanism
or infer properties from IsLoaded. Retain the stopped fixture/evidence.

### Guarded source vegetation creation checkpoint

Source exports and Sample_Veg definitions were read before implementation. Active
forms have no exported insertion events or numeric defaults; ID uses GenUniqueID
with a nonunique LONG index. Omitted addition properties remain unknown. Creation
therefore stays default-disabled (`VITE_VEGETATION_CREATE_EDITING=true` opts in),
and does not imply complete Access metadata/event parity.

Strict scoped transport accepts only explicit form/species/values, with no caller
ID, parent, Layer or unknown properties. Exact canonical-family species membership,
source visibility and shared Single bounds reject invalid proposals before writes.
There are no guessed zero covers, Layer values, totals, aliases or personal codes.
C/C-height require explicit non-NULL Cover6; A-height permits height-only rows.
The shared child transaction allocates/reserves identity, inserts, independently
observes ownership/species/every proposed numeric or NULL value, then audits and
commits. Species/numeric trigger drift, audit failure and collisions roll back
data/history/reservations together. Existing writer callers retain their behavior.

Persistent source-tab-ordered labelled drafts use shared numeric parsing and
canonical references. Cancel/Undo write nothing; raw errors survive remounts and
gate Save/Lock/native close/context/unrelated edits. Failures retain the draft;
successful refresh independently verifies returned identity/species. Committed
refresh failure reports committed state and disables editing, preventing replay.

Focused final Go1.261s,191 frontend tests, check0/0, opt-in/default builds and
bindings15/128/50 with40 scoped operations pass. Full final `go test -race ./...`
passes (root478.504s). Native core497c6455... creates IDs2/3/4 with seven audits;
independent guard694e7ab2... rejects species/numeric trigger drift then creates
ID5 with two audits. All15 original tables/50 prior audits and original positive
ID1 survive; only four rows, nine audits and their shared reservations change.
Support/config/schema are preserved except intentional ledger contents. Harness
integer-zero/REAL and tuple/JSON-list expectations were corrected without replay.
The core25-control summary survives; its detailed array was not retained.
Defaulta7dd1be8... independently verifies25 enabled numeric controls plus
Species/Collected, readonly review, no creation/deletion controls or implicit
drafts and every fixture byte unchanged. Owned core/guard/default PID11272/
6388/12044 exited; inspector9392 closed. Exact tested default is promoted, with
previous accepted binaries archived. Never replay completed creation proofs.

Next bounded species batch: exported NotInList explicitly uppercases sysNewSpp
and opens USysAddSpp. Read its bound Code/ScientificName/EnglishName/LifeForm and
active close/open events, original VUser schema and coordinator ownership before
implementing a writable family boundary. Preserve NULL/empty metadata, duplicate
definitions and descriptions; do not assume attached multi-file atomicity or
change journal modes. Keep personal creation disabled until collision, ownership,
rollback/retry, aliases and safe project/draft coordination are verified.

#### Personal-definition boundary and opt-in metadata drafts

Original `VUser/Tables_Def/USysUserSpp_CreateSQL.txt` confirms Code TEXT8 with a
unique index, nullable ScientificName/EnglishName TEXT255, signed16 LifeForm,
Report SINGLE default1, nullable SppNumber LONG and Codetype TEXT1 with no default.
USysAddSpp exposes only Code/LifeForm/ScientificName/EnglishName; its value list has
12 lifeforms. Open prepopulates sysNewSpp then clears it, Close invokes DoCmd.Close,
and Update exits after "not working". Do not invent a Codetype='u' assignment:
new NULL-Codetype definitions do not enter normal U/X lists; the existing explicit
personal-code decision can retrieve them independently.

The scoped boundary owns a single VUser transaction
for the definition and one explicit CreateRecord snapshot in its original
USysAuditTrail. It is a desktop audit adaptation, not source audit/restoration
parity. Original tables/defaults/NULL-versus-empty metadata/descriptions survive.
It checks attached file identities and rejects a user file shared with another
role; the writer attaches VLists readonly for master/current/old-code collision
checks. NULL-code alias rows remain distinguishable from usable non-NULL aliases,
including empty codes; this reuses existing source-decision availability rules.
No caller-selected path, hidden fields, ID or project assignment is allowed.
The transaction independently checks all bound values and hidden defaults after
audit triggers. This does not promise cross-file atomicity: creating a reusable
definition is an explicit user-file operation, and assigning it to a plot remains
a separate conflict-checked draft Save. Existing WAL mode is retained unchanged.

Focused coupled Go race tests pass (final6.224s; scoped binding Go2.894s): exact UTF-16 limits, raw malformed
Unicode, NULL/empty metadata, all historical user rows and schema/descriptions,
other family/project/config bytes, master/alias/user collisions, audit failure,
insert/audit-trigger drift and retry, cancellation while queued/SQL-blocked,
concurrent duplicate creation, stale/closed contexts and readonly references.
Final full integration passes (root507.979s); earlier boundary race464.624s is
retained. Bindings15/129/51 and41 immutable scoped operations are wired.
Source-linked metadata drafts are native-Wails-verified and opt-in only
(`VITE_PERSONAL_SPECIES_EDITING=true`). Code/Lifeform are grouped side by side
when wide; labelled names stack beneath. Explicit NULL switches distinguish
empty names, source12 lifeforms are selectable without inferred classification,
and errors survive remounts. Ordinary plot Save never writes VUser; explicit
definition Save independently reloads metadata before staging an existing-personal
decision. Cancel/Undo make no new writes; a saved reusable definition deliberately
survives plot Undo. Known committed cleanup/refresh failures report committed
state, clear only the committed metadata and disable capabilities to avoid replay.

Native corec9f12600... proves seven cases: raw256/remount/Save/Lock/actual-close
guards, Cancel/Undo, strict transport, audit rollback/retry, explicit user-only
Save, independent lookup, saved-definition preservation and separate plot Save.
Independent guard uses visible C-row -9 (row0 has historical Cover6=NULL), validates
255 UTF-16 units, source Lifeform3, exact name case/spacing and stored metadata/
audit trigger drift rollback/retry. Its initial two pre-write harness stops
(invisible C-row0 and already-open editor) changed no data; no completed writes
were replayed. Exactly two user definitions/two user snapshot audits and one
separately committed project species audit change; all15 project tables/59 prior
project audits and all original user tables/prior user audit/schema/descriptions/
other family/config bytes survive apart from those explicit changes.
195 frontend tests, final targeted26/check0/0 and opt-in/default builds pass.
Defaultbb3652f3... verifies25 enabled numeric controls plus Species/Collected,
readonly new-personal lookup, unknown review/Undo, disabled personal/source
creation/deletion and every fixture byte unchanged before/after clean close.
Core17888/guard3976/default12924 exited; inspector9392 closed. Exact accepted
default is promoted; prior a7dd1be8... and core/default are archived with hashes.
Never replay personal.py/personal-guard.py/personal-delivery.py.

### Explicit species decisions in source creation checkpoint

New source-row proposals now support reviewed old-code replace/keep and existing
personal-code decisions, reusing the existing source UCase/reference validators
and shared allocation/reservation/audit transaction. Strict transport adds only
decision/entered/selected provenance; caller identities/paths/hidden fields remain
unavailable. Retyping Species invalidates the decision and reviewed choices;
numeric edits and remounts retain them. NULL alias codes are unusable, duplicate
definitions remain separate, master aliases retain precedence, and saved personal
definitions with Codetype=NULL do not acquire invented dropdown membership.
New-row Save writes only the project; personal-definition Save remains separate.
Unknown-code metadata creation directly from a new-row proposal is still disabled.

Focused coupled Go2.888s and full `go test -race ./...` root475.761s pass.
197 frontend tests, check0/0, opt-in/default builds and bindings15/129/51 pass.
Native core8e2dde81... verifies seven cases: ambiguous review, unresolved remount/
Save/Lock/actual-close refusal, Cancel with zero writes, scoped invalid/stale
requests, explicit replacement/keep/personal creation and audit rollback/retry.
IDs6/7/8 add exactly seven project audits and zero user audits. All15 original
project tables/60 prior audits, existing rows/reservations, user/reference/config
bytes and schema survive apart from intended project changes/reservations.
Default8e63916d... verifies25 enabled numeric controls plus Species/Collected,
readonly saved-personal lookup, all unfinished creation controls disabled and
every fixture byte unchanged. Core5568/default9904 closed; inspector9392 closed.
Exact accepted default is promoted; builds and previous bb3652f3... are archived
with hashes under native-vegetation-species/archives/creation-decisions-checkpoint.
Never replay creation-decisions.py completed modes or prior proofs.

Next complete active metadata/calculation events. New-row personal metadata is
separately bound and verified below, without combining user/project writes. Full Access addition/deletion/
personal-form effective properties remain unmeasured and source workflows stay
default-disabled; full species/FS882/application parity is not yet complete.

Static follow-through narrows the next event batch: the five XL form update
handlers call UpdateMetadataSppList, but its exported implementation in
V7mdlAttachMasterLists immediately branches to MyExit before opening/updating
project metadata. Do not implement that unreachable write. The A-cover and D
duplicate-species merge blocks are likewise after unconditional Exit Sub.
C/C-height NULL Cover6 messages do not set Cancel; A/A-height's cover message is
in AfterUpdate, not a blocking validation rule. Preserve explicit draft Save and
source-view removal without turning these messages into rejects or deletions.
NotInList routes focus to Cover1 (A/A-height), Cover6 (C/C-height), or Cover7 (D);
reconcile that with the desktop's intentional mutually gated draft workflows
before claiming event/focus parity. These static findings are not a new native
Access runtime measurement or permission to repeat the stopped property probe.

### Personal metadata from new-row proposals checkpoint

The shared metadata editor now distinguishes an existing row/original expected
value from a unique new-row proposal key. Neither metadata nor creation transport
sends that UI identity to the writer or invents a physical ID. Reviewed unknown
codes preserve original form/raw context before and after explicit VUser Save.
Metadata entry blocks proposal mutation/cancellation and ordinary Save/Lock/close.
First Undo cancels metadata without losing the original proposal; second Undo
cancels the proposal without writes. Known committed cleanup/refresh/source-match
failures still prevent replay. Independent metadata reload stages the existing
personal decision; project Save/Undo remain separate and retain saved definitions.
This uses the existing original-table VUser writer, not a new storage boundary.
Both creation and personal flags must be enabled; default delivery keeps them off.

Focused coupled Go2.511s/full race488.508s,200 frontend tests, check0/0 and
opt-in/default builds pass. Native coreccb31517... verifies eight cases: raw256/
remount/Save/Lock/actual-close/two-stage Undo, user-audit rollback/retained retry,
explicit user-only Save, proposal Undo retaining the definition, separate project
audit rollback/retry, and unchanged existing-row metadata identity/cancellation.
One ZPNROW01 definition adds one VUser snapshot audit; subsequent ID9 creation adds
two project audits. Original15 project tables/67 prior audits, original VUser
tables/three prior audits, nullable defaults, schema/descriptions and other
family/config bytes are preserved apart from those intended changes/reservation.
Default50866a49... verifies25 enabled numeric controls/Species/Collected,
readonly saved definition with literal names/NULL metadata, disabled unfinished
creation and all fixture bytes unchanged. Core8104/default14528 exited;9392 closed.
Exact accepted default is promoted; current core/default and prior8e63916d...
are sealed with hashes in archives/creation-personal-checkpoint. Never replay
completed creation-personal.py modes or preceding proofs.

Next active event UX is bounded to source warnings/focus adapted to explicit
draft Save, not unreachable metadata/duplicate-merge code. Parent code-check,
project-metadata and profiling entrypoints remain separate workflow deliverables.
CheckSpeciesCodes actively includes a blanket VUser LifeForm999 deletion; do not
inherit that destructive cleanup as a desktop requirement.

### Nonblocking C/C-height source notice checkpoint

Changed existing-row Cover6 values from non-NULL to NULL now produce the source's
nonblocking warning, adapted to explicit draft Save. The notice explains removal
from both C source views without physical deletion. Invalid numeric input remains
blocking, not a NULL assignment. Correction/Undo clear the notice; remount and failed
Save retain it, and successful feedback captures it before drafts clear.
Shared source cells highlight actual focus with the existing visual vocabulary and
keep a keyboard outline; this does not claim automatic Access focus navigation.

Focused Go0.752s/full race459.824s,202 frontend tests/check0/0/default build pass.
Native seven-case proof checks actual focus styling, warnings, remount/correction/
Undo, audit rollback/retained retry and physical-row retention after view removal.
Only ID8 Cover6 becomes NULL, adding one project audit and no VUser audit.
Independent readonly delivery of the same20a6c3e7... bytes verifies25 labelled
numeric controls, Species/Collected, disabled unfinished workflows and every fixture
byte unchanged. Owned9884/12208 exited;9392 closed. Exact accepted bytes are promoted;
archives/cover-notices-checkpoint preserves them and previous50866a49 builds with
hashes. Do not replay completed cover-notices.py modes.

Remaining deliberate navigation,
project metadata and profiling remain active deliverables. Neither unreachable
metadata/merge code nor blanket personal-definition cleanup becomes a requirement.

### Shared A/A-height cover-warning checkpoint

The exported AfterUpdate concatenates Cover1/2/3/TotalA/Cover4/5/TotalB and warns
when the result is NULL. VBA's [documented ampersand semantics](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/ampersand-operator)
make this an all-seven-NULL test, not a sum/positive-cover rule. Zero is a value;
heights do not substitute for cover. The shared desktop helper merges only changed
numeric drafts with the complete original row and also covers Species, Collected
and opt-in height-only creation. Malformed/no-op drafts do not create numeric
notices. Collected keeps its original source form through remount/alternate-view
clicks without adding it to the backend transport. Notices do not block Save or
infer cover/Layer/delete records. Successful Species/Collected/creation feedback
uses independently reloaded physical rows; explicit Save replaces Access's
automatic next-record/focus navigation.

All256 A/A-height cover-presence combinations plus NULL/zero/heights, identity,
remount/correction and shared workflow wiring are tested. Focused coupled Go3.833s/
full race460.449s,207 frontend tests/check0/0/core+default builds pass. Native core
a13aca2c... passes nine cases, changing only ID7 Species/Height1/Collected and
creating ID10 with explicit Species/Height1. Five project audits/zero user writes;
all old tables/history/other fields/reservations/schema/support/config are preserved.
Independent defaultcddefe55... proves25 labelled numeric controls/Species/Collected,
disabled unfinished workflows and every fixture byte unchanged. Owned7548/7260
exited;9392 closed. Exact default is promoted; core/default/previous20a6c3e7 and
receipts are archived with hashes in archives/a-cover-notices-checkpoint. Do not
replay completed a-cover-notices.py modes.

### Named-scope parent species-code checking

The source parent call scans the project or selected SU, not merely the visible
plot. Source Change All broadens to the entire project; Ignore All writes
temporary LifeForm999 definitions that Close/completion blanket-delete. The
verified desktop adaptation instead names its context-selected project/SU scope,
reviews physical rows and literal master/user targets, and applies explicit
replacements only within that scope. Ignore/Ignore All stay runtime-only.
Existing LifeForm999 definitions are preserved. No temporary definitions or
cross-file atomicity are claimed.

Multi-plot patches reuse the existing child transaction, ownership, lock, physical
domain and audit helpers. Final observations run after every mutation/audit,
including later-row/audit-trigger effects. Strict transport preserves explicit
NULL originals, rejects malformed Unicode/caller scope and omits no-op assignments.
Target review retains duplicate definitions and NULL/empty metadata, independently
of child dropdown membership. A persistent paginated panel owns raw errors,
proposals and close/context/Save/Lock/Undo gates. Shared personal fields preserve
their labels/IDs; separate VUser Save verifies the original source and reloaded
definition, and Undo retains the saved definition. Committed refresh failures
disable replay.

`VITE_SPECIES_CODE_CHECK_EDITING=true` remains opt-in. Personal entry additionally
requires `VITE_PERSONAL_SPECIES_EDITING=true`. Focused Go6.230s/full race478.834s,
213 frontend tests/check0/0 and distinct core/default builds pass; bindings are
15 services/132 methods/55 models,44 scoped adapter operations. Coreffc0b393...
passes eight native cases: whole-project scope, three distinct target definitions,
pagination/remount/raw errors, actual-close/context/generic Save/Lock refusal,
runtime-only Ignore All, second-plot audit rollback/retained retry, independent
personal Save and Undo/reload. Only two Species assignments/two project audits
and one user definition/one user audit are committed. Old tables/rows/history,
LifeForm999 sentinel, schema, reference/config bytes are preserved. Go tests also
cover selected-SU exclusion, NULL originals, stale/cancelled requests, malformed
storage and later-trigger drift.

Defaultde15b9c3... independently verifies35 labelled existing vegetation controls,
disabled checker/creation/personal workflows and every fixture byte unchanged.
Owned5028/18364 exited;9392 closed. Archives/species-code-check-checkpoint seals
exact core/default/previouscddefe55 bytes and receipts. No completed native writes
may be replayed; the readonly harness continuations are sealed separately.
This completes the bounded checker, not the full child/application migration.

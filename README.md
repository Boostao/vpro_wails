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
| FS882 parent field editing | **98/98 mapped parent fields are verified writable**, including conditionally source-authorized BEC Master, strict SoilDrainage, X/Y scalar coordinates and Photo text. This completes the bounded parent-field baseline, not all form events, children, projections, picture management or application migration. |
| FS882 children and height | Other8, Humus12 and Mineral18 source-bound fields, create/delete and persistent conflict-checked drafts are native-verified. Soil edits save atomically across rows/tables, retain raw numeric/text errors through remounts, and use nullable canonical-family suggestions with explicit Reload/Retry. Source depth ordering and physical domains preserve unchanged historical values. `VITE_SOIL_CHILD_EDITING=false` provides a read-only soil opt-out. All12 Veg Other numeric attributes have verified persistent multirow drafts and canonical suggestions; `VITE_VEGETATION_ATTRIBUTE_EDITING=false` is their read-only opt-out. Collected buttons share source NULL/C/V cycle drafts across all five cover/height grids, with atomic final-value audits and unchanged noncycle history; `VITE_VEGETATION_COLLECTED_EDITING=false` disables them. All36 mapped vegetation fields have physical storage/restoration guards. Eleven cover/total fields and six heights now share source-bound existing-row drafts across all25 controls; `VITE_VEGETATION_NUMBER_EDITING=false` (or the retained `VITE_HEIGHT_EDITING=false`) makes them read-only without disabling Species/Collected. The old experimental vegetation grid is read-only; generic row creation/deletion controls are disabled. Guarded source creation/deletion with explicit species decisions and separately saved personal definitions for existing/new-row drafts are verified opt-in adaptations below. Full vegetation event parity, pictures and calculations remain incomplete. |
| Audit and lifecycle | Data/audit transactions, bounded selective restoration, Undo, and native window/context Save/Discard/Cancel are verified, including hidden-invalid parent/height/Other drafts, failed save/config publication and stale-context rejection. Cover restoration, remaining child dirty-state propagation and multiwindow coordination remain incomplete. |
| Guarded vegetation deletion | **Implemented and native-Wails-verified, opt-in only:** `VITE_VEGETATION_DELETE_EDITING=true`. Review identifies the entire physical record, not just visible cover/height cells. Explicit confirmation rechecks source membership and a raw typed full-row snapshot inside the shared deletion/reservation/audit transaction. Cancel/Undo write nothing; review and errors persist through remounts and failed deletion, blocking ordinary Save/Lock/Save-and-close and unrelated edits. Hidden-field drift, audit rollback/retry, BOOLEAN=-1 audits and reserved deleted IDs are verified. Effective omitted Access deletion properties remain unknown: a disposable property probe stopped at an unacknowledged startup relocation dialog. This is a safer desktop adaptation, not a claim of complete Access deletion parity; deletion remains disabled by default. |
| Guarded vegetation creation | **Implemented and native-Wails-verified, opt-in only:** `VITE_VEGETATION_CREATE_EDITING=true`. Source-ordered labelled drafts require an exact canonical species or an explicit old-code replace/keep or existing-personal-code decision, plus explicit numeric/NULL values, without guessed Layer, zero covers or automatic totals. Decisions reuse the existing source UCase/alias-precedence validators; numeric edits never discard a decision, while retyping Species requires fresh review. Source visibility, Single domains and independent stored species/numeric observations gate the shared allocation/reservation/audit transaction. C/C-height require explicit Cover6; A-height can remain absent from its cover view. Errors survive remounts and block Save/Lock/close; cancellation writes nothing, failures retain drafts and committed-refresh errors prevent replay. Saving a new row never writes VUser or invents dropdown membership. With personal editing also enabled, reviewed unknown codes open a separately saved personal-definition draft; proposal Undo retains saved definitions. Effective Access addition defaults and complete metadata/events remain unknown. Default delivery preserves existing editors and leaves creation/deletion disabled. |
| Species selection and explicit decisions | Existing-row canonical-list selection and explicit old-code replace/keep or existing-personal-code decisions are native-verified across five grids and default-on; `VITE_VEGETATION_SPECIES_EDITING=false` is the read-only opt-out. Review preserves every alias definition and master-alias precedence, including nullable personal metadata. Persistent raw errors and decisions survive remounts, Reload and failed Save; original values and source-row membership gate atomic project/audit writes. Event uppercasing is verified only for ASCII; exact literal non-ASCII list selection remains allowed. Exact-case list matching and explicit draft Save are desktop adaptations, not general Access text-matching parity. |
| Vegetation source warnings and focus | C/C-height Cover6 changes to NULL show a nonblocking notice: Save removes source-view membership, not the physical vegetation record. A/A-height warns when all seven A/B covers are NULL, including height-only records, across numeric, Species, Collected and opt-in creation. Zero is a cover value; heights do not supply missing covers. Notices survive remounts and failed Save, clear on correction/Undo and remain in successful Save feedback; no cover is inferred or record deleted. Malformed numeric input remains blocking. Source cells highlight focus and retain a keyboard outline; automatic Access focus transfers remain a separate adaptation. |
| Parent species-code check | **Implemented and native-Wails-verified, opt-in only:** `VITE_SPECIES_CODE_CHECK_EDITING=true`. Read-only review names the entire project or selected Working Unit and includes physical rows outside the visible plot. Explicit literal registered replacements save atomically across plots with original-value, scope, lock, identity and final stored-value checks. Duplicate master/user definitions and NULL/empty metadata remain distinct. Raw errors/proposals survive tab changes and block ordinary Save/Lock/close/context switching; failed writes retain reviewed retry. Ignore/Ignore All are runtime-only and never create or blanket-delete LifeForm999 definitions. With personal editing also enabled, unknown codes can receive a separately saved reusable VUser definition; Undo never deletes it. Named-scope matching, explicit Save and nondestructive ignore are safety adaptations, not complete Access checker parity. |
| Project metadata | **Existing-record editor implemented and native-Wails-verified, opt-in only:** `VITE_PROJECT_METADATA_EDITING=true`. Explicit physical-record selection opens70 source-labelled responsive controls with persistent nullable text/year/collection drafts, canonical references and original project/master history suggestions. Raw numeric/UTF-16 errors survive tab remounts and block Save/Lock/native-close/context switching; failed reference reloads disable stale controls until recovery. All70 assignments produce73 atomic field/version/date audits; second-audit rollback retains retry, and committed-refresh failures prevent replay. Full typed originals, immutable IDs/ProjectID and final whole-row observations prevent stale or hidden changes. Nine collection fields use complete/partial/none options, not BOOLEANs; eight strict quality combos store registered `Note`, not `Item`. Versions preserve table-object descriptions without an `Unknown` fallback. Changed DEIF/DTE/LMH25 standards require keeping current drafts or explicitly previewing/applying24 source literals. Application stages drafts only, preserves other raw errors and permits later manual edits;25 explicit fields/3 stamps produce28 atomic audits. **Blank creation is independently opt-in:** also set `VITE_PROJECT_METADATA_CREATION=true`. An explicit empty-schema proposal requires the selected plot's already-existing literal ProjectID within20 UTF-16 units; it never assigns parent identity. Create allocates a reserved signed32 ID, preserves all73 other columns as NULL (including version/date stamps), and atomically writes one complete typed75-column creation audit. Deleted/audited IDs remain reserved, full parent storage drift rolls back, failures retain retry, and committed-refresh errors prevent replay. Undo allocates nothing; ordinary editing/Save follows separately. Explicit template creation is described below; metadata restoration remains unavailable. |
| Personal species definitions | **Implemented and native-Wails-verified for existing-row and opt-in new-row unknown-code decisions:** `VITE_PERSONAL_SPECIES_EDITING=true`. Source Code/Lifeform/Scientific Name/English Name drafts retain UTF-16 errors and NULL/empty metadata through remounts. A discriminated editor identity captures either an existing row/original value or a unique new-row proposal key; no physical ID is invented. Explicit definition Save writes only original VUser.USysUserSpp and one transactional CreateRecord snapshot in VUser.USysAuditTrail; plot Save/Undo is separate, and Undo never removes a deliberately saved reusable definition. File identity/role, user/master/usable-alias collisions, stored metadata/defaults and audit observations are guarded. Report=1, SppNumber=NULL and Codetype=NULL are preserved; no U/X membership is invented. Failed writes retain drafts; known committed cleanup/refresh failures block replay. Source native defaults/events remain incompletely measured, so creation stays default-disabled. Full event parity remains incomplete. |
| Soil classification | Two independent nullable four-UTF-16-unit editors default on. Native selection/manual entry, NULL, Undo, Lock, hidden validation, atomic rollback/retry and actual window-close recovery are verified, alongside frozen catalogue browsing. |
| Bedrock classification | Three independent nullable four-UTF-16-unit editors default on, using the frozen87-row catalogue. Native full-item/raw entry, NULL, hidden validation, rollback/retry, Lock and clean close pass. Source effective properties remain unmeasured; this is an explicit safer adaptation, not exact Access input-mechanism parity. |
| Ordinary terrain/classification codes | 21 independent nullable editors default on: Realm, coarse-fragment lithology, surface/subsurface terrain, flooding, humus, hydrogeology, rooting type/particle size and water source. Field-specific UTF-16 bounds, per-list failures/Retry, draft-bound review, hidden validation, rollback and native close recovery are verified. SoilDrainage uses its separately verified strict membership policy. |
| Source Enter navigation | **Bounded native-Wails-verified adaptation, opt-in only:** `VITE_SOURCE_ENTER_NAVIGATION=true`. Plain parent input Enter advances through available displayed fields in responsive logical order, not exact Access TabIndex order. The five source-linked A/C/D and Humus/Mineral grids advance to the next existing record in the same literal column. No wrap, record allocation or Save occurs. Closed/disabled/readonly controls are skipped; invalid raw input retains focus. Lists, selects, multiline input, IME, modifiers and other child forms retain existing native behavior. Memo fields can be destinations while their own Enter still inserts a newline. This is a focus-local runtime policy, not an imported application-wide Access option or YAML preference. Full Access navigation parity remains incomplete. |
| Stored plot-profile review and execution preview | **Implemented and native-Wails-verified, opt-in:** `VITE_PROJECT_PLOT_PROFILE_REVIEW=true`; add `VITE_PROJECT_PLOT_PROFILE_RUN=true` for execution. Original nine-field schemas, physical identities, historical/extra values and `_table_metadata` descriptions are preserved. Explicitly reviewed project-local lump definitions and subvariety choices feed isolated SQLite TEMP execution; Env/Veg/Lump remain selected-project/SU scoped even with external rules. Eight source-ordered steps produce 11 of 52 canonical plots in both tested modes. Invalid/duplicate Orders, stale snapshots, unsupported fragments and unverified locale/date/non-ASCII semantics fail explicitly; defective source Lump/Any/Add equality remains unavailable. Preview never automatically writes counts or applies navigation. Separate gates below govern selection, rule writes and navigation. Static fidelity/native Wails validation is not native Access execution parity. |
| Existing project-profile rule editing | **Implemented and native-Wails-verified, independently opt-in:** add `VITE_PROJECT_PLOT_PROFILE_EDITING=true` to the review gate. Eight responsive labelled fields per physical rule retain raw signed16/255-UTF-16/nullable drafts across tabs. Table-to-Field proposals are visible; unrelated values are not cleared. Env field suggestions belong to the selected project, Veg suggestions preserve grouped VLists codes and NULL/empty definitions, and Lump suggestions require explicit project-lump review. Layer/Operator changes enforce the source lists; other suggestions do not invent effective LimitToList behavior. Only changed assignments are saved; historical invalid/extra fields and PlotCount remain untouched. Full original snapshots, physical identities, new Order collisions and file-role checks protect the owned transactional writer. Rules and complete typed before/after snapshots commit together to project-local `__VPRO_ProfileHistory`; schema, inserted history and final rules are independently observed. This is desktop technical history, not a native Access audit/restoration claim. Missing/altered history, count drift, stale snapshots and ownership changes roll back; committed refresh/cleanup failures block replay. Independent Save/Undo preserve parent drafts, while raw errors/dirty rules block Run/Reload/Lock/close/context. Native tests verify literal/NULL writes, collision rejection, actual history-trigger rollback/retry, remount/native-close guards, narrow visible labels and exact original fixture restoration. Rule creation/deletion require the separate gates below; technical-history restoration remains unavailable. All profiling gates remain default off. |
| Project-profile rule creation/deletion | **Implemented and native-Wails-verified, separately opt-in:** review/editing plus `VITE_PROJECT_PLOT_PROFILE_CREATION=true` or `VITE_PROJECT_PLOT_PROFILE_DELETION=true`. Creation reviews eight explicit nullable assignments without a fake identity or inferred defaults/counts; extra/default/generated fields make it unavailable. Deletion requires full typed confirmation. Raw proposals/errors survive remounts and own Run/Reload/Lock/close/context guards without discarding parent drafts. Signed64 IDs remain reserved across deletion/history; exhaustion, malformed aliases, collisions and ownership changes fail. Rules, reservations and full typed technical history commit atomically; rollback retains retry state and committed cleanup failures block replay until Reload. Native rollback/retry, NULL creation, ID nonreuse, visible labels and exact fixture restoration are verified. These are bounded desktop adaptations, not Access AddNew/audit parity. External/other-table writes require the independent session authorization below; new profile-file/table creation and restoration remain unavailable. Save as SU has its separate gate below. |
| Project-profile result navigation | **Implemented and native-Wails-verified, separately opt-in:** review/run plus `VITE_PROJECT_PLOT_PROFILE_FILTERING=true`. Explicit Apply/Clear/Previous/Next reuse Save/Discard/Cancel and independently revalidate stored inputs and exact membership/counts in the same SQLite snapshot. Context/SU/profile changes retire the ephemeral recordset; failed publication retains it with visible errors and confirmed plot changes remount the editor. Ancestor-disabled publication preserves draft ownership. No persistent filter, count rewrite, preference or implicit SU is created; navigation is not writer authority or hierarchy/report scope. Empty/>200 results warn without destructively clearing a prior filter. Native canonical52/11 navigation, narrow labels, cancellation/discard, one independent parent Save/audit, revalidation, SU retirement and exact fixture restoration pass. Restoration remains unavailable; Save as SU requires its separate opt-in/review below. |
| Independent stored profile selection | **Implemented and native-Wails-verified, separately opt-in:** `VITE_PLOT_PROFILE_SELECTION=true`. Inspect original project/core or an explicit existing absolute SQLite file; unavailable physical schemas show reasons. Select preserves literal rules/descriptions and rotates the owned editor identity without changing project/SU/hierarchy. Shared Save/Discard/Cancel retains drafts on cancellation/failure and retires stale lists/navigation only after publication. Runtime YAML atomically persists `Current.CurrVegProfile/ProfilePath`; missing ProfilePath preserves the previous inactive preference/project-local baseline, while explicit None survives restart without fallback. External/other-table profiles start read-only in UI and backend; the independent session authorization below is required for writes. Canonical project-owned rule editing remains available and parent/child writers never follow the profile attachment. Native Cancel/Discard/Save, readonly52/11 navigation, one exact independent parent audit, narrow visibility, default-off delivery and persisted-None restart pass; original fixture/support/configuration bytes are restored. All nine profiling gates remain default off. New profile-file/table creation and attachment administration remain separate gaps. Save as SU has its separate gate below. |
| Reviewed profile Save as SU | **Implemented and native-Wails-verified, separately opt-in:** review/run plus `VITE_PROJECT_PLOT_PROFILE_SAVE_SU=true`. Independently review stored result membership, the original empty SU template, selected SiteUnit records and optional table-object descriptions. Explicit Create rechecks them after Save/Discard/Cancel and publishes only a new absolute `.db` file; existing files are never replaced. Original `<Name>_SU` naming, unique PlotNumber/nonunique SiteUnit indexes, NULL/empty/literal values and available typed description metadata are retained; absent metadata stays absent. SU rows and complete technical provenance commit together before atomic no-replace publication. Cancellation, collision, stale input, malformed Unicode, UTF-16 bounds, template changes, rollback/cleanup and retry are tested, including exact0/200/201 rows. Publication does not change project/SU/hierarchy, original databases or configuration. Native Cancel/collision/Discard/Save, selected SiteUnits, one independent parent audit and narrow visible labels pass; every fixture byte is restored. Existing-file destinations, automatic attachment and master authorization remain unavailable. All nine profiling gates are default off. |
| Independent profile write ownership | **Implemented and native-Wails-verified, separately opt-in:** selection plus `VITE_PLOT_PROFILE_WRITE_OWNERSHIP=true`; retain the separate review/editing/creation/deletion gates for each rule operation. Review the exact selected file/table, complete rules and typed descriptions, then explicitly authorize or revoke through shared Save/Discard/Cancel. Publication rotates the editor identity without any database or YAML write. The grant is deliberately session-only: context changes/restart clear it because a persisted path cannot prove physical file identity. Rules, table-qualified reservations and technical history transact in the selected profile file/table; parent/child writers and Env/Veg/Lump remain project/SU scoped. Support-file aliases, stale/replaced ownership, cancellation, changed review and incomplete transport fail explicitly. Focused/full race,251 frontend tests/check0/0 and both builds pass. Native Cancel/Discard, external Save/history rollback/retained retry, one independent parent Save/audit, revocation,600px visible labels and restart/blank create/delete/deleted-ID nonreuse pass. Original fixture/support/configuration and prior completed external checkpoint bytes are restored. This is an explicit desktop ownership adaptation, not measured Access AllowEdits/AddNew parity. |
| Import/export, reporting, maps and administration | Not complete. Unimplemented navigation remains disabled. |
| Project metadata templates | **Implemented and native-Wails-verified, independently opt-in:** `VITE_PROJECT_METADATA_TEMPLATE_CREATION=true` plus the editor flag; the blank-creation flag is not required. Select one physical matching VMetaData template explicitly and review32 labelled ordinary assignments from the source33-column mapping. The existing parent ProjectID is immutable. Complete original master candidates/schema and empty project review guard creation; canonical master uniqueness is retained, while legacy duplicate candidates never imply bulk copy or first-row selection. Non-NULL timestamps require explicit signed16 year/NULL decisions; incompatible numeric codes require literal text/NULL decisions. New text/domain validation applies even to historical source values; nothing is silently trimmed or coerced. Review/Undo allocate nothing; raw errors/remount/close gates and rollback/retry persist. Create reuses reserved signed32 allocation, one complete75-column transaction/audit and full parent observations; all41 unmapped fields/stamps remain NULL. Committed refresh failure prevents replay; ordinary editing follows separately. Project, reference and master databases remain distinct; the master is attached read-only and never changed by creation. |

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
the derived editor read models. Support files remain readonly by default. The
separately opt-in personal-species workflow writes only a VUser definition/audit
transaction; it never commits the plot as a fallback or claims cross-file atomicity.
Original NULL Codetype/default Report values and descriptions are preserved rather
than inferred. Reference, master-metadata and other administrative writes remain unavailable.

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
`VITE_ORDINARY_PARENT_EDITING=false` disables the fourteen surveyor/text/depth/cover/
note editors; they default on. Short text preserves source UTF-16 bounds (20/50/30),
depths use signed 16-bit Access Integer bounds and five summaries use finite Single
physical bounds without rounding, balancing or an invented 0-100 restriction.
Memos preserve multiline raw text without an invented short-text limit.
Unchanged historical invalid values are omitted from UPDATE assignments. New
values and restoration targets are validated; malformed raw JSON Unicode is rejected
before decoder repair. Invalid numeric drafts survive page remounts and block
Save/Lock/close; valid correction, Save and Undo use the existing identity lifecycle.
Vegetation notes Tab (including Shift-Tab) activates Soils, preserving the source
event's ignored Shift modifier. These are source-bound/adapted desktop policies,
not claims of unmeasured effective Access input parity. Native verification on one
disposable fixture compared all 15 tables and preserved historical audits, adding
exactly fourteen field audits plus one explicit memo-to-NULL audit.
`VITE_PARENT_FLAGS_EDITING=false` separately disables SpeciesListComplete and
UpdatedFromCards; they default on. NULL is visibly indeterminate with an explicit
Clear action, rather than silently displayed or persisted as false. This nullable
presentation adapts the source checkboxes without assuming their omitted TripleState
property. Storage and audits retain NULL/0/-1; unchanged historical true representations
are omitted from unrelated UPDATE assignments. Native remount, cross-Env/Admin audit
rollback/retry, NULL/false/true, Undo and Lock preserve all15 tables and32 old audits,
adding exactly six intended flag audits and no phantom option history.
`VITE_SOIL_DRAINAGE_EDITING=false` separately disables SoilDrainage; it defaults on.
Source LimitToList and TEXT5 are preserved through exact full-item membership and
five-UTF-16-unit validation. The 14 reference rows retain the empty Item metadata
distinct from NULL; only 13 nonempty canonical Items are selectable. No silent
recasing, completion or trimming occurs. This input mechanism is a deliberate
desktop adaptation, not measured Access auto-expansion parity. New/changed codes
and restoration targets require a verified reference; NULL and unchanged historical
values remain available without it. Invalid drafts survive remount and block
Save/Lock/close. Native corruption/Retry, rollback/retry and NULL verification
preserved all15 tables and32 historical audits, adding exactly two drainage audits.
`VITE_MASTER_BEC_EDITING=false` separately disables BEC Master editing; it defaults on
only for the source-authorized configured user, `Will MacKenzie` (case-insensitive,
without trimming). This is a workflow policy tied to the existing audit identity,
not an authentication boundary. Source Form_Load unlocks that user; the username
check inside BeforeUpdate is commented out. Desktop Save/Create/Update and audit
restoration enforce the restriction explicitly as a safer adaptation.
The nullable Admin.BECSiteUnit TEXT100 editor preserves raw text, NULL, partner fields,
3504 Master reference rows and duplicate metadata. Newly unmatched/unavailable codes
require separate draft/value-bound acknowledgement, never Working Unit acknowledgement.
Unchanged historical invalid values are omitted from unrelated saves, including
unprivileged saves; restoration targets are validated afresh. Reference-loading
inputs are disabled. Native raw100/NULL, hidden validation/Lock/close refusal,
rollback/retry, unprivileged readonly and direct backend refusal preserve all15 tables,
32 historical audits and restored support/config bytes, adding exactly two Master audits.
`VITE_ADDITIONAL_PARENT_EDITING=false` separately disables XCoord, YCoord and Photo;
they default on and reuse the ordinary scalar editor. Static exports show three
bound, unlocked textboxes with no individual events: nullable Env.XCoord/YCoord
Single values and Env.Photo TEXT50. X/Y are not latitude/longitude or a declared
projection; no geographic limits, automatic conversion or partner updates are
invented. Photo is raw text metadata, not file attachment or picture management.
Physical bounds, UTF-16/Unicode validation, unchanged historical omission,
fresh restoration guards and draft/original-bound numeric errors are shared.
Native two-error remount/Save/Lock/close refusal, raw50/non-rounded Single values,
rollback/retry, NULL, Undo and Lock preserve all15 tables,32 historical audits
and support/config bytes, adding exactly six intended scalar audits.
The picture manager, picture display, projection and bulk/reverse-copy actions
remain unavailable. Parent-field coverage does not enable those workflows.

Other editing defaults on; `VITE_OTHER_EDITING=false` makes its source grid and
create/delete actions read-only. The eight exported SubOtherXL bindings are
DataName TEXT50, DataItem/UserItem1-3 TEXT255 and three nullable BOOLEAN flags.
UsysOther routes the selected project's original table, linked by PlotNumber;
the source form's only active update event audits its ID. The desktop preserves
raw text, UTF-16 bounds, malformed-JSON rejection before repair and true=-1 storage/
audit. Backend physical guards do not invent a zero-length prohibition; clearing
an editor text control explicitly submits NULL. Unbound Other.Flag is untouched.
Unchanged historical invalid text is omitted from assignments; fresh restoration
targets must be physically valid, including accepted table/field recasing aliases.

All eight table cells now stage explicit Save/Cancel drafts instead of immediate
blur saves. This is a documented lifecycle adaptation: errors/raw text survive
tab remounts and block Save/Lock/close and unrelated mutations. Only changed cells
are patched, with exact original NULL/value expectations and plot/ID ownership.
Multirow writes/history share one transaction; stale/duplicate/deleted identities
are refused. Create retains the existing two-field dialog; optional items/flags
can then be entered in the source table. Delete has a native confirmation and
retains identity reservations, so later creates cannot reuse deleted IDs.
Deletion history and reservation safety are desktop adaptations, not new runtime
proof of Access delete auditing or its random identity generator.
Committed writes followed by refresh failure are reported as committed and disable
editing rather than inviting duplicate creation or retry.

The native Other proof checks12 scenarios, all15 original project tables and32
historical audits, exactly16 intended audits and the existing reservation-ledger
contract. Support/config bytes and canonical R/Access/SQLite sources are unchanged.
Default assets exactly match the tested opt-in assets; final delivery also proves
Unicode-folded malformed header keys cannot bypass the reused raw-token guard.
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
Shared verification caching is implemented for Parent, Geology, Soil, Region and
Site catalogues. Mount verifies the full file/profile and publishes an immutable
metadata snapshot; warm lookups clone that snapshot without additional hashes,
SQL queries or profile scans. Temporary readonly SQL handles close before
publication, allowing replacement/removal. File identity/size/modtime changes
invalidate the cache; Windows also checks ChangeTime. Explicit Retry forces
verification, and failures return no choices until successful revalidation.
Metadata is not a cryptographic guarantee: a host preserving every observed
attribute can retain the old verified snapshot until forced Retry, never read
unchecked new rows. Non-Windows invalidation uses identity/size/modtime only.
Three100-call backend runs measured warm54-63us versus forced full verification
3.09-3.15ms, not UI latency. Native corruption/remount/Retry/restoration preserves
invalid drafts and all project/audit/config/support bytes.
Scoped FS882 header/capability/child/audit reads now accept the injected Wails
context and use QueryContext/QueryRowContext. Supersession/unmount explicitly
cancels retained generated read promises; Save/Lock/restoration commits are not
cancelled or retried. Queued operation leases and SQL pool waits observe cancellation.
Native generated-promise cancellation interrupts SQLite, releases the switch lease
and permits an identical subsequent read. beta.26 reports a delayed backend
acknowledgement: only `context canceled` RuntimeErrors belonging to explicitly
cancelled owned read promises are handled; unrelated failures remain visible.
Project browse/hierarchy and all four reference read methods now also receive
injected contexts; coordinator and snapshot lock waits observe cancellation.
Browse/hierarchy generations and species searches cancel superseded requests and
reject stale results. Native generated browse/species promises interrupt SQLite
and recover unchanged. The five shared Parent/Geology/Soil/Region/Site catalogue
services now use injected contexts for queued locks, verification SQL and cloning;
their field/reference consumers cancel supersession/unmount and forced Retry reads.
Native checks preserve metadata, invalid drafts and every fixture database/config
byte. Synchronous file reads are checked before/after, not promised interruptible.
Cancellation acknowledgements alone are handled; corruption errors from cancelled
reads remain visible. BEC/Quality/Working Unit catalogue queries also use injected
contexts, including dynamic project/SU schema and choice queries. BEC retains
successfully cached zones while replacing cancelled pending generations. Working
Unit cancels choices only: its getter initializes a persistent preference fallback,
so initialization and setters are deliberately not auto-cancelled or retried.
Native expensive queries interrupt and recover with identical metadata; invalid
Quality drafts survive remount. State/discovery contexts now reach connection
opening, project descriptions, SU authorization/schema and hierarchy inspection;
cancelled inspection is an error, never a partial successful state or diagnostic.
App refresh generations cancel discovery and reject late publication; context
switch commits remain untracked. Native metadata SQL cancels/recoveries preserve
identical state and fixture bytes. C3's planned active retrieval scope is delivered;
Cancellation itself does not enable unavailable workflows.
See C1-C5 in
[MIGRATION_PLAN.md](MIGRATION_PLAN.md) for bounded performance/cleanup gates,
including incremental packages and optional typed catalogue transport consolidation.

## Migration references

- [Distribution & Releases](.github/workflows/release.yml): Multi-platform CI workflow building artifacts for Windows, macOS, and Linux across AMD64 and ARM64 architectures.
- [Download Portal (GitHub Pages)](docs/index.html): Web portal with automatic OS/architecture detection, download links, and installation guide (successor to legacy vprouser.ca).
- [MIGRATION_PLAN.md](MIGRATION_PLAN.md): one ordered, workflow-based backlog.
- [WINDOWS_HANDOFF.md](WINDOWS_HANDOFF.md): current local state, paths and next gate.
- [Access contract](docs/FS882-6x4XL-access-contract.md): detailed source and measured
  behavior, including limitations and intentional adaptations.
- [AGENTS.md](AGENTS.md): source-first development and bounded verification rules.

Historical continuation logs and build receipts remain in ignored
`evidence/private`; they are evidence, not the current work plan.

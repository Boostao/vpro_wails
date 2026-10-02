# Windows continuation

Updated 2026-10-02. Read [MIGRATION_PLAN.md](MIGRATION_PLAN.md) for priorities and
[README.md](README.md) for delivered capabilities. This file describes current
state only; it is not a chronological execution log.

## Environment

- Repository: `C:\Users\BrunoTremblay\Work\vpro_wails.worktrees\access-parity-windows-handoff`.
- Canonical Access exports: `C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64_forAI`.
- Installed Access cores: `C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64`.
- Wails beta.26, Svelte 5, Vite 8; PowerShell 5, 64-bit Go/CGO and MinGW.
- C: expansion is verified: about 108 GB free. The prior capacity blocker is retired.

Per-shell PATH additions:

```powershell
$env:PATH = 'C:\Users\BrunoTremblay\tools\go\bin;C:\Users\BrunoTremblay\tools\mingw64\bin;C:\Users\BrunoTremblay\tools\node-v22.14.0-win-x64;C:\Users\BrunoTremblay\go\bin;' + $env:PATH
$env:CGO_ENABLED = '1'
```

Use Windows filesystem paths and `npm.cmd`. Environment changes do not persist
between shells.

## Current implementation

- Foundation F1-F3 is **active, native-verified and promoted**:
  `databasefamily.go` embeds sealed original VPro64/VLists/VUser/VMetaData/
  VMessageBoard bytes separately from derived catalogues and installs missing
  files without overwrite. `sqlitecontext.go` owns a pinned SQLite coordinator,
  readonly attachments and source-driven TEMP project/reference/SU/hierarchy
  views. Final full Go race passes (root361.871s). Tests cover all-file byte preservation, metadata distinctions,
  external quoted/Unicode/URI paths, shared-file family identity, cancellation,
  missing/ambiguous/old versions, incomplete schemas, unauthorized SU and close.
  URI helpers escape filenames and resolve writer paths absolutely.
  Active bootstrap/reads/writers now use the owned context. ContextService binds
  all49 editor operations to immutable identities, leases running operations and
  rejects stale calls or legacy unscoped mutation/switch bypasses.
- Current/candidate `bin/vpro-current.exe` and `bin/vpro-current-candidate.exe`
  contain the native-verified complete Other plus Humus12/Mineral18 editor
  and vegetation physical-domain/12-attribute/Collected cycle editing,
  plus default-on species decisions/source cover/height drafts and default-disabled
  guarded personal/source creation/deletion:
  `    b7c09b6db6b31211a8595fad97df7b002868301142bbaa503633a50fbe442d53`.
  Published species60575340... and preparation4eb0b8af... remain sealed in the
  private species archive. Numeric30199c86... remains archived; exact verified
  complete A/C NULL-cover notice/default delivery is archived; the accepted
  default checker-gated delivery is promoted without rebuilding.
  Previous20a6c3e7... and50866a49... are archived with accepted notice bytes and
  receipts. Previous8e63916d...
  and accepted core/default hashes are archived. Previous personal
  defaultbb3652f3... is archived with exact accepted core/default hashes. Previous creation
  defaulta7dd1be8..., deletiond8d51c7f... and accepted cores/guards are archived.
  Soil is default-on; `VITE_SOIL_CHILD_EDITING=false` is the read-only opt-out.
  Source Humus DESC/Mineral ASC depth ordering, physical domains and historical
  omission remain intact. Shared expected-value transactions span rows/tables
  and audits. All30 controls retain raw numeric/text/memo errors across remounts
  and gate Save/Undo/Cancel/Lock/close/context/unrelated writes. Canonical-family
  nullable suggestions retain duplicates; Reload/Retry retains drafts.
  Focused Go1.853s/core race456.021s/final race428.787s pass;173 frontend tests,
  check0/0/default+opt-out builds; bindings15 services/111 methods/40 models.
  Evidence: `evidence/private/native-soil-editor`. Core12 cases/39 intended audits
  plus final default delivery/two independent-client audits verify all fields,
  NULL, stale rejection, rollback, reference failure/Retry, hidden errors,
  create/delete/ID reservation and Lock without phantom history. All15 original
  tables/32 old audits and expected-only fixture changes are checked; schema,
  support/config/external bytes restored. Owned core PID11148/final PID6780
  exited; inspector9392 closed. Completed proofs must not be replayed.
  Accepted core86032440... and domain5d69931d... executables are sealed there.
  Earlier ordering/domain evidence remains preserved in native-soil-order and
  native-soil-domain. Canonical Sample hash remains e63f0c2a....
  Vegetation's36 mapped fields now have physical create/update/restoration
  guards, raw JSON Unicode checks and historical assignment omission, without
  enabling new controls or changing the existing height-grid `<100` rule.
  Source LL/PV are Long, not Integer; all other mapped integer attributes are
  signed16. Focused39.230s/full race498.416s pass; exact existing frontend
  assets/bindings remain embedded. `evidence/private/native-vegetation-domain`
  verifies75 rejected requests and only two intended DC edits/audits, returning
  data to the baseline. All15 original tables/56 old audits/ledger/support/config
  preserved; rollback trigger removed. Owned PID5860 exited/inspector9392 closed.
  Exact no-write visual checkpoint was sealed without replaying completed writes.
  Soil editor96697f18... is archived there. Species/vegetation full drafts and
  event workflows remain incomplete; static metadata/merge code after immediate
  exits must not be inherited as active behavior.
  All12 USysVegOtherXL existing-row numeric attributes are now default-on with
  `VITE_VEGETATION_ATTRIBUTE_EDITING=false` read-only opt-out. Shared patches and
  scoped canonical suggestions retain145 nullable/duplicate definitions; Cultural2
  uses Cultural1, PV orders by Item and AF remains free numeric. Errors/drafts
  persist through shared lifecycle and reference Reload/Retry; species and
  creation/deletion remain unavailable here. Source has no exported attribute
  audit event; desktop transactional audits/raw-code policy are adaptations.
  Focused1.763s/full race504.457s;177 frontend tests/check0/0/opt-in/default/opt-out
  builds; bindings15/114/41. `evidence/private/native-vegetation-attributes`
  proves14 cases/18 audits, all15 original tables/32 old audits and exact-only
  changes; no patch identity ledger allocation. Support/config/external bytes
  restored and rollback trigger removed. Core PID14488/default PID3260 exited;
  inspector9392 closed. Readonly default delivery preserves every fixture byte,
  zero audits. Default assets are byte-identical to core assets and embedded in
  exact promoted eb3a009b...; no later untested rebuild. Core executable and
 115ad838... fallback are sealed there. Never replay completed proofs.
  Collected is default-on across all five cover/height source grids;
  `VITE_VEGETATION_COLLECTED_EDITING=false` disables it. Source NULL/C/V clicks,
  ASCII case/fullwidth database-equivalent variants and unchanged noncycle
  history use explicit expected-value/click-count patches, final-value audits
  and persistent mutually gated drafts. Existing nil-omitting VegRecord transport
  is preserved without losing empty strings. Focused0.651s/full race493.062s;
 180 frontend tests/check0/0/default+opt-out; bindings15/116/42. Native13 cases/
 8 audits preserve15 tables/32 prior audits/schema/support/config/external bytes.
  Stale rejection and second-row audit rollback retain drafts; Retry commits
  once. The no-write initial rejection/harness continuations never replayed
  writes. Core PID13356/default PID15216 exited; inspector9392 closed.
  Readonly default delivery proves five visible labelled controls/all bytes
  unchanged/zero audits. Default assets equal accepted opt-in bytes; exact
 156aeb46... is promoted. Evidence/private/native-vegetation-collected seals
  core/rejected executables, source readonly collation copy and eb3a009b fallback.
  The active source bootstrap shares `config.yml` across selection, coordinates,
  Working Unit and audit/user preferences. Defaults preserve R vocabulary;
  all three JSON sources remain untouched and are hash-recorded/idempotent.
  Unknown/inactive YAML types are preserved; invalid input/conflicts and failed
  writes are explicit. Legacy Go `User` and measured no-SU Master initialization
  remain intact; fresh installs use `Admin`.
  Actual Wails proof/reopen: `evidence/private/native-yaml`; all15 tables checked,
 31 old audits unchanged, exactly3 intended edits/audits. Three real
  Windows-locked setters failed without corrupting YAML/effective audit state,
  retry succeeded and restart did not replay JSON. Owned proof/reopen processes
  exited; debug port9391 is closed. Full Go race passes (root332.140s), all134
  frontend tests/check0/0/build pass; real bindings regenerated. This sealed
  F1 executable is archived under `evidence/private/native-yaml`.
- F2-F3 evidence: `evidence/private/native-context`. Actual Wails proved Cancel,
  rejected Save, locked-config failure after successful old-context Save, retry
  without replay, Discard, hidden-invalid remount rejection, height draft
  Cancel/Save, stale write rejection, separate external SU/hierarchy restoration,
  readonly restart and retained-config missing-path recovery. Recovery disables
  editors; correct the reported YAML/path and restart.
  Managed15/external15/choices17 tables and31 historical audits per project copy
  are preserved; exactly one managed Realm audit and one external Height1 audit
  were added. All five support files and the custom VUser override are unchanged.
  Final default-on readonly delivery verifies visible, unclipped same-name project
  paths and unchanged data/config/support bytes. Owned delivery PID17440 exited;
  inspector9392 is closed. The temporarily unavailable fixture was restored and
  the known zero-byte URI-helper artifact removed with a cleanup receipt.
  All137 frontend tests/check0/0/build pass; explicit external opt-out builds.
  Actual bindings:15 services/99 methods/33 models.
  `Desktop.DatabasePaths` accepts explicit absolute overrides for the five support
  roles. An absent Desktop.SchemaVersion initializes to1 even with an existing
  partial Desktop overlay; explicit NULL/wrong versions still fail.
- C1 plot pooling is implemented, tested and promoted. sqliteContext owns the
  lazy two-connection project pool; every plot read/write releases its borrow
  rather than closing the owner. Active hot reads no longer rediscover projects
  per request. Foreign keys and busy_timeout5000 apply to both connections.
  Full race root357.549s; focused pool tests2.304s pass, including concurrent reuse,
  failed publication, queued switch, closed-owner rejection and atomic audit
  rollback/retry. Three100-read benchmarks: pooled0.38-0.40ms versus retained
  per-operation adapter3.39-3.69ms (backend only).
  Native500 readonly calls/switch/stale rejection/close preserve the existing
  managed/external/choices tables, support/user bytes and final YAML. Handle
  samples442/446/446/446 plateau over the400-read phase; owned PID3052 exited and
  inspector9392 closed. Receipts pool-readonly/pool-plateau/pool-promotion.json in
  native-context. Foundation fallback f1c9ddd... is sealed there as
  vpro-foundation-verified.exe. Frontend/bindings remain unchanged.
- C2 shared catalogue caching is implemented, tested and promoted. The five
  Parent/Geology/Soil/Region/Site services publish verified immutable metadata
  snapshots and close temporary SQL handles. Warm calls clone snapshot rows;
  identity/size/modtime changes and Windows ChangeTime invalidate verification.
  Explicit UI Retry calls ReloadCatalogue; failures return no stale success.
  Tests prove one mount hash/profile scan, zero additional warm hashes/scans/bytes,
  pointer isolation, replacement/removal/restoration, successful concurrent reads/
  reloads before close and same-size/restored-modtime Windows corruption.
  Full race root338.652s,139 frontend tests/check0/0/build; actual bindings
 15 services/104 methods/33 models. Three100-call backend runs: warm54-63us versus
  forced full verification3.09-3.15ms, not UI latency. Native160 warm reads/eight
  forced reloads, corruption/remount/failed Retry/atomic restoration/recovery retain
  invalid drafts and reject Save; all project/audit/config/support/catalogue bytes
  unchanged. Owned PID4528 exited; inspector9392 closed. cache-native/cache-promotion
  receipts and representative failure/restored visuals are in native-context.
  Pool fallback9e293175... is sealed as vpro-pool-verified.exe.
  Metadata cannot guarantee freshness if every observed attribute is preserved:
  warm retains only the old verified snapshot; forced Retry detects corruption.
  Non-Windows uses identity/size/modtime only.
- C3a scoped FS882 retrieval is implemented, tested and promoted. Eight injected
  ContextService read contexts flow through capability/header/child/audit SQL and
  the owned pool. Queued operation leases and pool waits observe cancellation.
  FS882 tracks generated read promises, cancelling supersession/unmount only;
  mutation/Save/Lock/restoration commits are unchanged. Full race root348.272s,
 142 frontend tests/check0/0/build; real binding shapes remain15/104/33.
  Actual generated SDK cancellation interrupts an expensive query on an isolated
  project copy; switch lease release0.328s, subsequent header identical. The
  synthetic schema was removed and exact original bytes restored after close;
  project/audits/config/catalogues/family all match. Evidence/promotion:
  evidence/private/native-read-cancel. Owned PID15024 exited; inspector9392 closed.
  Cache fallback e486a63d... is sealed there as vpro-cache-verified.exe; failed
  acknowledgement candidate/receipt are retained separately.
  beta.26's delayed backend `context canceled` acknowledgement is handled only for
  an owned explicitly cancelled read promise/RuntimeError; all unrelated errors
  remain visible.
- C3b browse/hierarchy/reference is implemented, tested and promoted. Injected
  project/hierarchy contexts reach the pinned coordinator; all four reference
  methods use context-aware SQL and return row iteration errors. Closed reference
  services reject reads explicitly. Coordinator/pool and scoped snapshot lock waits
  now observe cancellation. App browse/hierarchy and FS882 reference/species
  requests own their generations, cancel supersession/unmount and reject stale
  results. No mutation/switch cancellation or implicit retry.
  Focused race3.235s/full race339.024s;143 frontend tests/check0/0/build; real
  binding shapes15/104/33. Native generated browse/species cancellation interrupts
  SQLite; switch release0.375s; subsequent header/browse/species/list metadata
  identical, no unhandled errors. Exact project/reference bytes restored after
  synthetic schema removal/owned close; audits/config/catalogues/supports match.
  browse-native/browse-promotion receipts are in native-read-cancel; owned PID13204
  exited and inspector9392 closed. Scoped fallback e3bfec43... sealed there as
  vpro-scoped-read-verified.exe. Failed probe3076 was explicitly cancelled/closed,
  both project/reference bytes restored; its receipt is retained.
- C3c shared Parent/Geology/Soil/Region/Site reads are implemented, tested and
  promoted. Injected contexts cover queued exclusive locks, verification SQL
  and snapshot cloning. Six field/reference consumers cancel owned lookup and
  Retry promises on refresh/dispose; stale state and mutation behavior stay intact.
  Focused race4.518s/full race346.630s;144 frontend tests/check0/0/build; bindings
  remain15/104/33. Native five generated reload cancellations/recovery,
  supersession/disposal and corrupt/restored Retry retain invalid drafts and all
  database/audit/config/support bytes. Short native scans prove transport ownership,
  not mid-SQL interruption; synchronous file reads only check context before/after.
  Non-cancellation corruption errors remain visible, including cancelled reads.
  Failed overly strict probe assertion/cleanup receipt is retained.
  Catalogue-native/catalogue-promotion receipts are in native-read-cancel;
  owned PID3036 exited and inspector9392 closed. Browse fallback1c6487... is sealed
  there as vpro-browse-read-verified.exe.
- C3d BEC/Quality/Working Unit catalogue reads are implemented, tested and promoted.
  Injected contexts cover queued locks/pool waits and dynamic Working Unit
  project/SU transactions/schema/choices; cancelled iteration returns no partial
  rows. BEC retains fulfilled zones but replaces pending cancelled generations.
  Quality/Working Unit choices cancel supersession/disposal. Working Unit mode
  initialization and setters remain uncancelled because they persist preferences.
  Focused race19.966s/full race335.572s;146 frontend tests/check0/0/build; bindings15/104/33.
  Native expensive BEC/Quality/Master/environment queries interrupt/recover
  identical metadata within3s; invalid16-unit Quality draft survives remount and
  rejected Save. All fixture bytes restored after owned close. Probe correction
  from7 (valid) to16 units did not replay completed SQL phases; receipts retained.
  Remaining-catalogue-native/promotion receipts are in native-read-cancel;
  owned PID14916 exited, inspector9392 closed. Shared-catalogue fallback a2285ce...
  is sealed as vpro-shared-catalogue-read-verified.exe.
- C3e state/discovery is implemented, tested and promoted; C3's planned active
  retrieval scope is complete. Contexts reach readonly open/Ping, descriptions,
  SU policy/schema and hierarchy inspection; cancellation cannot become a partial
  state or diagnostic. App refresh owns generations and blocks stale publication.
  Switch/preference initialization/commit handshakes remain uncancelled.
  Wrapped query cancellations preserve the exact expected SDK message; distinct
  corruption and handle-close failures remain visible.
  Focused race9.785s/full race336.269s;147 frontend tests/check0/0/build; bindings15/104/33.
  Native metadata SQL cancels/recovers0.063s with identical state; visible app
  refresh succeeds. All project/audit/config/support bytes restored after close.
  State-native/state-promotion receipts are in native-read-cancel; owned PID13720
  exited, inspector9392 closed. Catalogue fallback80a008... is sealed there as
  vpro-all-catalogue-read-verified.exe. Parent migration resumes next; optional
  C4/C5 cleanup is nonblocking.
- Verified writable parent coverage: **98/98**, including conditionally authorized
  BEC Master. Full storage mapping and source
  layout are not full FS882 or application parity.
- Fourteen ordinary surveyor/text/depth/cover/note fields are default-on, with
  `VITE_ORDINARY_PARENT_EDITING=false` opt-out. Source UTF-16 bounds, Integer/Single
  physical domains, raw multiline memos and VegNotes Tab reuse shared lifecycle;
  omitted effective input constraints remain unknown. One native fixture checked
  all15 tables, fourteen edits plus one memo NULL audit, historical preservation,
  two-scope invalid remount/Save/Lock/close refusal, Undo, audit rollback/retry and
  Lock/Unlock. Proof/visuals/receipt: evidence/private/native-ordinary-parent.
  Owned PID12120 exited; inspector9392 closed. Discovery fallback8a0833... is archived
  as evidence/private/native-read-cancel/vpro-state-read-verified.exe.
  Full race371.176s,153 frontend tests/check0/0/build; bindings15/104/33.
- SpeciesListComplete and UpdatedFromCards are separately verified/default-on;
  `VITE_PARENT_FLAGS_EDITING=false` opts out. Nullable checkboxes visibly distinguish
  NULL/false/true and offer Clear; storage/audits preserve NULL/0/-1.
  One native fixture verified all15 tables,32 old audits and exactly six new flag
  audits, rollback/retry, remount, Undo and Lock without phantom history.
  Owned PID6160 exited; inspector9392 closed. Default assets exactly match the
  accepted opt-in payload. Full race370.621s,155 frontend tests/check0/0/default+
  optout builds; bindings15/104/33. Proof/visual/failed-dispatch cleanup:
  evidence/private/native-parent-flags. Closed ordinary fallback4d0a86... is sealed
  as evidence/private/native-ordinary-parent/vpro-ordinary-parent-verified.exe.
- SoilDrainage is separately verified/default-on; `VITE_SOIL_DRAINAGE_EDITING=false`
  opts out. Source LimitToList/TEXT5 uses exact canonical membership, not silent
  recasing/completion/trimming. Fourteen metadata rows retain one empty Item;
  thirteen nonempty Items are selectable. Native strict invalid/remount/
  Save-Lock-close refusal, Undo, rollback/retry, corruption/Retry and NULL pass.
  All15 tables/32 old audits/support/config bytes preserved; exactly two new audits.
  Full race373.627s,157 frontend tests/check0/0/default+optout builds;
  bindings15/104/33. Default assets exactly match accepted native assets.
  Owned PID1288 exited; inspector9392 closed. Proof/visual/promotion:
  evidence/private/native-drainage. Closed flag fallbackf4f0d7... is sealed as
  evidence/private/native-parent-flags/vpro-nullable-flags-verified.exe.
- BEC Master is separately verified/default-on only for the configured source
  user Will MacKenzie; `VITE_MASTER_BEC_EDITING=false` opts out. Backend write/
  restoration enforcement is a safer adaptation of source Form_Load unlocking,
  not an active source BeforeUpdate guard or authentication boundary.
  Nullable TEXT100/raw Unicode, separate acknowledgement, unchanged historical
  omission and reference-busy disabled controls preserve ordinary Working Unit.
  Native raw100/NULL, hidden remount/Save-Lock-close refusal, rollback/retry,
  unauthorized readonly/direct scoped backend refusal pass. All15 tables/32 old
  audits/support bytes preserved; two Master audits. Disposable user preference
  roundtrip/config bytes restored exactly. Full race385.848s,159 frontend tests/
  check0/0/default+optout; bindings15/106/33. Default assets match native payload.
  Owned PID2336 exited; inspector9392 closed. Proof/visuals/failed-loading cleanup/
  promotion and sealed drainage fallback6a6085...:
  evidence/private/native-master-bec/vpro-drainage-verified.exe.
- Final XCoord/YCoord/Photo scalar editing is verified/default-on;
  `VITE_ADDITIONAL_PARENT_EDITING=false` opts out. Static exports show unlocked,
  bound textboxes without individual events: nullable Single X/Y and TEXT50 Photo.
  No projection/geographic range/rounding/path handling or picture-manager activation.
  Shared validation, numeric sessions, historical omission and restoration guards
  are reused. Native two-error hidden remount/one-correction/Save-Lock-close refusal,
  raw50/full-precision values, atomic rollback/retry, NULL/Undo and Lock pass.
  All15 tables/32 old audits/support/config preserved; exactly six scalar audits.
  Canonical scientific audit verification continued without replaying the Save.
  Full race473.062s,161 frontend tests/check0/0/default+optout; bindings15/106/33.
  Default assets exactly match accepted native payload. Owned PID17832 exited;
  inspector9392 closed. Proof/visual/promotion and sealed Master fallback3568c8...:
  evidence/private/native-final-scalars/vpro-master-verified.exe.
- Responsive FS882 presentation is complete across all five source pages,
  child tables, toolbar/tabs and collapsible project context. Semantic field
  groups reflow4/3/2/1 columns with40px controls; raw source geometry remains
  evidence, not the rendering contract. Routine guidance/reference definitions
  are collapsed below the fields; safety feedback and required review stay visible.
- Other's8 source-bound fields/create/delete/drafts are native-verified/default-on;
  `VITE_OTHER_EDITING=false` disables its grid/actions. Reused source renderer,
  physical/raw JSON Unicode/historical omission/restoration guards, nullable
  BOOLEAN true=-1, expected-value patches and shared identity/audit transactions.
  Hidden two-error remount/one-correction Save-Lock-close refusal, Undo/NULL,
  multirow rollback/retry, exact UTF-16 values, native delete confirmation and
  reserved deleted IDs pass. All15 original tables/32 old audits preserved,
  exactly16 intended audits; identity ledger is the only expected new table.
  Support/config/canonical sources unchanged. Parent+height+Other lifecycle gates
  are coordinated; committed writes with failed refresh explicitly disable editing.
  Native core PID17384 and final-delivery PID15852 exited; inspector9392 closed.
  Default assets match native opt-in exactly;168 frontend tests/check0/0/default+
  optout; bindings15/108/36. Focused15.970s/initial full race495.442s;
  final-source full race418.460s passes.
  Proof/delivery/visual/default-assets and sealed parent72f240... fallback:
  evidence/private/native-other/vpro-parent-verified.exe. Core baa73e... archive
  retains native-tested payload before the preserved Unicode-fold token guard.
- Soil read-only catalogue: 39 great-group rows plus 62 subgroup rows; 1,010 typed
  metadata cells, including NULL/empty distinctions and duplicate definitions.
  Native browsing, checksum failure and warm Retry are verified.
- Soil editors/physical write guards default on; `VITE_SOIL_CODES_EDITING=false`
  opts out. Nine native scenarios plus actual-close Cancel/failure/retry/save/exit
  passed on one isolated four-plot fixture: all 15 tables compared, original 31
  audits preserved and exactly eight intended Soil audits added.
- Prior parent-code fallback is archived as
  `evidence/private/native-context/vpro-pre-foundation.exe`, SHA256
  `636c6817c2a5a1278e5b60360781a7f61f05c5000d4583215249bbf3a8263a92`.
  Shared Region/Site/Soil lookup, Unicode and draft-acknowledgement helpers are
  integrated; Region/Soil physical write/restore guards share an implementation.
  Exposure membership and historical malformed-text policies remain distinct.
  Parent-code proof/visuals/promotion receipt are in
  `evidence/private/native-parent-codes`; the closed responsive baseline is
  archived there as `vpro-current-prior.exe` with its595e3293... seal.
  Responsive-only evidence remains in `evidence/private/native-responsive-final`.
- The 21 additional ordinary parent-code editors default on with
  `VITE_PARENT_CODES_EDITING=false` opt-out. Realm is on Site; 20 fields are on
  Soil/Terrain. Per-list loading/failure/Retry, field-specific lengths, explicit
  full Items, raw casing, NULL and scope/draft-bound review reuse shared helpers.
  SoilDrainage is independently verified under its strict membership workflow.
  Frozen catalogue:16 lists/347 rows/3,470 cells; the UI now owns all16 lists.
- BedrockGeology1/2/3 default on with `VITE_GEOLOGY_CODES_EDITING=false` opt-out.
  Its87-row/870-cell frozen DAO catalogue and generic guards are integrated.
  Nine actual native cases checked15 tables, preserved31 old audits and added
  exactly7 intended audits. Historical source placement is retained in the
  layout resource; current controls render in the responsive Geology group.
  Proof/visuals/promotion receipt: `evidence/private/native-geology`.
  Default assets match the native-tested opt-in payload; opt-out also builds.
  The previous shared-editor baseline is archived under
  `evidence/private/native-code-regression/vpro-current-verified.exe`.
- Cleanup corrected the all-fields test's Soil update samples and the foreign
  audit-plot fixture without weakening foreign-key enforcement.
- Focused Parent catalogue/write/restore tests pass (root50.045s);
  latest `go test -race ./...` passes (root357.549s). All137 frontend tests,
  Svelte check (0 errors/warnings) and production build
  pass. The existing large-chunk build warning remains. Native CDP tooling uses
  isolated `tools\vpro-native-test-tools` websocket-client 1.9.2, not an app dependency.
- Final native proof confirmed actual1400x900 startup, maximized and
  1400/1024/760/560 window layouts (13 checks), readable labelled controls without
  page-wide horizontal overflow, resize/remount draft and review preservation,
  hidden Save/Lock refusal, Undo, atomic failed save and real WM_CLOSE
  Cancel/failed-save/retry/exit. All15 tables checked,31 old audits preserved,
  exactly4 intended new audits. Candidate/embedded assets and closed source/
  catalogue seals were verified before same-volume promotion.
- Current native parent proof passed ten cases and three responsive layout checks.
  All21 fields were saved through real controls; exact maximum bounds, raw apostrophe,
  trailing spaces, a50-unit astral humus phase and full listed selection persisted.
  NULL, hidden Save/Lock/WM_CLOSE refusal, Undo/Lock, disposable catalogue checksum
  failure with independent warm Retry, atomic failure and real close Cancel/
  failed-save/retry/exit passed. All15 tables checked,31 original audits unchanged,
  exactly24 intended audits. A clean read-only reopen captured1400/560 visuals,
  observed disabled SoilDrainage and exited without table changes. Default frontend
  assets match the native-tested opt-in bytes exactly; opt-out also builds.

## Soil evidence and remaining boundary

Reuse the completed evidence under `evidence/private/soil-classification-native`:

| Evidence | Established fact |
| --- | --- |
| `ordinary-adapted-6-measured-contract.json` | Effective LimitToList=false/AutoExpand=true, independent TEXT4 fields and frozen catalogue agreement. Its two misfocused selection writes are excluded from guarded-write parity. |
| `ordinary-adapted-7-measured-contract.json` | Physical apostrophe typing/Undo, guarded raw1234/Z9 and NULL saves;23 rejected guards invoke no Save callback. |
| `ordinary-adapted-8-measured-contract.json` | Four UTF-16 units `[65,233,20013,66]` persist. Case-only Text `ca` against `CA` leaves DAO/final control `CA`; the exact responsible event is not proven. |
| `ordinary-adapted-9-harness-manifest.json` | Remaining-only offline harness preparation passed; no native grant or launch occurred. Do not run it automatically under the old field-by-field plan. |

Every observed source Save also emitted three phantom NULL-to-NULL checkbox
audits; do not port them. Source astral/overlength/Lock-child/save-failure observations are unmeasured.
Desktop behavior is tested, not asserted as exact source parity: raw casing,
explicit completion, no truncation, transactional history and no phantom audits.
Exports show no Soil-specific events; shared `Form_BeforeUpdate`/`AuditTrail` and
`optLockData_AfterUpdate` save-before-lock behavior were reused.

Original full installed-reference baseline is unpassed because MSComctl2.2 is
missing outside the traced closure. Explicit owned-only exclusion allowed a
startup-suppressed FS882 readonly dependency boundary; source DAO5 metadata and
returned DAO12 objects are not interchangeable proof.

Adap8 restoration/owner exit is sealed, but its normal final Option-pair artifact
and explicit native helper-removal proof are absent. Do not backfill them.
Original five restored byte seals prove no persisted modifications, not every cleanup step.

## Ownership and retained artifacts

Go/capacity/frontend/source agents are idle with leases released. Remaining-parent
grouping/reference capture is complete; no Access UI probe is active.
Owned Wails proof processes, including parent16632 and clean-visual11336, exited;
inspector9383 is closed.
A promoted-app read-only visual check (PID15860, now closed) captured the complete
source controls in `native-geology/bedrock-source-layout-native.png`; all15 project
tables and38 audits stayed unchanged.
User-owned Access PID15912 remains present, with
creation 2026-09-30T01:16:15Z and Office16 MSACCESS path; do not interact with it.
The earlier cleanup claim that protected PIDs were absent was incorrect.
Always inspect current PID/start/path ownership before future native work.

- Keep `bin/vpro-region-code-current.exe`, `bin/vpro-site-code-current.exe` and the
  read-only `bin/soil-code-reference-proof` candidate for current comparisons.
- 36 closed obsolete top-level binaries were byte-verified and moved to
  `evidence/private/maintenance-20261001/archived-builds`; `manifest.json` maps
  original paths to archived paths/hashes. Historical receipt paths may need that
  mapping; absence from `bin` is not missing evidence.
- Original README/handoff/AGENTS/session memory are retained byte-for-byte in
  `evidence/private/maintenance-20261001/continuation-history.zip`.
- Existing source databases, observations, visuals and historical failed proofs
  were not deleted. No further disk-recovery work is scheduled.
- Bedrock source preflight and87-row/870-cell DAO fixture are retained under
  `bedrock-source-preflight` and `bedrock-codes-dao`; they are not editor parity.

## Resume gate

The last promoted baseline is green; Soil/shared mechanics, Bedrock, responsive
presentation and the 21 ordinary parent-code editors are complete. Svelte/Tailwind CSS Grid preserves source relationships
and labels, using the Shiny module as a presentation precedent; no second UI
framework was installed.
ParentCodeService's21 physical save/raw-JSON/restore guards and frontend workflow
are native-verified and default-on (adapted desktop behavior, not unmeasured Access
input parity). Current bindings are15 services/104 methods/33 models.
Architecture clarification is delivered: F1 YAML/legacy migration and F2-F3
SQLite database-family/external-path context/native safe switching pass.
Readonly inspection of R's family/config/startup/context and matching Access
exports is complete. Sample bytes equal R's seed; `_table_metadata` remains
general table Description storage. Go's vlists.db is a derived Species/Lists
store, not the canonical support family. Preserve VPro64/VLists/VUser/VMetaData/
VMessageBoard and per-project files; do not replace them with editor fixtures.
The chosen active context is SQLite with owned/pinned TEMP views; optional DuckDB
must not be an offline dependency. Verification used disposable data/config only;
no R/Access/production data writes or new Access instance. Foundation promotion
receipt and sealed fallback are under `evidence/private/native-context`.
Peer-review follow-up is tracked in MIGRATION_PLAN.md C1-C5: C1 session-owned plot
pool and coordinated ownership and C2 shared catalogue verification cache are delivered;
cancellation through active bindings/SQL/frontend read promises alongside affected
paths. Package extraction and optional typed catalogue transport follow incrementally
between milestones, not as a broad rewrite. C3a scoped, C3b browse/reference and
C3c shared-catalogue, C3d BEC/Quality/Working Unit and C3e state/discovery reads are
delivered. C1-C3 are complete; C4-C5 are optional/nonblocking. The promoted executable
contains verified F1-F3 plus C1 pooling/C2 caching/C3a-C3e retrieval cancellation.
The fourteen ordinary surveyor/depth/cover/note fields are now native-verified.
The two nullable flags, strict SoilDrainage, source-authorized BEC Master and
final X/Y/Photo scalar batch complete the98-field writable parent baseline.
Continue actual FS882 child workflows, not another parent-field inventory.
Other8/Humus12/Mineral18, all12 existing-row vegetation attributes and the shared
five-grid Collected cycle are verified/promoted; continue distinct species/cover/height creation/
deletion and calculation events.
Species preparation was published as f286d74; explicit decisions are now
native-verified and promoted. Cancellable
canonical five-field UNION/alias readers return exact547/4742/3319 source classes,
nullable metadata and ambiguous aliases; reference-preparation full race passed
(root502.078s). Existing-row species drafts/decisions now default on with
`VITE_VEGETATION_SPECIES_EDITING=false` read-only opt-out. All five grids share persistent raw errors,
Save/Undo/Cancel/Lock/close/context gates and atomic expected-value patches.
Canonical membership uses the leased readonly family connection, not the
project-only writer pool; source-row predicates are checked in the transaction.
Focused Go1.525s;183 frontend tests/check0/0/default+opt-in builds; bindings15/
120/46 and35 scoped adapter operations. Native8 cases/3 intended audits preserve
15 tables/32 prior audits and support/config bytes. The rejected no-write writer
alias candidate5526fb0f... was closed/sealed; corrected1b7e408e... is archived
with opt-in assets under evidence/private/native-vegetation-species. Corrected
PID11648 and rejected PID4880 exited; inspector9392 closed. No writes replayed.
Default source-grid species controls cannot fall through to unrestricted edits.
Prior156aeb46... and preparation4eb0b8af... remain archived; preparation full race
passed (root472.744s). Explicit replace/keep/existing-personal decisions preserve
alias precedence, nullable definitions and original expectations. Review owns
cancellable guarded reads; decisions survive remount/Reload/failed Save.
Source UCase is bounded to ASCII; literal non-ASCII list selection stays allowed.
Save independently revalidates identities/source values; unknown personal creation
stays disabled. Focused Go1.887s,184 frontend tests/check0/0 and all builds pass;
bindings15/121/46 and36 scoped operations. Candidate60575340... native9 cases/
5 audits preserve15 tables/35 prior audits and support/config bytes. Default
delivery verifies five enabled labelled controls, no implicit draft, all fixture
bytes unchanged/zero new audits. Default assets exactly equal accepted opt-in
assets; opt-out is archived. Core PID14436/default PID14640 exited; inspector9392
closed. Final race passed in species-decisions-integration (root517.356s).
Exact60575340... is promoted without rebuilding accepted assets; application-only
publication follows the ordinary commit/push path. Unknown-code
personal creation needs a bounded VUser/project write contract. Remaining
cover/height creation/deletion/calculations follow. Never replay completed
proof.py, delivery.py, decisions.py or decisions-delivery.py.
Species decisions were published as e251bf3 with a clean0/0 upstream state.
Current numeric workflow: eleven cover/total fields plus six heights share drafts
across all25 source controls. Original forms/values gate the shared transaction;
NULL view removal is not row deletion. Default-on; VITE_VEGETATION_NUMBER_EDITING
or retained VITE_HEIGHT_EDITING set to false provides readonly numeric controls.
Species/Collected remain available. The experimental grid is readonly and
generic vegetation creation/deletion controls are disabled, not generic bypasses.
Focused Go4.194s/185 frontend tests/check0/0/default+opt-out builds pass; bindings
15/123/47 and37 scoped operations. Native core6 cases/4 audits, default25 controls/
1 audit and opt-out25 disabled controls/zero writes preserve15 tables/40 prior
audits plus support/config bytes apart from intended changes. Core2f8d74bd...,
default30199c86... and opt-out assets/binaries remain sealed under the same
private species fixture. Numeric delivery remains sealed; completed numeric proofs
must never be replayed. Core PID15460/default14804/opt-out8040 exited.
Guarded deletion is implemented/native-Wails-verified but remains default-disabled;
VITE_VEGETATION_DELETE_EDITING=true enables the reviewed explicit-confirmation UI.
Full physical raw-byte snapshots/source membership gate the shared deletion,
reservation and mapped/hidden-field audit transaction. Review/error persist through
remount, ordinary Save/Lock/Save-and-close guards and failed deletion; Undo/Cancel
write nothing. Native coref2b6c939... verifies eight cases/sealing checks, one
intended row (-10) deletion, five audits and reserved ID. All15 original tables/
45 prior audits and support/config are preserved except intended changes and the
standard identity-ledger materialization; temporary trigger/schema restored.
The initial schema assertion lacked the expected ledger; sealing replayed no writes.
Defaultd8d51c7f... verifies25 enabled numeric controls/Species/Collected, disabled
source deletion, readonly review and every fixture byte unchanged. Focused Go0.933s,
187 frontend tests/check0/0/opt-in+default builds; bindings15/126/49,39 scoped calls;
full Go race488.272s passes. Deletion default is archived; current/candidate now
contain the exact accepted creation-default delivery below.
Core PID9676/default1244 exited; inspector9392 closed. No owned Wails app remains.
Access property capture is blocked: installed-core disposable copy reached an
unacknowledged relocation dialog despite AutomationSecurity=3. No form opened;
only verified owned Access PID6008 was stopped. Source bytes and original registry
Location are unchanged; owned windows are gone. Effective omitted deletion/addition
properties remain unknown. Do not reopen/repeat this broken source probe.
Evidence is under native-vegetation-species, including stopped source fixture.
Never rerun completed deletion.py/deletion-delivery.py or numeric/species proofs.
Source creation is implemented/native-Wails-verified, default-disabled with
VITE_VEGETATION_CREATE_EDITING=true opt-in. Explicit canonical species/numeric/
NULL values use source order, shared parsing/lifecycle and identity/audit writer;
no guessed Layer/zeros/totals or caller ID. C/C-height require explicit Cover6;
A-height permits height-only rows. Independent stored source/species/numeric
observations precede audit/commit; trigger drift rolls back all writes/reservations.
Final focused Go1.261s/191 frontend tests/check0/0/opt-in+default builds pass;
bindings15/128/50,40 scoped calls; full final race478.504s passes.
Core497c6455... creates2/3/4 with seven audits; independent guard694e7ab2...
rejects species/numeric drift then creates5 with two audits. Original15 tables/
50 prior audits/positive ID1/support/config/schema survive apart from intended
rows/audits/reservations. Typed zero/REAL and tuple/list harness corrections never
replayed writes. Core25-control summary exists, detailed array was not retained.
Defaulta7dd1be8... independently verifies25 enabled numeric controls plus
Species/Collected, readonly review, disabled creation/deletion, no implicit drafts
and every fixture byte unchanged. Creation default is archived; current/candidate
contain the accepted personal-default delivery below.
Core11272/guard6388/default12044 exited; inspector9392 closed.
Accepted binaries are sealed under native-vegetation-species; never rerun
creation.py/creation-guard.py/creation-delivery.py or their completed continuations.
Personal source NotInList assigns UCase(NewData)
to sysNewSpp and opens USysAddSpp, whose exported Code/ScientificName/EnglishName/
LifeForm bindings and active events are identified. VUser definition Save and
project Save are separate; do not infer cross-file atomicity or change
journal modes. Original VUser table exports are now read: Code TEXT8 unique,
ScientificName/EnglishName TEXT255 nullable, LifeForm signed16, Report default1,
SppNumber/Codetype NULL defaults. USysAddSpp has12 lifeform choices, Open sets
sysNewSpp then clears it, Close invokes DoCmd.Close and Update is inactive.
Current personal workflow: personalspecies.go/personalspecies_test.go and
sqlitecontext.go attachment file identities. Scoped single-VUser
definition/CreateRecord audit transaction preserves original tables/defaults/
metadata; VLists is readonly, shared-role user writes reject, and project Save is
separate. Existing-row/new-row unknown-code metadata panel is opt-in only with
VITE_PERSONAL_SPECIES_EDITING=true. Explicit NULL/name drafts retain raw errors
through remounts and block plot Save/Lock/close. User Save reloads independent
metadata then stages a separate existing-personal decision; plot Undo retains
deliberately saved definitions. Committed cleanup/refresh errors disable replay.
Source creation now reuses existing species decision/reference validators for
explicit old-code replace/keep and existing-personal decisions. Reviewed original
form/raw entry is persistent; retyping Species invalidates review/decision, while
numeric edits/remounts/failed Save retain it. New-row Save never writes VUser.
Nullable-Codetype personal definitions remain reusable without invented dropdown
membership. New-row personal metadata uses a unique proposal editor key, not a
physical row ID. Original form/raw context is matched before/after separate
VUser Save; proposal mutation/cancellation waits for metadata Save/Cancel. First
Undo retains the proposal; second cancels it. Explicitly saved definitions survive
proposal Undo and later project-audit failures. Existing-row identity stays intact.
Focused coupled Go2.511s/full integration race488.508s;200 frontend tests/check0/0/
opt-in+default builds pass; bindings15/129/51,41 scoped operations.
Coreccb31517... verifies eight native cases, ZPNROW01/one VUser audit and separate
ID9/two project audits. Raw256/remount/Save/Lock/actual-close/two-stage Undo,
user/project audit rollback/retry and existing-row regression pass. All15 project
tables/67 prior audits, VUser tables/three old audits, old rows/reservations,
support/config and schema preserved. Current fixture has77 project audits/five user audits.
ZPNAT01 remains row0; ZPNAT02 is ID8; ZPNROW01 is ID9. ID8 now retains its physical
record/species/Height6 with Cover6=NULL and is absent from both C source views.
Triggers removed.
Default50866a49... proves25 enabled numeric controls/Species/Collected, readonly
saved-personal lookup, all unfinished creation controls disabled and every fixture
byte unchanged. This previous default is archived; archives/
creation-personal-checkpoint seals core/default/previous8e63916d with hashes.
Older personal/creation-decision proofs and archives remain unchanged.
Core8104/default14528 exited. Previous20a6c3e7... verifies the nonblocking C/C-height
NULL-cover notice and focus styling. Focused Go0.752s/full race459.824s,202 frontend
tests/check0/0/default build pass. Native seven-case proof saves only ID8 Cover6=NULL
and one project audit, retaining its physical record/species/height; malformed input,
remount/Undo, audit rollback/retained retry and successful feedback pass.
Independent readonly delivery reopens identical candidate bytes, verifies25 labelled
numeric controls/Species/Collected, disabled unfinished workflows and every fixture
byte unchanged. Owned9884/12208 exited; inspector9392 closed. Exact candidate bytes
are archived; archives/cover-notices-checkpoint seals accepted default,
previous50866a49 builds and receipts with hashes.
Archivedcddefe55... adds shared A/A-height all-seven-cover warnings to numeric,
Species, Collected and opt-in new-row workflows. Zero is not NULL; non-NULL heights
never infer cover. Original Collected source is retained through remount/alternate
view clicks and omitted from its unchanged transport. Successful Species/Collected/
creation notices use independently reloaded rows. No automatic record/focus
navigation or machine-wide Enter preference is inherited.
Focused coupled Go3.833s/full race460.449s;207 frontend tests/check0/0/core+default
builds pass. Corea13aca2c... passes nine native cases: height-only notice, invalid/
remount/Undo, audit rollback/retry, Species, Collected and separately cancelled/saved
creation. ID7 becomes ABIE_RK/Height1=-2.125/Collected=C with all seven covers NULL;
ID10 is ABIE_RK/Height1=-3.125 with no inferred cover/Layer. Exactly five project
audits and zero user writes; old records/history/reservations/schema/support/config
preserved. Defaultcddefe55... independently verifies25 enabled labelled numeric
controls/Species/Collected, disabled unfinished workflows and every fixture byte
unchanged. Owned7548/7260 exited;9392 closed. Previous default is archived;
archives/a-cover-notices-checkpoint seals core/default/previous20a6c3e7 and receipts
with hashes.
Currentde15b9c3... preserves those warnings and adds the verified, opt-in
named-scope parent checker. `VITE_SPECIES_CODE_CHECK_EDITING=true` enables it;
personal entry additionally requires `VITE_PERSONAL_SPECIES_EDITING=true`.
Readonly physical review names project/selected SU and preserves duplicate
master/user definitions and NULL/empty metadata. Explicit replacements span
plots atomically using the shared child/audit transaction and final stored-value
observations after all triggers. Ignore decisions are runtime-only; no LifeForm999
cleanup or hidden project-wide expansion is inherited.
Coreffc0b3931253596adcc118283311e4518772d505dd5f254789bc165e4c3535ce passes
eight native cases: named scope/three definitions, pagination/raw errors/remount,
actual-close/context/Save/Lock guards, Ignore All, cross-plot rollback/retry,
independent personal Save and Undo/reload. Only fixture IDs-200/-201 become
ACAROSPO, adding two project audits. Separately ZCKNEW02 saves NULL science/lifeform
and empty English with original defaults, adding one user audit. ID-202 stays
unchanged; the prepared ZCKSEN99 LifeForm999 definition is retained. Preparation
added29 explicitly named test rows, a sentinel and one duplicate target definition
after sealing the preceding accepted75/4 boundary; it never changed source files.
All old data/history/schema/reservations/other support/config bytes are preserved.
Go also verifies selected-SU exclusion, NULL originals, stale/cancelled requests,
malformed storage/transport and later/audit-trigger drift.
Focused Go6.230s/full race478.834s,213 frontend tests/check0/0/core+default builds
pass. Bindings15/132/55;44 scoped operations. Independent defaultde15b9c3...
verifies35 labelled numeric/Species/Collected controls, disabled opt-in workflows
and every fixture byte unchanged. Owned5028/18364 exited;9392 closed. No active
native/race process remains. Archives/species-code-check-checkpoint seals exact
core/default/previouscddefe55 builds and receipts; three readonly harness
continuations retain byte-preservation evidence, and no completed writes replayed.
Publication includes application source only; private evidence/binaries/dist are
ignored. Use git HEAD/upstream for the current commit; predecessore71c5da.
Never replay code-check.py, a-cover-notices.py, cover-notices.py or creation-personal.py completed modes.
Next: deliberate navigation and separately scoped parent metadata/profiling.
The plan records the metadata prerequisite: project75-column storage, master
templates42 columns and legacy VUser metadata15 columns are distinct; dates/code
types and the source wrong-recordset/ambiguous-copy logic must not be guessed.
Metadata readonly checkpoint fbb1991 is published, full race551.592s; previous
protected SHA256
a8536ad5e213913b6d1299e5689c76d4175feca07efba829149bf80e6c13d61d.
Archives/metadata-review-checkpoint retains previousde15b9c3/accepted readonly bytes,
source contract and unset/populated native/restoration receipts. Do not replay.
Existing-record writer and opt-in desktop editor are now native-verified.
Unpromoted bin/vpro-project-metadata-edit.exe SHA256
fdbf3c15b3d0abf002aceaea94be712a995dbbb1edb421ca95b1278b5c0f239a.
Focused metadata/context3.449s/full race564.604s,213 frontend tests/check0/0/build pass;
bindings15/134/62,46 scoped operations. Source SQL fixture tests all70 ordinary
columns; nine collection groups use1/2/3, not BOOLEAN. Eight strict quality combos
bind registered Note from PlotQualitySite; Site remains free. Years stay signed16,
counts/BAPID signed32 and text/memo preserve source UTF-16 bounds and literals.
Writer checks full originals/schema/physical ID/ProjectID, selected parent scope,
file ownership and final complete stored row after all updates/audits. It preserves
unchanged invalid values, stamps actual reference table-object descriptions/date,
rejects unsupported metadata restore aliases and requires explicit keep-current-drafts
or confirmed source population for recognized standards. Runtime Lock is not a stored column.
Native PID13564 verifies all70 assignments/73 exact audits, eight rejected requests,
second-audit rollback/retained retry, duplicate-candidate preservation and stable
no-op Save; metadata action stays disabled. Native window/inspector closed.
The one fixture is restored to exact preceding77 project/5 user audit bytes.
metadata-edit.py and metadata-edit-{preparation,progress,native,restored}.json plus
metadata-edit-project-before.db seal this proof; never replay completed modes.
Full race metadata-edit-race passed. No owned native process remains.
Archives/metadata-edit-checkpoint seals the accepted unpromoted writer, protected
readonly baseline, exact assets, source fixture, native receipts/backup and race log.
Existing-record UI is opt-in with `VITE_PROJECT_METADATA_EDITING=true`; default
metadata action remains disabled. `resources/project-metadata-layout.json` retains
246 source nodes/74 bindings; six responsive groups provide70 labelled controls.
Eighteen canonical references and ten history combos preserve source ownership,
literal bound Items/Notes, duplicate/NULL/empty definitions and free history entry.
Resident nullable text/numeric/collection drafts survive tab remounts; ordinary
Save/Lock/native-close/context switching cannot bypass review ownership. Failed
reference reload disables stale editing/selection/Save, and Undo waits for reads.
Committed refresh/cleanup errors clear proposals and block completed-write replay.
Focused Go4.352s,221 frontend tests/check0 errors/0 warnings and opt-in/default
production builds pass; bindings15/135/64 and47 scoped operations.
Native UI core bin/vpro-project-metadata-ui-core.exe SHA256
4a727dee4f78763d453b043d0745b7344e63bc00a5b8925bfc5a2b428727a6a6
proves all70 live fields/73 audits, rollback/retry, NULL/empty, numeric errors and
actual close/context/Save/Lock gates; committed-refresh recovery adds only2 audits.
Final opt-in bin/vpro-project-metadata-ui-ready.exe SHA256
7088f1702040436fe0c2557f84ad9f78c0383982cc3a2ce1ed716913647dd0ec
proves70 visible/labelled controls and responsive native stacking, failed-reference
readiness, UTF-16 overlength/remount/Undo and recovered Save with only4 audits.
Native PIDs6208/17656 close, exact original77 project/5 user audit fixture bytes restore.
Default bin/vpro-project-metadata-ui-default.exe SHA256
04c4291f7bbe37e9d7e9947dd7186cb606fc2fa8fd37c0d09ea16b189a65f250
verifies default-disabled metadata action and existing Site/Vegetation without writes.
Owned default PID12136 closes; inspector9392 is closed. The sole fixture is restored.
metadata-ui.py, metadata-ui-ready.py and metadata-ui-default.py receipts seal the
proofs; never replay completed modes. Initial UI wait corrections were read-only
continuations, not repeated writes.
Full race metadata-ui-race passes (root519.188s) against final default assets.
No owned native/race process remains. Archives/metadata-ui-checkpoint seals81
hashed files: accepted core/final/default binaries, exact final assets, source,
proof/restoration receipts/backups and preceding protecteda8536ad5....
UI checkpoint default04c4291f... was promoted by exact copy; the current successor
is the accepted source-standard delivery below.
Confirmed source-standard population is also implemented in the opt-in editor:
DEIF/DTE/LMH25 changes require explicit keep-current-drafts or preview/Apply of24
literal source defaults. Preview/cancel write nothing; Apply stages only those
fields, preserving unrelated errors/drafts and later manual edits. Standard
retyping resets acknowledgement. The backend never invents default assignments.
resources/project-metadata-standard.json matches the retained source procedure
fixture; source literal spelling is preserved.
Focused metadata Go5.405s/source-fixture1.322s,222 frontend tests/check0 errors/
0 warnings and both builds pass; bindings/scoped transport unchanged15/135/64/47.
Opt-in bin/vpro-project-metadata-standard-core.exe SHA256
f5b80c3ef7ef6c852759d493e694b43dc844ebaef22e621c3cfb4448fa7b3727
verifies24 source literals, keep/cancel, preserved errors/remount/Undo and
second-audit rollback/retained retry followed by25 explicit fields/3 stamps/
28 exact audits. Owned PID6608 closes and original77 project/5 user audit bytes
restore. Default bin/vpro-project-metadata-standard-default.exe SHA256
18b5d6931e4ef45cf33b1079adc2d94fb72d9dcde7f259587aecaa51a5815e45
verifies disabled metadata action and existing Site/Vegetation without writes;
owned PID2040 closes. Inspector9392 is closed. metadata-standard.py and
metadata-standard-default.py receipts are sealed; never replay completed modes.
Full race metadata-standard-race passes (root576.719s) against final default assets.
The initial root suite passed511.250s but overall discovery tried compiling partial
closed archive sources. Ignored archives/go.mod now isolates that subtree; new
source snapshots use source.zip. The subsequent full suite passes without exclusions.
Archives/metadata-standard-checkpoint seals50 hashed files: exact core/default
assets/binaries, native/restoration receipts, source.zip, final/initial test logs
and preceding protected04c4291f.... Standard delivery18b5d693... is now the
archived predecessor of the accepted blank-creation delivery below.

Blank creation is native-verified and independently opt-in:
`VITE_PROJECT_METADATA_CREATION=true` plus the existing editor flag.
Source insertion preserves all73 nonidentity fields/stamps as NULL; it is not an
ordinary form Save. A complete empty75-column review and existing literal parent
ProjectID within20 UTF-16 units are mandatory. Parent identity is never assigned.
Shared signed32 allocation reserves stored/deleted/audited metadata IDs. One
complete typed CreateRecord audit, reservation and row insertion share a transaction;
final row/schema/candidates, audit, ownership/context and complete raw Env/Admin
parent storage observations reject hidden or audit-trigger drift.
Resident proposals/remount/native-close/Undo and failed retry are guarded; committed
refresh failures cannot replay writes. Ordinary title editing/stamping is separate.
Bindings15/136/65 and48 scoped operations; final focused9.525s,223 frontend tests,
check0 errors/0 warnings, both builds and complete race543.668s pass.
Initial coreb7ac1bbd.../owned16864 and final guarded6c2742a0.../owned10052 each
verify reserved1/2/3 -> ID4, one creation audit, separate title/four audits,
rollback/retry and refresh recovery. The guarded core rejects an audit-trigger
OfficeNotes change. Both exact77 project/5 user audit fixtures are restored.
Final default bin/vpro-project-metadata-blank-default.exe SHA256
c22c00fe7bd82914216e2c031e7f2a4e0ec1ec625479a3a54c5b2cf0026f0da4
verifies default-disabled metadata action/creation and existing Site/Vegetation
with all fixture/support/config bytes unchanged. Owned4984 closes;9392 is closed.
metadata-blank.py, metadata-blank-guard.py and metadata-blank-default.py completed
proofs/receipts are sealed; never replay prepare/run/restore modes.
Archives/metadata-blank-checkpoint seals59 hashed files plus its manifest, preceding
protected18b5d693..., accepted core/guard/default binaries/assets, source.zip,
backups and focused/frontend/race/native/restoration receipts. Accepted
blank deliveryc22c00fe... is archived as the predecessor of the template delivery.

Explicit master-template creation is native-verified and separately opt-in:
`VITE_PROJECT_METADATA_TEMPLATE_CREATION=true` plus the editor flag, independently
of blank creation. The shared JSON/VBA fixture retains the exact33 source columns;
one explicitly selected physical master row supplies32 editable ordinary proposals.
Complete original master candidates/schema and an empty project review guard Create.
Non-NULL timestamps and incompatible numeric codes require explicit year/text/NULL
decisions; new bounds/domains reject historical invalid values. No identity repair,
first-row selection, duplicate bulk copy or wrong-recordset branch is inherited.
Canonical master uniqueness remains unchanged; legacy duplicates are fixture-tested.
The existing reserved allocation/full audit/raw-parent transaction is reused, with
the master attached read-only. All41 unmapped fields/stamps remain NULL. Proposals
retain remount/close/Undo/error/retry; committed refresh cannot replay creation.
Focused8.883s,224 frontend tests/check0 errors/0 warnings, both builds and complete
race560.168s pass; bindings15/137/66 and49 scoped operations.
Core bin/vpro-project-metadata-template-core.exe SHA256
b36e97bd7ad29ddf69cc12cf7c06ff5a854854ee34951001a9aade0ed4b896ea
verifies32 visible labels/nine explicit conversions, parent-drift rollback/retained
retry, reservedID4, one independently matched75-cell audit and committed-refresh
recovery without replay. Owned5444 closes; exact project/master fixture bytes restore.
Default bin/vpro-project-metadata-template-default.exe SHA256
3db65721cf3775429af712370ac51f93dbd9efc942646186ab884a55f4a8c0f4
verifies disabled metadata workflows/existing Site/Vegetation with all bytes unchanged;
owned6668 closes. Inspector9392 is closed. No native/race process remains.
Archives/metadata-template-checkpoint seals53 hashed files plus manifest, preceding
c22c00fe..., exact core/default assets/binaries, source.zip, accepted proof/restoration
receipts/backups and logs. Accepted3db65721... is now the sealed Enter predecessor.
Never replay metadata-template.py prepare/run/restore, metadata-template-default.py
or the completed seal-metadata-template.py.
Bounded Enter navigation is native-verified, independently opt-in:
VITE_SOURCE_ENTER_NAVIGATION=true. Plain parent input Enter follows responsive
displayed logical order, not exact Access TabIndex. Only source-linked A/C/D and
Humus/Mineral scopes advance to the next existing row in the same literal column.
No wrap/allocation/Save/YAML option; actual visibility/ancestor-disabled/readonly
checks skip unavailable destinations and invalid raw input retains focus.
Lists/selects/multiline/IME/modifiers and other child scopes retain native behavior;
memo fields can be destinations without intercepting their own newline.
228 frontend tests/check0/0, both builds, focused9.549s/full race480.370s pass.
Initialf2aaa618.../owned3916 proves seven zero-write cases/all five record scopes;
final9fc42392.../owned4524 adds memo destination/newline/Undo. Read-only reference-
readiness/selector/Undo continuations are retained; completed input is not replayed.
Default bin/vpro-source-enter-default.exe SHA256
b7c09b6db6b31211a8595fad97df7b002868301142bbaa503633a50fbe442d53
verifies unchanged Enter, disabled metadata and active Vegetation with all bytes
unchanged; owned1348 closes. No owned native/race process remains.
Archives/source-enter-checkpoint seals74 hashed files plus manifest, preceding3db,
all three accepted binaries/assets, source, original fixture backup and proof/logs.
Independent manifest verification passes; original project/support/config bytes
and canonical Sample remain exact. Current/candidate are exact acceptedb7c copies.
Never replay source-enter.py prepare/run/restore, source-enter-ready.py,
source-enter-default.py, their completed tails or seal-source-enter.py.
Next is bounded readonly original plot-profile rule review, then ordered execution:
FS882.btnVegProfiling opens USysPlotProfiling; btnGetSummary calls ProfilePlots in
V7mdlPlotProfiling, not Profile4Presence. Preserve original `_Profile` tables/nine
bindings and establish family ownership; scratch results/counts belong to owned
SQLite TEMP state, not shipped supports. Keep the profiling action disabled.
Metadata-specific restoration and full navigation/focus parity remain open;
never copy master timestamps/numeric codes blindly or enable unavailable controls.
UpdateMetadataSppList is an immediate GoTo MyExit; duplicate merges are behind
unconditional Exit Sub. Source nonblocking NULL-cover messages must not become
rejects/deletions. CheckSpeciesCodes includes unsafe blanket user cleanup, not a
desktop requirement; do not blindly translate it.
Source effective properties/full event parity remain unknown; no further
native Access property probe is authorized by old evidence.
The original42-field checklist and readonly
15-list/260-row capture are sealed under `evidence/private/workflow-fs882-parent`.
SoilDrainage's explicit LimitToList=NotDefault now has a separately verified
exact-canonical membership policy, not unrestricted raw-code writes.
Omitted ordinary-combo effective properties are unknown:
raw-case/full-item/nullable/no-truncation behavior is a deliberate desktop policy,
not inferred Access parity. X/Y textboxes have no individual conversion events;
Photo text is not picture management. Projection, picture-manager/display,
bulk/reverse copy and remaining child/calculation events are still separate gaps.
See MIGRATION_PLAN.md. No native Access instance was launched for this batch.
No old grant, archived process identity or historical hash authorizes a new run.

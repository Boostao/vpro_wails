# Windows continuation

Updated 2026-10-01. Read [MIGRATION_PLAN.md](MIGRATION_PLAN.md) for priorities and
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
  all25 editor operations to immutable identities, leases running operations and
  rejects stale calls or legacy unscoped mutation/switch bypasses.
- Current and candidate executables contain the same verified default-on payload:
  `bin/vpro-current.exe` and `bin/vpro-current-candidate.exe`, SHA256
  `a2285ce37861acd7691be24351bdced9ca7eecef001eb3b5611dd97ca39564ef`.
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
  there as vpro-browse-read-verified.exe. C3 remains open for BEC/Quality/Working Unit
  and state/discovery reads and their frontend ownership.
- Verified writable parent coverage: **77/98**. Full storage mapping and source
  layout are not full FS882 or application parity.
- Responsive FS882 presentation is complete across all five source pages,
  child tables, toolbar/tabs and collapsible project context. Semantic field
  groups reflow4/3/2/1 columns with40px controls; raw source geometry remains
  evidence, not the rendering contract. Routine guidance/reference definitions
  are collapsed below the fields; safety feedback and required review stay visible.
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
  SoilDrainage remains disabled. Frozen catalogue:16 lists/347 rows/3,470 cells;
  the UI owns15 lists, excluding the distinct SoilDrainage workflow.
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
C3c shared-catalogue reads are delivered;
the rest of C3 and C4-C5 are pending. The promoted executable contains verified
F1-F3 plus C1 pooling/C2 caching/C3a-C3c retrieval cancellation.
Then resume ordinary surveyor/depth/cover/note fields. The original42-field checklist and readonly
15-list/260-row capture are sealed under `evidence/private/workflow-fs882-parent`.
SoilDrainage has explicit LimitToList=NotDefault and needs a distinct membership
policy; it must not silently inherit unrestricted raw-code writes.
Omitted ordinary-combo effective properties are unknown:
raw-case/full-item/nullable/no-truncation behavior is a deliberate desktop policy,
not inferred Access parity. Master BEC, BIT/NULL, X/Y and pictures remain separate;
do not enable them as incidental text fields. See the current remaining21 list in
MIGRATION_PLAN.md. No native Access instance was launched for this batch.
No old grant, archived process identity or historical hash authorizes a new run.

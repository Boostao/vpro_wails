# Windows continuation

Updated 2026-10-03. Current machine state only. Start with
[MIGRATION_PLAN.md](MIGRATION_PLAN.md) and [client scope](docs/CLIENT_SCOPE.md);
capability/gate reference is [README.md](README.md).

## Paths and tooling

- Worktree: `C:\Users\BrunoTremblay\Work\vpro_wails.worktrees\access-parity-windows-handoff`.
- Access corpus/source: `C:\Users\BrunoTremblay\Work\VPRO_ACCESS`;
  static exports: `VPro64_forAI`; installed cores/help: `VPro64`.
- R precedents: `C:\Users\BrunoTremblay\Work\vpro`; do not modify source data.
- Session: `C:\Users\BrunoTremblay\.copilot\session-state\ea0fd9cc-803b-46c0-adf3-b19c465b28f3`.
- Wails beta.26, Svelte5, Vite8; Go/CGO/MinGW, Node22; PowerShell5.
- Local PDF reader: Git's `pdftotext.exe`. Isolated PyMuPDF environment:
  `<session>\files\corpus-tools\Scripts\python.exe`; no app dependency added.

Per shell (environment changes do not persist):

```powershell
$env:PATH = 'C:\Users\BrunoTremblay\tools\go\bin;C:\Users\BrunoTremblay\tools\mingw64\bin;C:\Users\BrunoTremblay\tools\node-v22.14.0-win-x64;C:\Users\BrunoTremblay\go\bin;' + $env:PATH
$env:CGO_ENABLED = '1'
```

Use Windows paths and `npm.cmd`. Full integration uses
`go test -race -timeout 20m ./...` (not `-timeout20m`).
Generate actual bindings with
`wails3 generate bindings -f '-tags production' -clean=true -ts -i`.
Do not replace embedded frontend assets during compilation/tests.

## Protected delivery

- Branch/upstream: `agents/access-parity-windows-handoff` /
  `origin/agents/access-parity-windows-handoff`.
- Latest native application checkpoint: `5925cd2` shared CHARS, locally committed;
  no push. Closed opt-in/default proof binaries removed only after exact shrub
  archive comparison. Current/candidate remain the sealed10c5 default.
- Published forward predecessor: `35d9302`; accepted reverse SU Into Env local
  commit `96dca8d`, with assessment/documentation ancestor `6ac6f77`.
- Current extended-shrub acceptance: focused/coupled race9.482s; full race
  root858.736s;280 frontend tests, check0 errors/warnings; opt-in/default builds, eight native
  cases and zero-write default proof passed. Bindings15 services/166 methods/
  114 models/two enums.
- Protected exact accepted default/candidate:
  `vpro-current.exe`, `vpro-current-candidate.exe`, both SHA256
  `10c5b9bdd753cff08ad251ac83fa72a2c865e892d3041d862c68611a72e83e0d`.
  Extended-shrub opt-in core SHA256:
  `e39d5027e8ccbac9580f4a012652a20a3c924019a4c24cf2bd2c3d0a2dccc4be`.
  Independent feature gates remain off; no unavailable workflow was enabled.
- Immutable forward `archives\environment-su-checkpoint`:51 files plus manifest,
  exact core/default assets/binaries, source/logs/native receipts and before/after
  data; verifies the49-file metadata predecessor and preserves old protected build.
  Older metadata75/Find59/SU84 evidence remains intact.
- Reverse successor: `archives\su-environment-checkpoint`:59 files plus manifest,
  exact binaries/
  assets, source/logs/native receipts, rollback diagnostic, data/visual and
  restoration proof. Prior forward seal and protected default preserved before
  promotion; never rebuild the accepted default during promotion.
- Pure Go Long Environment planner predecessor: `7fa2436`.
  Focused/coupled race23.414s; full race root666.610s passed.
  `archives\long-environment-planner-checkpoint`:six files plus manifest,
  exact source/schema contract, source and focused/full receipts; reverse59-file
  predecessor and protected executable unchanged at that checkpoint.
- Report preview successor: `archives\long-environment-preview-checkpoint`.
  Exact core/default assets/binaries, source/logs/native receipts and600px visual;
  planner/reverse seals verified unchanged, reverse protected default retained
  before exact promotion. Owned read-only whole selected-SU scope supports
  external SU paths. No profile filtering, export or summary calculation.
  Initial YAML title retained; title edits are explicitly preview-only.
- Shared extended-shrub successor: `archives\extended-shrubs-checkpoint`.
  Exact core/default assets/binaries, source/logs/native receipts, whole-project/
  exact three-audit differences and600px visual; report51-file predecessor
  unchanged, report protected default retained before exact promotion.
  One native three-cell write; original16 hashes restored. The initial harness
  tried Cancel after a semantic no-op correctly hid the toolbar; four verified
  no-write cases were retained, then only the unfinished tail continued.
  No successful write was replayed. Source SubVegAXL metadata packaged read-only;
  checkbox is presentation-only, height interaction explicitly cover-only.
- Long Vegetation preparation is a pure Go successor, not a new executable or
  native report. It uses typed snapshots, original15-field MAX reduction,
  physical SU memberships/provenance and original LayerCode layer conversion.
  Five focused tests cover independent disposable SQLite MAX, read-only bundled
  metadata, zero/negative/extended/historical values, exact integers, duplicates,
  permutation/output independence, cancellation and malformed storage.
  No service/bindings/UI changes; no native suite or production/source writes.
  Focused/shared race1.261s and full integration root672.153s passed.
  `archives\long-vegetation-preparation-checkpoint`:nine files plus manifest,
  source/schema/layer contracts and exact focused/full receipts. The57-file
  shrub predecessor and current/candidate executable remain unchanged.

## Cleanup and evidence

- 75 closed top-level builds removed from the active bin only after exact archive
  hash verification:54 reused existing sealed copies,21 newly archived.
  Duplicate removal freed4,059,075,072 bytes; newly archived files were preserved.
  Mapping: `archives\closed-builds-consolidation-20261003\manifest.json`.
  Resolve historical receipt binary paths through that mapping, never rebuild
  an old proof just because its original bin path no longer exists.
- Prior plan/handoff/README/AGENTS were snapshotted byte-for-byte before
  consolidation. Assessment extracts, hashes, cleanup receipt and delegate
  outputs live under `<session>\files\asset-assessment`,
  `client-scope-review` and `field-manual-review`.
- Resources/testdata/generated bindings remain tracked. `.gitignore` already
  excludes private evidence, archives, runtime data/config, tools and builds.
  The large private evidence corpus was retained; unknowns are not deletion targets.
- Current sole vegetation fixture: `evidence\private\native-vegetation-species`.
  Extended-shrub opt-in/default native owners exited normally; all16 original
  project/support/config file hashes restored. Temporary report external SU
  remains absent.
  No pending native write or suite needs continuation.
  **Never replay completed** extended-shrubs.py prepare/run/continue/default,
  long-environment-preview.py prepare/run/default,
  su-environment.py prepare/run/finish/default,
  environment-su.py prepare/run/default or older
  metadata/profile/Find/vegetation modes.
- No owned bin process was observed during cleanup. No Access process was
  touched. Before any future native action inspect current PID/start/path/owned
  windows; historical IDs are not authority. Do not kill processes by name.

## Resume gate

Long Vegetation: user's investigation request was used for one selective ACE
query probe on a newly created disposable accdb. `DAO.DBEngine.120` opened no
source VPRO database or Access application and spawned no native owner.
No registry/application-preference changes were requested.
Four grouped/ungrouped and all-plot/observation-average
cases verify non-NULL pivot-column presence, zero/negative covers, duplicate
SU weights and NULL cover output. Identity vpRoundUp wrapper was omitted.
`evidence\private\native-long-vegetation\presence-query-results.json` retains
exact SQL/results; `presence-query.accdb` is solely disposable. Never replay
`probe-presence.ps1` against that existing fixture; it rejects overwrite.
`constant-null-query-results.json` verifies NULL group/species do not match
and empty text does, using physical typed tables in the same owned fixture.
Its first constant-expression saved-query variant unexpectedly dropped a NULL
left row; that optimizer behavior remains unknown. Two attempts only, no
broader oracle debugging. Do not replay `probe-null-list.ps1`.
The pure crosstab kernel matches all four cases, keeps independent denominator,
strict thresholds and constant-list bypass, and NULL missing-list statistics.
The first integration run was stopped before completion to correct constant-list
NULL equality; only the final post-correction integration receipt is acceptance.
Final focused race1.241s (four crosstab/five preparation tests) and full Go race
root664.970s passed. `archives\long-vegetation-crosstab-checkpoint` preserves14
files plus manifest: exact code/fixture/source contracts, seven selective ACE
cases, disposable database and focused/full logs. Preparation's immutable
nine-file seal, original bundled layer database and current/candidate10c5
executable remain unchanged. No frontend/binding/build/native-desktop action.
No quality/reference/grouping/lumping integration or report service/UI claim.
Current protected executable remains unchanged. Retain YAML=-1/source behavior;
do not substitute R quick/all-veg formulas or silently ignore inactive options.

Reverse transfer is implemented/native-verified, independently default-off.
The first late-trigger byte assertion stopped the harness without saved before
hashes; its original discrepancy remains unknown. Independent full-table/no-history
observation and one retained rejected-action diagnostic verified settled exact
byte rollback. A later harness-only NameError required post-commit read-only
cleanup continuation; the successful transfer was never replayed. All original
fixture bytes are restored and both owners closed. No production/source writes.
The current desktop target and UI-free companion R package are distinct from the
historical Shiny/cloud agreement. Unmarked screenshots are candidate retain,
not approved as-is; colour-only decisions and SU append/new-table semantics remain
open in [client scope](docs/CLIENT_SCOPE.md).

Current successor: internal Long Vegetation layer planner and owned read-only
helper integrate original physical Veg/selected SU/master USysAllSpecs/LayerCode,
including external SU. One independent delegated task owned only the strict
YAML options decoder/tests; the primary integrated planner/ownership/evidence.
Native ACE in the same sole disposable query fixture verified eight rows of
English grouping and constant-list Layer/Spp-only fanout. Exact results:
`layer-join-results.json`, matched by tracked testdata fixture. Never replay
`probe-layer-join.ps1`; the original crosstab14-file predecessor remains sealed.
No Access application, production/source data, frontend/bindings/native desktop
build was changed. Internal Go method is not Wails-exposed.
Combined22 focused tests passed race5.224s; full Go race root659.145s passed.
Independent read-only review found no significant defects. Its pending-race
acceptance note is now resolved by the completed/read full result; its lack
of full native report/Wails acceptance remains an explicit boundary.
`archives\long-vegetation-layer-checkpoint`:18 files plus manifest, exact
source/fixture/contracts and focused/full receipts. The14-file crosstab
predecessor and protected current/candidate10c5 executable remain unchanged.

Next: expose typed context/path-bound Long Vegetation options/preview service,
then responsive default-off report UI with existing draft/navigation/close
guards and native zero-write acceptance. Preserve source defaults and explicit
inactive-option errors; unit long names, quality/non-layer modes, summary/title
persistence and publication remain separate work.
Source LongVeg bypasses presence/mean-cover
thresholds when constant-list is on; retained YAML default is -1. That behavior
is provisionally retained while the user is unavailable. Pure preparation is
Go-tested and source joins selectively ACE-probed; no public Long Vegetation
service/UI or native application suite was launched.
Static contracts reside under session
`files\overnight-form-contract` and `files\overnight-long-report`; they are not
runtime parity proof. Do not resume stale
parent-field inventories, broad infrastructure refactors or unapproved Access
oracle debugging. Full migration and deliverables6-10 remain open.

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
- Published forward predecessor: `35d9302`; accepted reverse SU Into Env local
  commit `96dca8d`, with assessment/documentation ancestor `6ac6f77`.
- Current report-preview acceptance: focused race3.015s; full race root942.439s;
  276 frontend tests, check0 errors/warnings; opt-in/default builds, five native
  cases and zero-write default proof passed. Bindings15 services/166 methods/
  114 models/two enums.
- Protected exact accepted default/candidate:
  `vpro-current.exe`, `vpro-current-candidate.exe`, both SHA256
  `144b48d3504683e90a73da9f8ec81889bc5db73749218ebbd1a0d31b6e7f0316`.
  Report-preview opt-in core SHA256:
  `43209c78fc743e93cea1bf7908653a8cbb7a43ae4c540b35b502a99791de0cea`.
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
  Report opt-in/default native owners exited normally; all16 original project/
  support/config file hashes restored and temporary external SU removed.
  No pending native write or suite needs continuation.
  **Never replay completed** long-environment-preview.py prepare/run/default,
  su-environment.py prepare/run/finish/default,
  environment-su.py prepare/run/default or older
  metadata/profile/Find/vegetation modes.
- No owned bin process was observed during cleanup. No Access process was
  touched. Before any future native action inspect current PID/start/path/owned
  windows; historical IDs are not authority. Do not kill processes by name.

## Resume gate

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

Next: shared CHARS extended-shrub behavior, then FS1333 and priority
vegetation reports. Static contracts reside under session
`files\overnight-form-contract` and `files\overnight-long-report`; they are not
runtime parity proof. Do not resume stale
parent-field inventories, broad infrastructure refactors or unapproved Access
oracle debugging. Full migration and deliverables6-10 remain open.

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
- Last published application milestone: `35d9302` forward Env Into SU;
  metadata restoration predecessor `392ec99`.
- Acceptance: focused race9.700s; full race root629.894s;269 frontend tests,
  check0 errors/warnings; opt-in/default builds and six native cases passed.
  Bindings15 services/162 methods/101 models/two enums;62 context operations.
- Top-level `bin` contains only current baseline and candidate:
  `vpro-current.exe`, `vpro-current-candidate.exe`, both SHA256
  `ddbc793d3d177b8cfef16b37fa4cfa1f891f1b2fa7213939949fa372cdf0fd75`.
  The accepted forward-transfer core is archived, not a current default build.
  Independent feature gates remain off; no unavailable workflow was enabled.
- Immutable `archives\environment-su-checkpoint`:51 files plus manifest,
  exact core/default assets/binaries, source/logs/native receipts and before/after
  data; verifies the49-file metadata predecessor and preserves old protected build.
  Older metadata75/Find59/SU84 evidence remains intact.

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
  Both accepted SU native owners exited normally; original project/support/config
  bytes restored. No pending native write or suite needs continuation.
  **Never replay completed** environment-su.py prepare/run/default or older
  metadata/profile/Find/vegetation modes.
- No owned bin process was observed during cleanup. No Access process was
  touched. Before any future native action inspect current PID/start/path/owned
  windows; historical IDs are not authority. Do not kill processes by name.

## Resume gate

This round was assessment/documentation/archived-build cleanup only. No application
behavior, fixture settings, production data or gates changed; no new native suite.
The current desktop target and UI-free companion R package are distinct from the
historical Shiny/cloud agreement. Unmarked screenshots are candidate retain,
not approved as-is; colour-only decisions and SU append/new-table semantics remain
open in [client scope](docs/CLIENT_SCOPE.md).

Next round: review the scope ledger, finish the bounded reverse SU Into Env
contract/implementation, inventory FS1333/two-page/CHARS differences, then a
priority read-only long vegetation/environment report slice. Do not resume stale
parent-field inventories, broad infrastructure refactors or unapproved Access
oracle debugging. Full migration and deliverables6-10 remain open.

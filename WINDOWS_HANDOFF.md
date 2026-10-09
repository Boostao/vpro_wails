# Windows handoff

Current operational state only. Portable capability and backlog information is
in [README](README.md), [MIGRATION_PLAN](MIGRATION_PLAN.md) and
[the evidence summary](docs/MIGRATION_EVIDENCE.md). Machine-specific paths and
bulk execution receipts are maintained privately rather than published here.

## Current work

- Forms/reports migration remains incomplete and is paused for feature-sized
  publication organization.
- Historical SIVI creation Undo is the latest accepted bounded desktop slice.
  Its private successor preserves 2,650 files/987 application sources, with
  597 unchanged Go inputs, 952 frontend tests and five normally closed native
  owners. Exact scope/exclusions/hash are in the portable evidence summary.
- The original working branch retains local snapshot `2e03915` and the preceding
  nine local commits. Its attempted bulk push was stopped before publication;
  do not reset, amend or rewrite that branch as cleanup.
- Corrected branch `reviews/access-parity-feature-series-v2` contains 50
  feature-sized commits from `0112b98`, feature tip `7ff3507978d0`. Each updates
  [feature capability boundaries](docs/FEATURE_CAPABILITIES.md); its 987
  application/597 Go inputs exactly match the accepted source freeze.
  The original bulk and first feature series remain untouched. Do not stage
  mutable original files or unfinished drafts into the corrected series.
  Remote publication and PR are not yet complete.
- The introduced-test audit covers 755 Go tests; 40 omitted by earlier focused
  selectors were executed at their exact corresponding feature stages. This
  includes all 20 new picture-metadata/extractor tests in feature 47.
- Fresh recomposed actual-main desktop validation completed five normally
  closed owners and ten Create/Undo cycles with explicit pre-Undo physical
  receipts. All 16 prepared and 39 protected identities were independently
  rechecked. These bounded results do not establish overall form/report parity.
- The first publication race run reached its 45-minute runner alarm during a
  late, seven-second-old test; the failed receipt is retained. The unchanged
  60-minute retry passes all packages, root 3522.388 seconds, with all 597 Go/17
  production asset inputs rehashed before and afterward. Focused independent
  publication review reports no significant issues. Receipt hashes and scoped
  limitations are in [publication validation](docs/MIGRATION_EVIDENCE.md#feature-series-publication-validation).
- The two untracked picture-creation kernel/test drafts remain private,
  paused and unreviewed. Focused tests/vet do not establish native acceptance
  or permission to add a public service.
- No migration native/Access owner or integration test shell remains active.
  Do not replay completed
  fixture preparation, mutations, closures or predecessor seals.

## Environment

Windows PowerShell 5, Go with CGO/MinGW, Node/npm and Wails v3 beta.26 are
available on the migration VM. Canonical Access exports are local to that VM
and remain read-only; use the machine's private path registry to locate them.
The repository contains no installed-tool or private-corpus path contract.

Environment changes do not persist between PowerShell calls. Set the Go,
MinGW and Node directories in the current shell as needed; use `npm.cmd`.
Do not clear shared caches or overwrite accepted frontend bundles.

```powershell
$env:CGO_ENABLED = '1'
go test -race -timeout 60m ./...
```

Generate actual nullable interface bindings with:

```powershell
wails3 generate bindings -f '-tags production' -ts -i
```

For isolated Go 1.25 embed overlays use `GODEBUG=goindex=0` and verify exact
`go list` embed patterns before compiling. Do not substitute a browser build
for Wails binding/lifecycle acceptance.

## Resume after publication

Preserve the accepted source identities and all private evidence. Complete
source-bound picture creation/selected-file authority, then picture deletion/
restoration and remaining form/report deliverables. Keep experimental gates
independently default-off. Do not reopen stopped Source4/Access Val calibration
or infer overall completion from this publication.

# VPRO Desktop

Go/Wails v3 beta.26/Svelte 5 migration of VPRO Access, with SQLite canonical.
**Experimental: useful entry and reporting workflows exist, but this is not a
complete or production-ready Access replacement. Use disposable data for writes.**

## Start here

- [Current readiness and delivery backlog](MIGRATION_PLAN.md): the single current
  workflow-level status, supported preview and next delivery target.
- [Windows handoff](WINDOWS_HANDOFF.md): current machine, tools and ownership.
- [Client scope](docs/CLIENT_SCOPE.md): retained requirements and separate tracks.
- [Capability contracts](docs/FEATURE_CAPABILITIES.md): implementation boundaries,
  not a list of features enabled in every build.
- [Verification evidence](docs/MIGRATION_EVIDENCE.md): scoped results and receipt
  identities, not whole-application acceptance.

## What can be used?

The ordinary build provides existing SQLite-family selection, project/plot
browsing, read-only hierarchy browsing and bounded FS882 entry. The 98 mapped
parent fields and ordinary child editors have native verification within their
documented boundaries.

SIVI/CHARS, two-page entry, metadata, profiles, pictures, report previews and
XLSX/KML/PDF publication have separate development opt-ins. Some are extensively
verified; that does not make them available in the ordinary build or complete
their original Access workflow.

Production Access import/conversion, project creation, merge/splinter/compare,
complete report formatting/printing and the full Access ribbon remain unfinished.
See the [readiness matrix](MIGRATION_PLAN.md#current-readiness) rather than
inferring availability from menus, source files or a passed component test.

## Qualified Windows preview

The current supported private preview is:
`VPRO-Demo-2026-10-18-startup.zip`, 17,522,075 bytes.

SHA256:
`bad5cec8574d6dbdfbf9ceaf9d8157d5c4d856ea1e88176812c77a2d468cacfb`.

Extract the entire archive and run `Launch-Demo.exe`, not `VPRO-Demo.exe`.
Microsoft Edge WebView2 Runtime is required. Every launch creates a fresh retained
disposable run; changes persist in that run, not in the seed or next launch.
This is a private preview, not a signed production installer.

Its embedded `package-manifest.json` pins the exact binary, source-input identity,
31 frontend and 28 runtime opt-ins. Its `DEMO.txt` gives the qualified walkthrough:

1. Select P1 and open the FS882 editor; edit, Undo/Save, then inspect P2/P1.
2. Open separately enabled SIVI/CHARS entry and its available linked workflows.
3. With a clean unlocked selected plot, review/edit an explicit metadata record.
4. Inspect Summary, saved User log paging and the bounded Matrix XLSX workflow.
5. Explicitly select the synthetic **DemoReports** SU for Long Environment and
   Long Vegetation: load titles/options, preview, review and explicitly publish
   each supported XLSX to a new destination. Title Save/Undo is separate from
   workbook publication. Select the original **Report** SU for Summary.

The manifest pins source base `8184550` plus the exact reviewed deferred-entry
delta; it does not identify the executable as unmodified base source. FS882 and
CHARS entry panels now load on demand, keeping their existing context/form/plot
keys and eager draft/operation guards. Actual initial transitive JavaScript falls
from5,202,547 to928,901 bytes (82.15% less). Total JavaScript is5,227,533 bytes,
24,986 bytes more; this is not a measured startup-time or binary-size reduction.

Native Windows acceptance holds module requests across Home, plot and SU-context
replacement, verifies only the current owner mounts, and checks visible import
failure/Home recovery and clean close while a module is held. Parent Undo,
P1-P2-P1, one actual parent Save/two audits, invalid child/close guards, one held
child Save/two audits and explicit restoration pass without replay. Child history
is retained and marked restored; only its selected audits are pruned. The same
saved family reopens with exact parent/child values, normal SIVI77/CHARS71 fields,
metadata and responsive980/1400 layouts. Long report reads retain72 Environment
rows and13 Vegetation rows. All sixteen closed family files remain exact after
those reads. Focused110 cases, check0/0, production builds and independent review
pass. Backend sources are unchanged; no new full-root race is claimed.

The unchanged launcher and seed retain the prior31/28 report profile. The
preserved report ZIP, SHA256
`f9d1d26c94b119165cd0bec0511a996933e1b33e7340fd917066cc5751cc9383`,
owns216 canonical Environment values, Vegetation pivots, two title Saves/two XLSX
publications, independent file reopening and report cancellation/stale/no-replace
guards. Those mutations were not replayed on the startup candidate. Missing names,
NULL Strata and duplicate DemoReports membership remain preserved.

Other parent/metadata Saves, Summary/log/Matrix and child lifecycle acceptance
remain scoped to the preserved23/22 predecessor,
SHA256 `fcf4104370f00f7305b46aa26c64b0d663f7c22a0089991fd2933a21a585d41b`.
Their source contracts/flags are retained; not every child mutation was rerun on
the new binary. Close metadata review explicitly before changing context.
**Later source additions are not automatically included or qualified in this ZIP.**
The [delivery plan](MIGRATION_PLAN.md#supported-preview-profile) owns promotion.

## Build and develop

Prerequisites: Go 1.25, Node/npm, Wails v3 beta.26 and platform-native Wails/cgo
dependencies. Windows tool locations are in the [handoff](WINDOWS_HANDOFF.md).
Use `npm.cmd` in Windows PowerShell if execution policy blocks `npm.ps1`.

```powershell
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.26
wails3 dev
```

Development uses the native Wails window and Vite on port 9245.
A browser preview cannot validate Go bindings or native close behavior.

```powershell
wails3 build
wails3 task windows:build:sqliteonly
```

The explicit SQLite-only Windows profile omits optional DuckDB linkage without
removing canonical SQLite workflows. It does not enable experimental features.
The qualified preview executable is43,058,688 bytes,30,720 bytes larger than
the report preview; deferred loading is not a binary-size reduction. An earlier
controlled same-assets comparison measured about 38.4 MB SQLite-only versus 87.8 MB with
optional DuckDB; do not confuse those different build identities.

### Validation

Run focused checks for the affected workflow first. Frontend application changes
also require tests, type-check and a production build:

```powershell
Set-Location frontend
npm.cmd test
npm.cmd run check
npm.cmd run build
```

For acceptance candidates, build into an isolated output directory and preserve
the accepted frontend assets. Generate Wails bindings with `-ts -i`, not `-ts`
alone. See the [handoff](WINDOWS_HANDOFF.md) for the Windows Go/cgo environment,
overlay discovery and disposable native profiles.

At a coherent application integration boundary:

```powershell
go test -race -count=1 -timeout 150m ./...
```

Do not overlap root Go suites. The latest accepted run took about 83 minutes;
documentation-only changes do not require rerunning it. Passing tests is not
whole-form parity or a substitute for independent review/native acceptance.

## Data and architecture

- Preserve the original SQLite database family, per-project physical names and
  `_table_metadata` Description storage. Catalogues are read models, not a
  replacement for that family.
- Reuse F1-F3 configuration/context infrastructure: runtime/default YAML,
  explicit lossless JSON migration, owned external attachments/TEMP views and
  draft-safe switching. Offline startup must not require DuckDB extensions.
- Mutations, source audits and typed history share transactions. Keep physical
  row ownership distinct from nullable/mutable application IDs.
- Preserve historical values, NULL/empty distinctions, Access BOOLEAN true=-1,
  UTF-16 text bounds, cancellation and exact-request recovery without replay.
- Access reading belongs to go-mdbtools. Existing private import/staging and
  template boundaries are not a production importer or project creator.
- UI-free R automation and cloud/publication products remain separate agreed
  tracks, not delivered features or silently cancelled requirements.

The root application package is currently too flat. Consolidation will follow
demonstrated domain/dependency boundaries, with tests retained beside Go
packages; moving tests alone does not reduce application complexity. Frontend
fixtures/helpers should be separated from production source during that work.
Do not remove safety checks or explicit parity cases to meet a LOC/file target.

## Releases

The [manual release workflow](.github/workflows/release.yml) builds fourteen
Windows/macOS/Linux payloads and verifies package/source/CPU/checksum identities
through [release tooling](scripts/release.py). Hosted
[run 38929085479](https://github.com/Boostao/vpro_wails/actions/runs/38929085479)
passed all seven jobs on `842ca946ac810393f9d548349f693ac0f51e1f13`;
`v0.1.0-demo.20261016.2` is a draft/prerelease.

That is default-profile packaging acceptance, not promotion of the private
preview or all-platform native functional acceptance. Windows packages may be
unsigned; macOS is ad-hoc signed, not notarized.
The [download portal](docs/index.html) selects stable releases, not drafts.
No current Pages deployment success is inferred from a successful release run.

## Historical ledgers

The former long root documents are preserved in Git, not copied into another
current-status file:

- [Pre-reset README](https://github.com/Boostao/vpro_wails/blob/3bf43c4454bae07bcd482c04f7d665341c0c065e/README.md)
- [Pre-reset migration ledger](https://github.com/Boostao/vpro_wails/blob/3bf43c4454bae07bcd482c04f7d665341c0c065e/MIGRATION_PLAN.md)

Those immutable snapshots contain historical technical notes and superseded
statuses. Use the current plan for decisions. Private fixtures, receipts,
Access copies and closed builds remain unchanged and must not be deleted.

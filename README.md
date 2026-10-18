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
`VPRO-Demo-2026-10-18-linked-soil.zip`, 17,549,069 bytes.

SHA256:
`4f3f6ed92a9d38cd02bc10e4eafd88dd1af867030a567cbef883f894d9f19fd5`.

Extract the entire archive and run `Launch-Demo.exe`, not `VPRO-Demo.exe`.
Microsoft Edge WebView2 Runtime is required. Every launch creates a fresh retained
disposable run; changes persist in that run, not in the seed or next launch.
This is a private preview, not a signed production installer.

Its embedded `package-manifest.json` pins the exact binary, source-input identity,
40 frontend and 36 runtime opt-ins. Its `DEMO.txt` gives the qualified walkthrough:

1. Select P1 and open the FS882 editor; edit, Undo/Save, then inspect P2/P1.
2. Open separately enabled SIVI/CHARS entry and its available linked workflows.
3. With a clean unlocked selected plot, review/edit an explicit metadata record.
   Alternatively, use **Load complete entry** for the complete normal/CHARS
   two-page **parent** editor. Save/Undo there, then close its review explicitly.
4. Inspect saved User log paging and the bounded Matrix XLSX workflow.
5. Explicitly select the synthetic **DemoReports** SU for Long Environment and
   Long Vegetation: load titles/options, preview, review and explicitly publish
   each supported XLSX to a new destination. Title Save/Undo is separate from
   workbook publication.
6. On **DemoReports**, open **Summary Vegetation**, load saved options/quality
   and retain None grouping/display6/average1/show1. Select environmental rows,
   SU Table and ReportSummary, review the complete plan, then explicitly publish
   to a new `.xlsx` destination and acknowledge the outcome.
7. In P1 complete entry, load linked Humus/Mineral. Explicitly create a Humus
   horizon in Normal and a Mineral horizon in CHARS, acknowledging each receipt.
   Edit Depth Up/memo, Save explicitly and acknowledge; switch variants to
   inspect the saved rows. **Undo drafts does not delete a committed horizon.**

Application source is committed `bc9e129`; app/backend/seed/defaults are unchanged.
Three existing linked-soil review/writing/creation gates extend the preceding
profile. No new application kernel, service or dependency was introduced.
The new native journey checks normal117/CHARS119 hosts, two explicit creations
with complete18/28-cell defaults, two field Saves, six exact physical source
audits, four persisted histories and permanent ID reservations. All historical
physical peers remain exact. P1 starts without soil rows; unrelated historical
records are not repointed. The server-owned positive allocator is an explicit
adaptation, not Access random `GenUniqueID()` compatibility.

Invalid drafts block creation/parent/variant/Home/native close. Fresh CancelClose
acknowledgements precede Undo and held-response release. One held creation is
cancelled and recovered read-only, never replayed. Saved rows survive variant
remounts; the other15 family files remain exact. Three independent backend-off
reopens refuse review/writing/creation, load saved fields/source choices and
preserve all16 files. All four owners close normally. Fields, actions and
disclosures are measured at980/1400.

All105 related client cases, launcher races, soil Go races, check0/0, isolated
Vite/Go builds, exact embed discovery and independent read-only review pass.
No new full-root race, implicit creation, soil deletion/restoration or whole-form
callback qualification is claimed. Ordinary Save and Enter cannot create soil.

The immutable Summary predecessor remains unchanged, SHA256
`507acfd1dc5ee943c261019a504839cb8287fd0bcfde63c8b3a49d9b9904437c`.
It owns the combined workbook's one publication,13 independently calculated
scientific-name vegetation rows,30 environmental cells against the reviewed
plan, four physical memberships and very-hidden provenance. Environmental
calibration also retains kernel lineage. P1 appears twice; P3 has physical LF1
zero cover, not missing vegetation. Absent master long names produce no invented
ReportSummary unit-name row. Common/guide/Constancy, original Report callbacks
and full formatting/printing remain unqualified in this profile.

The immutable entry-performance predecessor remains unchanged, SHA256
`0de8422f3e7d24d73fe9155ba03b9aefebb3cf3694d3ba09a4e6eb043c9fb76e`.
It owns normal117/CHARS119 layouts, one three-scope Save/three audits, saved
reopening, backend denials and native typing medians8.8/9.4ms. Its immutable
reference-snapshot optimization remains: clean/dirty session view medians fell
98.85/95.72ms to0.65/0.50ms on the captured6,040,765-byte wire. These are inherited
bounded measurements, not fresh timing or initial-load/SQL speedup claims.

Current initial transitive JavaScript is988,749 bytes versus5,202,547 before
deferral (80.99% less); total JavaScript is5,306,806. The extra entry surface
slightly increases payload and EXE size; no new startup or binary-shrink claim.
Cold-load/child Save/restore proofs retain the startup predecessor's exact binary
lineage. Long report publications retain the report predecessor's lineage; other
metadata/child lifecycle mutations retain their23/22 predecessor scope. They
were not replayed on this binary. The [delivery plan](MIGRATION_PLAN.md#supported-preview-profile)
pins those preserved archives. Close metadata/complete-entry reviews explicitly
before changing context.
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
The qualified preview executable is43,138,560 bytes, 41,984 bytes larger than the
Summary preview; deferred loading is not a binary-size reduction. An earlier
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
[run 39149407850](https://github.com/Boostao/vpro_wails/actions/runs/39149407850)
passed all seven jobs on `7f4dd231559ae9bf40f654c885722dfce9b5bcf9`;
`v0.1.0-demo.20261018` is a draft/prerelease. All fourteen payloads and the
checksum manifest are attached; source/CPU witnesses, remote byte verification
and independent API digest checks pass.

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

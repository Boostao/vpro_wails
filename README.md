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
`VPRO-Demo-2026-10-18-coordinate-kml.zip`, 17,571,483 bytes.

SHA256:
`3e09db767075a2f194b542ca60ade4b22e075231140aa36174b4a248ba08f6de`.

Extract the entire archive and run `Launch-Demo.exe`, not `VPRO-Demo.exe`.
Microsoft Edge WebView2 Runtime is required. Every launch creates a fresh retained
disposable run; changes persist in that run, not in the seed or next launch.
This is a private preview, not a signed production installer.

Its embedded `package-manifest.json` pins the exact binary, source-input identity,
49 frontend and 45 runtime opt-ins. Its `DEMO.txt` gives the qualified walkthrough:

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
8. In Summary's explicit DemoReports scope, select effective references and
   common names; review/publish to a new destination and acknowledge. Select
   guide names for a separate publication: this seed's guide names are NULL,
   not ordinary-name fallbacks. Load presentation, stage Constancy without Save,
   clear environmental rows and review/publish its separate vegetation workbook.
   Acknowledge, then Undo the presentation draft before leaving.
9. In clean unlocked P1 complete entry, load Tree/Shrubs vegetation. Edit A1
   (Cover1) and cycle Collected, Save explicitly and acknowledge. In CHARS,
   load vegetation and edit B3 (Cover5a); Save and acknowledge. Reload the Normal
   alias before another edit. Herb/Moss have zero C/D query memberships in this
   seed: their physical rows remain read-only, not creation opportunities.
10. For existing-list Species selection, first **Review Species definitions
    (read only)**. Choose an exact LF1..LF4 code in Normal or CHARS, optionally
    coedit covers, Save explicitly and acknowledge. Reload the other alias before
    further edits. Manual/alias/NotInList/personal creation remain unavailable.
11. Reports > **Saved plot label PDF — Portable**: review P1, inspect raw
    assignments and enter a new full PDF destination in an existing folder.
    Export once, recover readonly if needed, then acknowledge. P2 demonstrates
    empty assignments; P3's NULL Zone explicitly refuses. Correct and review again.
    Receipts survive remount. This is not physical Print/positions/Print All.
12. On a fresh run, load P1 complete entry, enter Latitude50.25/Longitude123.25
    and **Save complete entry** once. Close complete-entry review, then Home.
    Reports > **Plot location review** shows one DISTINCT row. In **Google Earth
    location preparation**, load fields, choose literal PlotRepresenting, set
    Place Name and review raw locations, then review/export KML to an explicit
    new destination. This seed has two P1 SU memberships: both raw rows and both
    placemarks are intentionally retained. Acknowledge the retained outcome.
    Viewer launch, remote icons, saved preferences and implicit Save remain off.

Source is committed `4b328ad`; production app/backend/seed/defaults are unchanged.
Four existing Location/Google Earth/KML gates extend the preceding profile.
No application kernel, service, dependency or seed change was added.

Current native acceptance saves exactly two P1 coordinates with two audits and
one technical history;389 historical audits, all peers/unmapped cells and the
other15 files remain unchanged. Location has one DISTINCT row; direct Env/SU
preparation and one KML export retain both physical memberships. Independently
reopened UTF-8/XML/hash proves the literal title and two P1 placemarks, each
`-123.25,50.25,0`; raw NULL description remains NULL and renders as a period.
Actual held-review Cancel/remount rejects its late result. Actual successful
publication is held/released once; acknowledgement/remount retains the receipt
without XML. Four fresh native-close acknowledgements precede release/ack.
Controls/actions/all report cells/long receipts are visible/contained980/1400.
Four saved reopens disable one runtime gate each: six backend denials,
independent KML reads and ordinary117/saved coordinates remain available.
All five owners close;16 saved files and complete physical peers stay exact.
All62 focused clients, launcher races, coupled Go races137.302s/check0/0 and
isolated Vite/Go/exact26 embeds pass; independent review finds no material issues.
Native was not first-pass: missing harness `ast` before UI actions, then an
incorrect one-row raw expectation. Both same-owner corrections are preserved;
no Save/export replay or application repair. No new full-root race/oracle,
formal KML schema/viewer, DMS/single-plot callbacks or complete-parity claim.
This KML service does not have the PDF service's readonly recovery API.

The immutable45/41 saved-label predecessor remains unchanged, SHA256
`35714f1c79ba444912d218b39fe33e9276fa75d4b19c5d2319cac4e0800235d4`.
It owns one P1 PDF/five exact strings/Arial/Letter page, NULL/empty assignments,
P3 refusal/retry, held-review cancellation and lost-response readonly recovery,
retained receipt and two closed owners with16 exact files/full snapshots.
Its cleanup/preflight harness failures and corrections remain preserved.
Those label publications were not replayed on this binary. Single-line printable
ASCII/calibrated en-US/runtime fonts remain bounded; physical printing,
positions/stock/Print All, live staging and Unicode/multiline/overflow remain off.

The immutable44/40 Species predecessor remains unchanged, SHA256
`6fc79320bc1ae16bae2e2bb345042dcc15611b12d1ec662fc95f3e132ce705af`.
It owns Normal LF1->LF2/Cover1 and CHARS LF2->LF3/Cover5a: two Saves/four audits/
two histories on row1/ID-100001, complete peers/metadata/reference preservation,
joint soil guards, recovery/alias/remount and six denials/three saved reopens.
All four owners close;16 files exact each. Four master LF1..LF4 codes/personal0
and original/planned common-seven-cover requirement retain their exact scope.
Manual/alias/NotInList/personal creation, populated personal/duplicate-reference/
whole-Unicode, populated C/D, heights/identity/lifecycle and implicit callbacks
remain excluded. Those mutations were not replayed here.

The immutable43/39 vegetation predecessor remains unchanged, SHA256
`5c3e9ece6accd6fffdfed392e8aae99fa6819d2a54a57276d5cc724525f7f57f`.
It owns Normal Cover1/Collected and CHARS Cover5a, two Saves/three audits/two
histories, TEXT1 NULL/C/V, invalid100/narrowed100/overlength guards, read-only
recovery, alias/remount/saved reopen and independent denials. C/D source membership
is0 with13 read-only physical rows each. All three owners close. Its combined124
startup cases and one corrected bound-editor test fixture retain their identity;
the Collected journey was not replayed on this binary.

The immutable reference-report predecessor remains unchanged, SHA256
`b31cc3038bf5742486a84dfc6bff3406da17a5769b7eff568f600555cbc94a5e`.
It owns three common/guide-NULL/Constancy publications,39 independent vegetation
rows/physical names,278 typed cells including60 reviewed environmental cells,
numeric Lifeforms1..13/full provenance, cancellation/close/acknowledgement guards
and a references-off reopen with16 exact files. P3's physical LF1 zero counts
in Constancy (frequency1.00, others0.75, mean1.56), not the None stage, whose
denominator remains four. Environmental calibration retains kernel lineage.
Its inspector corrections and reviewed walkthrough correction remain preserved;
those publications were not replayed on this binary. Use a fresh launcher run
for unchanged report examples after vegetation edits. Populated guide names,
original Report/schema-renaming callbacks and formatting/printing remain unqualified.

The immutable soil predecessor remains unchanged, SHA256
`4f3f6ed92a9d38cd02bc10e4eafd88dd1af867030a567cbef883f894d9f19fd5`.
It owns normal117/CHARS119 hosts, two explicit creations with18/28 defaults,
two field Saves, six exact audits/four histories/permanent reservations, untouched
peers, read-only outcome recovery, retained remounts and three gate-off reopens.
Four owners close; other15 files stay exact after writes/16 exact per reopen.
Those mutations were not replayed on this binary. Positive allocation is an
adaptation, not Access random `GenUniqueID()` compatibility. Ordinary Save/Enter
cannot create soil; implicit creation and soil deletion/restoration remain off.

The immutable Summary predecessor remains unchanged, SHA256
`507acfd1dc5ee943c261019a504839cb8287fd0bcfde63c8b3a49d9b9904437c`.
It owns the combined workbook's one publication,13 independently calculated
scientific-name vegetation rows,30 environmental cells against the reviewed
plan, four physical memberships and very-hidden provenance. Environmental
calibration also retains kernel lineage. P1 appears twice; P3 has physical LF1
zero cover, not missing vegetation. Absent master long names produce no invented
ReportSummary unit-name row. Its no-reference publication was not replayed here;
the41/37 successor separately qualified the three reference-based outputs.

The immutable entry-performance predecessor remains unchanged, SHA256
`0de8422f3e7d24d73fe9155ba03b9aefebb3cf3694d3ba09a4e6eb043c9fb76e`.
It owns normal117/CHARS119 layouts, one three-scope Save/three audits, saved
reopening, backend denials and native typing medians8.8/9.4ms. Its immutable
reference-snapshot optimization remains: clean/dirty session view medians fell
98.85/95.72ms to0.65/0.50ms on the captured6,040,765-byte wire. These are inherited
bounded measurements, not fresh timing or initial-load/SQL speedup claims.

Current initial transitive JavaScript is1,057,710 bytes versus5,202,547 before
deferral (79.67% less); total JavaScript is5,390,313. The EXE grows29,184 bytes
versus the saved-label predecessor; no new startup or binary-shrink claim.
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
The qualified preview executable is43,196,928 bytes,18,432 above the Species
predecessor; deferred loading is not a binary-size reduction. An earlier
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

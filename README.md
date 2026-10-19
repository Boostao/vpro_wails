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
`VPRO-Demo-2026-10-18-profile-transfers.zip`, 17,597,033 bytes.

SHA256:
`25403bd9be9dff7c583cc4b762479d106d2a108a8ca39010b4d06a93758f137d`.

Extract the entire archive and run `Launch-Demo.exe`, not `VPRO-Demo.exe`.
Microsoft Edge WebView2 Runtime is required. Every launch creates a fresh retained
disposable run; changes persist in that run, not in the seed or next launch.
This is a private preview, not a signed production installer.

Its embedded `package-manifest.json` pins the exact binary, source-input identity,
67 frontend and 50 runtime opt-ins. Its `DEMO.txt` gives the qualified walkthrough:

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
13. Explicitly select the project-owned **Sample_Profile** source and open P1.
    **Review project profile rules**, then **Review and select project lump table**.
    The original eight rules match no plots. On physical rule8 set Species=LF1,
    Operator=>, Criteria=0 and Operation=Add plots; **Save profile rules** once.
    Preview matches P1/P2, excluding P3's zero cover. **Apply reviewed profile
    navigation**, open P1, use Next/Previous, then Clear profile navigation.
    Ordinary plot Save never saves rules. This is a standalone adaptation, not
    the original mapped Access callbacks or implicit count/filter changes.

14. Forms > **Plot Pictures — Read-only**: select literal P1, **Load linked
    pictures**, select physical row1, preview and open its full-size viewer.
    The manager uses its owned600x360 synthetic image; ordinary P1's linked
    picture panel uses a different owned400x240 image for the same row.
    Invalid directories/names and missing files refuse explicitly, without repair.

15. In P1 FS882, **Find Plot** > enter literal `P2` > **Find in current scope**.
    Review does not navigate. Only **Open found plot** changes the editor,
    rechecking the target after shared Save/Discard/Cancel. Cancel retains an
    unsaved parent draft; explicit Discard permits navigation without saving that
    draft. Empty, case/space variants and missing identifiers refuse; already-open
    targets cannot replace the editor. No record is created.

16. In Forms > **Plot Pictures — Read-only** (or ordinary P1 FS882 > Site &
    Location > Pictures), **Load linked pictures**. Selecting duplicate-ID rows 1
    or 2 (both ID 101) and clicking **Edit selected picture metadata** refuses
    because an unambiguous signed32 Long ID is required. Select row 3 (ID 103),
    click **Edit selected picture metadata**, set Picture Directory to `Default`,
    and click **Save picture metadata only**; row 3 then previews the owned 600x360
    manager image. Empty Picture File Name refuses while NULL Picture File Name and
    empty/NULL Picture Directory are allowed. **Discard draft and reload picture**
    restores stored values without undoing committed Saves. If a Save response is
    interrupted, **Resolve picture Save receipt** checks durable
    `__VPRO_PictureMetadataHistory` via `LookupReceipt` without repeating the write.
    Complete two-page picture children remain read-only.

17. In Forms > **Plot Pictures — Read-only**, select literal `P1` and **Load
    linked pictures**. Select a row with a unique signed32 Long ID (such as row 3,
    ID 103 or row 8, ID 108) and click **Review deletion of selected picture
    reference**. Confirm the checkbox and click **Delete reviewed picture
    reference**, then **Acknowledge committed picture receipt**. The reference is
    removed from `tblVPics`, its ID is permanently reserved in
    `__VPRO_PictureDeletionIdentity` and its complete 5-column original is recorded
    in `__VPRO_PictureDeletionHistory`; the image file is untouched. Open
    **Historical picture deletions**, click **Load deletion history**, click
    **Review historical restoration**, confirm and click **Restore historical
    picture reference**, then **Acknowledge committed picture receipt**. If a
    mutation response is interrupted, **Recover picture receipt read-only** queries
    `LookupReceipt` / `LookupRestorationReceipt` without repeating the mutation.
    Ordinary FS882 and complete two-page child panels do not expose deletion.

18. With **Sample_Profile** selected, open P1 and click **Review project profile
    rules**: click **Review new profile rule**, uncheck the NULL boxes, enter
    `Order=10`, `Table=Veg`, `Field=Species`, `Operator=>`, `Layer=Any`,
    `Species=LF1`, `Criteria=2`, `Operation=Add plots`, and click **Create reviewed
    profile rule**. Physical rowid 9 is allocated and permanently reserved in
    `__VPRO_ProfileIdentity` with durable history in `__VPRO_ProfileHistory`. Click
    **Review and select project lump table**, then **Preview saved species-only
    matches** or **Run stored profile preview**. When matched plots have unique
    `SiteUnit` records in the selected SU (such as `P2` in `Report_SU`), click
    **Review Save as SU** to publish a new external SQLite SU file (**Create
    reviewed SU file**) or click **Review Save as SU in current project file** to
    create a new `*_SU` table in the owned project (**Create reviewed SU in project
    file**), each with `__VPRO_ProfileSUHistory` provenance. Click **Review deletion
    of rule 9** and **Confirm profile rule deletion** to remove rule 9 while keeping
    rowid 9 permanently reserved. In FS882 with an active SU selected (such as
    `ProfileProj`), use **SU Into Env** to review and copy selected SU values into
    `Sample_Admin.UserSiteUnit` (`__VPRO_SUEnvironmentHistory`), and **Env Into SU**
    to review and copy `Sample_Admin.UserSiteUnit` values into the selected `*_SU`
    table (`__VPRO_EnvironmentSUHistory`).

Binary source is clean `c760425`. Seven existing frontend flags
(`PROFILE_SPECIES_PRESENCE_PREVIEW`, `PROJECT_PLOT_PROFILE_CREATION`,
`PROJECT_PLOT_PROFILE_DELETION`, `PROJECT_PLOT_PROFILE_SAVE_SU`,
`PROJECT_PLOT_PROFILE_SAVE_SU_PROJECT`, `SOURCE_ENV_SU_TRANSFER`,
`SOURCE_SU_ENV_TRANSFER`) and one existing runtime flag
(`PROFILE_SPECIES_PRESENCE_PREVIEW`) extend this private profile (67 frontend /
50 runtime flags); production app, Go backend, bindings, defaults and all 19 seed
files remain unchanged. Frozen full-root race integration
(`ce0dbd2d25dd266944ed31536267b24b53d234c178c35da63ff180537cccb7f0`, exit 0 in
4878.25s) is inherited because no production Go/C/frontend file changed.

Current native acceptance across three closed owners (PIDs 456, 7840, 2028)
verifies rule 9 creation, edition (`Layer=Any`, `Criteria=2`) and deletion on
`Sample_Profile` with permanent reservations `[1..9]` in `__VPRO_ProfileIdentity`
and 4 durable records in `__VPRO_ProfileHistory`; species-only presence preview
missing-Lump refusal and readonly Table/Species intersection after explicit Lump
review; ordered preview NULL-Layer refusal and 9-step `P2` match; Save as SU
duplicate-plot refusal on `Report_SU` when `P1` is matched twice plus verified
creation of both external `ProfileExportSU.db` (`ProfileExt_SU`) and in-project
`ProfileProj_SU` with `__VPRO_ProfileSUHistory` provenance; and owned-project
**SU Into Env** (`__VPRO_SUEnvironmentHistory`) and **Env Into SU**
(`__VPRO_EnvironmentSUHistory`) transfers on `P2` plus complete-entry
`UserSiteUnit` empty-string refusal and explicit `Clear to NULL` save. Four panel
families are measured at 980/1400, and two fresh specific `CancelClose`
acknowledgements precede mutations. Reopen with
`VPRO_PROFILE_SPECIES_PRESENCE_PREVIEW=false` and `None` profile verifies
persisted tables, external SU file hash (`b7a791007d3e21dfc09f00a67235a7c37346827979f715fe2b709de073431f9f`)
and 4 backend denials while ordinary 117-field entry remains available. All 17
non-project, non-config run files stay byte-exact. 37 combined client cases,
launcher races, focused Go profile/transfer race 83.25s, check 0/0, isolated
Vite/Go/exact 26 embeds, independent review (`gemini-4-argon`), all 23 ZIP
payload hashes and final prepare-only pass. Native acceptance is not first-pass:
attempt 1 (PID 456) closed cleanly before any mutation to add
`VPRO_PROFILE_SPECIES_PRESENCE_PREVIEW` to the launcher, and same-owner
continuations on PIDs 7840 and 2028 were retained without replaying any mutation.
External profile write grants, external profile file/table creation and original
Access callbacks remain unqualified.

The immutable60/49 picture-lifecycle predecessor, SHA256
`b64ea45f46c56f7f8ada1d8a2aa346bdfe1ae517452b5c4f732fcb9e79332384`,
owns Picture Manager reference deletion and historical restoration on P1 (rows 3
and 8, permanent ID reservations `[103, 108]` in `__VPRO_PictureDeletionIdentity`,
two records with `Restored` provenance in `__VPRO_PictureDeletionHistory`),
duplicate-ID 101 refusal, unconfirmed review and unacknowledged receipt close
barriers, `Discard uncommitted review`, held-Delete `LookupReceipt` recovery and
read-only `LookupRestorationReceipt` verification, absence of child-panel deletion
controls and 18 reopen denials across four closed owners. Those picture-lifecycle
mutations were not replayed on 67/50.

The immutable58/47 picture-metadata predecessor, SHA256
`8f79b91415a46e62d08e9c88722bb7dda946fe48d22b4ad5637fe96a18cf27b8`,
owns Picture Manager and ordinary FS882 child `PicDir`/`PicName` editing,
duplicate-ID 101 and empty/256-unit `PicName` refusals, `Discard draft and reload
picture`, three exact Saves (`__VPRO_PictureMetadataHistory` IDs 1..3, zero
`AuditTrail` changes), held-Save `LookupReceipt` recovery without replay, tab
remount retention, two-page picture child read-only preservation and six reopen
denials across three closed owners. Those metadata mutations were not replayed
on 60/49.

The immutable57/46 Find predecessor, SHA256
`8e0874cf0f6165422661fc08cd5d110aca6e8cf136ff55a58f41876b5b781c4d`,
owns strict literal scoped Find and revalidated Open (`typeof(...)='text'` and
`CAST(... AS BLOB)` in `Env`, `Admin` and `USysEnv`), first-pass P2 review without
leaving P1, five literal refusals, already-open target refusal, dirty Cancel/
Discard, revalidated P2/P1 navigation, held-lookup `CancelClose` guard, 18
independent affinity cases, 90-minute full-root race integration and one closed
owner with all 19 files exact. Those Find steps were not replayed on 58/47.

The immutable56/46 picture predecessor, SHA256
`cf11508f34a5af0aab0f1943f4115be07fefccecfbc372149362122579a7e775`,
owns Picture Manager and ordinary linked-child selection, independent600x360 and
400x240 synthetic PNG bytes/hashes/dimensions, duplicate-ID/NULL/empty metadata,
invalid directory/filename/missing-file/foreign-parent refusals, held-image
Cancel/late-remount rejection, reading-off reopen denials and two closed owners
with all19 files exact. A later independently reviewed supplement on that same
56/46 binary qualifies Normal117/CHARS119 complete-entry picture children and
held-read source/close/remount guards across two additional closed owners; those
picture journeys were not replayed on58/47. Picture Add/lifecycle, original
callbacks and registry discovery remain unqualified.

The immutable54/45 profile predecessor, SHA256
`6034be31ab38ee2ed5dc593409a366fdb791ef3d5f3414b0e0a6775b747dcea5`,
owns one four-text rule8 Save/one history/zero source audits, preserved389 audits/
counts/Description, explicit78-row lump, zero then P1/P2 matches, eight steps,
actual Apply/Next/Previous/Clear, invalid-draft/held-response guards and saved
source-authority reopen. Its29 clients/coupled race74.402s and two-owner native
proof are inherited, not replayed on57/46. Profile lifecycle/external grants/
Save as SU/species-only/subvarieties and original mapped callbacks remain off.

The immutable49/45 coordinate/KML predecessor remains unchanged, SHA256
`3e09db767075a2f194b542ca60ade4b22e075231140aa36174b4a248ba08f6de`.
It owns the coordinate Save/two audits/one history, DISTINCT Location1 versus
direct Env/SU2 rows/KML2 placemarks, independent XML/hash reopening, held
review/export/receipt guards, six denials/four saved reopens and five closed
owners. Those mutations/publication were not repeated on56/46. KML has no
PDF-style readonly recovery API; viewer/preferences/DMS callbacks remain off.

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

Current initial transitive JavaScript is1,126,518 bytes versus5,202,547 before
deferral (78.35% less); total JavaScript is5,461,565. The EXE grows6,144 bytes
versus the picture-lifecycle predecessor; no new startup or binary-shrink claim.
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
The qualified preview executable is43,301,888 bytes,6,144 above the
picture-lifecycle predecessor; deferred loading is not a binary-size reduction. An earlier
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

Do not overlap root Go suites. The latest accepted run took about 81 minutes;
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

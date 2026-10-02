# FS882-6x4XL Access source contract and probe

This is a source trace plus limited native evidence, not proof of complete write parity. All Access mutations below occurred only in ignored disposable copies. The Wails prototype now exposes experimental writes; do not use them on production data.

## Fixture and isolation

- Access source database: [VPro64.accdb](../../VPRO_ACCESS/VPro64/VPro64.accdb), copied from the local `VPRO_ACCESS` checkout. The source file SHA-256 was `01481B94569C7172F32A812E65365F78CEA5E440DD63220D91A37DADE5778423`.
- The copy was opened with Access 16.0. DAO reported zero linked TableDefs. The local `Sample_Env`, `Sample_Admin`, and other Sample tables are in the same file, so one full-file copy isolated this probe.
- Project: `Sample`. Registry settings under `HKCU\Software\VB and VBA Program Settings\VPro64` were snapshotted before opening the copy and restored after Access closed. The original Access file was not edited.
- The initial Sample row counts were Env 52, Admin 52, Audit 389, Veg 1633, Humus 65, Mineral 99, and Other 1. `AuditStrength` was 1. `Sample_Env.PlotNumber` is required and unique; `Sample_Admin.Plot` is required and unique.
- The database copy remains under ignored `evidence/private/fs882-xl-oracle-20260928/`. Do not commit the database copy.
- That first fixture was superseded by `evidence/private/fs882-xl-manual-20260929/`, which also copies the support `.accda` files beside `VPro64.accdb` (startup fails without them) and carries the tracer, the `Form_Error` hook, and the fixture-only VBA patch described below. All instrumented results in this document come from that fixture. The VPRO registry snapshot is kept with the fixture as `registry-backup.reg` and re-imported after each session.

## Source trace

Sources are the SaveAsText export and R module: [FS882-6x4XL.txt](../../VPRO_ACCESS/VPro64_forAI/Forms/FS882-6x4XL.txt), [V7mdlSetCurrent.txt](../../VPRO_ACCESS/VPro64_forAI/Modules/V7mdlSetCurrent.txt), [V7mdlAudit.txt](../../VPRO_ACCESS/VPro64_forAI/Modules/V7mdlAudit.txt), and [mod_fs882_6x4xl.R](../../vpro/R/mod_fs882_6x4xl.R).

- The parent Access form is bound to `USysEnv`. `SetCurrentProject` creates this query as an inner join of `<Project>_Env` and `<Project>_Admin` on `Env.PlotNumber = Admin.Plot`.
- `PlotNumber_AfterUpdate` calls `DoCmd.RunCommand acCmdSaveRecord`. `PlotNumber_LostFocus` sets a blank `StartDate` to `Year(Now)`, immediately sets it back to `Null`, saves again, and jumps to `MyExit`. The older Admin lookup/insert code below that jump is unreachable in the exported procedure.
- A user-provided native Access/VBA screenshot of run-time error 3314 shows the debugger stopped on the `DoCmd.RunCommand acCmdSaveRecord` statement inside `PlotNumber_LostFocus`. This confirms the failing statement in that captured interaction; it does not independently establish row deltas or what happened after dismissing the error.
- Parent `Form_BeforeUpdate` calls `AuditTrail Me`. The Save button also calls `acCmdSaveRecord`. The Restore button calls `RestoreAuditRecords Me.PlotNumber` and requeries the audit subform.
- XL child controls use these forms and record sources: `SubVegAXL_BC` / `USysVegA`, `SubVegCXL` / `USysVegC`, `SubVegDXL` / `USysVegD`, the A/C height forms / filtered queries over `UsysVeg`, `USysVegOtherXL` / `USysVegOther`, `SoilHumusXL` / `UsysHumus`, `SoilMineralXL` / `UsysMineral`, `SubOtherXL` / `UsysOther`, and `USysAuditXL` / `UsysAuditTrail`. The XL picture subform uses `tblVPics`. Parent/child links use PlotNumber. Child forms call `AuditTrail Me, , Me.ID` from `Form_BeforeUpdate`.
- `AuditTrail` inserts audit rows through its own Recordset writes. There is no transaction spanning this audit write and the parent/subform record save. `RestoreAuditRecords` has separate row-level restore/error behavior; a bounded native matrix is recorded below.
- The R module's explicit header and child save paths are useful precedents, not proof of Access behavior.

## Instrumented probe harness (2026-09-29)

Earlier observations were made by hand and were partly unreliable. They were replaced with a scripted, repeatable harness that drives the real Access form and records what the VBA engine actually does. Everything runs against the ignored disposable fixture `evidence/private/fs882-xl-manual-20260929/`.

The harness has four parts:

1. **Shift-key startup bypass.** `keybd_event(VK_SHIFT)` is held across `OpenCurrentDatabase`, which suppresses the VPro splash form, the database-relocation modal, and the floating menu. `Application.SetOption "Error Trapping", 2` (break on unhandled errors only) is then required, otherwise Access breaks into the VBE even for trapped errors.
2. **An execution tracer.** A `zzTraceLog` module appends timestamped entries to `trace.log`. A `ZZTrace "<label>"` call is injected as the first statement of nine parent-form handlers plus `V7mdlAudit.AuditTrail`. Each entry records the active control and the form's `Dirty` / `NewRecord` / `PlotNumber` / `Plot` state.
3. **A `Form_Error` hook.** `Private Sub Form_Error(DataErr, Response)` logs `DataErr` and sets `Response = acDataErrContinue`, so Jet engine errors are recorded instead of blocking on a modal. The form's `OnError` property must be `[Event Procedure]`; injecting the procedure alone does nothing.
4. **VBA-side scenario functions.** Each scenario runs as a single `Application.Run` call. Crossing the COM boundary while a save is failing makes Access reject every subsequent Automation call with `0x800A9C68` and silently break into the VBE, so an interaction that can fail must complete entirely inside VBA.

Screenshots use `PrintWindow(hWnd, hdc, PW_RENDERFULLCONTENT)` against the Access window, which works even when the window is occluded.

### Fixture-only VBA patch

The shipped `PlotNumber_LostFocus` calls `DoCmd.RunCommand acCmdSaveRecord` while `Admin.Plot` is still Null, which raises error 3314 and blocks all further probing. The fixture's copy was patched to exit early on a blank `PlotNumber`, set `Me!Plot = Me!PlotNumber` when `Plot` is Null, and trap save errors:

```vba
Private Sub PlotNumber_LostFocus()
    On Error GoTo LFErr
    If Len(Nz(Me!PlotNumber, "")) = 0 Then Exit Sub
    If IsNull(Me!Plot) Then Me!Plot = Me!PlotNumber
    DoCmd.RunCommand acCmdSaveRecord
    Exit Sub
LFErr:
    ZZTraceMsg "LostFocus TRAPPED err=" & Err.Number & " " & Err.Description
End Sub
```

This patch exists only in the disposable fixture. Canonical Access sources were not modified. The row deltas, audit behavior, and engine error codes below are Jet/Access engine behavior and are parity-relevant; the 3314 avoidance itself is an artifact of the patch.

## Verified event ordering

Opening the form produces `Form_Open` -> `Form_Load` -> `Form_Current` -> `Form_Current`. `Form_Current` fires twice because `Form_Load` calls it explicitly. Moving to a new record fires `Form_Current` again with `NewRecord = True` and both `PlotNumber` and `Plot` Null.

Leaving `PlotNumber` while the record is dirty fires `PlotNumber_LostFocus` -> `Form_BeforeUpdate` -> `AuditTrail` -> commit. `AuditTrail` therefore runs *before* the row is committed, and it is not part of the commit.

## Verified row deltas

Seven scenarios, each on the same fixture, measured as deltas of `Sample_Env` / `Sample_Admin` / `Sample_Audit`.

| # | Scenario | AuditStrength | Env | Admin | Audit | Result |
| --- | --- | ---: | ---: | ---: | ---: | --- |
| a1 | New plot, then `Elevation = 333` | 1 | +1 | +1 | +3 | Committed; no error. |
| a2 | New plot, then `Elevation = 444` | 2 | +1 | +1 | +4 | Committed; no error. |
| a3 | New plot reusing an existing PlotNumber | 1 | 0 | 0 | **+9** | Error 3022; record stayed dirty. |
| a4 | Edit `Elevation` 333 -> 777 on an existing plot | 1 | 0 | 0 | +4 | Committed. |
| a5 | Edit `Elevation` 777 -> 888 on an existing plot | 2 | 0 | 0 | +4 | Committed. |
| a6 | Type a PlotNumber on a new record, then `Undo` | 1 | 0 | 0 | 0 | Fully discarded. |
| a7 | Type a PlotNumber, then `DoCmd.Close acForm, ..., acSaveNo` | 1 | **+1** | **+1** | +3 | **Committed anyway.** |

### Insert is a single atomic multi-table write

`USysEnv` is `SELECT DISTINCTROW Env.*, Admin.* FROM Env INNER JOIN Admin ON Env.PlotNumber = Admin.Plot`. `DISTINCTROW` makes the join insertable, so one save creates the `_Env` row and the `_Admin` row together. There is no separate Admin insert step; the dead code after `GoTo MyExit` in the shipped `PlotNumber_LostFocus` that opened a DAO recordset on `<Project>_Admin` is an obsolete two-step version of the same outcome.

The commit happens on the **first** `PlotNumber_LostFocus`, not on the explicit Save. In a1 and a2, `Elevation` was typed *after* the row already existed and was folded into a second save that changed no row counts.

### Audit is gated by AuditStrength, and is not transactional

`AuditTrail` (in `V7mdlAudit`) writes rows through its own DAO recordset from `Form_BeforeUpdate`. It skips controls named `PlotNumber` and `ID`, and branches on `clsVProReg.AuditStrength` (registry `HKCU\Software\VB and VBA Program Settings\VPro64\Audit\AuditStrength`, default 1): `>= 1` audits edits of existing non-null values, `>= 2` also audits newly entered data, `= 3` also audits deletions.

- At strength 1 an edit writes a row carrying both `BeforeEdit` and `AfterEdit`, e.g. `EditField=Elevation; BeforeEdit=333; AfterEdit=777`.
- At strength 2 new data writes a row with `AfterEdit` and no `BeforeEdit`, e.g. `EditField=Elevation; AfterEdit=444`.
- `Table` is stored as the bare suffix `_Env`, not the project-qualified table name.

**Every save writes three phantom rows** for `Check369`, `Check371`, and `Check373` with no `BeforeEdit` and no `AfterEdit`. These are unbound/default-valued checkbox controls that Access reports as changed on each update. They account for the `+3` baseline in every row above and are noise, not user intent.

The important defect: in a3 the insert failed with 3022, yet audit grew by nine rows. `AuditTrail` had already written six rows during the refused `LostFocus` save and three more during the refused explicit save, while `Env` and `Admin` stayed unchanged. **Audit rows are written for records that never commit, and each retry adds more.** Over the probe session a single never-committed PlotNumber accumulated 53 orphan audit rows.

### Duplicate key

A duplicate `PlotNumber` fails with Jet error **3022** ("...would create duplicate values in the index, primary key, or relationship..."). The form stays dirty and stays on the new record; the user must Undo or correct the value. Without a `Form_Error` handler Access raises this as an unhandled VBA error, and `DoCmd.RunCommand acCmdSaveRecord` then raises 2501, which breaks into the VBE.

### Close does not discard

`acSaveNo` in `DoCmd.Close` refers to the form's *design*, not its data. Closing the form with a dirty new record commits that record (a7). Only an explicit `Form.Undo` discards it (a6). Any desktop "close without saving" affordance must call the equivalent of `Undo` first; mirroring `acSaveNo` would silently save.

### Lock / unlock (`optLockData`)

The `optLockData` option group has toggle buttons 1 (Lock) and 2 (Unlock, default).
When locked (1):
- An explicit save is triggered: `DoCmd.RunCommand acCmdSaveRecord`.
- Editing is disabled across the board by setting `AllowEdits = False` on the parent form and on all data-entry subforms:
  - `Me.AllowEdits = False`
  - `Me.SubVegA.Form.AllowEdits = False`
  - `Me.SubVegC.Form.AllowEdits = False`
  - `Me.SubVegD.Form.AllowEdits = False`
  - `Me.SoilHumus.Form.AllowEdits = False`
  - `Me.SoilMineral.Form.AllowEdits = False`
  - `Me.SubOther.Form.AllowEdits = False`
  - `Me.USysAudit.Form.AllowEdits = False`
When unlocked (2):
- All the above `AllowEdits` properties are set back to `True`.
- This is purely runtime UI-level enforcement in Access (not a database or row-level lock). For the Wails desktop app, a lock state simply puts the UI editor and all child grids into read-only mode and triggers a save of any pending draft.

### Child record identity (`GenUniqueID()`)

Child tables (`_Veg`, `_Humus`, `_Mineral`, `_Other`, etc.) have an `ID` column with Access default expression `GenUniqueID()`.
- Disposable probes observed 32-bit signed LONG values generated on `AddNew` (e.g. `1790718042`, `-393088439`). These observations do not establish the generator's implementation or collision policy.
- Imported Access IDs must be preserved. In the current Sample SQLite schema, Humus, Mineral, and Other IDs are integer primary keys, but Veg ID is an ordinary INTEGER; do not assume every child table has an autoincrement primary key. Allocation, collisions, and identity-based audit restoration need explicit tests.

### Audit restoration (`RestoreAuditRecords`)

Analysis of `RestoreAuditRecords(PlotNumber)` in `V7mdlAudit`:
- Filter: queries `UsysAuditTrail` for `PlotNumber = '<current>' AND Restore = Yes ORDER BY Table, EditWhen DESC`.
- Asks user: `"Remove selected audit record(s) after restore?"` (`vbYesNoCancel`).
  - If Cancel: exits without making changes.
  - If Yes: deletes the audit record (`MyRS.Delete`) as it restores each field.
  - If No: restores the value but leaves the audit trail intact.
- Restoration logic:
  - Updates target table (`_Env`, `_Veg`, `_Humus`, `_Mineral`, `_Other`) matching by `PlotNumber` and (for child tables) `ID`.
  - Sets `RestoreRS(EditField) = MyRS!BeforeEdit`.
  - Special handling for `_Veg`: skips direct restore of every `Cover*` audit. After non-Cover Veg restoration, `CleanVegPlot` prunes rows with NULL Cover1-10/TotalA/TotalB, ignoring heights, Cover5a/b/c and other attributes.
  - Traps error 3314 (`Resume Jump`) to ignore missing required fields during individual field restoration.

## Requirements this places on the Wails implementation

1. Create the `_Env` and `_Admin` rows in **one** SQLite transaction. Do not model them as two sequential inserts.
2. Write audit rows **inside that same transaction** and roll them back with the data. Do not reproduce Access's orphan-audit behavior; record the divergence instead.
3. Do not emit the `Check369` / `Check371` / `Check373` phantom audit rows. Audit only controls whose value actually changed.
4. Map a unique-key collision to the 3022 equivalent, keep the editor dirty and focused on the plot field, and surface a non-blocking error.
5. Respect `AuditStrength` with the same thresholds (1 edits, 2 new data, 3 deletions) and keep skipping `PlotNumber` and `ID`.
6. Treat "close" as an explicit choice between commit and discard. Never inherit Access's implicit commit-on-close.
7. Support locking: when locked, commit pending changes and disable edits across header and child tables.
8. Support audit restoration: allow selective rollback of field values from audit history, with prompt to prune or retain the restored audit entries.

## Still unverified

- First insert and failure in each XL child grid via native UI interaction, and `AuditTrail Me, , Me.ID` verification on live child forms.
- Switching the selected record while dirty, and the deletion path at `AuditStrength = 3`.
- Complete Access-equivalent workflows, authorization, and native UI behavior. Stored-binding coverage and transactional rollback are tested; specialized workflows and source child runtime behavior remain incomplete. Exposed experimental writes must remain confined to disposable project copies.
- Multi-row restoration ordering/partial failure in native Access and complete native Wails restoration UX. The bounded native matrix below covers single-row cancellation, retain/prune, invalid Admin paths, NULLs and destructive cleanup; it is not complete restoration parity.

## Verified bounded desktop header implementation

The header DTO has all 98 unique stored parent properties (88 Env, 10 Admin) and one unbound runtime lock. Stored properties use verified aliases, active-schema capabilities, and one transaction spanning both tables and audit. The original 30 properties have verified writable Site controls; the other fields remain read-only until their workflows are verified. Complete storage mapping does not establish complete form parity.

- UTM easting/northing and slope are nullable fractional numbers; StartDate is a nullable integer year, not a calendar-date string. SiteNotes and OfficeNotes map to Env and Admin respectively.
- Create rejects an existing identity and Update rejects a missing identity. Failures preserve the editable draft and do not change data/audit. Unsupported supplied values and nonfinite numbers fail explicitly.
- Audit thresholds follow the traced source: changed nonnull values at strength >=1, null-to-value additions at >=2, value-to-null deletion only at 3. Unchanged fields and identities are not audited. Data and audit roll back together, intentionally fixing the Access orphan-audit behavior.
- Native all-field tests on disposable SQLite data verified new/existing Undo, four real reference pickers, Save, fractional values, null clearing and one expected level-1 audit. SQL inspection independently checked all 30 persisted values. Earlier native tests also verified collision/invalid-year rejection and persisted reload.
- The Site renderer consumes extracted parent/attached-label geometry and preserves relative paper positions. PlotNumber is x825/y18 within a 951x564 CSS-pixel page; SiteNotes is x9/y468 and OfficeNotes starts at y522. Missing explicit UTM widths derive from cached right edges minus Left (1440 twips = 96 pixels). No ancestor offset is added for attached labels. Static source properties are not proof of runtime VBA visibility, lookups, or calculations.
- Source-only actions remain disabled, except verified cover/height viewing and bounded height drafts, selective restoration and safe coordinate modes. Vegetation, Veg Other, Soil/Terrain and Other render embedded source layouts, including FormHeader and Detail labels. Main native-window Save/Discard/Cancel is verified; richer context switching, height insertion/deletion/species, UTM conversion and Cover restoration are not complete.

## Verified bounded desktop child implementation

- All nonidentity stored XL bindings are mapped: Veg 31, Humus 12, Mineral 18 and Other 8. Five retained legacy Veg fields bring the writable backend counts to 36/12/18/8. Heights, vegetation attributes, humus structure/abundance, mineral descriptors and auxiliary user fields are nullable typed data, not guessed defaults.
- Vegetation physical writes now enforce exported Species TEXT8 (required),
  Layer TEXT2, Collected TEXT1,21 Single fields, LL/PV signed32 Long and ten
  signed16 Integer attributes. All mapped text tokens are checked before JSON
  repair; unchanged historical assignments are omitted. Fresh restoration uses
  the same physical bounds across accepted table/column aliases. This does not
  enable new controls or replace species membership/event policies. The existing
  bounded height-grid cover rule remains separate; Cover restoration remains
  explicitly unsupported. Focused tests39.230s/full race498.416s and actual
  scoped Wails75 rejected requests/two intended DC audits preserve original data,
  history and support/configuration; evidence/private/native-vegetation-domain.
- USysVegOtherXL now has verified persistent existing-row drafts for LL, AF, DC,
  UT, VI, PV, PG, FFA, Cultural1/2 and Other1/2. Ten canonical-family suggestion
  groups preserve145 nullable/duplicate rows; Cultural2 uses Cultural1, AF is
  a textbox and PV orders by Item (ItemOrder only breaks equal-item ties).
  Source has no exported BeforeUpdate audit event for this form; transactional
  desktop attribute audits are a safety adaptation. Omitted combo properties
  are not inferred: optional raw numeric-code acceptance is a desktop policy.
  Species and row creation/deletion stay unavailable in this workflow.
  Full race504.457s/177 frontend tests/check0/0/builds and actual Wails14 cases/
 18 audits verify multirow Save, NULL, historical omission, hidden errors,
  close/context refusal, Undo, stale rejection, rollback and reference Retry.
  Default-on readonly delivery preserves all fixture bytes with zero audits;
  `VITE_VEGETATION_ATTRIBUTE_EDITING=false` is an explicit read-only opt-out.
  Evidence/private/native-vegetation-attributes; exact eb3a009b... promoted.
- Remaining unverified child controls are read-only. Schema capabilities distinguish unsupported columns from supported SQL NULL. Creation supplies explicit null values for the expanded contract; native Cancel creates no row and Create leaves unspecified numeric data NULL.
- Explicit child Update supports imported zero IDs without invoking compatibility creation. Plot/ID ownership, stale or duplicate identities and NULL Veg identities are guarded. Positive signed32 allocation and a transactionally updated reservation ledger prevent deleted-ID reuse.
- Data changes, deletion and identity-bearing mapped-field audit share one transaction. Thresholds remain >=1 for changed nonnull values, >=2 for null additions and exactly 3 for deletion. Failed operations refresh and remount the displayed rows so rolled-back values are not left visibly accepted.
- `Queries/USysVegOther.txt` selects the attribute columns from `UsysVeg` without a nonnull-attribute predicate. The Veg Other page therefore includes every vegetation row for the current plot. Normal A/C/D grid membership is based on nonnull covers, including zero, and can show the same record in multiple grids.
- The child VBA does not recompute `TotalA`/`TotalB` after a cover edit; desktop edits preserve these independently stored fields.
- Access BOOLEAN values require explicit normalization: true `-1`, false `0`, and NULL remain distinct. Both parent and child readers bypass the SQLite driver's BOOLEAN affinity conversion; writes/audits serialize true as `-1`. Native testing reproduced the former false conversion and verified its fix, legacy-edit preservation, and exact SQL values.
- Native evidence under ignored `evidence/private/expanded-child-parity/` verifies all four child families, new attributes, zero/NULL membership, true/false/NULL flags and creation cancellation. These checks supplement Go full-field/schema/audit/rollback tests; they are not native Access proof of the remaining child reference, addition, metadata or deletion workflows.

## Verified cover/height viewing transition

Source: `FS882-6x4XL.btnCoverAndHeight_Click`, and the inline RecordSource SQL in `SubVegAhtXL` / `SubVegChtXL`. A temporary probe module in the owned disposable visual Access copy set the Vegetation tab and focused the source button; real Space-key toggles triggered its VBA. Screenshots and snapshots were captured before, in height mode, and after returning; the probe module was deleted afterward.

| Observation | Initial | Cover/height | Returned cover |
| --- | ---: | ---: | ---: |
| A/C normal grids visible | true | false | true |
| A/C height grids visible | false | true | false |
| D.Left (twips) | 8370 | 10900 | 8320 |
| VegNotes and lblNotes Width (twips) | 11700 | 14250 | 11700 |
| Toggle value | NULL | -1 | 0 |
| Dirty | false | false | false |
| Env/Admin/Veg/Audit rows | 52/52/1633/389 | unchanged | unchanged |

Desktop viewing preserves these source positions, including the distinct initial/returned D position and the wider content's horizontal scrolling. The action never saves. A-height includes nonnull cover/total fields or Height1-5; C-height still requires nonnull Cover6, including zero. Native Wails tests verify a height-only row appears only in A-height, the zero-cover C row remains, and an exact all-seven-table SQL digest stays unchanged. Existing-row height/cover editing and Collected cycles are verified separately; species and insertion/deletion in those grids remain unavailable.

### Verified shared Collected click

All five active XL vegetation forms export identical Collected_Click branches:
NULL -> C -> V -> NULL, with no branch changing other stored strings. Their
Option Compare Database uses case/width-equivalent C/V values; readonly DAO
evidence on a disposable exact source copy confirms ASCII case/fullwidth
equivalence and rejects accented C. General locale collation is not extrapolated;
the unsupported DAO StrComp expression was not used as event proof.

Desktop buttons stage the source click rather than permit unrestricted text.
Explicit ID/nullable expected/click-count transport preserves imported zero IDs,
empty text and nil-omitting legacy transport. Shared multirow data/audit patches
reject stale ownership and omit unchanged historical assignments. Multiple clicks
audit only the saved final value; a full roundtrip has no audit. Explicit Save/
Undo/Cancel and retained drafts across tabs/views are deliberate desktop lifecycle
adaptations. Source BeforeUpdate audit paths are reused conceptually; unreachable
metadata/duplicate merges are not inherited.

Full race493.062s/180 frontend tests/check0/0/default+opt-out builds and native
Wails13 cases/8 audits verify five live controls, shared row state, NULL, unknown
history, fullwidth clicks, lifecycle, stale rejection and second-row rollback/
retry. All15 tables/32 old audits/schema/support/config/external bytes are retained
apart from exact planned edits. Default readonly delivery proves all bytes
unchanged and zero new audits. `VITE_VEGETATION_COLLECTED_EDITING=false` disables
the controls. Evidence/private/native-vegetation-collected; exact156aeb46... promoted.

### Existing-row species selection and explicit source decisions

All five Species combos explicitly export LimitToList=NotDefault. Source
USysAllSpecies unions five fields from USysAllSpecs/USysUserSpp, excluding S/s.
A/A-height lists accept U/X lifeforms1-4, C/C-height5-8/12 and D1/2/9-11.
Canonical readers preserve NULL/empty metadata and duplicate definitions; the
three source classes return547/4742/3319 rows. Old-code lookup preserves every
matching master definition, including two different ACAROSPO replacements.
Source NotInList asks replace/keep, then checks the user catalogue or opens the
personal-list form. Arbitrary DLookup-first behavior is not inherited: Review
preserves every alias definition and requires explicit replace/keep. Only when
no non-NULL master alias exists does it offer existing personal-code definitions,
including NULL metadata outside dropdown lifeform membership. Unknown-code
personal-list creation stays default-disabled; the existing-row opt-in adaptation
is described below. USysAddSpp's Update button exits.

Existing-row selection/decisions are default-on, not full NotInList parity.
`VITE_VEGETATION_SPECIES_EDITING=false` retains disabled labelled species controls.
Ordinary list matching requires
the literal returned code, without trimming, completing or recasing. Exact-case
matching and explicit Save/Undo/Cancel are adaptations. Malformed/new overlength
text blocks saving while unchanged historical text is omitted. Draft/error state
is shared across cover/height grids and survives tab remounts. Review owns
cancellable reads and checks form/raw entered/original identities before offering
choices. Explicit decisions retain entered/selected identities through Reload and
failed Save; new manual input clears the old decision, not its original value.
Save revalidates reference precedence/identities and independently computes the
source UCase result. Event conversion is bounded to ASCII and explicitly rejects
unverified non-ASCII casing; exact literal non-ASCII list selection is allowed.
Source focus targets Cover1/6/7 differ by form. Desktop drafts intentionally
postpone unrelated cover editing until Save instead of focusing disabled cells.

The shared child transaction checks original species, row ownership and the
source-form row predicate before assignment/audit. Canonical membership uses the
leased readonly family connection: project writer pools intentionally do not
own support aliases. Native8 cases/3 audits cover all five lists, alias ambiguity,
Undo, hidden raw errors, Save/Lock/close, valid correction, wrong-list rejection,
multirow audit rollback/retained retry and stale/raw transport rejection.
All15 tables/32 old audits and support/config bytes are preserved apart from the
planned species edits. Corrected1b7e408e... opt-in core is archived under
evidence/private/native-vegetation-species. Default candidate4eb0b8af... passes
readonly native delivery: five disabled labelled species controls, unchanged
Collected availability, every fixture byte retained and zero new audits. The
preparation full race passes (root472.744s); f286d74 published that bounded scope.
The decision candidate60575340... passes native9 cases/5 intended audits:
explicit replacement and keep, existing personal UNKNOWN1 with NULL metadata,
blocked unknown code, multirow rollback/retained retry and unchanged VUser bytes.
All15 tables/35 prior audits and support/config bytes are preserved apart from
intended species changes. Default delivery passes five enabled labelled controls,
no implicit draft and zero writes; every fixture byte remains unchanged.
Default assets are byte-identical to accepted opt-in assets; the read-only opt-out
build is distinct and archived. Exact60575340... is sealed in the same private
evidence directory and promoted after decision integration race passed
(root517.356s).

### Source cover/height draft boundary

Active AXL_BC/CXL/DXL/AhtXL/ChtXL exports bind25 numeric controls to eleven
cover/total fields and six heights. Every bound cover/total exports
`ValidationRule="<100 Or Is Null"` and InputMask CC. Heights export no individual
conversion events/rules. Strict desktop numeric input deliberately retains
fractions/negative values and raw invalid text instead of mask truncation.
TotalA/B are explicitly entered source fields, not automatically recalculated.

The shared height parser/session now spans all five grids. Per-field original
source forms and expected values accompany changed numbers; the shared child
transaction checks source bindings/row predicates, conflict and physical domains
before mutation/audit. Unchanged historical invalid fields are omitted. Failed
batch Save retains raw partner drafts for retry. Clearing Cover6 removes a row
from C/C-height views but does not delete its vegetation identity. Source A/D
duplicate merge code remains unreachable; inactive metadata writes are not added.

Native core2f8d74bd... verifies25 visible labelled controls, Undo without writes,
hidden Cover9=100 blocking Save/Lock/actual close, multirow cover/height rollback/
retry and NULL view removal (6 cases/4 audits). Default30199c86... delivery verifies
25 enabled controls and one independent Cover2 assignment/audit. Read-only opt-out
verifies25 disabled numeric controls, enabled Species/Collected and zero writes/
all fixture bytes unchanged. All15 tables/40 old audits and support/config bytes
are retained apart from the five intended numeric changes. The old experimental
grid is a read-only preview, and unverified creation/deletion controls are disabled.
`VITE_VEGETATION_NUMBER_EDITING=false` or retained `VITE_HEIGHT_EDITING=false`
disables numbers without enabling a generic write fallback. Final integration
race passes (root479.695s); exact default30199c86... is promoted, with the prior
species60575340... baseline preserved in the private archive.

### Guarded vegetation deletion: verified desktop adaptation

Source deletion is opt-in only with `VITE_VEGETATION_DELETE_EDITING=true`; omitted
AllowDeletions/AllowAdditions and native deletion events remain unmeasured.
The disposable installed-core property probe stopped on the unknown VPro
relocation dialog, without acknowledging it or opening source forms. Macro
automation security did not suppress that startup. The owned process was stopped;
canonical core bytes and the pre-dialog registry Location remain unchanged.
No exact native Access deletion-UX/default-property claim is made.

Review captures every physical column, source identity, rowid and raw typed bytes,
including hidden values and NULL/empty distinctions. Malformed species cannot be
silently repaired for display; unsupported BLOBs reject explicitly. The shared
writer checks source predicate/full fingerprint before deleting, reserving the ID
and auditing mapped plus extra physical fields in one transaction. Flag true
audits as -1. Changing form/ID/fingerprint or any hidden value invalidates review.
Historical non-species text is hashed without JSON decoder repair.

The review persists independently of active tabs. Only its explicit Confirm can
delete; ordinary Save/Save-and-close, Lock and unrelated edits cannot bypass it.
Undo/Cancel reload without writes; failed deletion retains review and its error;
successful deletion followed by failed refresh reports committed state and
disables further editing. A stale review must be cancelled and recreated.

Native Wails coref2b6c939... verifies review/remount/Save/Lock/actual-close guards,
Undo/Cancel, hidden Flag drift, wrong transport, hidden-field audit rollback,
retained retry, removal from all five views and replay rejection. Exactly one
row (-10) and five field audits changed. All15 original tables/45 prior audits,
support/config bytes and temporary schema are preserved; the standard shared
identity-ledger table/index and one reserved ID are the intentional schema/data
adaptation. The initial schema assertion was corrected to include that proven
ledger; completed writes were not replayed. Defaultd8d51c7f... delivery preserves
all fixture bytes and25 enabled numeric controls with deletion disabled.
Focused Go0.933s,187 frontend tests/check0/0/builds and full race488.272s pass.

### Guarded vegetation creation: verified desktop adaptation

`VITE_VEGETATION_CREATE_EDITING=true` enables a separate persistent source-bound
draft. Default delivery leaves creation/deletion disabled; omitted Access addition
properties and complete metadata/events are still unknown. Static insertion
events/default numeric values are absent from active exported forms; absence is
not proof of runtime defaults. Species TEXT8 is required and ID uses GenUniqueID;
there is no inferred Layer or numeric zero assignment.

Explicit form/species/values transport rejects caller identity/parent/Layer,
unknown properties, raw malformed Unicode, wrong source fields, nonfinite values
and Single-domain violations. Species must literally match its canonical source
list, without implicit casing, alias resolution or personal metadata creation.
C/C-height require non-NULL Cover6, including valid zero. A-height accepts explicit
height-only rows that need not enter its cover view. Covers/totals retain source
`<100` rules; heights retain their own physical-domain rules.

The existing child writer owns allocation/reservation, insert and audit/commit.
It independently observes source visibility, ownership, exact species and every
provided numeric/NULL value after insertion and before auditing. Trigger drift
rolls back the entire mutation. Frontend errors/raw input survive tab remounts;
Cancel/Undo make no writes, failures retain drafts, and Save/Lock/native close/
context/unrelated operations share lifecycle gates. Committed refresh failure
clears only the committed draft and disables editing instead of inviting replay.

Native core497c6455... verifies cancellation, raw100/remount/Save/Lock/actual-close
guards, strict transport, audit rollback/retry, C-height explicit Cover6=0 and
A-height-only creation. IDs2/3/4 add seven audits. Independent guard694e7ab2...
rejects species/numeric trigger drift and retries once: ID5 adds two audits.
All15 original tables/50 prior audits, original ID1, support/config and schema
remain unchanged except four new rows, nine audits and shared reservations.
Integer-zero/REAL and tuple/list harness expectations were normalized without
replaying completed creations. The passed25-control summary remains; detailed
core observations were not retained. Defaulta7dd1be8... has its own complete
25-control array, Species/Collected enabled, readonly review and every fixture
byte unchanged before/after clean close, with creation/deletion disabled.
Focused final Go1.261s,191 frontend tests/check0/0/builds and full race478.504s pass.
This initial creation checkpoint did not create user definitions or accept explicit
personal/alias new-row decisions. The later explicit-decision checkpoint below
extends proposals without combining project and user-definition saves.

### Existing-row personal definitions: verified opt-in desktop adaptation

Original VUser exports define Code TEXT8 unique, ScientificName/EnglishName
TEXT255 nullable, signed16 LifeForm, Report SINGLE default1, and nullable
SppNumber LONG/Codetype TEXT1 with no default. USysAddSpp binds four fields,
supplies12 lifeform options, prepopulates sysNewSpp on Open then clears it, and
Close calls DoCmd.Close. Update is inactive. No Codetype='u' assignment or source
audit event is exported; native effective properties/full dialog behavior remain
unmeasured. Do not infer dropdown membership from successful physical storage.

`VITE_PERSONAL_SPECIES_EDITING=true` enables a separate existing-row metadata
draft after unknown-code review. Code uses the bounded verified ASCII UCase
event; names retain literal Unicode/case/spacing with raw255-UTF-16-unit guards.
Explicit NULL switches retain empty names distinctly, source12 lifeforms or NULL
are selectable, and metadata/error survive remounts. Free manual LifeForm entry
is not enabled; the backend retains its separately checked signed16 domain.
Ordinary plot Save/Lock/Save-and-close are blocked during metadata entry.

Explicit definition Save owns only one VUser transaction against original
USysUserSpp/USysAuditTrail. A CreateRecord JSON snapshot is a desktop audit
adaptation, not Access audit/restoration parity. Attached file identity/role,
master/current/usable-old-code/user collisions and readonly VLists are checked.
NULL aliases stay different from non-NULL empty definitions. Bound values,
Report=1/SppNumber=NULL/Codetype=NULL and the actual audit snapshot are observed
after triggers before commit; failure rolls back everything and retains drafts.
Existing WAL mode is unchanged; no attached multi-file write/atomicity is assumed.
Independent reload then stages an existing-personal decision; plot Save remains
separate. Plot Undo deliberately retains a definition the user explicitly saved.
Known committed cleanup/refresh failures disable editing instead of replaying.

Native corec9f12600... verifies seven cases: raw256/remount/Save/Lock/actual-close,
Cancel/Undo, strict transport, user audit rollback/retry, explicit user-only save
and separate project Save. Independent guard verifies255 units, source Lifeform3,
literal name case/spacing, stored metadata/audit drift rollback/retry and Undo.
Two pre-write harness stops (C-row0 lacks Cover6; editor already open) changed
no data or history; visible C-row -9 was used, with no completed write replay.
Exactly two user definitions/two user snapshot audits and one project species
audit change. Original15 project tables/59 old project audits, original user
tables/prior user audit/schema/descriptions and other support/config are preserved.
Defaultbb3652f3... verifies25 enabled numeric controls/Species/Collected, readonly
new-definition lookup, unknown-code review/Undo and every fixture byte unchanged,
with all creation/deletion UI disabled. Core17888/guard3976/default12924 exited.
Focused final race6.224s/scopedGo2.894s,195 frontend tests/final targeted26/check0/0/
builds and full integration507.979s pass. Creating personal metadata directly from
a new source-row proposal and active metadata/calculation event parity remain incomplete.

### New-row species decisions: verified opt-in desktop adaptation

The five active exported NotInList handlers first look up USysAllSpecs.OldCode,
ask replacement/keep, and assign source UCase. Only absent usable master aliases
fall through to USysUserSpp; unknown-code metadata opens USysAddSpp separately.
New-row drafts now reuse the same explicit decision/reference validators as
existing rows, without a phantom editor identity or automatic alias selection.
Duplicate aliases and nullable descriptions remain observable; NULL Code aliases
are unusable, unlike non-NULL empty codes. Source case conversion remains ASCII
bounded, while exact non-ASCII canonical-list selection remains unchanged.

Review captures original form/raw code. Retyping Species removes review/decision;
numeric edits, remounts and failed Save retain it. Strict transport permits only
form/species/values and optional decision/entered/selected provenance. The shared
writer checks the actual current alias/personal reference and independently stored
row after insertion, before project audits/commit. Collision, source-reference
drift, numeric/species drift and audit failures roll back row/history/reservations.
Saved nullable-Codetype personal definitions are reusable without list membership
or another VUser write. At this decision checkpoint, creating personal metadata
from a new-row proposal was unavailable; the separately saved flow below extends
it. Default source creation/deletion/personal-definition flags stay off.

Native core8e2dde81... verifies seven cases, including ambiguous review,
unresolved remount/Save/Lock/actual-close guards, cancellation without writes,
invalid/stale scoped requests, explicit replace/keep/personal creation and retained
audit rollback/retry. New IDs6/7/8 add seven project audits and zero user audits.
All15 original tables/60 prior audits and all original rows, user/reference/config
bytes and schema are preserved apart from those rows/audits/reservations.
Default8e63916d... independently verifies25 labelled enabled numeric controls,
Species/Collected, readonly personal lookup, disabled unfinished creation and
every fixture byte unchanged. Core5568/default9904 exited; inspector9392 closed.
Focused Go2.888s/full race475.761s,197 frontend tests/check0/0/builds pass.
Protected binaries are exact accepted default bytes; prior builds are archived
with hashes. No Access oracle or production/R/Access source writes were used.

### New-row personal metadata: verified independent-save adaptation

The original USysAddSpp bindings/Open transfer/Close path are reused for reviewed
unknown new-row codes when both creation/personal flags are enabled. The draft
source is discriminated: existing row ID/expected value or unique proposal key.
The key is editor identity only and is omitted from both transports. Original
form/raw species context is matched before and after the explicit VUser save;
metadata freezes proposal editing/cancellation and blocks ordinary Save/Lock/close.
First Undo cancels metadata, retaining the untouched proposal; second cancels the
proposal. Separate VUser Save reloads observed metadata then stages a user decision,
without inserting a project row. Proposal Undo retains the deliberately saved
definition, and later project Save is a distinct transaction. Known committed
cleanup/refresh/source-match errors prevent retrying the committed definition.

Native coreccb31517... verifies eight cases: raw256/remount/Save/Lock/actual-close/
Undo, VUser audit rollback/retry, definition-only Save and proposal Undo, separate
project rollback/retry, and retained existing-row metadata identity/cancellation.
Exactly one ZPNROW01 definition with literal scientific name, empty English name,
NULL LifeForm/Codetype and original hidden defaults adds one VUser snapshot audit.
Later ID9 adds only Species/Cover2 project audits. Original project tables/67
prior audits, user tables/three old audits, schemas/descriptions and other support/
config bytes remain intact. Default50866a49... verifies25 numeric controls,
Species/Collected, readonly new metadata, disabled unfinished creation and every
fixture byte unchanged. Core8104/default14528 exited;9392 closed.
Focused Go2.511s/full race488.508s,200 frontend tests/check0/0/builds pass.
Original R/Access/production sources were not written; no source oracle was run.

### C/C-height NULL-cover notices and source focus styling

Exported C/C-height BeforeUpdate handlers warn about NULL Cover6 without setting
Cancel. The desktop emits a nonblocking notice for changed non-NULL Cover6 drafts,
explaining that Save removes both C source-view memberships without deleting the
physical record. Malformed numeric input remains an error, never a NULL assignment.
The notice survives remount/failed Save, clears on original correction/Undo and
appears in successful Save feedback. Focused cells use the existing emerald styling
and a keyboard outline; explicit Save and mutually gated editors remain intentional
adaptations, not automatic Access focus-transfer parity.

Native20a6c3e7... passes seven cases, including actual activeElement/background/
outline observations, visible notices, remount/invalid correction/Undo and audit
rollback/retry. Only ID8 Cover6 changes to NULL with one project audit; its physical
row/species/Height6 persist while C/C-height source views omit it. Original tables,
prior history/reservations, schema and support/config bytes remain intact.
Independent readonly delivery of identical bytes verifies25 labelled numeric
controls, Species/Collected, disabled unfinished workflows and all fixture bytes
unchanged. Owned9884/12208 exited;9392 closed. Focused Go0.752s/full race459.824s,
202 frontend tests/check0/0/default build pass. No native Access oracle was run.

## Verified bounded selective restoration

`SetAuditRestoreSelection` atomically stores a verified selection; `RestoreSelectedAuditRecords` accepts exact string row IDs and cancel/retain/prune actions. The legacy marked-row API remains. Typed mapping covers 97 nonidentity parent and 61 non-Cover child fields. Invalid/foreign/missing rows, duplicate identities, unsupported fields, stale history chains, malformed values and failed triggers abort the entire transaction. Restore ordering follows source Table ascending then EditWhen descending, with descending rowid for deterministic timestamp ties.

Veg Cover-prefixed restoration is explicitly unsupported rather than silently skipped or pruned. Field restoration never automatically deletes vegetation records or modifies the identity ledger. This intentionally fixes the destructive source cleanup behavior below; records can be removed only through a separately confirmed deletion workflow. The public CleanedVegRows result stays compatible and is always zero.

Native Access evidence under ignored `evidence/private/restore-parity/` verified 11 controlled scenarios and 12 real dialog screenshots. The fixture used a new full disposable copy, matched source branches apart from temporary logging, and restored its original module/registry state afterward; canonical hash stayed unchanged.

| Scenario | Native Access observation | Desktop decision |
| --- | --- | --- |
| Env Cancel / No / Yes | Cancel unchanged; No restored200->100 and retained audit; Yes restored and pruned | Explicit cancel/retain/prune; data and history transactional |
| Parent OfficeNotes save/restore | Audit labeled `_Env`; restore failed3265; `_Admin` path failed3078 | Use verified actual Admin storage mapping; invalid source labels fail explicitly |
| Humus ID74001 UpperDepth | Restored12.5->NULL; history retained | Typed nullable restoration and exact child identity |
| Other ID74002 | Restored only the selected ID; ID74003 untouched; history pruned | Exact plot/ID ownership |
| Cover1 restore | Value stayed20; Yes deleted its audit anyway | Explicit unsupported error; never prune skipped history |
| Height1 restore plus cleanup | Veg4->1: height-only, DC-only, Cover5a-only rows deleted; zero-cover survived | No automatic cleanup; preserve every unrelated row |

Go retain/prune tests preserve that exact four-row fixture and existing/absent ledger state; strict typed/stale/foreign/schema/rollback guards remain.

The native Wails UI uses exact string row IDs, per-row availability, explicit selection count and an accessible Cancel/retain/prune dialog. Isolated verification under ignored `evidence/private/restore-ui/` covers IDs9007199254740993 and above, Cancel data/history preservation, parent Admin/boolean and fractional values, child NULLs, stale and prune-trigger rollback, refreshed visible values, cleared selection marks and lock/dirty/loading/context gates. Exact SQL comparisons preserve four vegetation rows and no identity-ledger side effects. Default enablement followed native proof; a frontend build with `VITE_AUDIT_RESTORE=false` disables this workflow.

Cover restoration, multi-row native Access ordering/failure and broader restoration semantics remain incomplete. The desktop does not claim parity with the documented destructive Access defects.

## Native main-window lifecycle

Wails beta.26 closing hooks cancel before frontend approval; `ShouldQuit` also guards application quit. A single exact pending request ID must be confirmed or cancelled. Missing/stale/duplicate approval never authorizes exit. The frontend flushes focused input and either closes a clean editor, denies closing during a pending operation or shows an accessible Save/Discard/Cancel dialog.

Ignored `evidence/private/close-parity/validation.json` records actual WM_CLOSE, repeated requests, Cancel/Discard exact SQL preservation, UPDATE-trigger failed-Save rollback, successful retry with one audit before process exit, invalid integer refusal, raw DOM Undo recovery and preservation of incomplete child entry. An exclusive disposable SQLite lock proved busy denial during a real pending Save. Discard drops only unsubmitted drafts; already completed child edits remain persisted.

This is main-window lifecycle verification, not full multiwindow or context-switch semantics.

## Coordinate oracle and bounded backend

Ignored `evidence/private/coordinate-parity/report.json` contains36 native Access scenarios plus source, screenshot, UI text, stored-value and audit evidence. Modes DD/DM/DMS change visibility and per-user preference only. Source DD retains Double/sign/NULL; DM/DMS updates both axes using Single/Nz, can discard signs, convert an untouched NULL to0 and quantize an untouched coordinate. Invalid text/overflow produce errors13/6, including a captured native End/Debug prompt and non-destructive recovery. Audit strengths0/1/2 and phantom checkbox records were measured.

The desktop intentionally validates one edited axis without changing its partner, preserves float64 and numeric +/- signs, rejects partial/nonfinite inputs and minute/second values outside[0,60). Geographic limits are latitude90/longitude180, with zero subordinate components at endpoints. No source hemisphere convention is assumed. DD/DM/DMS source control/label visibility uses immutable runtime overrides and unchanged geometry, including source `lblLonM` visibility in DM.

`CoordinateService` conversion/decomposition never writes project data. Mode preferences use a separate atomically replaced per-user `coordinate-settings.json`, defaulting to DD only when absent and reporting malformed files/write failures. A numeric-sign toolbar outside the source canvas is an intentional safety addition; source inputs, radios and labels retain their positions/fonts. Explicit runtime-visible attached labels bypass only a hidden textbox/combo owner, never page/form visibility.

Ignored `evidence/private/coordinate-native/validation.json` records native mode/view no-write SQL digests, bit-exact signed Double values and unchanged partner, NULL partner preservation/complete clear, actual Save rollback/retry, mode persistence across process restart, partial/range refusal, corrected validation alerts, Undo, lock and missing-schema Pending. Native default-enable proof saved latitude-0.875 with one audit and untouched longitude180. Editing defaults on only after this proof; `VITE_COORDINATE_EDITING=false` disables it at build time. Header writable coverage is now32 of98 stored fields.

## BEC classification oracle and verified bounded workflow

Ignored `evidence/private/bec-parity/report.json`, `matrix.json` and `reference-shape.json` record28 Access scenarios with runtime control text, screenshots and exact Env/Admin/audit snapshots. The verified fields are `Env.Zone`, `Env.SubZone` and `Env.SiteSeries`, with nullable destination lengths4/8/5. There are no separate stored Variant/Phase selections in this workflow. `BECSiteUnit` is a separate master workflow and is excluded.

- Zone uses distinct zone/description pairs. SubZone focus refreshes by Zone, or all zones when NULL. Source `LimitToList=False` accepts unknown length-valid codes, preserving raw case and leading zeros.
- Changing/clearing a parent preserves existing dependent codes; NULL parents leave the source SiteSeries list stale. SiteSeries's `FindNext` loop skips the first match, duplicates the last and yields an unrelated first record on no-match. Apostrophes can raise source SQL error3075.
- The authoritative catalogue has281 zone/subzone rows and3,532 SiteSeries rows, including983 duplicate code keys. Selecting a code cannot imply a uniquely selected definition; preserve and expose all matching metadata.
- Source audit0/1/2 semantics were observed, including phantom option-control entries and a false committed classification audit after a fixture-rejected save. Desktop data/history must remain atomic; the forced table validation rule is test instrumentation, not a claim about a production constraint.

The source-positioned desktop controls refresh parameterized choices without automatically clearing stored codes, expose duplicate definitions and require explicit acknowledgement when a changed classification is unmatched/incomplete or its catalogue cannot be checked. Blank entry means NULL. Historical unknown values do not block unrelated edits; unchanged BEC columns are omitted from UPDATE. Lengths count UTF-16 units, matching Access. A frozen separate `bec.db` preserves all source metadata with source/snapshot/database checksums; all96,769 native export cells were independently checked. Its73 empty SiteSeries codes remain stored but explicitly unselectable.

Ignored `evidence/private/bec-native/validation.json` records actual Wails source positions (Zone9/270, SubZone87/270, SiteSeries153/270.2 CSS pixels), code/NULL/case preservation, duplicate metadata, no-match/NULL list refresh, acknowledgement across initial-load/tab remount, source bounds, apostrophe-safe lookup/save, rejected-save rollback/retry, unrelated imported-code preservation, lock/schema Pending, real catalogue SQL failure/retry and native WM_CLOSE invalid-save refusal/Cancel. Independent baseline reconstruction verifies all15 project tables: only intended classification/notes changes and the exact five expected audits remain. Default enablement was verified by actual native Save, not browser preview.

`VITE_BEC_EDITING=false` disables new Zone/SubZone edits while retaining prior SiteSeries text editing. Verified writable parent coverage is35 of98; SiteSeries was already in the original30. BEC Master editing and reverse/bulk unit copying remain disabled; ordinary Working Unit and bounded height editing are verified below.

## Verified bounded Working Unit workflow

Ignored `evidence/private/working-unit-source/contract.json` traces the controls, row sources and directly coupled procedures with14 unchanged canonical-source hashes. The ordinary path is now augmented by59 measured native cases under ignored `evidence/private/working-unit-native/report.json`; the reverse shortcut remains source-only and excluded.

- `btnCoptToWorkingUnit_Click` only assigns `Me.UserSiteUnit = Me.BECSiteUnit`: a nullable field-value copy into the current plot's **Admin.UserSiteUnit**, not copying plots, SU tables, master records or projects. It can overwrite/clear a target without warning and contains no explicit save or transaction.
- Working Unit modes choose existing active-project Admin values, a separate MasterSiteUnitList catalogue, or distinct values across the selected SU table. They change per-user preference/choices, not plot fields. This master catalogue is not the frozen BEC classification catalogue.
- Right-mousedown separately assigns a selected SU value into **Admin.BECSiteUnit**. It lacks explicit NoMatch, duplicate-identity, escaped-criteria and authorization checks. Whether a subsequent ordinary click also fires remains unverified; this shortcut stays disabled.
- Source audit labels both physically Admin fields `_Env`. The desktop retains its actual Admin mapping and atomic data/history policy; the hard-coded source user-name unlock is not an authorization contract.
- `InsertSuIntoEnv` is a different bulk action and must never be substituted for this field copy.

The59-case oracle verifies BoundColumn1, ColumnCount3, LimitToListFalse, nullable Admin.UserSiteUnit TEXT100 with zero-length rejection, manual unknown/apostrophe/leading-zero entry, staged overwrite/NULL/same-value copy, Undo, physical activation/typing, mode preferences and no-SU Master fallback. Dirty drafts survive source mode changes without project data writes. Mode restart was not independently measured in Access; desktop restart has now been verified in independent native processes.

Readonly native reference export retains3,504 Master and4 User rows, all five typed metadata fields, signed-string identities and34 duplicate Master codes. Master choices filter Level11 directly; personal rows must not be unioned into this mode. Env choices span the project's Env/Admin join, while SU choices span the entire selected authorized table, not only current plot membership. NULL/empty reference codes are retained but not offered as editable blank choices.

Source failed/canceled saves commit false UserSiteUnit history at levels1-3, and every source save can emit three phantom option-control history rows even at level0. The desktop must couple real Admin data/history in one transaction, correctly label `_Admin`, omit unchanged fields and never audit preference controls. Ordinary programmatic copy also bypasses source AllowEdits; the desktop intentionally applies lock/loading/pending gates and inline overwrite/NULL-clear confirmation.

Desktop editing now defaults enabled after actual Wails proof; `VITE_WORKING_UNIT_EDITING=false` disables edits/copy/modes. The source-positioned UserSiteUnit control is x201/y18/width174 CSS pixels after subtracting the existing Site-page origin from source absolute twips. It stages only UserSiteUnit, requires explicit acknowledgement of newly unmatched codes and preserves codes during choice changes. BECSiteUnit remains read-only; no reverse assignment, bulk copy, external attachment or Master write is enabled.

Ignored `evidence/private/working-unit-wails/validation.json` records actual no-write staging/cancellation, overwrite and NULL confirmation, code acknowledgement across tab remount, stable Env mode across tabs/Undo, duplicate descriptions/scientific metadata/signed-string identity, raw101 refusal, Lock, real Admin rejection/rollback/retry, exact100 UTF-16 Unicode/leading-zero/apostrophe saves, NULL insertion/clear, historical101 preservation on unrelated Save, genuine catalogue error/retry, missing-schema Pending, whole authorized SU including a nonmember, independent process preference restart and native invalid/valid Save-and-close. New-form native collision leaves all15 tables unchanged; actual Create adds only the expected Env/Admin rows and preserves NULL BEC Master.

The packaged reference has3,504 Master and4 retained User rows, independently checked across all17,540 typed cells. A per-user checksum mismatch fails explicitly without overwrite. Independent captured-seed verification checks all15 project tables, preserves every partner/reference/child/metadata field and expects exactly five real `_Admin` audits. Native default strength1 deliberately omits NULL addition/clear history; focused Go tests and the source matrix cover strengths0-3. No phantom checkbox/preference history or false rejected-save audit is reproduced. Parent writable coverage is now35; this remains bounded FS882 parity, not complete application migration.

## Verified bounded Data Quality workflow

The ignored `evidence/private/data-quality-native/report.json` records84 measured native cases,95 attempts including preparation, and zero integrity errors. Owned disposable copies, registry/options and instrumentation were restored and closed; canonical files and the protected Access process were untouched. The readonly `plot-quality-site.reference.json` captures all50 typed cells across five rows and ten columns.

SitePlotQuality, VegPlotQuality and SoilPlotQuality are nullable **Admin TEXT(15)** with no stored zero-length strings. All three use the same PlotQualitySite row source, ordered by ItemOrder: **NA, Excellent, Good, Fair, Poor**. NA is literal text, not NULL. Note letters E/G/F/P are metadata, not bound codes. Unknown/manual/apostrophe values are permitted and raw case is stored. No scientific metadata or field-specific update handlers exist; parent BeforeUpdate calls AuditTrail.

Source audit thresholds are edit>=1, addition>=2, clear=3, unchanged none. Failed/canceled saves leave false history, retries duplicate it, option controls can generate phantom audits and physically Admin changes are mislabeled `_Env`. Case-only quality edits store the new case without a quality audit. Keyboard overlength input truncates;14 ASCII characters plus an emoji can leave a lone high surrogate instead of valid Unicode. Source Lock blocks editing, but coupled focus handling can raise SaveRecord2046.

The bounded desktop implementation deliberately preserves raw nullable codes and metadata, rejects new over15-unit or malformed UTF-16 values without truncation/replacement, and keeps unchanged historical invalid-length fields out of unrelated UPDATE statements. Raw case-only changes use the existing exact-value audit policy: this is an intentional history correction, not literal reproduction of Access's omission. Blank editor input means NULL. All three changes share the existing atomic Env/Admin/audit Save; no Metadata, SpeciesListComplete, reference, checkbox or child writes are intended.

Source-positioned controls remain x51/y384,402,420/width96 CSS pixels within the Site page. Editing defaults enabled after actual Wails proof; `VITE_QUALITY_EDITING=false` disables it. Inputs retain the entire raw entry instead of HTML maxlength truncation. Every JSON quality-key occurrence is checked case-insensitively before Go's decoder can replace an unpaired surrogate; valid pairs, literal backslash-u text and legitimate U+FFFD are preserved. Direct Go and selective-restore inputs share Unicode/15-unit guards.

Ignored `evidence/private/data-quality-wails/validation.json` records actual native source geometry and five literal choices, all three fields' twelve overlength/malformed attempts, affected-field error markings, NULL draft/clear/literal NA insertion, tab/Undo/acknowledgement preservation, semantic no-op and lock/loading gates. A genuine Admin rejection retains all drafts and rolls back all15 tables/history; retry stores exact case-only,15-unit Unicode and apostrophe values with three true `_Admin` audits. Historical16-unit text survives an unrelated Save. Genuine catalogue failure clears stale choices and Retry recovers all five; missing SoilPlotQuality is explicitly Pending without changing partners.

Actual WM_CLOSE refuses malformed Save-and-close, focuses Cancel and retains raw code units; valid Save-and-close commits one exact audit before exit. A default-build ordinary Save adds the fifth audit. Independent captured-seed verification checks every row in all15 tables, including partners, children, SU and Metadata. Native default strength1 intentionally emits no NULL-addition/clear audit; Go/source tests cover strengths0-3. Separate native new-form collision/Create leaves existing tables/history unchanged and adds only expected Env/Admin rows with three quality codes and the pre-existing unmapped SV_FloodPlain FALSE schema default.

Frozen `resources/plot-quality-site.db` independently preserves all50 typed source cells, nullable Boolean metadata and exact IEEE64 ItemOrder values. Per-user checksum conflicts fail without overwrite.75 frontend tests, clean Svelte check/build and full Go race pass; generated bindings cover8 services/59 methods/26 models. The inspector-off normal app is launched from an independent disposable profile. Verified writable parent coverage is now38 of98, not full FS882/application parity.

## Verified bounded substrate workflow

Ignored `evidence/private/substrate-native/native-contract.json` reconciles120 measured cases with zero integrity errors. All six Env fields are nullable SINGLE, with no effective input mask, format, default, validation rule or field-specific events. Real keyboard negative/zero/fractional/99/100/101/NULL values are accepted. Combined sums60/100/120 save without balancing or partner changes; neither a percentage-looking caption nor an R precedent establishes a0-100 rule.

Source Single storage quantizes `1.234567890123` to `1.2345678806304932`; source audit displays `1.234568`. MaxFloat32 is accepted and3.5E38/NaN/Infinity rejected. Dirty tab roundtrips remain staged until Save; Undo preserves stored data and Lock blocks changes. Coupled legacy focus handling can raise SaveRecord2046. Source audits use edit>=1/add>=2/clear=3, unchanged none; phantom checkbox, orphan failed/cancelled and duplicate-retry history remain source defects.

The native-verified desktop preserves float64 within the finite Single range, intentionally following the verified height precision adaptation. It keeps nullable, negative and above100 values without balancing; finite historical overflow can remain unchanged on unrelated saves, but new overflow/nonfinite values are rejected. Lexical partial/invalid entries are retained across Site remounts and block Save/Lock/native close. Shared numeric parsing retains height's separate `<100` cover rule; it is not applied to substrate fields.

Source controls occupy two columns x459/603, rows y384/402/420, widths65.8/59.8 CSS pixels, centered Arial10. Editing defaults enabled after actual Wails proof; `VITE_SUBSTRATE_EDITING=false` disables it.83 frontend tests, clean Svelte check/build and full Go race pass; API/DTO/services/bindings stay unchanged. Verified writable coverage is now44 of98, not full FS882/application parity.

Ignored `evidence/private/substrate-wails/validation.json` records36 valid and54 invalid raw cases across all six fields, staged sum120 without balancing, invalid partial text across tabs, hidden-invalid Lock refusal, Undo, ordinary Lock and lexical no-op. Genuine data and audit failures retain raw drafts and roll back all15 tables/history. Retry stores exact1.234567890123 float64 precision, negative/fractional/100/101/zero values with six true `_Env` audits; NULL clearing and100 insertion create no default-strength1 history.

A historical3.5E38 value survives unrelated Save even with a BEFORE UPDATE OF rejection trigger, proving actual column omission rather than just equal final values. Missing SubstrateWater is explicitly Pending. Actual WM_CLOSE denies partial-input Save-and-close, focuses Cancel and preserves raw text; valid negative-fraction Save-and-close adds the seventh audit, and an ordinary default-build Save adds the eighth.

Successful Save must invalidate/rebase lexical sessions when the parent replaces `original` but keeps the same `draft` object; otherwise old expected values and raw formatting persist. Native post-save tab/Undo/no-op proof and a regression test cover this. Separate new-form collision/Create proves exact nullable precision/negative/100/101/zero values and existing legacy schema defaults without changing previous records or audits.

Independent captured-seed reconstruction preserves all15 tables, five pre-existing quality audits, raw true-1 SpeciesListComplete/Flag/Temporary, partners, children and SU. No invented sum/percentage rules, balancing or Metadata/child updates are introduced. All owned candidates/inspectors were closed, and the newest normal app runs from an independent disposable profile with debugging off.

## Verified bounded disturbance/exposure codes

The source/native oracle resolves122 cases:112 original native actions, five unsupported nonempty-Note actions and five supplemental native actions, with zero pending/integrity errors. All five Env ComboBoxes use two display columns, BoundColumn1 and AutoExpand; only literal Item is stored. Disturbance is nullable TEXT8 with LimitToList=False, while Exposure is nullable TEXT2 with LimitToList=True. Physical fields reject stored empty strings; keyboard clearing stores NULL.

Keyboard lowercase/prefix entry can select canonical codes. Programmatic assignment bypasses Exposure membership and preserves raw case while omitting field history. Source overlength disturbance typing can truncate an astral character to a lone surrogate. Failed/cancelled saves leave orphan history; retry duplicates it. Both lists use edit>=1/add>=2/clear=3 and unchanged none. Repeated slots are accepted and partners remain unchanged. An owned missing RowSource causes authoritative DAO failure but a silent one-row native combo; that row is not a valid catalogue.

The frozen139-row reference preserves127 disturbance and12 Exposure records, all1,390 typed cells across10 columns, exact nullable Double/Boolean/text values, observed order and source ordinals. M.f/M.s retain both descriptions/orders. Disturbance Notes are NULL; Exposure Notes are non-NULL empty. NULL/empty Items stay inspectable diagnostic rows, yielding126 selectable disturbance and11 selectable Exposure rows. There are no invented scientific columns or code-key-based duplicate precedence rules.

Desktop policy deliberately rejects new empty/malformed/overlength text without truncation. Disturbance manual values remain literal, with explicit UI acknowledgement for unmatched values. New Exposure values must equal a full canonical Item on every Create/Update/Save/restore path; the source's programmatic bypass is not ported. X is a valid one-character code and NA is a literal code, not NULL. Prefix/case suggestions require explicit selection rather than silently replacing raw text. Historical values are omitted from unrelated UPDATE statements; clearing NULL and unrelated saves can proceed without a healthy catalogue.

Site controls retain x675/735/789/843/897, y420, widths60/54/54/54/54 CSS pixels. Editing defaults enabled after native proof; `VITE_SITE_CODES_EDITING=false` disables it. A shared raw JSON token guard rejects lone surrogates and invalid UTF-8 before Go's decoder can repair them, including duplicate/case-folded/escaped keys. Data and exact-value `_Env` history share one transaction; case-only history is an intentional source-defect correction.

Ignored `evidence/private/site-code-wails/validation.json` records37 valid/33 invalid raw attempts across five controls, all1,390 native-bound metadata cells, duplicate meanings/repeated slots, explicit canonical selection, tabs/Undo/Lock/loading, real data and audit failures/rollback/retry, exact eight-unit Unicode/apostrophe and raw case saves, NULL clear/addition, genuine opened-catalogue checksum failure/Retry, historical-column rewrite rejection, schema Pending, and actual invalid/valid WM_CLOSE. Separate native collision/Create proof adds only intended Env/Admin values and the existing SV_FloodPlain FALSE default.

Independent captured-seed reconstruction checks all15 tables,13 pre-existing quality/substrate audits and raw true-1 flags, expecting exactly seven new code audits. No partner, child, Metadata, SpeciesListComplete, reference or phantom writes occur. All91 frontend tests, clean Svelte check/build and full Go race pass; bindings contain9 services/62 methods/27 models. Latest normal app is inspector-off on an independent four-plot profile. Verified coverage is49 of98, with full FS882/application migration still open.

## Region/district and ecosection source evidence history

The partial counts and stops below are historical evidence, superseded by the completed ordinary implementation and native verification section that follows.

FSRegionDistrict is the centered two-column Env TEXT7 ComboBox at raw twips270/2610/2070/270, tab12, with Region rows ordered by ItemOrder. Ecosection is the centered two-column Env TEXT3 ComboBox at8730/3150/1360/270, tab34, with the source lowercase `ecosection` filter and no ORDER BY. Neither instance has a specialized event handler; ordinary history uses parent BeforeUpdate/AuditTrail.

Native measurements currently resolve17/72 cases (16 actions and one unsupported Note case), with55 pending and four unexecuted supplemental context probes. Both physical fields are nullable, disallow zero-length strings and have empty default/mask/validation properties. Both effective controls have BoundColumn1, LimitToList=False and AutoExpand=True. Region keyboard manual/apostrophe codes are accepted; recognized lowercase/prefix entry canonicalizes, overlength input truncates including split surrogates, and rejected saves leave orphan history duplicated on retry. These partial measurements do not establish the remaining Ecosection/audit/cancellation/context behavior.

The immutable readonly native snapshot preserves164 rows/1,640 typed cells:27 Region and137 Ecosection rows, all10 real columns and actual native order. Access's lowercase source list filter matches the capitalized Ecosection list. The older five-column packaged catalogue's26/136 counts and SQLite case-sensitive lookup are not authoritative substitutes.

Cancellation preparation failed with unknown owned Access error2186 in Design view before the ordinary action. No unknown modal was clicked; all19 owned instances exited, the VM lease was released and the grant revoked. Completed cases have zero15-table/partner integrity errors and Error Trapping2->2 readbacks; the terminated preparation attempt has no final option readback. Do not count instrumentation failure as cancellation behavior or invent missing restoration evidence. Both desktop controls remain read-only, no policy is implemented and coverage stays49/98 until the source and actual Wails workflow are verified.

Latest reconciled valid count supersedes the initial17:21/72 resolved (20 native and one unsupported Note),51 pending and four supplemental contexts pending, with zero completed-case15-table/partner integrity errors. Additional Region cases show programmatic `zz` persists, lowercase `rcb.dcc` persists without case-only history, an empty assignment attempt leaves storage unchanged and strength0 edit saves without field audits. The corrected view guard prevents2186 and actual2001 cancellation classification passes25 offline regressions. A retry capture is retained but excluded: VBA End reset a private saved-event variable, restoring empty BeforeUpdate instead of captured `[Event Procedure]`. Artifact-backed exact restoration must be verified before retry and original data/history behavior proven; this is not source cancellation behavior. All five latest attempts finalized Error Trapping2->2; the two earlier terminated attempts still lack final readbacks.

## Region/ecosection ordinary implementation boundary

The completed ordinary two-field oracle supersedes the partial counts above:72 resolved (70 native actions/two unsupported Note),32 audit-strength cases, zero valid-case15-table/partner errors. Shared End recovery passes17 tests and the classifier25; both cancellation cases restore the exact captured event before retry, preserve original handler bodies and pre-End context/preferences/strength, refuse target data, then save the intended value with the expected field audit. All79 recorded owned instances exited;52 new final Error Trapping2->2 proofs exist, while two earlier terminated final readbacks remain unknown.

Both manual-code fields are nullable/nonempty with physical UTF16 limits7/3 and LimitToList=False/AutoExpand=True. Keyboard known lowercase/prefix completion canonicalizes; programmatic raw case persists without source case-only history. Keyboard clear stores NULL; programmatic empty is rejected. Source overlength truncation/surrogate splitting and orphan/retry history are measured defects. Desktop policy preserves raw well-formed case/apostrophe/whitespace, never autofills the partner, rejects new empty/malformed/overlength text without repair, omits unchanged historical invalid columns, allows NULL/unrelated saves without catalogue availability and commits exact `_Env` data/history together.

The source-positioned frontend uses Region/district x9/y144/width138 and Ecosection x573/y180/width1360/15 pixels, height18, from exported raw coordinates minus page origin135/450 twips once. It provides explicit suggestions/duplicate metadata, exact-draft unmatched/unchecked-code review, lookup error/Retry, schema Pending and shared Save/Lock/close busy/validation gates. Editing now defaults enabled after actual native proof; `VITE_REGION_CODES_EDITING=false` disables it. All99 frontend tests pass; focused/full Go race passes (root151.501s), bindings are10 services/65 methods/28 models and Svelte check reports zero errors/warnings. The default production build passes, with all17 asset payloads byte-identical to the native-tested opt-in build. Verified writable parent coverage is51/98.

Ignored `evidence/private/region-code-wails/continuation-validation-release.json` completes the representative native integration. Its42 preserved PNG/JSON captures include six continuation setup/staging captures; they are not42 independent tests. All1,640 native-bound reference cells and source geometry pass. Native proof covers raw/manual/review/suggestion behavior, shared busy gates, tabs/Undo/Lock/hidden-invalid, real data and audit failure/full rollback/retained-draft retry, exact case-only audits, both fields' NULL add-clear/no-op, historical UPDATE OF omission, opened-catalogue checksum failure/exact-byte Retry, NULL/unrelated-save catalogue independence, schema Pending, actual WM_CLOSE choices/invalid/failed-save recovery, and Create collision/new identity. Three audit matrix cases ran natively;29 inherit the completed source/Go matrix, explicitly not an exhaustive native claim.

Independent all15-table reconstruction preserves20 old audits, partners, children, SU, Metadata and raw true-1 flags, with exactly11 new audits (31 total). NEWRC001 is created at audit strength0. Final project SHA is `5966c9ba08811b3303f9e633c77b2deeb089c1aaa28664cf9afe763e891f082f`. All six continuation candidates/WebViews exited, inspector closed, catalogue/preferences/triggers/schema restored, protected/canonical identities unchanged and grants revoked. The original partial stop remains preserved. Exact clean-navigation handling uses a one-shot owned identity/URL/type/message/clean-SQL witness and CDP response, never a window.confirm override.

The tested binary payload `b1ee9fe6c95e4fc5d3e80812aace42a017505dd3468db54bc31c2d10083903b0` was promoted by same-volume rename to `bin/vpro-region-code-current.exe`. A normal inspector-off app starts from an independent seven-file exact seed copy, remains responsive and leaves the final project bytes unchanged. The Home capture shows the five-plot Sample profile. No new native-proof run or speculative relink is claimed for this byte-identical promotion.

Supplemental source context proof is1/4 valid with three entries pending. The Region `gRo`/NtsMapSheet `pen` capture is retained/excluded: intended `RCB.DMH` input was not achieved, so no legitimate partner mutation is inferred. Controlled source lookup failure and alternate project/reference version remain unmeasured. The desktop's unavailable-reference and partner-preservation behavior is now verified in Wails; it does not fill the missing Access observations. Full FS882/application migration remains open.

## Soil classification source evidence history

The next bounded candidate is the independent pair of parent Env fields SoilClassGroup/SoilClassSubGroup, not embedded soil-horizon columns. Both belong to Detail/tabPages/Soil/&Terrain/group@6423. Source twips for SoilClassGroup are3600/1260/805/270, tab17, with the Great group label at2520/1260/1080/270. SoilClassSubGroup is1710/1260/805/270, tab16, with Soil subgroup at270/1260/1440/270. Both exported ComboBoxes are centered Arial10, two columns of720;2520 and list width3240.

Their independent DISTINCTROW Item/ItemDescription queries order by ItemOrder and use exact ListName strings `SoilClassGroup` and `SoilClassSubgroup` respectively. No ordinary control-specific handler, parent-code filter, subgroup derivation, cascading clearing or autofill was found. The parent BeforeUpdate calls AuditTrail Me, Lock saves before disabling edits and the Audit tab saves before requery. These source paths do not prove soil-specific failure or effective runtime behavior.

The exported Env DDL declares TEXT4 for both; Required/AllowZeroLength/validation and effective BoundColumn/LimitToList/AutoExpand remain unmeasured. Existing nullable DTO/storage mappings and generic identity/schema/transaction guards are implemented. Dedicated four-unit/Unicode/empty/catalogue/historical-omission guards and an ordinary parent soil editor are missing; SourcePage currently displays these controls disabled.

Offline packaged Sample uses nullable VARCHAR without an enforced four-unit bound. Packaged vlists contains38 great-group and61 subgroup rows, maximum code lengths3/4, with no NULL cells or ASCII-NOCASE duplicate codes in these subsets. This is not current Access catalogue provenance: native attachment/version, all10 typed columns, raw duplicates/NULL/empty metadata and ordering ties require a readonly native fixture.

No VM activity, implementation or write enablement occurred during this preflight. A separately authorized selective disposable oracle must establish unresolved physical/effective constraints, current full-column reference data and genuinely distinct representative behavior. Reuse the verified parent BeforeUpdate/AuditTrail evidence explicitly where source procedure and semantics match; do not repeat its exhaustive audit matrix per field. Keep physical/raw-input/audit/collision/rollback combinations in Go/frontend tests and verify binding-dependent interactions in Wails. Do not infer a group/subgroup hierarchy from scientific names or introduce autofill. Desktop membership/manual-code policy and API/UI integration await that measured contract; current verified coverage is51/98.

### Subsequent readonly Soil catalogue milestone

Readonly DAO on the exact closed owned VLists copy captured39 SoilClassGroup rows and62 SoilClassSubgroup rows, including one empty Item per list:101 rows/1,010 typed cells,99 nonempty reference-selectable entries. All10 source columns, native order, exact casing, nullable booleans, NULL/empty strings and double bits are retained. Nine integrity tests and exclusive original/reopened byte seals passed without opening the Access UI. This supersedes the offline subset as current reference-data evidence, not as effective form or write-policy proof.

The immutable readonly SoilCodeService and generated bindings are implemented; focused tests and full Go race pass (root147.924s). Installation never replaces an existing catalogue, and list reads validate its bytes, profile, provenance and complete normalized metadata. The captured DAO cell-payload hash and normalized storage metadata hash deliberately have different representations and are independently checked; they are not interchangeable. No parent Soil writer, four-unit input policy or cascading behavior has been enabled.

The Soil/Terrain reference viewer is opt-in (`VITE_SOIL_CODES_REFERENCE=true`) and contains no inputs, draft binding or write calls. It retains empty/NULL metadata and explicit error/Retry state. All104 frontend tests, Svelte check0/0 and both default-off/opt-in production builds pass. Actual native Wails verified all101 rows/1,010 typed metadata cells/1,313 DTO properties in four accepted captures. Corrupting only the owned catalogue produced two explicit checksum errors and cleared both lists; exact bytes restored and actual warm Retry recovered39/62 rows. All eight copied database hashes,15 project tables and31 audits remained unchanged, with zero observed project-write calls. Candidate10744 and six proven descendants exited gracefully, port9382 closed, protected five unchanged and grant revoked.

The two parent Soil controls and33 source-page inputs remained readonly. Existing experimental Humus/Mineral Add actions were enabled and not tested in this readonly run; there is no child-default-off assertion or broader child write/source parity claim. Their earlier bounded desktop CRUD proof is separate from unresolved Access child workflows. Keep this reference milestone opt-in, without normal-app promotion or an increase to51/98 writable parent coverage. Native release SHA `75c52f8050891d5ecf71011cc6ac7e78c09aa29aff391e16bd90039e37b277dc`.

Disposable reference-baseline attempts have not yet completed the installed-reference gate. Reference-3 stopped on a strict unmapped path without retaining the refused identity; do not infer that identity or claim live mapping success. Exact-owned cleanup retained initial/final Error Trapping2 and restored only the changed front with all five original seals. The diagnostic collector now persists raw identities before path inspection; unknown libraries still stop without guessed mappings or installs. No ordinary Soil source actions have run.

### Verified ordinary Soil editing

Source measurements establish independent nullable TEXT4 fields, Required=false,
AllowZeroLength=false, LimitToList=false and AutoExpand=true. Physical apostrophe
entry/Undo, raw/manual and independent NULL saves were observed on disposable
adapted copies. Four UTF-16 units `[65,233,20013,66]` persisted. Case-only `ca`
against stored `CA` remained `CA`; the responsible event is not proven.
Exports have no Soil-specific event handlers. Shared parent BeforeUpdate/AuditTrail
and save-before-lock paths are reused, not rediscovered for every field.

The desktop deliberately requires explicit full-item completion, preserves raw
casing, rejects overlength/malformed input without truncation, commits data/history
atomically and omits source phantom checkbox audits. Unknown source physical astral,
overlength, Lock-child and failed-save behavior remains unknown; the original full
installed-reference baseline is still unpassed.

One actual native Wails fixture verified nine scenarios: catalogue/source placement,
listed Item selection, apostrophe/Undo, astral four-unit/raw lowercase entry, hidden-tab
overlength Save/Lock refusal, independent NULL clearing, shared parent Lock,
SQLite-trigger failure/rollback, and combined retry/save. Actual WM_CLOSE additionally
proved Cancel, failed-save window retention/rollback and retry/save/exit. All 15
project tables were compared; 31 original audits survived and exactly eight intended
Soil audits were added. Compact observations and visuals are retained under
`evidence/private/native-current`.

Editing defaults on with explicit `VITE_SOIL_CODES_EDITING=false` opt-out.
All 114 frontend tests, Svelte check0/0 and default/opt-out builds pass.
Default assets are byte-identical to the opt-in assets embedded in the tested
binary, promoted to `bin/vpro-current.exe` without changing its payload.
Verified parent coverage is53/98; full FS882/application replacement remains open.

Ordinary Region/Site/Soil now share nullable UTF-16 validation, reference grouping,
suggestions and draft-bound acknowledgement. Region/Soil write/restore guards are
also shared; Exposure membership stays explicit, and existing Region/Soil policies
for unchanged malformed historical bytes are not silently unified. All117 frontend
tests/check/build and full Go race (root177.917s) pass. A fresh native fixture repeated
the nine scenarios and actual-close failure/retry against the integrated helpers,
again preserving31 old audits and adding exactly8 intended audits. Source placement
and public service APIs remain unchanged.

### Verified adapted Bedrock group

BedrockGeology1/2/3 are independent nullable Env TEXT4 fields, with the same
BedrockType Item/ItemDescription list ordered by ItemOrder and no instance-specific
handlers. Source controls retain x183/243/303, y30, width895/15 and height18 pixels.
Soil/Terrain omits Top, unlike the Site page: do not subtract the Site origin.
The frozen readonly DAO fixture preserves87 rows/870 typed cells. Its nonselectable
empty Item, duplicate/NULL/empty metadata and ordering are preserved.

The exported form's omitted effective properties remain unmeasured. The desktop
explicitly adopts raw nullable four-unit codes, full-item completion, exact casing,
no truncation/cascade, transactional history and unchanged historical omission.
These are safer source-bound adaptations, not claims of exact Access keystrokes.
CoarseFragLith has a separate TEXT12 bound; its verified workflow is described below.

Generic physical guards cover all Create/Update/Save/raw-JSON/restore routes.
Actual native Wails verified full listed Items across all three fields, raw
apostrophe/trailing-space and astral four-unit values, independent NULL, hidden
overlength Save/Lock refusal, real SQLite-trigger rollback/retry, shared Lock and
clean WM_CLOSE. All15 tables were compared,31 old audits preserved and exactly7
intended audits added. The first placement assertion expected y0 incorrectly:
its unchanged-state stop is retained, and the same owner continued with y30.

Bedrock defaults on with `VITE_GEOLOGY_CODES_EDITING=false` opt-out. All123 frontend
tests/check0/0/default and opt-out builds pass; full Go race passes (root210.109s).
Default assets are byte-identical to the native-tested opt-in payload promoted as
`bin/vpro-current.exe`. Compact evidence is under `evidence/private/native-geology`.
Coverage at the Bedrock milestone was56/98; parent and application replacement remain incomplete.

## Ordinary parent-code workflow

The source-first remaining-parent checklist groups21 independent combos with no
field-specific event handlers. They use the shared `Form_BeforeUpdate`/`AuditTrail`
and save-before-Lock event path. RealmClass belongs on Site; the other20 fields
belong on Soil/Terrain. List names and lengths are explicit in
`parentcodeheader.go` and `frontend/src/parentCodeEditor.ts`: coarse-fragment
lithology TEXT12 (not Bedrock TEXT4), humus phase TEXT50, flooding frequency TEXT7,
and each other field's own source bound. Surface/subsurface partners remain
independent; hydrogeology and humus do not imply cascading.

ParentCodeService preserves the frozen16-list/347-row/3,470-cell fixture. The
editor uses15 of these lists; NULL/empty Items and duplicate definitions remain
metadata, not selectable empty assignments. SoilDrainage has explicit exported
LimitToList=NotDefault and is excluded until strict membership/reference availability
is implemented. Other omitted effective combo properties remain unmeasured.
The desktop deliberately uses raw casing, explicit full-item selection, no implicit
completion/truncation, nullable fields and atomic data/history; this is an adapted
source-bound workflow, not exact Access keystroke parity.

All21 physical fields have Create/Update/Save/raw-JSON/restore guards. Frontend
per-list loading/errors/Retry, scope- and draft-bound review, UTF-16 validation
and hidden error persistence share the proven reference helpers. Unchanged
historical invalid values are omitted from assignments. Save/Lock/close cannot
bypass an invalid field on a hidden tab; valid correction and Undo clear its error.

Actual Wails proof under ignored `evidence/private/native-parent-codes` passed ten
cases and three layout checks at1400/1024/560. All21 fields were written using real
controls, including full listed selection, apostrophe/trailing-space maximum-length
raw values and a50-unit astral humus phase. NULL clear, remount/review preservation,
hidden Save/Lock/WM_CLOSE refusal, Undo/Lock, disposable checksum failure/per-list
Retry, real SQLite-trigger rollback and close Cancel/failed-save/retry/exit passed.
All15 tables were compared,31 old audits preserved and exactly24 intended audits
added. A subsequent clean read-only reopen observed disabled SoilDrainage and
closed without data changes; its first tab click waited too little for other Site
lookups, so the same unchanged fixture/window continued after readiness.

Editors default on; `VITE_PARENT_CODES_EDITING=false` opts out. All134 frontend
tests, check0/0, default/opt-out builds, focused Parent tests and full Go race pass.
Default assets match the tested opt-in payload byte-for-byte. Current writable
coverage is77/98; this does not complete the parent or application migration.

## Verified bounded height editing

Ignored `evidence/private/height-editing-native/native-contract.json` / `.txt` reconcile104 measured native cases, plus a rejected explicit NULL AutoNumber fixture, with zero evidence-integrity failures. Canonical files/protected Access remained untouched; owned processes closed and registry/options were restored.

- Heights1-6 accept negative/zero/NULL without a source unit rule. Single storage quantizes precision; `1.234567890123` becomes `1.2345678806304932`, while audit display becomes `1.234568`. NaN/Infinity and3.5E38 fail2113 without saving.
- Covers1-6/totals are nullable Single with `<100 Or Is Null`; the native `CC` mask silently turns typed100 into10. The desktop intentionally validates100 as invalid, never truncates it; negative values remain allowed.
- A height-only row saves. Clearing C Cover6 warns but saves NULL; the cached source row remains until requery. The desktop warns that explicit Save will remove C-view membership without deleting the vegetation record.
- Real keyboard height edits bypass native parent Lock. A failed source save leaves an orphan height audit. View switching commits dirty heights implicitly; explicit Undo preserves data/history.
- Native IDs are non-PK AutoNumber with a nonunique index. Explicit append accepts0/duplicate signed IDs; NULL append fails. Stale3020 save does not recreate a deleted row. Desktop identities stay collision/staleness guarded; insertion/deletion/generator and species workflows are separate.

The native-verified desktop uses explicit height/cover drafts and batch Save/Cancel, preserving drafts across view/tab changes. It preserves float64 precision within the physical Single range, accepts native negative/zero/NULL semantics without invented units, guards every variant with the plot lock, and couples data/history in one expected-value-checked transaction. No automatic cover totals, partner fields, Metadata or SpeciesListComplete writes are intended. Height deletion/creation/species/collected remain unavailable in these grids.

Editing defaults enabled after actual Wails proof; `VITE_HEIGHT_EDITING=false` disables height-grid edits. `PlotService.UpdateHeightRecords` accepts distinct explicit IDs and matching nullable values/expected keysets for Height1-6, Cover1-6 and TotalA/B. It updates existing identities only, never creates or reserves an ID. Normal Veg Create/Update/compatibility Save use the same verified-number guard for newly changed fields; unchanged finite historical invalid values remain preserved and can be cleared.

Native proof under ignored `evidence/private/height-native/validation.json` covers no-write staging/cancellation/invalid input, source layout/membership, view/tab preservation, Lock and other-mutation gates, atomic multirow rollback/retry, raw precision/partner preservation, ID0, duplicate/stale/deleted identities, schema Pending and actual WM_CLOSE Save/Cancel/refusal. Duplicate-identity refusal also remounts the rejected raw input instead of leaving it visibly accepted. A successful write followed by failed refresh explicitly reports that the write committed and disables capabilities until recovery; it does not invite a duplicate retry.

Independent captured-seed reconstruction verifies all15 project tables and exactly five intended audits, accounting separately for named external42/deletion test fixtures. Cancel reloads current data after a conflict. Clearing C Cover6 warns before Save and removes C-grid membership after refresh without deleting the vegetation row. Parent writable coverage is35 after the separate Working Unit workflow; this is bounded height editing, not complete FS882 or application parity.

Existing audit thresholds remain unchanged: nonnull edits at strength1+, NULL additions at2+, NULL clears at3. No computed option checkbox history is generated. Hemisphere conventions, UTM conversion and Access's exact width-dependent derived display rounding remain unverified; preserved raw precision is the deliberate desktop behavior.

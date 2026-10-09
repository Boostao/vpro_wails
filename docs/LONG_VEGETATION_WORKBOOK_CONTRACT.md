# Long Vegetation workbook draft

## Availability

The private generator implements unit worksheets and the source ReportSummary
worksheet. The independently reviewed owned publisher has a registered public
DTO facade and interface bindings, default-off under
`VPRO_LONG_VEGETATION_WORKBOOK`, paired with literal frontend
`VITE_LONG_VEGETATION_WORKBOOK`. The existing Long Vegetation route's
`VITE_LONG_VEGETATION_REPORT` gate is still required; Code/Strata/Lifeform gates
remain independent when those retained settings are selected. This bounded
unlumped desktop workflow passes native and integration checks; it does not
establish all Access report modes. Existing preview behavior is unchanged.

Source: `VPro64_forAI/Modules/V7mdlReportsLongVeg.txt`, `LongVegReport`,
`AddLifeformCodes` and `LongVegOptionText`; launch preferences/events are in
`Forms/USysLongVegOptions.txt`. AllSpecs version comes from the table Description
property, mapped to SQLite `_table_metadata`, not species rows or an R version.

## Saved title preference boundary

The current title editor is implemented behind independent default-off
`VPRO_LONG_VEGETATION_PREFERENCES` and `VITE_LONG_VEGETATION_PREFERENCES`.
The route still requires its report gate; this does not enable any other option,
report mode, worksheet publication or printing.

Static source: `USysLongVegOptions.Form_Load` reads `LVReportTitle`;
`rptLvTitle_AfterUpdate` assigns it through `clsRepOpt.LVReportTitle`.
The class getter/let at249-254 use the literal registry value and the absent-key
default `Long Vegetation Report`. `V7mdlReportsLongVeg`101 reads that property for
the report title. No species-attribute join or numeric Val calibration is involved.

The desktop adaptation uses explicit owned Load/Save/Undo and expected/proposed
YAML CAS, not implicit AfterUpdate or registry writes. Only `LVReportTitle` can
change. NULL/missing/wrong-type values are unavailable, not empty/default titles;
empty, whitespace, CRLF, historical long text and unchanged historical NUL stay
distinct. Changed titles require complete Unicode without NUL; strict nested
JSON rejects unknown, recased and decoded duplicate keys before repair.

The shared retained title controller reuses the accepted Long Environment
behavior with independent exact-owner registries. Validation survives remounts
and blocks Save/report operations/native close while correction and Undo remain
live. Loads are cancellable and generation-checked; Save cannot be cancelled.
Acknowledgement retains known/unknown irreversible receipts. Lost authority
requires explicit owned Load before another Save, never blind mutation replay.
Drafts do not change previews or workbooks: Save, acknowledge and reload options.
Title changes invalidate prepared UI workbooks and backend approval hashes;
attached/external tests prove zero-output stale refusal, fresh publication and
unchanged calculations/provenance/data/audits.

Focused Go race30.483s,47 coupled client cases including actual Svelte proxies,
all1126 registered frontend cases (318+448+360), check0/0, private default/enabled
builds5.17s/5.20s and production/vet pass. Interface generation has no warnings
and changes exactly the two shared binding files plus the new service file.
Independent exact-baseline review reports no significant issues. Four normally
closed actual-main owners pass16 native cases: independent title-off/backend-off
gates, three-width title/button visibility and containment, explicit Load,
valid-draft remount, literal/committed-warning/unknown-delivery Saves, held read
cancellation, native-close/navigation barriers and retained correction/Undo/
receipts. The actual prior workbook approval refuses zero output after a title
Save; one fresh exact-hash XLSX contains the literal title and report calculations/
provenance are unchanged. Historical NUL/CRLF no-op and correction pass:
textarea display normalizes CRLF to LF, but an untouched Save transports the
original NUL/CRLF and preserves YAML bytes. The extracted Environment editor
retains independent Load/draft/remount/Undo/no-op behavior on that same owner.
All16 fixture files/22 protected identities restore;306 files seal the closed
owners, outputs and receipts. Complete-source1138-input full integration passes
exit0 in3641.968s, including generated bindings; all frozen inputs and ordinary/
production discovery remain unchanged. This is bounded native
desktop adaptation, not source registry/AfterUpdate or complete report parity.

## Saved Average/Order preference boundary

The independently default-off `VPRO_LONG_VEGETATION_STATISTICS_PREFERENCES` and
`VITE_LONG_VEGETATION_STATISTICS_PREFERENCES` gates expose explicit owned
Load/Save/Undo for two source option groups only. `rptLvAvgType` retains the
source label **Calculate** and choices **By n Plots** (10) / **Characteristic**
(20); `rptLvOrderBy` retains **Order by**, **Species** (10) / **Presence** (20).
`USysLongVegOptions.Form_Load` reads these settings and their leaf AfterUpdate
events assign `clsRepOpt.LVAvgType` / `LVOrderBy`; the source defaults are 10/20.
The existing report consumes them without new calculation or ordering logic.

Desktop Save deliberately replaces registry/AfterUpdate timing with strict
two-key YAML expected/proposed CAS. Missing, NULL, wrongly typed or unsupported
saved values are unavailable, not repaired or defaulted. Raw select values must
be exactly `"10"` or `"20"` before numeric transport. All four combinations and
independent one-field changes are tested; unchanged assignments are omitted and
no-op Save preserves exact bytes. Title persistence remains independently gated
and one-key only. No group, threshold, quality, Source4/Val, printing, registry,
project data or source audit writes are granted.

The shared preference lifecycle retains per-context drafts, validation and
irreversible receipts separately for titles and statistics. Invalid drafts block
Save/report operations/native close while correction/Undo remain live. Reads are
cancellable and generation-checked; Saves are not. Unknown outcomes require
acknowledgement and an explicit owned Load without discarding receipt evidence.
Save invalidates prepared workbooks; attached/external cases prove zero-output
stale refusal and exact fresh publication.

Implementation validation passes coupled report race421.149s and final focused
statistics race133.783s;57 coupled client cases, all1136 registered frontend cases
(328+448+360), check0/0, private statistics-off/enabled builds and production/vet.
Interface generation has no warnings and exactly three binding files change.
Exact-baseline independent review reports no significant issues. Four normally
closed actual-main owners pass20 native cases: independent UI/backend gates;
owned literal Load, source labels and1400/600/320 select/button containment;
valid-draft remount; invalid empty selection with correction/Undo and native-
close/navigation barriers; held read cancellation/stale-result suppression;
four once-only mixed/one-field commits across all four combinations with held,
committed-warning and unknown deliveries; retained irreversible receipts and
explicit recovery; typed no-op/stale refusal. The actual old workbook approval
refuses zero output; one fresh exact-review/receipt/file-SHA workbook publishes
once and is independently reopened. Historical Average15 remains unavailable,
while untouched NUL/CRLF title no-ops preserve transport/YAML for both Vegetation
and the shared Environment editor; Environment drafts retain remount/Undo.
All16 fixture files/22 protected identities restore;339 files seal the four
closed owners. A harness-only wrong publication label stopped before dispatch;
continuation resumed after the four sealed Saves without replay. Complete-source
1146-input integration passes exit0 in3525.956s; all frozen inputs/generated
bindings and ordinary/production discovery remain unchanged. This is bounded desktop native
adaptation, not source registry/AfterUpdate or complete report parity.

## Saved report switch preference boundary

Independent default-off `VPRO_LONG_VEGETATION_SWITCH_PREFERENCES` and
`VITE_LONG_VEGETATION_SWITCH_PREFERENCES` gates expose explicit owned
Load/Save/Undo for four equivalent pure leaf source events:

| Source control | Exact caption | Saved key | Source default |
|---|---|---|---|
| `optScreenReport` | Screen report (skip some print page formatting - faster) | `LVQuickReport` | -1 |
| `rptLvSpaceBetweenGroups` | Space between groups | `LVSpaceBetweenGroups` | 0 |
| `optCreateSummary` | Create report summary | `LVReportSummary` | -1 |
| `optConstantSppList` | Use constant species list | `LVConstantSppList` | -1 |

The form's Load reads these settings and each leaf AfterUpdate assigns its
corresponding `clsRepOpt` property. The quality-enforcement callback also invokes
Form_Load and is excluded from this equivalent batch. Existing report/workbook
readers consume the four settings; no new arithmetic, layout or printing engine
is added. The Screen report caption is source evidence, not a new print promise.

Desktop Save adapts registry/AfterUpdate timing to strict four-key YAML CAS.
Wire values must be booleans, with exact required keys; NULL, duplicate decoded
keys, malformed Unicode and wrong types are refused. Saved YAML accepts only
native booleans or source -1/0 integers. Missing/NULL/invalid storage is unavailable,
not defaulted. Unchanged assignments preserve their original type and bytes;
changed assignments use native YAML booleans. No-op Save preserves the file.
All sixteen combinations and each field's stale workbook approval are tested.
Project data, source audits, registry, grouping, thresholds, quality and Source4/
Val are unchanged.

The shared retained lifecycle uses a separate exact-owner switch registry.
Unloaded boxes explicitly do not represent saved values. Draft validation blocks
Save/report/native close while correction/Undo remain live. Reads are cancellable
and generation-checked; Saves are noncancellable. Irreversible known/unknown
receipts survive remount and acknowledgement. Unknown authority requires explicit
owned Load, not blind replay. Draft changes invalidate prepared workbook reviews;
saved constant-list changes also invalidate incompatible loaded report options.

67 coupled client cases and all1146
registered frontend cases (338+448+360), check0/0, private switch-off/enabled
builds and production/vet pass. Interface generation has no warnings and exactly
three binding files change. Coupled Go race passes282.577s. Independent exact-
baseline review has no significant issues. Four normally closed actual-main
owners prove24 cases: independent gates; exact source captions and1400/600/320
label/checkbox/button visibility and containment; valid draft remount/Undo;
actual-handler injected nonboolean validation with live correction; held read
cancellation/stale-result suppression; held noncancellable Save/native-close/
navigation barriers; committed-warning/unknown receipts and explicit Load.
Five enabled and one historical commits are sealed once, with independent
four-field and mixed changes. Layout/summary preserve the calculation preview;
constant-list changes the owned settings. Historical invalid source1 remains
unavailable; planned disposable correction/retry, four-integer no-op and
single-field omission preserve the untouched -1/0 values. All retained no-op/
committed receipts survive remount/acknowledgement. Actual stale workbook
approval refuses zero output; one fresh exact-review/receipt/file-SHA XLSX
publishes once and is independently reopened. All16 files/22 protected identities
restore and no owner remains.

Two additional incomplete no-mutation owners are separately hash-archived,
not counted as acceptance: missing existing report flags, and a broad wait that
mistook a navigation alert for asynchronous native-close cancellation. Exact
closing text fixes the witness. A fixture constant-list0 no-op stopped the held
commit expectation; same-owner continuation resumed after three sealed commits,
acknowledged that observed no-op and changed its actual opposite. A helper-
boundary recovery resumed before further dispatch. No completed mutation or
publication was replayed.378 files are sealed; complete-source1155-input
integration passes exit0 in3728.741s with unchanged inputs/generated bindings
and ordinary/production discovery. Final acceptance is separately associated;
the immutable native seal retains its original then-pending observation.
This is bounded desktop adaptation, not original registry/AfterUpdate timing
or complete report/printing parity.

## Saved threshold preference boundary

Independent default-off `VPRO_LONG_VEGETATION_THRESHOLD_PREFERENCES` and
`VITE_LONG_VEGETATION_THRESHOLD_PREFERENCES` gates expose owned explicit
Load/Save/Undo for `rptLvShowSppPres` (**Show species with presence > than**) and
`rptLvShowSppMC` (**Show species with mean cover > than**), with their percent
suffixes. The unbound textboxes have no exported Format, InputMask or validation
bounds. Their pure leaf AfterUpdate events assign `LVPresenceGreaterThan` /
`LVCoverGreaterThan`; both source property defaults are0.

The desktop boundary deliberately uses typed finite DOUBLE numbers and explicit
YAML two-key expected/proposed CAS, not source registry or automatic AfterUpdate.
The existing lossless YAML-number reader refuses missing/NULL/text/nonfinite
storage and integers not exactly representable as DOUBLE; nothing is repaired
or defaulted. Raw drafts retain their exact text and validation. Added whitespace,
empty/NULL, malformed numeric/Unicode text, overflow and nonzero underflow are
refused before Save; strict JSON requests independently refuse NULL, wrong types,
duplicate decoded keys and nonzero underflow. There is no invented0-100 bound.
Unchanged assignments remain omitted, preserving historical integer storage,
and no-op Save preserves exact file bytes. The threshold comparison is numerical;
signed zero is equivalent to zero, not a separate ownership identity.

Presence remains percent in saved settings; the existing report divides it by100.
Existing strict greater-than comparisons, calibrated formatting/calculations and
constant-list bypass are unchanged. No new filtering kernel, range, grouping,
quality/enforcement, Source4/Val, registry/project data/source audit writes,
printing or default promotion is granted.

The accepted shared lifecycle retains raw draft errors and irreversible receipts
under a separate exact-owner registry. Reads are cancellable/generation-checked;
Saves are noncancellable. Invalid drafts hold Save/report/native close while
correction/Undo remain live. Unknown outcomes require acknowledgement and owned
Load without erasing receipts. Threshold edits invalidate workbook reviews;
saved changes invalidate incompatible loaded options. Attached/external cases
test each key's stale approval refusal and unchanged data/audits.

Focused Go race33.176s,76 coupled
client/1155 full registered frontend cases (347+448+360), check0/0, private
threshold-off/enabled builds and production/vet pass. Interface generation has
no warnings and exactly three binding files change. Final coupled race261.813s
passes. Independent exact-baseline review has no significant issues. Four
normally closed actual-main owners prove21 cases: independent gates; exact
source percent labels/1400/600/320 field/button containment; valid101 draft
remount/Undo; five invalid raw drafts with live correction/Undo and Save/
native-close/navigation barriers; cancellable held read/stale-result suppression;
noncancellable held Save and retained committed-warning/unknown receipts with
explicit owned recovery. Four enabled and one historical commits are sealed
once; mixed/independent fields produce independently filtered baseline rows
without changing denominator, cells or provenance. Actual typed no-op/stale/
nonzero-underflow requests preserve exact bytes. Four irreversible receipts
survive actual remount/acknowledgement.

Historical inexact9007199254740993 remains unavailable rather than rounded.
Planned disposable correction/retry, an actual two-integer no-op, one cover1.5
Save omitting the unchanged integer presence and both receipts on remount pass.
Actual old workbook approval refuses zero output; one fresh exact-review/
receipt/file-SHA XLSX publishes once and is independently reopened. All16
originals/22 protected identities restore; no native owner remains. No source
application fix or harness continuation was needed.325 files are sealed.
Complete-source1164-input full race passes exit0 in3701.860s, finishing
2026-10-09T12:45:22.417614Z. Frozen inputs and ordinary/production discovery
remain unchanged; all325 sealed hashes verify. Final acceptance is separately
associated without rewriting the immutable native seal.
This remains bounded desktop adaptation, not original registry/
AfterUpdate or complete form/report/printing parity.

## Saved quality Include Nulls boundary

The Data Quality group's three **Include Nulls** controls retain Site/Veg/Soil
relationships and source identifiers `optSiteQualityNull`, `optVegQualityNull`,
`optSoilQualityNull`. Each AfterUpdate is a pure assignment to its respective
`DataQualityFilterSiteNullLV`, `DataQualityFilterVegNullLV` or
`DataQualityFilterSoilNullLV` property. Source absent defaults are boolean true
for Site and exact string `"True"` for Veg/Soil; these are not installed over
missing or malformed desktop storage. Form_Load reads the flags without
enforcement-based Enabled assignments.

Independent default-off `VPRO_LONG_VEGETATION_QUALITY_NULL_PREFERENCES`/
`VITE_LONG_VEGETATION_QUALITY_NULL_PREFERENCES` gates own only these three YAML
keys. The extracted quality-specific reader preserves existing bool/-1/0 and
exact legacy `"True"`/`"False"` semantics for all three domains. Ordinary report
switches do not acquire string acceptance. Changed assignments use YAML
booleans; unchanged historical representations are omitted, with byte-exact
no-ops. Missing/NULL/malformed values remain unavailable; wire values are three
strict required booleans, not legacy strings or truthy coercions.

Retained exact-owner Load/Save/Undo uses the accepted shared lifecycle:
unloaded null drafts, cancellable generation-checked reads, noncancellable
Save, persistent validation/irreversible receipts and explicit owned observation
after unknown outcomes. Root, sibling and workbook barriers and preview/review
invalidation are wired. Correction/Undo remain available for invalid drafts.
Editing these flags does not enable enforcement; existing readonly options and
preview remain authoritative for activity. Enforcement/Form_Load callbacks,
quality minimum combos, new rank/arithmetic logic, registry/data/audits,
Source4/Val, naming reset, lumping/variants/unit groups and printing are excluded.

Reviewed/bounded native-accepted and complete-source integrated. A95-file frozen
predecessor baseline and two verified canonical hashes define this batch.
Focused race4.947s,84 coupled client/1173 full frontend cases/check0/0 and
warning-free interface generation pass. Eight independently expected bool
combinations cover all64 nullable domain masks, strict wire/proxy detachment,
historical no-op/omission, ownership/cancellation/collision/cleanup/committed
retry and retained component lifecycle. Coupled attached/external active/
inactive workbook and quality/switch race passes218.679s. Private builds,
production/vet and exact three-file binding delta pass; independent baseline
review has no significant issues. Four normally closed actual-main owners prove
21 cases/five once-only commits: independent gates/exact three-width domain
labels, retained draft/read/Save/receipt/close lifecycle, inactive unchanged
calculation/provenance, mixed historical storage/refusal/correction/byte-exact
no-op/omission and active qualification without changing minima/enforcement.
Old approval refuses zero output; one fresh exact-SHA XLSX publishes once and
independently reopens. All16 originals/22 protected identities restore;
338 files are sealed/no owner remains, with no native application fix or
harness continuation. Complete-source1183-input full race passes exit0 in
4201.292s, finishing2026-10-09T16:05:14.474180Z. Frozen inputs and ordinary/
production discovery remain unchanged. All338 sealed hashes verify; final
acceptance is separately associated without rewriting the immutable native seal.

## Saved grouping preference boundary

See also the separately gated [quality Include Nulls draft](#saved-quality-include-nulls-boundary).

The source `rptLvGroupBy` caption is **Group by**, with Layer=1, Strata=2,
Lifeform=3, None=4 and default1. Its AfterUpdate is a pure assignment to
`clsRepOpt.LVGroupBy`. Independent default-off
`VPRO_LONG_VEGETATION_GROUP_PREFERENCES`/
`VITE_LONG_VEGETATION_GROUP_PREFERENCES` gates own only this YAML key, not Access
registry behavior or automatic writes.

The reader requires actual integer storage in1-4; missing, NULL, text, booleans,
fractional storage and unsupported values remain unavailable without defaults.
An owned review includes saved validity separately from immutable backend mode
availability. Existing client Strata/Lifeform gates further constrain changes.
No new client None gate is introduced. New unavailable mode assignments refuse;
unchanged valid unavailable modes may remain byte-exact no-ops and do not enable
their reports. Strict required JSON rejects duplicate/escaped-colliding keys,
malformed Unicode, NULL and noninteger choices.

Load/Save/Undo reuse the accepted retained lifecycle. The independent registry is
created once at component-module scope to retain context-owned drafts/errors/
receipts across remounts while capturing immutable client flags. Invalid raw
choices block Save/report/native close but retain correction/Undo. Reads cancel
and generation-check; Save is noncancellable, and unknown outcomes require
acknowledgement plus explicit owned Load, never replay. All sibling/root holds
and workbook/settings invalidation are wired. Only LVGroupBy changes; existing
grouping planners, quality, selected-field/lumping/variants/unit groups, source
data/audits, registry, printing and Source4/Val remain outside this batch.

Coupled validation exposed the owned Lifeform publication path's integer group
cells being rejected by the generic text-only identity writer; older synthetic
Lifeform tests had used text groups. Source `SELECT LifeForm`,
`CopyFromRecordset` and column-A `NumberFormat="0"` establish numeric workbook
cells. The narrow correction reuses the existing nullable signed16 Lifeform
reader, writes numeric cells with integer formatting and compares numeric group
gap keys. Other identity cells remain text-only. Independent extrema/zero/NULL/
gap/refusal cases and corrected synthetic fixtures cover the actual storage;
no group arithmetic changes. The initial496.375s failed race remains failed
evidence; final numeric-cell race passes3.127s and coupled race passes507.713s.

Reviewed and bounded native-accepted. Focused Go race4.777s,85 coupled client/
1164 registered frontend cases/check0/0/private default/enabled builds/
production/vet and exact three-file interface binding delta pass. Coupled Go race
passes507.713s; exact85-file-baseline independent review has no significant
issues. Five normally closed actual-main owners prove23 cases/five once-only
commits: independent gates/exact labels/three-width controls, changed draft
remount/Undo, invalid draft close/navigation guards, cancelled stale read,
held/warning/unknown Save receipts, all four reused planners and unavailable
historical no-op/correction/retry. Two additional independently planned exact-byte
no-op receipts prove known/unknown recovery and actual remount without new commits.
Old workbook approval refuses zero output; one fresh exact-SHA Lifeform XLSX
publishes once and independently reopened OOXML retains numeric group cells.
All16 originals/22 protected identities restore;430 files are sealed/no owner
remains. Four same-owner harness continuations corrected planned scope guards,
fixture Strata2 versus exported default1 and the standard stale binding-error
shape; no completed mutation/publication was replayed or native application fix
needed. Complete-source1174-input full race passes exit0 in3658.827s, finishing
2026-10-09T14:33:49.319176Z. Frozen inputs and ordinary/production discovery
remain unchanged; all430 native seal hashes verify. Final acceptance is separately
associated without rewriting the immutable native seal's then-pending observation.

## Unit sheets

- Title, unit/unique long name and unlumped space occupy rows 1-3.
- Row 4 contains group, species, optional English Name/Code, presence, mean cover
  and sorted pivot plot headings. Observations begin at row 5.
- Preserve typed numeric results, NULL blanks, original row order and unit
  identity. The source's 250-plot per-unit limit remains.
- Group-gap scanning begins at worksheet row 7, stops at the next blank group,
  and scans at most 1,000 transitions. Rows 5/6 do not receive a gap.
- Zero-count units retain explicit skipped-unit evidence. A nonempty unit with
  no observation/pivot evidence is refused, not replaced by guessed headings.
- Non-quick unit sheets retain source landscape printing, .75-inch side/bottom
  margins, .5-inch top margin and page/date footer. Freeze panes and useful
  widths are desktop presentation adaptations.
- Formula-shaped text stays literal. Reuse Environment workbook XML/text,
  UTF-16 bounds and source sheet-name validation. Refuse collisions rather than
  renaming or overwriting sheets.

## ReportSummary

`LVReportSummary` defaults to true in the source. A requested summary requires
captured summary evidence; it must never be silently omitted.

The private summary builder requires a reviewed calendar date, complete physical
Env/SU rows and Description definitions specifically owned by `USysAllSpecs`.
It preserves:

- Source labels and rows 4-13, grouping/order captions and numeric thresholds.
  Presence uses the source's divided-by-100 ratio in B9, not the stored
  percentage preference (50 becomes numeric 0.5).
- Original Env record count, including NULL plots and duplicates. The source
  labels this "Number of plots in database" and places the Env table name under
  "Vegetation table:"; these labels are not silently corrected.
- Count of all non-NULL SU PlotNumber rows, including empty text and duplicates,
  when quality filtering is off. This is not a distinct-plot count.
- Every SU membership at D:E, in captured row order, including NULL/empty cells
  and physical row identities in hidden provenance.
- Literal Description text, including empty text; NULL stays NULL/visibly blank.
  Absent metadata and absent definitions both display the source error fallback
  `Unknown`, but have different recorded states. Duplicate definitions and
  malformed/unsupported metadata refuse output; no arbitrary first match.
- The summary sheet follows unit sheets. Case-insensitive unit name collisions
  with `ReportSummary` refuse the complete workbook. Compare with
  `strings.EqualFold`, matching Excelize's worksheet resolution: lowercasing
  alone misses names such as `S` versus the Unicode long s.

Intentional presentation adaptations:

1. The source concatenates locale-dependent `Date`. The draft requires an
   explicit `YYYY-MM-DD` date (years 0100-9999). The future owned review must
   capture that date and reuse it at publication, including across midnight.
   It is not the fixed 2000-01-01 document timestamp.
2. The source writes Lifeform legend E:F, then overwrites its codes with SU
   memberships D:E. The draft preserves the exact 0-13 descriptions at G:H,
   separate from memberships, instead of reproducing this destructive overlap.
3. Source QCLV replaces the selected SU with temporary `USysDeleteMe_SU`.
   For enforced quality, summary count/memberships use the accepted report's
   filtered join occurrences, retaining duplicate weights and complete quality
   provenance. Repeated memberships keep their original physical IDs, never
   invented temporary row IDs. The visible table caption remains the original
   selected physical table, not a phantom temporary table; no tables or registry
   selection are created/changed.

The generator currently accepts only the existing unlumped preview. Its summary
therefore explicitly displays `None_Lump`; this does not infer the unmapped
Access `CurrLump` selection. Public review must require an explicit supported
unlumped scope and refuse combine-variants/lumping until implemented.

## Provenance and outstanding acceptance

Very-hidden `_VPRO_Source` stores chunked hex of the full typed preview and
layout, including reviewed date, raw Description definitions, counts and typed
memberships. Identical captured inputs produce identical bytes. All errors,
cancellation and workbook cleanup failures return no successful partial bytes.

The private publisher requires explicit `unlumped` scope and a literal default-off
`VPRO_LONG_VEGETATION_WORKBOOK` constructor gate. It captures fresh strict YAML
options/layout and complete relevant physical tables within owned context/read
snapshots. Approval covers the whole parsed configuration, raw captured tables,
scope, reviewed creation date and workbook bytes. Canonical parsed YAML authority
is intentional: comments/formatting do not invalidate review, any parsed preference
change does, including an otherwise-unreported key.

Export reuses the reviewed date instead of consulting the clock again. Fresh
initial/prelink/postlink source checks use the shared no-replace artifact publisher.
Known outcomes are `published`, `published-with-errors` and `not-published`;
published errors explicitly forbid replay. Registration remains default-off.
Frontend shared publication sessions retain unknown receipts through tab remount
and explicit acknowledgement, invalidate old preparation and require fresh review.
Unresolved irreversible calls block navigation, native close and overlapping reads;
only source/review reads are cancellable.

## Bounded acceptance

- Complete frontend580 tests, Svelte check0 errors/warnings and isolated enabled/
  workbook-off frontend/production Wails builds.
- Independent kernel/shared-source, owned-publisher and final desktop reviews;
  resolved Unicode-fold sheet collision and exact positive threshold regression.
- Twelve native cases/four normally closed owners: actual1400/600px labels and
  containment, reviewed publication/collision, held irreversible known/lost
  receipts, navigation/native-close/read overlap, remount/acknowledgement,
  independent gates and held read cancellation/stale suppression/explicit retry.
- Two actual98167-byte outputs each independently match reviewed hash/typed XML,
  eleven unit sheets,869 observation rows and ReportSummary.
- Every original disposable data/config/audit identity restores after closure;
  no database seeds, source temporary tables or audit writes. The first
  zero-output candidate was normally closed/hash-archived before replacing it
  with the correctly gated retained Strata/Code candidate.
- All-package race passes with fresh root1508.347s and680 unchanged frozen source
  identities. The [immutable101-file archive](../archives/vegetation-workbook-desktop-checkpoint/evidence-manifest.json)
  seals the exact integration evidence.

Source4 attributes, lumping, combined variants and empty pivot units remain
unavailable; no parity claim for those modes. The full forms/reports migration is
not complete.

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

## Saved Show naming boundary

The source `rptLvShowCommon` offers **Show english name** (1), **Show 8 char.
code** (2) and **Show none** (0). Its active AfterUpdate writes
`LVShowEnglishName`, clears `cmbShowField` and writes `LVShowField=""`.
Staging another choice and returning to the original can therefore produce a
field-only change; final mode inequality is not sufficient provenance.
Untouched Save and Undo must preserve the exact historical pair, including
whitespace/case/NUL in the unused field. The source getter's absent default1
overrides the exported control default2; the editor does not invent defaults.

The independently default-off `VPRO_LONG_VEGETATION_NAMING_PREFERENCES` and
`VITE_LONG_VEGETATION_NAMING_PREFERENCES` implementation is reviewed/bounded
native-accepted, not automatic Access event-timing parity. Its strict required DTO compares the integer mode and literal
string field atomically using the existing changed-only preference CAS; a local
mixed-primitive adapter keeps YAML scalars primitive, without widening other
callers. New field assignments can only clear to the source-required empty
string. Retained exact-owner Load/Save/Undo, validation and irreversible receipts
share the existing close/report/sibling/workbook barriers. New Code selection
requires the existing backend and frontend Code capabilities; an unchanged
unavailable historical mode remains a byte-exact no-op, not report enablement.

Historical selected-field mode4 remains visible/read-only/no-op/correctable,
but custom-field selection and report execution are unavailable. Its separate
source picker is a Value List (`RedBlueList;Wetland_Ind;WeedStatus`), not a
generic field picker or proof of a mandatory membership bound. Enforcement is
also separate: its callback calls Form_Load, resets `LVUnitGroups=1` and
enumerates printers. This naming editor does not reproduce those effects,
automatic registry/event timing, new calculations, data/audit writes, physical
printing or default promotion.

Naming/owned persistence/independent physical-source/reopened-workbook
race106.301s, coupled shared/group/quality race455.825s and102 coupled/1191 full
frontend cases, check0/0/private builds/production/vet/exact interface bindings
pass. Distinct119-file-predecessor review finds no significant issues. Seven
normally closed actual-main Wails owners prove27 native cases/seven once-only
commits/one independently reopened exact-SHA workbook. Proof covers independent
naming/backend/client Code gates, exact source captions/three widths/retained
lifecycle/held-warning-unknown receipts/native-close and navigation guards,
historical mode4/unavailable Code no-op/correction, explicit absent-field
refusal without default inference and changed-away/back same-mode field-only
clear. All16 originals/22 protected identities restore;225 files are sealed.
One closed zero-mutation wrong-root-report-gate build is retained in a hashed
50-file archive; failed harness runners remain failed and no successful
mutation/output is replayed. No application fix during native acceptance.
Frozen1200-input complete-source Windows race90m passes exit0 in4399.773s,
finishing2026-10-09T21:20:50.104414Z. Frozen inputs and fourteen-package
ordinary/production discovery are unchanged;225 sealed hashes and reviewed
application inputs verify with separately associated final acceptance, without
rewriting the immutable native seal. Custom-field execution and
broader workflow enablement remain excluded. Selected-field source preparation records
the separate Value List callback (nonempty sets mode4; empty retains mode)
and actual dynamic physical grouping/select/header requirement. The restored
imported fixture lacks all three suggestions; preserve explicit physical
unavailability, not invented schema, membership or output.

### Selected-field implementation frontier

A fresh271-file accepted predecessor and six canonical hashes freeze the
selected-field batch. The separate VLists master export defines26 physical
columns, including TEXT/LONG/BIT, and also lacks the three Value List suggestions.
The independent default-off paired CAS/facade, interface bindings, retained
controller and responsive field panel are implemented drafts. Nonempty Field
edits stage mode4; an empty edit retains the staged mode, including changed-away/
back provenance. Historical no-op bytes and unchanged invalid assignments remain
preserved. Physical column definitions expose `declaredType`, not inferred types
or mandatory membership. Root/sibling/workbook holds and irreversible receipts
remain separate from field validation and correction.

Expanded selected-field consumer/backend/wire race178.979s,1202 full frontend
cases, check0/0, fresh private default/enabled builds and actual production/vet
pass. The consumer tests independently check duplicate reference weights,
NULL/empty and constant list/matched fanout, all four grouping modes and scalar
kinds, exact Go/client wire and actual client proxies. Reopened worksheets retain
literal headers, formula-safe text, native Long, genuine Boolean and binary64
DOUBLE cells. Changed selected-field configuration or physical source refuses
publication with no output; exact recovery permits original approval retry,
and an existing destination is never replaced.

Four independently expected disposable DAO grouping witnesses and
actual Excel `CopyFromRecordset` output are separately associated with an exact
reopened workbook SHA. Native TEXT A/a groups together in the observed order;
LONG returns Int32/numeric cells, BIT returns Boolean/Boolean cells and DOUBLE
retains binary64 numeric values. BIT NULL insertion was observed as false; this
does not prove NULL from a left join. Both private Excel owners are closed; the
first failed unqualified-alias query remains hashed and archived.

Mode4 typed report/workbook consumers are implemented drafts behind independent
`VPRO_LONG_VEGETATION_SELECTED_FIELD` / `VITE_LONG_VEGETATION_SELECTED_FIELD`
flags; the preference editor separately requires
`VPRO_LONG_VEGETATION_SELECTED_FIELD_PREFERENCES` /
`VITE_LONG_VEGETATION_SELECTED_FIELD_PREFERENCES`. All remain default-off.
Only owned TEXT/INTEGER/BOOLEAN/REAL declarations with original text/Long/-1-or-0/
DOUBLE storage are supported. Raw field references are bounded to the26 canonical
master identifiers, three source suggestions, calibrated `AltText`/`AltLong`/
`AltBool`/`AltDouble` and the `Literal` scalar fixture identifier, with ASCII
identifier recasing only. Physical membership and declared type are still
independently required; this list does not invent columns or restrict preference
storage. Other matching physical names, including spaces, expression punctuation
and reserved/uncalibrated names, remain unavailable to report consumers because
the source concatenates an unquoted reference.

TEXT permits only one non-NULL literal per effective species identity; NULL remains
distinct. A missing scientific name uses the master Code fallback, including mixed
fallback/non-NULL identities. Multiple different TEXT representatives refuse,
even when they might be distinct native groups: no general collation comparison
is inferred from `EqualFold`. Before aggregation and threshold filtering, all four
scalar kinds validate actually joined report inputs; restored output is checked
again before transport: non-ASCII/NUL effective species and byte-distinct
ASCII identities sharing a conservative case/punctuation/space refusal key remain
unavailable. This key never groups or rewrites data. Unjoined master identities
cannot veto a report; joining an uncalibrated identity does. Arbitrary valid
Unicode selected TEXT values themselves remain literal. Unknown types,
conversions, absent physical fields,
ambiguous identifiers and mixed signed-zero representatives also refuse rather
than invent grouping. Preference storage remains literal and unrestricted.

Coupled Long Vegetation backend regression passes race1050.317s. An additional
zero-output workbook refusal selector passes3.262s for unknown provenance and
malformed/converted text, Long, Boolean and DOUBLE storage.
Initial independent review found a NULL-scientific fallback bypass and acceptance
of physical names that cannot form the source's raw reference. Both have narrowly
scoped fixes with source fallback/mixed-identity/uncalibrated TEXT and owned stored-
literal/no-output regressions. Focused follow-up resolved raw identifiers but
found a remaining mixed case-equivalent effective identity bypass. The scoped
scoped guard addresses that for all scalar kinds, including NULL selected cells.
Final-row validation alone could hide unsupported groups behind thresholds. An
operation-local projection-owned guard now runs at the shared layer planner's
actual membership/master join, after Codetype exclusions and before aggregation;
None, Strata and Lifeform adapters retain it. A cover2/threshold3 witness proves
separate A/a byte buckets disappear without this guard. Owned all-four-grouping
preview/review refusal and corrected-source retry cover filtered fallback cases.
The current selected/coupled planner race passes194.330s; production/vet pass.
An initial test-only string threshold seed failure remains preserved separately.
Combined selected/legacy race207.635s and independent unjoined/
joined scope/cancellation authority race2.019s pass with original positive master
identities untouched; production/vet pass. Prior global guard failures and the
temporary positive-fixture adaptation are preserved as failed/superseded evidence,
not accepted source normalization. The original coupled
race1050.317s and immutable review packet remain preserved separately.
Distinct focused independent review now reports no significant issues: the mixed
effective-identity and prethreshold bypasses are resolved. Disposable actual-main
Wails acceptance seals33 cases/eight normally closed owners, seven once-only paired
commits/one unchanged historical no-op and four reviewed exact-SHA primitive XLSX
outputs, independently reopened by exact data-sheet relationships. All16 originals/
22 protected identities restore;216 compact files seal. Proof includes independent
editor/report gates, three-width labels/actions, raw NUL validation/historical
omission, literal/nonmember and staged-empty mode4 preferences, actual remount/
close holds, retained unknown receipts/owned reload and held-read cancellation.
The original full scope refuses an actually joined uncalibrated non-ASCII identity.
Positive typed acceptance uses independently observed original literal identities,
narrowing only disposable SU memberships; it never rewrites ScientificName.
Configuration and Boolean source drift refuse stale actual publication RPCs;
an empty physical scope refuses preparation first. All three emit no destination.
Harness-only shape/wording/sheet-selection failures are preserved; no completed
mutation or publication was replayed and no application fix occurred during
native acceptance. Complete-source Windows race90m now passes exit0/root4473.840s,
finishing2026-10-10T01:37:38.237113Z with1212 unchanged inputs and fourteen-package
ordinary/production discovery. All216 sealed native hashes verify; final integration
is associated separately without rewriting the immutable native seal.
This is bounded default-off acceptance, not universal selected-field workflow parity. The bounded A/a
witness is not proof of every Access collation equivalence. Do not replace the
selected scalar with a caption or an untyped name string, invent absent columns,
infer a getter default or claim universal native text collation.

## Saved quality minimum boundary

The three unbound controls `SitePlotQuality`, `VegPlotQuality` and
`SoilPlotQuality` retain the **Site >= / Veg >= / Soil >=** grouping. Their
active Change callbacks assign only `DataQualityFilterSiteLV`,
`DataQualityFilterVegLV` and `DataQualityFilterSoilLV`. All three suggest
`USysTableOfLists.Item` for `PlotQualitySite`, ordered by `ItemOrder`; the
exported controls do not impose list membership or the bound Admin TEXT15
limit. The source's absent registry default is `"Poor"`, not permission to
replace missing or malformed desktop values.

Independent default-off `VPRO_LONG_VEGETATION_QUALITY_MINIMUM_PREFERENCES`/
`VITE_LONG_VEGETATION_QUALITY_MINIMUM_PREFERENCES` gates expose reviewed,
bounded native-accepted behavior, not complete source-event parity. Owned Load includes current physical
VLists suggestions with complete typed metadata. Empty suggestions remain
available; duplicate, NULL and empty items remain distinguishable. Malformed
source Unicode refuses before transport. Suggestions never substitute for
the active `DataQuality` qualification definitions.

Three-string YAML CAS preserves literal blank/case/whitespace/nonmember and
over-15-unit values. No-op persistence is byte exact, unchanged historical
assignments are omitted, and changed malformed Unicode/NUL refuses without
repair. Retained per-context drafts, validation and irreversible receipts share
the existing preference lifecycle. Root, sibling and workbook holds and
preview/review invalidation are wired; correction and Undo remain live.
Explicit Save is a desktop adaptation, not Access Change Value/Text timing
proof. Editing minima does not enable quality enforcement or inherit
Form_Load/printer/unit-group reset callbacks.

The 110-file frozen predecessor and verified canonical Form/module hashes
bound this implementation. All 93 coupled client cases and 1182 full frontend
cases, check with zero errors/warnings, private default/enabled builds and
production compilation/vet pass. Coupled preference/quality/workbook race passes
254.947s; independently seeded per-domain qualification with contradictory
suggestion ranks passes6.702s. Exact index/models/new-service binding delta and
distinct predecessor review pass with no significant issues.

Five normally closed accepted actual-main Wails owners prove25 cases/six
once-only commits and two independently reopened exact-SHA workbooks. They
cover independent gates, three-width associated labels/input/button/disclosure
visibility/containment, unrestricted literals, retained invalid/correction/
Undo/remount/read cancellation and held/warning/unknown receipts/close guards,
complete owned physical suggestions including duplicate/NULL/empty entries,
malformed TEXT correction/retry, unchanged historical NUL no-op/omission, and
independently planned active DataQuality qualification rather than contradictory
suggestion ranks. Stale approval refuses zero output in active/inactive cases.
All16 originals/22 protected identities restore;184 compact files are sealed.
An additional wrong-prerequisite frontend owner closes before any Save/output;
its35-file hashed build archive and failed harness/diagnostic receipts remain
separate. Same-owner continuations never replay successful mutations.
No application fix was needed during native acceptance. Complete-source1191-input
Windows integration passes exit0 in4350.937s, finishing2026-10-09T19:23:24.039418Z.
Frozen inputs and fourteen-package ordinary/production discovery remain unchanged;
all184 sealed hashes verify and final acceptance is separately associated without
rewriting the immutable native seal. No
default promotion, registry/data/audit writes, printing or automatic Access
Change Value/Text timing claim follows.

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

## None report unit core boundary

`rptLvUnitGroups=3` is **None report units**, not `LVGroupBy=4` None species
grouping. Its selected-SU core is an implemented, independently default-off
draft (`VPRO_LONG_VEGETATION_NONE_UNITS` and
`VITE_LONG_VEGETATION_NONE_UNITS`). Distinct exact31-path complete-core review
finds no significant issues; race497.389s/1215 frontend cases/check/private
builds/production/vet pass. Preference editing is an implemented draft with its
own backend/client gates (`VPRO_LONG_VEGETATION_UNIT_PREFERENCES` and
`VITE_LONG_VEGETATION_UNIT_PREFERENCES`). Exact33-path preference review finds no
significant issues. Bounded actual-main Wails acceptance is complete below;
complete-source1249-input integration passes exit0/root4919.559s with unchanged
inputs/discoveries and149 verified native hashes.

Source `WriteBlankSiteUnits` creates the literal **All plots** by joining current
project Env to the current selected SU. QCLV changes that current list first when
enforcement is on, so the outer Env join consumes qualified occurrences, not an
existence-only membership filter. A disposable blank DAO fixture establishes
four repeated joins plus one empty-identity join,64 qualified rows becoming128
outer rows and ASCII case-equivalent joins. NULL plot identities do not join.
This is source calibration, not complete native desktop acceptance.

The desktop adaptation retains original physical membership IDs. Separate
derived occurrence witnesses identify the outer Env row, original SU row,
optional original quality occurrence index and All plots identity. Final
denominators and cover weights use derived occurrence counts exactly once.
Original quality SiteUnits remain unchanged, including NULL. Client validation
checks unique complete join witnesses, physical IDs, derived denominators and
separate quality/None multiplicity diagnostics, including actual reactive proxies.

The workbook summary uses the same derived list/count; its source table label is
`USysSuTableDynamic_SU`, while approval still binds the complete original physical
Env/SU/reference tables. No scratch table is deleted, created or edited.
Changed Env observations reject stale approval without output; restored original
evidence permits explicit retry. Source1 selected-SU behavior is unchanged.

The source-labelled **Unit Groups** editor retains1 Site unit table/2 Selected
fields/3 None. Only1/3 can be newly proposed; None requires independent backend
and client consumer availability. An unchanged historical unavailable2/3 can be
read/retained without rewriting YAML or enabling its report. Strict requests
reject unknown/duplicate/escaped duplicate keys, malformed Unicode and noninteger
choices. Shared compare-and-set owns only `LVUnitGroups`; all unrelated settings,
including enforcement and retained sibling drafts, remain unchanged.

Explicit desktop Save/Undo is the event adaptation, not automatic Access
AfterUpdate/Click timing. No selector window or scratch deletion is introduced.
Enforce rules still performs its paired unit1 reset; an old unit draft must
observe/correct that authority through owned Load/CAS rather than overwrite it.
Per-context draft errors survive remount and block report/workbook/close actions
while correction/Undo remain live. Known/unknown persistence receipts remain
separate from draft validity and cannot be erased by acknowledgement.
Coupled source/CAS/workbook race86.878s,1222 full frontend cases/check/private
builds/production/vet and cleanup/committed-cancel race2.263s pass.
Seven normally closed actual-main Wails owners prove17 cases/four once-only
commits/two exact-byte historical unavailable no-ops/one independently reopened
exact-SHA All plots workbook. Native proof includes independent gates, three-width
labels, actual reactive report handling, invalid/remounted drafts, cancelled late
reads, held known/warning/unknown receipts, symmetric sibling holds, exact source1
approval/output recovery, QCLV-first indexed witnesses and enforcement-reset
stale-CAS refusal. The retained native Summary_SU independently joins five times
over00337/108050; NULL/orphan rows do not join. This is distinct from the two-plot
backend fixture. Test-only cardinality/wire-shape assumptions were corrected on
the same verified owners without replaying successful writes; no application
fix was needed. All16 originals/22 protected identities restore and149 files seal.
Complete-source1249-input integration passes exit0/root4919.559s; inputs,
ordinary/production discovery and immutable native hashes verify separately.

Source2 selected fields and source3's all-project fallback remain separate
unavailable work. Distinct case-equivalent/non-ASCII plot joins and ambiguous
physical Env/Admin quality links still refuse rather than guessing broader
collation or parent fanout. No printer setter or physical printing is enabled.

### Selected unit fields preparation

The source2 selector reads the actual `UsysEnv` Field List and starts its ordered
Selected List with Zone/SubZone/SiteSeries. Build concatenates bracketed selected
fields with `&` and no separator. Selected-SU scope is a LEFT JOIN to `UsysEnv`,
which itself joins Env to Admin: unmatched memberships survive and duplicate
physical parents can multiply derived rows. Source report enforcement subsequently
prunes prebuilt memberships by qualified-plot existence, unlike None3's join to
each quality occurrence.

Static preparation freezes12 canonical hashes. Two closed blank DAO attempts
retain completed projection/join/pruning observations, but their runners remain
failed after parameter syntax and an incorrect expected-refusal assumption.
Read-only inspection of the second closed fixture establishes that a256-character
concatenation is silently stored as255 characters in TEXT255. A desktop candidate
must reject new over255 UTF-16-unit labels before mutation, not inherit truncation.
All-NULL concatenands remain NULL; NULL with empty text yields empty text.
Concatenation without separators can intentionally produce equal labels from
different field values. Read-only DAO metadata from an unchanged disposable
canonical byte-copy now proves dynamic PlotNumber/SiteUnit/Group
AllowZeroLength=false and Required=false, with nonunique indexes. A separate
exact-property blank DAO append drops empty-label rows under default flags,
whereas dbFailOnError refuses with zero inserted rows. NULL remains accepted;
empty must never be normalized to NULL. These are engine observations, not
DoCmd.RunSQL warning-suppression/partial-error UX proof. Full scalar/date/locale
combinations remain unproven. No source2 consumer or selector is enabled.

The pure planner now consumes explicit ordered TEXT/Long/Boolean field names from
physical Env/Admin, retaining original SU/Env/Admin witnesses and outer-SU
orphans/NULL plots. QCLV applies only qualified-plot existence, preserving prepared
peer multiplicity without multiplying it by quality occurrences. Unsupported
REAL/date fields, absent/ambiguous names, malformed/NUL text, new over255 UTF-16
labels and uncalibrated plot identities refuse with no partial plan. Focused
race2.095s and expanded source2/unchanged None planner race43.708s plus production/
vet pass. Exact-predecessor review identified premature quality pruning bypassing
excluded-plot overlength refusal. Every derived label now builds/validates before
quality-existence pruning; empty and other-qualified-plot cases explicitly refuse
the same256-unit proposal. Corrected source2/None race46.040s/production/vet pass;
focused independent follow-up confirms resolution with no significant new issues.
Complete-source planner integration1251 inputs passes exit0 with unchanged inputs/
discovery and predecessor native hashes. This is a raw read-only SELECT
projection, including empty versus NULL, not proof of destination acceptance:
a future consumer must explicitly refuse unsafe empty/new overlength destination
values without source row dropping/truncation. This planner has
no registered consumer/editor, no new YAML storage and no scratch writes;
source2 workflow availability is unchanged.

The initial consumer core separately validates destination constraints before
quality pruning, then prepares detached derived memberships for the shared report
kernel. All four species-grouping adapters use the same reviewed membership
boundary: repeated Env/Admin joins count once each, displayed membership ID lists
retain unique original physical IDs, and a single original member can participate
in different derived labels. Derived-join diagnostics are separate from original
physical membership multiplicity. NULL/orphan witnesses remain explicit and no
quality weights are applied again. Coupled source2/ordinary grouping race54.310s,
production and vet pass. Exact core review identified quadratic diagnostic
accumulation; emission now occurs once after completed membership counts.
The actual4000-join/2000-diagnostic kernel allocates18,568,464 bytes under32MiB;
an exact faulty predecessor overlay allocates1,035,314,616 bytes and fails the
same regression. Corrected coupled race58.109s/production/vet pass; focused
independent follow-up confirms resolution with no significant new issues.
No service/transport/editor registration or workflow availability is introduced
by this kernel batch.

Internal owned source2 reads now include original Env/Admin with quality off,
transport nullable original SU/Env/Admin witnesses and explicit ordered field
settings, and physically recheck derived workbook summary projection. Attached/
external reads, independent reopened Alpha/Beta/ReportSummary/provenance output,
field drift/refusal/exact recovery, and QCLV9 provenance versus2 source2 derived
rows are covered. Broader report/options/grouping/workbook race316.407s,
production/vet and1223 frontend cases/check/private default-enabled builds pass.
Warning-free interface generation adds only substantive index/models DTO deltas;
other binding bytes/newlines remain unchanged. Frontend deliberately rejects
draft selected-unit DTOs instead of accepting them as ordinary reports.
No selected-field selector/service is registered and config `LVUnitGroups=2`
continues to refuse. Exact18-path owned-source/summary review found no significant
issues. A distinct default-off client validator now checks complete nullable
original tuples/Cartesian joins, destination bounds, per-unit original IDs,
derived denominators/exclusions/multiplicity and original QCLV existence.
Independent strict-Go JSON matches the actual prepared kernel/transport;
actual Svelte proxies and derived workbook summary count are covered.
Selected-unit/None-wire race5.774s/production/vet,40 focused/1227 full frontend
cases/check/private default build pass. Exact6-path independent client review
found no significant issues. No panel calls the new argument: these are reviewed
internal boundaries, not session selection authority or source2 native acceptance.

The unregistered session selector core now owns ordered field drafts/revision CAS
and immutable Build receipts separately. Read-only Build binds complete original
Env/Admin/SU rows, context, configuration and ordered identifiers; it never saves
LVUnitGroups or scratch/YAML fields. Get observes source equality independently
of destination validation so invalid new labels/empty membership sets retain the
old receipt and correction identity while reporting it noncurrent. Build/output
validation remains strict; cancellation/cleanup/stale revision cannot install.
Retired contexts release receipts under owned switch locks and installation
retains only the current context. Initial review's two substantive findings are
fixed; focused independent follow-up found no significant new issues.
Source2/context race8.551s/production/vet pass. Main/panel/preference/output wiring
remains pending and source2 is not native-accepted.

Owned options/public/private preview/workbook now decode saved2 only with a
current session Build and independent backend consumer flag. Read-only approval
material normalizes only the explicit planned LVUnitGroups choice2, allowing its
own reviewed CAS transition while binding every other setting/physical row/order.
Legacy decoder/direct CAS deny new2; historical2 no-op/source1 recovery stay live.
Initial exact12-path authority review found revocation before workbook link and
a configuration gap before integer persistence. Corrected field installations
take owner then ledger so publication retains authority through link/postcheck;
the scoped integer writer checks complete normalized config under its existing
config lease. Deterministic actual publication/peer threshold race14.121s passes,
and exact negative overlays detect both faults. Corrected broader race376.354s/
production/vet passes; exact4-path independent follow-up resolves both/no new
issues. Main now independently gates the selected-unit consumer and readonly
selector service; interface bindings and the retained responsive Field List/
Selected List/Build SU List panel are implemented. A shared strict production-Go
DTO validates physical `declaredType` definitions and original joined witnesses
through the actual Svelte proxy boundary. Selector changes invalidate pending
options/preview/workbook reads. Cancelled/failed acknowledgements require owned
Load before retry; no source Save is replayed. Build does not save Unit Groups;
Load saved Unit Groups explicitly refreshes availability before choosing and
saving source2. Existing mutation receipts remain separately retained.
Selector/authority race30.224s, production/vet and full frontend tests/check/private
default/enabled builds pass. Exact33-path independent selector review and
exact2-path readonly recovery/root identity/provenance follow-ups report no
significant issues. Seven normally closed accepted actual-main owners prove13
native cases, four once-only commits, one exact-byte historical2 no-op and one
once-only UI-published qualified workbook independently reopened to the reviewed
SHA. Actual proxy/three-width selector controls, cancellation/remount/root close/
sibling holds, original physical joins, QCLV existence filtering, source1 recovery
and enforcement-reset/stale-CAS refusal are verified. All16 originals/22 protected
identities restore;236 files are sealed. An unfiltered orphan-derived empty unit
correctly retains the existing explicit empty-row/pivot workbook refusal; preview
availability is not workbook acceptance for that case. Complete-source1274-input
Windows race90m passes exit0 in4757.152s with unchanged inputs/package discovery;
all236 native sealed files verify, with final integration associated separately
without rewriting the immutable native seal. Independent gates stay default-off and broader
all-project/combined/lumped/printing/replacement acceptance remains incomplete.

### Whole-project projection boundary

The unregistered whole-project kernel implements the source selector's UsysEnv
Env INNER JOIN Admin branch separately from WriteBlankSiteUnits' Env-only
fallback. Selected-field occurrences carry original Env/Admin identities;
None occurrences carry only original Env identities, including unmatched,
duplicate, NULL and empty plot values. No SU membership is fabricated.
Ordered fields reuse calibrated scalar/identity/new-label guards, preserving
NULL versus empty, Boolean -1/0 and the 255 UTF-16-unit concatenated bound.
Focused project/selected-unit race30.143s, production build/vet and exact3-path
independent review pass. Source2 full fanout, source3 original rows, detachment,
complete physical rows, malformed/unmatched selected values and cancellation
at every observed check are covered. Public routing and quality inference are
not enabled: QCLV still reads the original selected SU before report mode
selection; a None_SU project fallback does not establish quality authority.

The qualified-project source2 kernel first builds the complete original project
projection, then reuses the accepted selected-SU projection's original quality
witness validation on the actual selected SU. It retains project tuples by
qualified plot existence only, without multiplying them by quality/reference/
membership fanout. Empty qualification stays empty; forged original witnesses
and invalid unqualified Build rows refuse before any output. Focused project/
selected-unit race29.982s, private production build/vet and exact2-path independent
review pass. Complete-source1278-input Windows race90m passes exit0 in4901.535s
with unchanged inputs/discovery. No public
project scope or source3 all-project quality has been enabled.

Shared physical Veg MAX/layer entry reduction now returns typed entry data
separately from selected-SU membership ownership. Ordinary preparation retains
its original schemas, membership identities, weights and output shape. Project
source2 preparation requires an actual selected SU, retains original project
Env/Admin tuples and reduces entries once per scoped plot/species, independently
of derived tuple or QCLV fanout. Focused project/selected/ordinary preparation/
None adapter race34.401s, private production build/vet and exact3-path independent
review pass; complete-source1280-input Windows race90m passes exit0 in5115.460s
with unchanged inputs/discovery. This is still
an unregistered kernel.

The generic ConvertProVeg2V2 chooses TempReportVeg when CurrPlotlist is not None
and otherwise retains its input argument. The active LongVegReport caller
always runs CreateTempVeg and assigns VegTableIn=TempReportVeg before passing
that argument to layer/None conversion. Thus no-SU LongVegReport also uses MAX;
an earlier raw-report inference is superseded by the complete caller trace.
The caller-corrected project constructor now permits explicit sourceSU=None
without fabricated SU data and without QCLV. Source3 preparation retains every
original Env occurrence and separate MAX entries. Focused race27.551s/private
production build/vet and exact2-path independent review pass. Typed project
kernel adapters pass affected race10.916s/production build/vet and exact6-path
independent review with exact original denominator/mean/pivot expectations.
They retain unit indexes into original Env/Admin or Env
tuples; legacy selected-SU transport refuses those reports until a distinct
public wire is reviewed. The owned physical project reader passes affected
race153.929s/private production build/vet and exact3-path independent review.
Actual attached/external and None contexts retain duplicate original tuples
across all four adapters; original-SU QCLV existence pruning does not weight
project denominators. Unsupported duplicate-parent quality remains unavailable.
Malformed UTF-8, missing physical tables, ownership and cancellation fail
explicitly; corrected source retries preserve pinned attachments.

The unregistered exclusive session-scope service uses explicit selected-list,
project-selected-fields and project-all-plots intent with canonical revision
CAS, retained independent field drafts and one active Build authority.
Source3 still requires the original None SU context; no-SU quality is unavailable.
Hashes bind physical rows, context/path ownership, complete configuration,
scope/revision and ordered fields. Scope installs serialize with final
publication through the SQLite owner mutex. Completed read cleanup precedes
ledger changes and ordinary unit-preference CAS.
Independent review identified a final Build source/configuration gap and a
pre-owner selected-getter scope gap. Final owned hash revalidation and an owned
getter scope recheck resolve both; deterministic gap/cleanup regressions,
affected race77.966s/private production build/vet and focused independent
follow-up pass. These completed source observations do not prevent writes
through already-borrowed pools. No public registration, project transport,
client/workbook/summary routing or native acceptance is claimed.
Complete-source1288-input integration passes exit0 in5108.634309s with unchanged
inputs/discovery. The distinct project transport now retains explicit sourceKind,
full original tuples and emitted/excluded original tuple indexes. Named units
without vegetation remain excluded exactly as the kernel diagnoses; transport
does not invent empty output units. Race50.537s/build/vet/exact3-path review pass.
Shared production-Go source2/source3 fixtures and interface bindings (-ts -i)
pass race11.456s,43 targeted/1231 full frontend cases/check0/0/isolated build and
exact6-path review. The ordinary client refuses project provenance, including
actual reactive proxies, until coherent project consumers exist.

Unregistered scope-owned options/preview require exact requested revision/Build
hash and an actually saved matching unit choice. Normalizing the planned Build
choice is not a preference Save. The original full project Build remains separate
from QCLV existence-pruned output and originalSU/reference witnesses. Focused
race62.663s/build/vet/exact3-path review pass. The project unit Save uses strict
scope/revision/hash/expected/proposed requests, completes initial and final
physical read cleanup before YAML CAS, checks complete normalized configuration
inside the final preference guard and holds the scope/publication owner lease.
Exact-byte no-ops, cleanup refusal, collision/retry and committed cancellation
receipts pass race59.748s/build/vet/exact2-path review. This backend is unregistered;
client scope/editor/root/lifecycle, project workbook/summary/publication and
actual-main native acceptance remain incomplete.

The1297-input complete-source Windows integration passes exit0 in4506.483215s
with unchanged inputs/discovery. Separate default-off project preview validation
reuses the reviewed ordinary settings, quality and row boundaries, while
retaining original project tuple ownership and exact emitted/excluded indexes.
OriginalSU qualification is existence evidence, never denominator weighting.
Source2/source3/qualified production-Go fixtures, actual Svelte proxies and
malformed scope/fanout/index/row cases pass affected race32.256s/build/vet,
full1238 frontend cases/check0/0/isolated build16.09s and exact7-path review.
The public project scope editor, workbook routing and native workflow remain
unavailable; this validator alone does not enable them.

The unregistered project field catalogue reads original physical Env/Admin
schemas without reading row values or substituting filtered TEMP views. It
preserves original/duplicate declarations and disabled uncalibrated field types
through the shared selected-field policy. Invalid physical row data cannot hide
the correction catalogue or authorize a Build. Owner/snapshot cleanup,
attached/external/None contexts, schema/view/missing-table correction, zero
writes and legacy catalogue equivalence pass race8.361s/build/vet/exact3-path
review; in-flight read cancellation/retry passes race2.384s.

The unregistered scope-owned Unit review requires exact requested revision/hash
and completed approved physical/config observation. It returns actual saved
LVUnitGroups from that same config, not the normalized planned Build choice;
thus a caller can obtain the real expected value for CAS before output becomes
available. Only the active source2/source3 choice is selectable. Site unit table
and the other project intent require an explicit scope switch. Malformed saved
modes, complete source/config drift, retired intent, cleanup/cancellation and
strict request failures refuse zero-success output. Read/CAS/preview,
byte-preserving zero writes and correction/retry pass race27.972s/build/vet/
exact2-path review. These APIs do not register or enable a public workflow.

Default-off client authority guards now validate exact scope/revision/Build
wrappers for field catalogue, options, preview, actual Unit preference reads and
mutation outcomes. They compare complete independently requested Build tuples;
unqualified output must match all of them, while originalSU-QCLV output must
match the exact qualified-plot subset without changing labels or physical peers.
Source3 retains independent source2 field drafts without applying them. Saved
preference values remain separate from exclusive availability and irreversible
committed cancellation/no-op/refusal receipts. Strict production-Go DTO fixtures
and physical projection pass race13.635s/build/vet;1245 full frontend cases,
check0/0, isolated build6.25s and exact9-path independent review pass.
Literal fixture digests exercise wire shape only; real complete approval hashes
remain covered by owned backend tests. Seven nullable interface DTO additions
match private Wails -ts -i output. The generator's overlay scans did not include
the new service; isolated private source generation produced60 services/322
methods/339 models with unchanged25 embed patterns. No public service or root
capability is enabled. Retained client sessions/editor lifecycle and coherent
workbook/publication/native routing remain the next implementation surfaces.

The default-off retained scope session preserves per-context local source2 field
drafts, validation and original server identity across remount/Load/scope switches.
Source3 retains inactive source2 drafts without applying them to output; dirty
inactive drafts still block native close. Proxy DTOs detach before the awaited
validation boundary, then generation is rechecked before publishing state.
Exact switch/Set-fields/Build acknowledgements and revision exhaustion guards
prevent partial or silently rewritten session plans. Initial readonly failures
do not invent mutation uncertainty or trap native close.

Cancel invalidates local generations, never cancels/replays underlying mutation
RPCs. Independent review found an early Load could clear uncertainty before a
held cancelled switch mutated the server. Outstanding promises and read-start/
settlement epochs now prevent that: Load begun while a mutation is pending, or
whose observation precedes settlement during validation, cannot restore output,
close or mutation readiness. A fresh owned Load after settlement is required.
Switch/field/held-validation interleavings explicitly cover the finding.
Full1258 frontend cases/check0/0/isolated build6.69s and focused independent
review follow-up accept the bounded fixed lifecycle.

The paired default-off Unit session reuses generic preference Save serialization,
unknown/known outcomes and acknowledgement barriers, while retaining complete
project revision/hash/expected/proposed receipts separately. It loads actual
saved values, requires fresh independently matching scope authority for Save and
permits only the active scope's choice. Save cannot be cancelled or bypassed
through generic preference Save. Scope changes and acknowledgement preserve
drafts and irreversible receipts; dirty reloads preserve/revalidate drafts.
Proxy DTOs detach before awaited read validation and stale generations cannot
publish into a changed scope.

Independent review found unknown Save recovery could lose the proposed draft
when actual values remained unchanged: the generic unknown outcome clears saved
state, so a saved-dependent preservation test incorrectly reset the draft.
Initialized unknown-authority drafts now survive recovery and revalidate against
actual values/current scope. Tests independently cover recovered actual1 versus
proposed2, scope3 correction after an unknown source2 request, and actual2
recovery without replay. Strict production-Go requests/owned CAS pass
race13.441s/build/vet;1268 full frontend cases/check0/0/isolated build5.48s
and exact7-path independent review/focused follow-up accept the fixed session.
Caller-owned project editor panels now bind these retained sessions without
public service imports or registration. They retain explicit source choice,
responsive Field List/Selected List order, disabled physical definitions and
an inactive field-draft disclosure. Actual saved mode remains visible and is
not normalized to the current planned Build. Full scoped mutation receipts and
safety feedback precede controls; correction/Undo and acknowledgement remain
available with distinct operation/mutation holds.

Cancel/unmount drops local generations and caller-owned read getter requests;
it never cancels/replays issued scope, Set-fields, Build or preference mutations.
Immutable DTO views use raw state. SSR/extracted-handler tests plus the retained
session regressions pass31 focused cases; full1276 frontend cases/check0/0/
isolated build5.13s pass. Exact4-path independent review finds no significant
issues. These are unmounted components, not actual client-runtime or native
acceptance. Public/root capability, coherent sibling barriers, workbook/
publication and actual-main Wails checks remain incomplete and unavailable.

The separate pure project workbook entrypoint now consumes the original typed
kernel and current scope/Build/settings through the shared XLSX renderer. Actual
None ownership retains an empty SU path; legacy selected-list workbook calls
explicitly refuse project provenance. Unqualified tuples match the complete
Build; qualified tuples match its exact original-SU plot-existence subset.
Original tuple denominators, labels, scalar output and worksheet guards remain
unchanged. Scope/transport/settings receipts detach from caller objects.

Independent XLSX reopening proves source2/source3/qualified values and counts,
including eight original quality occurrences versus one qualified project tuple
and two complete Build tuples. Affected project/transport/workbook race229.654s,
final DTO-detachment supplement2.241s, private production build/vet and exact
three-path independent read-only review pass. Requested project summaries require
the distinct original-project evidence entrypoint; missing evidence still refuses.
No public registration, publication/root routing or native acceptance is granted.

Distinct project summary evidence now recomputes complete original Env/Admin
field tuples or original Env-only rows with the proven planners, compares the
independent Build and exact original-SU qualified subset, and retains original
Env counts plus NULL/empty/duplicate Description states. Project tuples remain
separate from physical selected-list memberships. Shared header extraction
preserves ordinary summary output. Affected race13.339s/private production
build/vet and exact3-path independent review pass.

Source summary B12 counts the dynamic report unit list, but D:E separately reads
the current physical site-unit list. Canonical exports contain no `None_SU`
definition. Derived-project D:E is an explicit desktop adaptation with original
Env/Admin provenance, not literal source summary execution.

The explicit project with-summary renderer now writes this distinct evidence
and marks the D:E adaptation visibly at D3. Source labels/date/thresholds,
original Env count, derived non-NULL count and native Description remain intact.
Hidden project provenance retains complete scope/Build and original summary
tuples/quality separately; ordinary hidden metadata stays exactly Preview/Layout.
Missing, mixed, unrequested or mismatched summary evidence refuses zero output.
Independent source2/source3/qualified XLSX reopening, affected race234.343s/
private build/vet and exact4-path independent review pass. Capability/root
routing and actual-main native acceptance remain unavailable.

The project workbook service requires independent project-scope
and workbook capabilities plus strict revision/Build hash/unlumped/date/
workbook approval/destination requests. Owned review uses one config snapshot
and completed caller-owned physical snapshot for original report, complete
scope Build and native Description summary. The domain-separated approval binds
complete raw/config/metadata/date/scope and exact workbook bytes.

Publication holds the owner lease through the no-replace link and rereads exact
scope/source before linking. This is observation-time equality, not a write
lock on already borrowed physical pools. Postcommit drift/cancellation/cleanup
retains the committed path/SHA and explicit do-not-replay error. Standard
`not-published`/`published`/`published-with-errors` receipts remain distinct;
uncommitted staged witnesses may retain planned path/SHA, never commit authority.

Attached/external/actual None contexts, strict request/capability/saved-mode/
stale-scope refusals, once-only exact-SHA publication, original-SU qualification,
metadata drift, link lease and snapshot/cancel/corruption/retry checks pass
affected race220.163s/private production build/vet/exact3-path independent review.
Normal and qualified publication leave disposable source/config bytes unchanged.
Actual main registers both the project scope and project workbook services with
one startup parse of `VPRO_LONG_VEGETATION_PROJECT_SCOPE`. This strict immutable
flag defaults off and accepts only literal `true`/`false`. Project workbook
publication additionally reuses the independently parsed existing workbook
capability; the project flag never grants workbook publication by itself.
Actual-source Wails `-ts -i` generation discovers61 services/324 methods/341
models without warnings and preserves25 root embed patterns. Installation adds
only two service wrappers, two nullable interface DTOs and their exports.
Registration race2.051s, production build/vet,1276 frontend cases/check0/0,
isolated build5.33s and exact7-path independent review pass. Review explicitly
does not claim actual-main dispatch/default-off native denial.
The strict distinct project workbook client now requires independently current
scope/revision/complete Build, then validates exact original-SU-qualified output
tuples, separate original Env counts and native Description evidence. Ordinary
date/Description/worksheet guards are shared without changing selected-list
refusal or its output contract. The module uses `.svelte.ts` for typed,
compiler-supported `$state.snapshot` before defensive cloning; actual client
runtime proxy tests exercise that boundary. The retained per-context publication
session sends the full revision/Build hash/unlumped/date/workbook approval/
destination request, consumes review once, and retains unknown/committed-warning
receipts through acknowledgement. Explicit fresh review is required for retry.
Strict production-Go renderer/summary/request/outcome fixtures, affected
race16.969s/private build/vet,30 targeted/1284 full frontend cases/check0/0,
isolated build5.06s/exact7-path independent review pass.
The retained per-physical-context workspace and distinct SDK routing are
independently reviewed. External sibling invalidation revokes borrowed scope
and Unit observations, including during pending Save or unacknowledged receipt,
without cancelling issued mutations or erasing drafts/receipts. A fresh owned
scope Load/Build and actual saved Unit reload are required before output.
Workspace guards run before underlying session startup notifications. Inactive
source2 correction drafts retain strict close holds without blocking source3
Unit Save. All four review findings are closed; final47 focused cases and
read-only follow-up pass.

Coherent caller-owned scope/Unit panels and report/root mounting are implemented
behind default-off capabilities, including unmounted root holds and separate
project options/preview/workbook/publication routing. All1321 frontend cases,
check0/0/private actual-main builds and exact22-path independent review/focused
follow-ups pass. Ordinary panels inherit project operation/mutation holds while
retained receipt acknowledgement remains reachable. Unmounted project holds
block all three root background readers. Output unavailability alone does not
hold close; actual work/drafts/unknown mutation authority/receipts do. Workspace
report getters expose Cancel, without making publication cancellable.

Eight normally closed disposable owners prove23 bounded native cases and four
once-only Unit commits/four once-only independently reopened exact-SHA
workbooks. Original52-row Env/Admin project joins, actual None/Env-only scope
and saved3, original-SU QCLV plot-existence pruning, nine visible associated
controls at three widths, held scope/ordinary sibling/native-close guards,
actual getter Cancel/destroy/late completion/remount, unknown Save and
committed-warning publication receipts are covered. All16 original/22 protected
identities restore; no extra physical SQLite database or native owner remains.
The289-file immutable seal includes hashed closed-build archives and excluded
failure evidence. The zero-write backend-off close predecessor is failed,
not accepted; corrected successors close normally. Transport failure/warning
injections occur only after independently observed actual commit/link, never
instead of proving those effects. No completed mutation/publication is replayed.
Fresh corrected1333-input complete-source race passes exit0 in4069.67047s,
finishing2026-10-10T22:11:38.375650Z, with unchanged inputs and ordinary/production
package/embed discovery. All289 native sealed files and22 reviewed inputs verify;
final acceptance is separately associated without rewriting the native seal.
No default availability or whole-workflow/replacement parity is claimed.

The next combined/lumped calibration remains unavailable: two bounded disposable
DAO attempts stop before literal comparison/join/SUM observation. Actual local
table metadata confirms the distinct TEXT8/nonunique and TEXT50/unique profiles,
nullable BOOLEAN and AllowZeroLength=false; this is not arithmetic or callback
parity. Both failed fixtures retain closed cleanup and unchanged canonical
hashes; no third oracle debugging attempt is inferred.

Empty-output source tracing separates units admitted by the original raw
UsysVeg join from absent-vegetation units and admitted units filtered to zero
final species. The source creates the admitted worksheet before copying the
final recordset, but actual zero-row pivot fields and Excel SpecialCells behavior
remain unverified. Neither unconditional blank headers nor silent skipping of
all empty units is an accepted replacement behavior.

A separate bounded DAO admission/count fixture passes eight independently
specified cases and24 readonly reopened queries, with canonical hashes unchanged
and no fixture lock or Access/Excel owner. `SetCurrentProject` rewires `UsysVeg`
to all physical current-project Veg rows, independently of `UsysEnv`'s
Env/Admin join. An all-NULL-cover raw row still admits a non-NULL unit; duplicate
raw rows do not multiply its denominator. `PlotCount` counts physical SU tuples
including a NULL PlotNumber, whereas the final NULL-unit pass counts non-NULL
PlotNumber even without raw vegetation. A NULL-only scope still reaches the
initial no-data branch before Excel. The fixture projects the source query
operands, not a complete live report, empty-pivot metadata or successful Excel
output; those exclusions remain.

The separate empty Excel constants probe exhausted its two-attempt debugging
budget at the host's typed `CutCopyMode` setter, which rejects VBA False and
numeric zero as undefined interop enum values. Neither attempt establishes
source Excel failure or successful output. Both closed failed fixtures and
scripts retain hashes; fresh PID/DAO-lock absence and unchanged source hashes
are verified. No third attempt or empty-output enabling is implied.

## Summary Vegetation admission prerequisite

Summary Vegetation is a separate priority report, not Long Vegetation with a
transposed worksheet. Its private normal-SU admission read model uses raw current
project vegetation to admit non-NULL unit columns. `SV1` counts non-NULL
`PlotNumber` across the complete SU unit, including members without raw vegetation;
duplicate raw rows do not multiply that count. Physical membership IDs and raw
admission witnesses are retained separately. The original250-column preflight
counts all distinct SU groups, including NULL and orphan groups, not plots within
one unit.

`prepareSummaryVegetationUnits` is a pure, unexposed literal-identity kernel.
It does not create a workbook, infer zero-row pivot headers, implement Access
database collation, filter quality or change preferences. Empty admission is an
explicit empty read model. A future public caller must establish its supported
identity/collation scope and independently validate the final value/output plan.
The existing `prepareVegetationLifeformReferences` is the reusable five-field
filtered master/personal UNION boundary; no second summary UNION helper is added.
Three separate DAO projection cases prove NULL/s exclusion, whole-row duplicate
collapse, distinct same-code definition fanout and literal unmatched-species
fallback, with readonly reopens and unchanged original hashes. These observations
do not prove the complete summary crosstab or Excel behavior.

The private unit kernel passes focused coupled Go race3.156s, root vet and
production compilation. Explicit tests cover250/251 original groups, NULL and
orphan groups,251 members within one unit, raw duplicate witnesses, strict
physical identities, result independence and cancellation at every observed
checkpoint. Independent exact-new-file review reports no significant issues.
Fresh1335-input complete-source Windows race90m passes exit0 in4536.285859s,
finishing2026-10-10T23:33:22.211985Z, with unchanged inputs and discovery.
All1335 current inputs and two reviewed inputs reverify; final acceptance is
separately associated. No frontend/binding/public service is added.

The original `vpRoundUp2` body is separately calibrated in an isolated disposable
Access database:13 actual observations and13 readonly reopens pass, with source
body reexport equality and independently reopened exact DOUBLE input bits.
NULL stays NULL; negative, zero and inputs below0.01 return numeric0.01, whereas
inputs at/above0.01 return two-decimal strings. Source1.005 and both adjacent
DOUBLEs return `"1.01"`. Eleven canonical hashes remain unchanged; the owned
Access PID and DAO lock are absent after normal Quit. This proves the original
mixed-Variant helper only, not the summary query's aggregate, text criteria,
pivot field typing or Excel output.

Separate actual source-query calibration now proves the projected normal-SU,
Layer, DisplayValue6 path for both source Average1/2 branches, with readonly
reopens. `SV1` yields A4/B1: NULL PlotNumber does not contribute, the no-vegetation
member does, and duplicated P1 weights cover joins. A receives `"10.00"` by
nPlots versus `"13.33"` Characteristic; B receives `"5.00"`. A synonym removed
from the filtered reference UNION is unmatched at the final join, so its literal
species code remains. All five observed output fields (Layer, Spp, Order, A, B)
are DAO TEXT. Consequently `Max(MyCover)` is lexical: `"5.00"` dominates
`"10.00"` and `"13.33"` for Order. Do not replace that technical source column
with a numeric maximum while claiming literal query parity.

Both normally quit Access owners and DAO locks are freshly verified absent.
The first numeric-Order expected-result failure is preserved separately; its
actual text witness informed the independently checked final successor.
Canonical sources remain unchanged. These are source-query projections, not
the complete source report, empty-pivot handling, regional collation or Excel
output/printing acceptance.

### Private summary scalar cover cells

`calculateSummaryVegetationCover` consumes independently owned joined Count,
Sum/Avg and positive original nPlots. It does not select source rows, perform SUM,
infer crosstab field types or rank the final table. Average1 divides source Sum
by nPlots; Average2 uses the separately observed Avg. It reuses the calibrated
fifteen-significant-digit decimal formatter and preserves `vpRoundUp2`'s original
NULL/numeric-floor/formatted-text distinctions before pivot coercion. An absent
aggregate (Count0 with NULL Sum/Avg) is represented as NULL, not fabricated zero.
Duplicate cover counts may exceed nPlots; no lumping cap99 is introduced.

A separate six-case DAO predicate calibration verifies the exact
`Format(Count/nPlots,'#.####')>0` boundary:1/19999 and1/20000 include;1/20001,
1/2147483647 and standalone zero produce empty formatted text and a numeric
criteria type-mismatch, not successful false. Readonly reopens retain those
outcomes. The private scalar kernel explicitly refuses a nonzero count whose
format becomes empty rather than silently returning NULL. This does not claim
the complete source query's evaluation order for absent aggregate groups.
Malformed/inconsistent/nonfinite counts or aggregates refuse without partial
output. No public service, preference mutation, workbook or UI is enabled.

### Certified private summary aggregation

Seven disposable DAO SINGLE-backed Count/Sum/Avg cases and readonly reopens
distinguish ordinary arithmetic from order-sensitive DOUBLE accumulation.
Source `(2^100,1,-2^100)` yields0 while `(2^100,-2^100,1)` yields1. An exact
rational sum alone therefore cannot establish ACE parity for arbitrary covers.

`aggregateSummaryVegetationCovers` takes already joined, already SINGLE-staged
nullable covers and an independently owned nPlots. It rejects unstaged or
malformed values and admits only a conservative order-independent domain: express
each nonzero cover in the finest shared binary unit and require the sum of
absolute integer coefficients to be at most2^53. Every possible partial sum
then fits exact DOUBLE precision. The result retains non-NULL Count, exact
DOUBLE Sum and Sum/Count Avg for the separately calibrated cell function.
The bound is inclusive, cancellation cannot disguise an unsafe coefficient
range, and large common binary units are not rejected merely for magnitude.
Unsupported arithmetic refuses explicitly without partial aggregates.

This is a private narrower-domain calculation, not general Access aggregate
replacement. The caller still owns source join multiplicity, raw admission,
grouping/collation, unit denominators and crosstab output typing. No source
ordering is invented to recover an uncertified sum; no public report is enabled.

### Summary pivot typing and final-empty columns

Seven new projected original normal-SU/layer/DisplayValue6 query scenarios
independently reopen and seal with unchanged canonical hashes and normally
closed owned Access/DAO state. Both Average modes produce TEXT cells and TEXT
Order even when every `vpRoundUp2` result is the numeric0.01 floor. Mixed
numeric-floor/formatted-string helper results likewise become TEXT; lexical
Order remains5.00 above0.01.

All-NULL covers retain A/B pivot fields but produce zero SV3 rows without the
standalone count-mask criteria error. Mixed NULL/non-NULL covers retain A/B,
with a NULL A cell and5.00 B cell; the all-NULL species row disappears through
the original non-NULL Order filter. A raw-admitted B unit whose staged vegetation
is absent remains in SV1 with nPlots1 but disappears from SV2/SV3 field names,
not an invented empty B column. Remaining A rows independently yield7.50/2.50.

These are query typing/shape witnesses on literal synthetic data, not empty
Excel/header output acceptance, universal regional collation, source callback
or full Summary Vegetation parity. Preserve NULL cells, absent pivot columns
and zero rows as different outcomes.

### Summary layer/reference join boundary

Eight further original query scenarios (four joins in both Average modes)
independently reopen and seal on a normally closed disposable Access owner.
Layer1 uses the canonical `LayerCode.Layer` caption A1, not the input digit1.
The original nullable Layer10 mapping and a missing mapping group together:
two weighted P1 cover4 entries plus P2 cover8 produce4.00 by nPlots4 or5.33
characteristic. NULL Layer is a real nullable group key, not an invented caption.

The filtered five-field reference UNION retains distinct definitions with the
same code/scientific name but different EnglishName. These definitions multiply
cover numerator and count while leaving SV1 denominators unchanged: A20.00/B10.00
by nPlots instead of A10.00/B5.00, but characteristic averages remain13.33/5.00.
Distinct scientific names instead produce separate Alpha/Beta rows, each retaining
the original unit values. Never deduplicate this join by species code or name.

Existing `prepareLongVegetation` is source precedent for raw scope/max reduction
and layer observation expansion; existing `lifeformSummarySingle` supplies exact
rational-to-SINGLE assignment without DOUBLE double-rounding. The next private
summary layer assembly must explicitly wire these boundaries, the accepted
summary raw admission/count model, filtered reference helper, layer caption join,
certified aggregate and scalar-to-TEXT pivot. It must keep source join weights,
NULL groups/cells and absent columns distinct. No broader collation or source
assignment-dialog behavior is established by these literal synthetic witnesses.

The private `prepareSummaryVegetationLayer` and `planSummaryVegetationLayer`
now implement this narrower normal-SU/layer/DisplayValue6 Average1/2 boundary.
Raw preparation reuses the existing MAX reduction and expansion, validates
TempReportVeg required nonempty TEXT7 PlotNumber/TEXT8 Species and all fifteen
SINGLE assignments (including covers not expanded by LayerCode), then stages
original EntryDat_Veg observations. The staged planner also accepts explicit
NULL cover cells, preserving their distinct column-establishing behavior.
Raw NULL covers are omitted before that join and cannot invent columns.

The read model preserves all admitted units independently of pivot columns,
nullable layer/species grouping, literal lexical ordering, exact reference/SU
weights and separate stage/membership/conversion-layer/caption-map/reference/raw
origin witnesses. Numeric floor values become TEXT0.01 at the pivot boundary.
Errors or cancellation return no partial report. Unsupported historical text
bounds, invalid required identities, duplicate layer mapping keys and uncertified
arithmetic refuse rather than copying source silent truncation/omission.
Coupled focused race6.836s, root vet and private production build pass; distinct
read-only review reports no significant issues and exact two-file hashes are
retained. Fresh1341-input full race passes exit0 in4456.943228s with unchanged
inputs/discovery and separately associated acceptance after full rehash. This has no public
route, Excel output, source Load/event mutation or broader report-mode grant.

### Summary workbook projection evidence and exclusions

Static original BuildReport uses FirstRow4 without environment headers:
n Plots and positional SV1 counts at row4, original field names at row5, record
data initially at row7, then deletion of row6 moves data to row6. Normal mode
has Layer/Spp/Order followed by unit columns; no English field uses FirstCol4.
Original AddNPlots iterates SV1 order, not field-name lookup. A raw-admitted unit
missing from pivot columns therefore needs a separately proven association;
do not silently attach its count to another unit.

Two bounded disposable native projection attempts are closed and sealed as
failed runners. The first Access dynamic Run argument-count failure occurred
before Excel creation. The final established Eval boundary ran exact original
AddNPlots plus explicitly copied BuildReport cell/row statements and saved one
nonempty workbook, then a harness Python-True identity check rejected the Access
BOOLEAN return before COM cell readback. Both Access/Excel PIDs in the second
attempt are freshly absent and both DAO locks absent; no Office owner remains.
First owner identity was not retained, so only global fresh absence and normal
Quit are established there. No third attempt is authorized by this contract.

Independent readonly XLSX XML reopen verifies17 expected cells: numeric4/1
denominators at D4/E4, source headings at row5, literal TEXT10.00/5.00 and
0.50 cells at rows6/7 and the absent NULL E7 cell. This is a partial native
projection witness, not an accepted whole runner or full BuildReport behavior.
No missing-column case, template/style, Excel COM readback, report summary,
environment header, constancy, callback or printing is accepted. The earlier
empty-Excel/CutCopyMode and lumping budgets remain separately exhausted.

### Original Display Values1/2/3

Eighteen original normal-SU/layer scenarios (ordinary, reference-fanout presence
above1, explicit all-NULL staged covers; both Average modes and three display
values) independently reopen. Four exact loaded/reexported source function
bodies match: vpRoundUp2, Presence2Class, SignifClass and RkFormula.

Display1's caption is "% Presence and % Cover", but the source emits
two-decimal Count/nPlots ratios, not ratio*100: A0.75 - 10.00 or0.75 - 13.33,
B1.00 - 5.00. Its Order is the lexical presence prefix,1.00. Source reference
fanout legitimately yields1.50/2.00; it is not capped at100% or1.

Display2 formats the presence ratio before Presence2Class thresholds and
concatenates SignifClass of the formatted cover. AIV - 4 orIV - 5 and BV - 3
produce full lexical Order V - 3. Source presence>1 is outside Presence2Class's
defined intervals; the predicate drops that row, rather than inventing classV.

Display3 instead classes raw Count/nPlots: AIV - 10.00 orIV - 13.33 and
BV - 5.00 with Order V. Presence>1 produces an empty class prefix (" - 20.00"
or" - 13.33"); the prefix Order is observed NULL, and the source keeps that
row because this mode has no non-NULL Order filter.

Explicit all-NULL staged groups cause the original criteria type-mismatch in
Display1/3 (four independently reopened expected source errors). Display2
produces zero final rows. These differ from the accepted Display6 NULL behavior;
never reuse its suppression without a mode-specific guard. Raw NULL covers are
omitted by conversion, so these projected staged outcomes are not a claim that
normal raw preparation always reaches the errors.

The first native runner stopped at its first all-NULL mismatch after12 successful
reopened cases; its closed failed evidence is retained. The final successor
records all18 values/errors with canonical hashes unchanged, normal owned Quit,
fresh PID/DAO lock absence and12 sealed files. No Prominence/Goldstream,
regional collation, Excel, preference callback or public mode is enabled.

The private display extension now implements original values1/2/3 alongside6.
Existing preparation/planner wrappers retain6. Display1/3 preserve rows even
with NULL Order;2/6 retain the original non-NULL Order filter. Actual all-NULL
staged groups refuse for1/3, whereas an absent unit group remains a NULL cell
without evaluating invented source rows. Shared aggregate input validation and
cover Variant formatting are reused; mode3 does not inherit the source1/6
formatted count-mask gate. Source2 classes the formatted ratio and formatted
cover; source3 classes the raw ratio. Focused source18-case/threshold/cancellation
and predecessor regressions pass race3.465s, root vet and private production.
Distinct independent extension review reports no significant issues; four exact
reviewed file hashes are retained. Combined integration remains pending.

### Owned Summary Vegetation preview draft

`VPRO_SUMMARY_VEGETATION` and `VITE_SUMMARY_VEGETATION` independently default off.
The actual-main service reads the selected physical project Veg/normal SU and
VLists/VUser/LayerCode under the existing owned read snapshot; it never borrows
Long Vegetation preferences or requires Env/Admin for the unfiltered scope.
The draft supports Display Values1/2/3/6 and both Calculate options. Other
groupings, units, quality/strata/lumping, naming, headers, constancy, workbook
output and printing remain unavailable.

Original Load/View Report saved setters are not run. This explicit read-only
adaptation retains options/errors/results per immutable context. Busy reads and
invalid option drafts hold root close/navigation/context changes; Cancel and
valid correction remain live. Reviewed scalar DTOs are detached from actual
Svelte proxies and deeply frozen. Validation yields during traversal, and checks
generation after awaited work before assigning a result. Strict production-Go
roundtrip fixtures exercise all eight supported mode branches; client checks
validate typed shape, ownership, admission/witness consistency and lexical Order,
not an independently implemented second report arithmetic engine.

Focused summary race3.688s/root vet,1334 frontend tests/check0/0 and private
default/enabled actual-main builds pass. Distinct preview review reports no
significant issues with all16 candidate hashes verified. Five normally closed
Wails owners prove independent gates/eight mode branches/three responsive widths,
retained drafts/results/errors, yielded midvalidation Cancel/remount/late rejection
and retry, foreign-response refusal and invalid-draft native-close/navigation/
context holds while correction stays live. All fixture/config/protected bytes
remain unchanged. The initial Reports-button harness mismatch remains failed
evidence; its same readonly owner checkpoint passed with the actual disclosure.
Combined full race4042.382s passes with1352 unchanged inputs/discovery. Immutable
final acceptance verifies296 native evidence files and20 independently reviewed
hashes. No accepted assets or whole-report parity are promoted.

### Small SINGLE text notation witnesses

A separate normally closed disposable source owner verifies116 original
`vpRoundUp(CSng(ValueIn))` outputs and readonly reopens with independent
DOUBLE-input/SINGLE-output bits. All116 numeric TEXT values match independent
seven-significant half-away decimal rounding. Thirty-three powers10^-6 through
10^-38 and their neighboring SINGLEs, twelve negative witnesses and five
additional mantissas retain source bits.

Notation is not a magnitude-only threshold: simple1e-6 and1e-7 are fixed, but
their immediately lower SINGLEs become `9.999999E-07` and `9.999999E-08`.
Source1.234567e-7 becomes `1.234567E-07`, not a fixed fractional string. Therefore
the earlier simple-power witnesses do not justify a generic fixed/scientific
formatter. The significant-digit/magnitude matrix remains a source prerequisite
before unconstrained class-value presentation. Thirteen source evidence files
seal with unchanged canonical hashes and fresh owned PID/DAO-lock absence;
no public mode or locale change follows.

A138-case original/read-only matrix resolves the bounded notation rule.
Seven mantissas with1..7 significant digits across exponents-9..8, seven signed
values and five class-like values match independent expected strings and exact
bits. After seven-significant half-away rounding and trailing coefficient zero
removal, use fixed only when decimal exponent<=6 and retained fractional
digits<=7; otherwise scientific. Source0.0141421356 SINGLE becomes
`1.414214E-02`, whereas0.141421356 SINGLE becomes `0.1414214`. Thirteen files
seal with fresh owned PID/lock absence and unchanged canonical hashes. This
does not establish other locales or whole class/report arithmetic.

### Remaining grouping query boundaries

Six original GroupAndOrder12/13/14 Display6/bothCalculate projections and readonly
reopens use explicit source-derived staged inputs. Strata projects TEXT1;
LifeForm projects TEXT3 and switches its join to `EntryDat_Veg.Layer =
LayerCode.LayerCode`; None's literal NULL `Group` infers DAO BINARY9. Their
Order is DOUBLE `Max(CDbl(MyCover))`, not layer11's lexical TEXT maximum.
Independent duplicate-membership weights, two Calculate results and NULL blanks
match; eleven files seal with fresh owned PID/lock absence/canonical hashes.
The initial static guard caught Group13's second conditional JOIN block before
creating an owner/fixture. No failed native operation is claimed accepted.

These observations do not execute raw builders. Active `BuildDataTable` routes
Strata through `BuildStrataTable`, Lifeform through `BuildLifeFormTable` and
None through `ConvertProVeg2V2Null`, rather than simply regrouping layer rows.
The Lifeform builder's unconditional `GoTo MyExit` makes its later legacy
layer-by-lifeform statements dormant. Constancy overrides grouping/projection
and remains separately unavailable. Keep these raw preparation differences
explicit before any public grouping extension.

One new disposable owner now executes the original three raw builders and
`SetTo99`/`Null2Question` bodies without rewriting them. Ten supplied SINGLE
TempReportVeg rows, ordinary and distinct-reference-fanout scenarios yield58
independently expected stage rows across six builder executions, with readonly
reopens and exact output SINGLE bits. Relevant USysV2Veg field types/bounds and
isolated project/SU/unlumped getters are explicit; raw MAX creation and complete
template/default/index creation are not claimed.

- Strata A/B honor TotalA/B, otherwise cap sums at99 and admit positive results.
  Non-NULL C/D retain zero and negative values. Sub-shrub5a/b/c contribute to B.
- Lifeform sums1..9 plus5a/b/c, not10 or TotalA/B. It retains zero/negative rows,
  converts the sum through `SetTo99`'s SINGLE parameter before99.9 capping, and
  weights distinct references before final SINGLE insertion. Unmatched metadata
  produces the stage label `?`.
- None uses only Cover1..7, caps100, admits positive sums and writes ProjectID.
  Reference fanout does not change Strata or None preparation.

The source label exposed a coupled application gap:
`vegetationLifeformInsertionLayer` returned NULL before applying the SQL
`Format(NULL,'00')` empty-string boundary. The bounded correction now passes
empty text to `Null2Question`, producing `?`; direct `Null2Question(NULL)` still
preserves NULL. Regressions distinguish missing/nullable references, retained
zero/negative cover and cap/fanout from the later nullable master report group.
The stage key participates in the physical `LayerCode` join and missing-label
diagnostics; it is not the final numeric/NULL report group. Focused coupled
race6.915s, root vet/production compile and distinct read-only correction review
pass with both hashes and all6 original executions/58 expected rows verified.
One normally closed actual-main Wails owner proves the exact `text:?`
missing-label diagnostic and retained NULL master groups/positive-negative
covers without data/config/protected writes;213 native files seal. The native
fixture preserves its unique Layer1234567 index: a duplicate setup failed and
rolled back before any owner. Nonunique fanout remains fast-test evidence.
A sparse-cell harness comparison failed against the full nullable wire; the
same readonly owner resumed and passed. Full correction race4068.025s passes
with1352 unchanged inputs/discovery; immutable final acceptance verifies213
native files and both reviewed correction hashes. No new summary grouping or
asset promotion follows. Eleven source files seal
with exact original bodies, no canonical drift and fresh owned PID/DAO-lock
absence. This evidence is not whole-report acceptance.

The next scalar4/5 candidate is session-only during the frozen correction race.
Virtual Go overlay discovery, focused race9.007s, root vet/production compile and
distinct read-only review pass; the reviewer reports no significant issues
without a separate test/hash receipt. A portable fixture rehashes accepted source
evidence before retaining281 exact SINGLE-text,192 typed direct-helper and72
formatted-wrapper outcomes. The candidate uses the shared fixed-decimal and
SINGLE assignment boundaries, classifies before seven-significant text, retains
negative zero, and refuses negative sqrt/nonfinite or overflowing results.
The exact reviewed scalar/test and portable fixture are now materialized, with
root focused race8.814s/vet/production passing. The report extension retains
legacy `Preview(Calculate,Display)` and its eight production wire fixtures
unchanged; explicit `PreviewClass(Calculate,Display,Show)` carries historical1/
Class10/Value20 provenance only for4/5. Calculate remains a retained option but
these source formulas ignore it. Five physical reads/owned snapshot, raw Layer
conversion and certified aggregates are reused. Typed class results become
TEXT pivot cells with lexical Order; physical allNULL groups retain columns
without final rows, and count>nPlots is not clamped. Source18 crosstab
expectations across both Calculate options and facade zero-write/alias/
ownership/cancel/cleanup/retry cases pass focused race9.367s/vet/production.
Distinct report/client/scalar readonly reviews report no significant issues.
The generated interface bindings and read-only client now support explicit
historical1/Class10/Value20 Show drafts under existing independent default-off
flags. Source display>3 enables Show, including6 where arithmetic ignores it;
changing other fields retains Show rather than invoking original saved setters.
Strict12 actual-Go class wires preserve the old8 and validate281 independently
observed SINGLE strings. Full1340 frontend tests/check0/0, private default/
enabled actual-main builds and focusedGo race10.341s/production pass.

Five normally closed Wails owners verify all20 mode branches, three responsive
widths with actual labeled/visible/contained Show controls, retained Show20,
class midvalidation Cancel/remount/late rejection/retry, and invalid Show2
native-close/navigation/context holds with live correction. Fixture/config/
protected bytes remain unchanged;302 native files and25 inputs are sealed.
Readonly harness failures are retained: textContent included option descendants
in Show's label, and the old display4-invalid assumption became obsolete.
Corrected direct label observation and invalid Show2 preserve the source
contract; both same owners resumed without any write replay. No native owner
remains. Class full integration passes4268.337s against1359 unchanged inputs
and unchanged discovery; separate immutable acceptance verifies302 native files,
25 reviewed inputs and20 branches.
no accepted assets or whole-report parity are promoted.

### Private Summary grouped preparation

GroupAndOrder12/13/14 Display6 staged projections preserve the original joins:
Layer1234567 TEXT2 to Strata TEXT1, unique LayerCode TEXT3 to LifeForm TEXT3,
and literal NULL Group with DAO BINARY9 metadata. Order is DOUBLE Max(CDbl
of the formatted cover), not the legacy Layer lexical maximum. The shared
kernel retains old Layer/class behavior. Six independent source query cases,
DAO metadata and coupled race13.139s/vet/production pass; distinct bounded
read-only review reports no significant issues.

Private TempReportVeg-to-EntryDat_Veg staging validates the original fifteen
MAX-reduced cover columns and assigns all fifteen SINGLE fields, including
unused columns. Existing Strata and Lifeform helpers preserve totals/fallback
caps, zero/negative C/D, nullable question labels and reference fanout. None
sums only Cover1 through Cover7, caps100, retains positive rows only and writes
NULL Layer/current ProjectID; other builders retain NULL ProjectID. Six original
builder scenarios/58 independently observed rows, exact SINGLE bits and five
field definitions pass, alongside cancellation, detached output, malformed/
overlength identities, duplicate/incomplete rows, unused-cover overflow and
valid correction. Coupled race14.136s/vet/production and distinct read-only review
pass. Test-only integer fixture construction failures were corrected before
acceptance. Complete normal-SU/MAX preparation now feeds these builders (never
regrouping expanded Layer observations), then the original grouped query joins.
Twelve integrated grouping/Calculate/fanout cases retain lower-duplicate raw
provenance and fixed admission denominators; cancellation/detachment/correction
and three-character literal LayerCode tests pass coupled race13.677s/vet/production
and distinct review.

Explicit PreviewGrouped supports only12/13/14 Display6 under the unchanged
independent default-off gates. Optional groupAndOrder, groupField and orderField
carry exact DAO metadata; Order is REAL, covers remain formatted TEXT, and raw
builder witnesses have no invented conversion LayerCode identity. Old eight
and twelve class wires remain unchanged. Six actual-Go grouped wires, owned
zero-write/cleanup/refusal/retry tests and coupled race14.601s/vet/production pass.
The retained client validates exact group/metadata/ownership/numeric Order,
preserves per-context drafts and live correction, and rejects stale yielded
validation after Cancel/remount. All1345 frontend tests/check0/0/private builds
and distinct desktop review pass; interface bindings have no generation warnings.

Five normally closed Wails owners prove26 legacy/class/grouped branches and
three-width control/action visibility, associated labels and containment.
Group/result remount, held grouped validation Cancel/late rejection/retry and
invalid group15 Preview/native-close/Home/context holds with live correction
pass. All fixture/config/protected hashes restore;301 files/34 inputs seal.
Complete-source integration passes1368 unchanged inputs/discovery in4174.890s;
separate immutable acceptance verifies301 files/34 inputs/26 branches.

The additional grouped Display1/3/class4/5 and None14 Display2 extension reuses
the calibrated shared cell/projection kernel. Group12/13 Display2 remain
explicitly unavailable because original CDbl(composite MyCover) fails with DAO3464;
neither source nor desktop silently repairs that conversion. Group1/3 prefix
Order, None2 composite lexical Order and class4/5 late lexical Order carry TEXT
DAO metadata, unlike Display6 DOUBLE. Thirty-six independent source outcomes/
four refusals, class Calculate1/2 equivalence, cancellation and allNULL guards
pass race16.787s/vet/production and distinct private-kernel review.

PreviewGroupedDisplay preserves existing Display6 PreviewGrouped/old wire,
with fifty additional actual-Go wires and retained client drafts, Show/error/
generation/proxy safety. Coupled race18.976s/vet/production,1350 frontend tests,
check0/0/private builds and distinct desktop review pass. Warning-free interface
bindings retain source nullable contracts. Five normally closed Wails owners
prove76 branches, grouped-class remount, original criteria-failure draft
Preview/native-close/Home/context holds with live None2 correction, and held
additional-mode validation Cancel/remount/late rejection/retry.304 files/39 inputs
seal with all fixture/config/protected bytes unchanged. Additional complete-source
integration passes1373 unchanged inputs/discovery in3956.989s; separate immutable
acceptance verifies304 files/39 reviewed inputs/76 branches.
No default promotion or whole-report claim.

### Saved Summary option desktop candidate

Independent default-off `VPRO_SUMMARY_VEGETATION_PREFERENCES` and
`VITE_SUMMARY_VEGETATION_PREFERENCES` provide explicit Load/Save/Undo using the
existing Group by, Display Values, Show and Calculate controls. The Summary
preview gate remains separate and required for changed selections. Source
`SVGroupBy` values1..4 map explicitly to query11..14; `SVAvgType` is1/2,
`SVValue`1..6 and `SVShowClass`1/10/20. Historical finite numeric `SVShowValOpt`
is not a boolean or editable new filter. An explicit changed Display outside1/3
clears it to0; unchanged assignments and unrelated edits retain its original
typed YAML storage. Away/back display changes retain that clear provenance.
The original ValueLimit filter belongs to GroupAndOrder24, not normal11..14.

The owned five-key CAS reuses existing YAML persistence and request snapshots;
duplicate/escaped request properties, malformed Unicode, stale ownership and
invalid options refuse explicitly. Unchanged historical source-failing selections
remain readable/no-op without enabling their preview. Save receipts survive
committed cancellation and cleanup; unknown delivery requires explicit owned
Load, never replay. Draft validation, pending saves and unacknowledged outcomes
remain rooted across remounts; correction/Undo stay live. Full mutating source
Form_Load, constancy, units, quality, naming, headers and printers are not invoked.

Private leaf/facade and controller reviews pass. Materialized desktop coupled
race36.091s/vet/production, strict actual-Go fixture, warning-free interface
bindings and1364 frontend cases/check0/0/private build pass with accepted assets
unchanged. Exact desktop independent review and focused follow-up accept the
failed-reload and historical-selection lifecycle fixes;18 tests independently
rerun. Five normally closed disposable Wails owners verify independent gates,
five once-only preference commits, full YAML reopens, unrelated-edit nonzero
limit retention, unknown receipts, Cancel/remount and failed-reload recovery.
Historical source-failing selections remain no-op/save/Undo/navigation/close
available without enabling preview. All originals restore;291 files and31
reviewed inputs seal. Complete-source integration passes1382 unchanged inputs/
discovery, exit0 in4138.438s; separate final acceptance verifies291 native
hashes and restored originals without rewriting the seal. Explicit
Save is an adaptation, not automatic Access callback timing parity.

### Remaining Summary quality and naming source boundary

Six canonical sources seal34 exact preference leaves,14 direct callbacks and60
bodies. Minimum quality text callbacks use Change; Include Nulls uses AfterUpdate.
They write the generic quality keys, not the Long Vegetation LV-suffixed family.
The BEC minimum is the shared `DataQualityFilterBECUseLV` string; its NULL option
is `DataQualityFilterBECUse`, whose original getter default is integer1.
QC adds a literal BEC_Use TEXT comparison and NULL decision to the three quality
domains. QCLV has no equivalent BEC criterion and is not complete Summary proof.

A private nine-key reader/CAS preserves literal/unchanged historical values and
legacy boolean storage, checks shared expected values and excludes enforcement
changes. Focused race/vet/production overlay compilation and independent leaf
review pass. Its private independently gated owned facade/strict nine-key wire
and physical PlotQualitySite/BEC_Use suggestions also pass eight coupled race
tests, vet/production overlay compilation and distinct facade review.
Ownership, read cleanup, cancellation, collision, committed receipts, empty
suggestions and malformed physical TEXT correction/retry are covered.
The saved-quality draft is now materialized/registered behind independent
default-off backend/client gates with retained controller and responsive pane.
Four text and four NULL controls reuse the source bindings; enforcement stays
read-only. The identical Long Vegetation suggestion invariant now reuses the
shared typed reference validator. Exact production Go wire, genuine client
proxies, failed-reload retention, immutable receipts and root/sibling holds pass.
Opted-in preview requires quality Load and refuses saved enforcement=true in
both client and backend, rather than returning an unqualified report.
Existing feature-off preview remains unchanged. Focused backend race3.056s,
vet/production,40 coupled client cases and1377 frontend tests/check0/0/private
build pass. No accepted asset/default promotion. Exact23-file desktop review
finds no significant issues. Five normally closed Wails owners/four once-only
YAML commits prove independent gates, three-width labels, retained lifecycle
and bidirectional peer/root holds with both Summary sessions enabled.
All313 native files/31 reviewed inputs seal; fixtures/configuration/protected
assets restore. Complete-source1394-input race passes exit0 in3966.657s;
frozen inputs/discovery and313 native hashes/restored identities verify with
separate immutable final acceptance. The saved-quality freeze is lifted.
No qualified QC preview is claimed.

## Stored Summary presentation draft boundary

Eight original controls map to `SVReportTitle`, `SVShowEnglishName`,
`SVShowGuideName`, `SVShowHierarchyHeader`, `SVIncludeEnvFields`,
`SVReportSummary`, `SVSubSppAsSpp` and `SVConstancyFormat`. These are stored
configuration drafts, not report-output formatting parity. The title getter
defaults to "Short veg report title"; source Form_Load displays a computed unit
title without saving that assignment. Original labels/grouping are preserved,
not tiny absolute geometry.

The constancy-on callback stages `SVGroupBy=3`, `SVValue=1` and common names
off. Display1 does not clear `SVShowValOpt`. Switching constancy off only
re-enables controls; it does not restore old values. A staged on/off cycle
therefore retains the coupled plan even if the final flag is unchanged.
Common-name editing resumes after off. The strict eleven-key CAS checks all
shared values; full irreversible receipts retain the explicit callback flag.
Repeated acknowledgement cannot clear a newer draft's callback provenance.
Historical inconsistent values/unchanged title NUL and original Boolean
storage remain omitted; new malformed/NUL title refuses without normalization
or invented length bounds. Hierarchy-header changes refuse in the current
unsupported hierarchy scope; unrelated edits preserve its historical storage.

Independent `VPRO_SUMMARY_VEGETATION_PRESENTATION_PREFERENCES` and
`VITE_SUMMARY_VEGETATION_PRESENTATION_PREFERENCES` flags default off. The client
also requires the existing Summary report client flag. Three retained sessions
aggregate root close/navigation holds, without a sibling clearing another's
validation or receipt hold. Explicit Load/Save/Undo adapts source immediate
callbacks. The raw crosstab preview explicitly does not consume these drafts.
Guide physical renames, environmental field-list editing, hierarchy reports,
constancy output, summaries, printing and workbooks remain unavailable.

Private leaf/facade/client/pane reviews and focused stale-ack correction review
pass. Materialized focused backend race9.865s/vet/production,46 coupled client
cases and1392 full frontend tests/check0/0/private build81.125s pass;65 services/
334 methods/356 models generate warning-free interface bindings. All17 accepted
asset files remain unchanged. Exact20-file desktop independent review finds no
significant issues. Five normally closed Wails owners/four once-only commits
prove independent gates, three-session root/peer holds, known/unknown callback
receipts, cancellation/remount/read recovery, actual hierarchy RPC refusal and
same-final constancy on/off coupled commit.301 native files/29 reviewed inputs
seal; fixtures/config/protected assets restore. Four separately scrolled layout
segments per width accompany control-by-control visibility/viewport/containment/
label checks. Frozen1404-input complete-source Windows race90m passes exit0 in
3958.441s with unchanged inputs/discovery. Separate final acceptance verifies
native/review/restoration hashes; no default promotion.

### Remaining source consumer boundaries

Enforcement calls the mutating Form_Load and cannot be treated as a pure toggle.

Fourteen original QC BEC predicate cases pass on a disposable database and
independent readonly DAO reopens after normal owned Access close. Threshold
`1` admits `10` as TEXT, not a numeric rank. Empty, NULL, ASCII case and
leading/trailing whitespace outcomes are recorded independently. This calibrates
the literal predicate only, not complete QC join/fanout, arbitrary Unicode
collation or historical criteria containing quote syntax. Initial Access reopens
were ordinary; only the final independent DAO reopens were readonly.

Sixteen separate NULL-option combinations calibrate the complete original QC
insertion with synthetic USysEnv projection and independently readonly reopened
source SELECTs. Duplicate membership/reference fanout, foreign-domain exclusion,
missing/NULL-reference admission, BEC NULL/text and NULL SiteUnit are explicit.
This does not supply physical Env/Admin witnesses, conflict-first threshold
selection or broader collation/quote parity.

The guide-name report path changes physical VLists field names and writes the
common-name preference after capturing its local ShowCommon variable. Its
checkbox setter alone does not prove the generation path. The linked summary
field list is bound to the global `USysShortVegSummaryFieldList` table; Select
All/None updates every Report value. Those are database mutations, not saved
YAML preferences. Label is bound but explicitly locked; it is not an editing
requirement. The child displays only positive Order rows, while AddEnvFields
selects Report=Yes without that visibility filter. A private complete seven-cell/
physical-rowid reader and separate source index preserve duplicates, NULL/empty,
hidden rows and historical incompatible storage without coercion. Focused
race/vet/production55.438s and independent exact review pass. The separate owned
read facade passes coupled reader/facade race/vet/production61.703s and exact
independent review within its private boundary. Its initial passing artifacts
are retained; a separate-output linker retry passes29.250s after shared-memory
pressure clears, without owned source changes. OS-level attachment replacement
is not newly proved. It does not register bindings or enable UI/mutations. Both workflows
remain unavailable pending their writer/consumer/native contracts.
Private Report-only full physical-witness planning and caller-owned transaction
assignment primitives pass coupled race/vet/production47.141s/48.953s and
distinct exact reviews. Equal-content row replacements cannot be distinguished,
but changed-content automatic rowid reuse refuses; hidden/peer changes are
included in the full CAS witness. Net no-op does not execute an update. Actual
SQLite triggers prove ABORT/IGNORE/peer-drift rejection, caller rollback and
in-flight cancellation with attachment/read recovery. These primitives neither
acquire an ownership lease nor commit or write source audits. A strict request
prerequisite passes49.797s and exact independent review; it is not authorization.
The private owned committing coordinator and stable technical receipt pass
coupled race/vet/production56.250s, with exact independent review accepted
within its private coordinator boundary.
Idempotent replay/collision, complete source drift, receipt/trigger failure,
actual cancellation/rollback and replacement/refusal/retry are covered.
Replacement checks release only the fixture pinned reader alias; identity
observations do not prove an atomic external pathname-race guarantee.
The receipt is a desktop retry-safety adaptation, not source option auditing.
The private public-shaped Save facade passes coupled all-prerequisite
race/vet/production54.797s with exact independent review accepted. Its wire
preserves full plans and irreversible flags alongside drift/cleanup errors.
Native acceptance and formatted output consumer are still required.
Fourteen exact reviewed Go sources and the independent default-off main
registration are now materialized. Focused race10.586s/vet/production,71 related
Summary client tests/check0/0/private build13.49s pass. Warning-free interface
bindings change only index/models/new service. Exact materialized backend review
is accepted. The retained controller/registry reconciliation finding is closed:
keep historical authority separate from new reconciliationReview until explicit
correction.36 client tests/strict TS and distinct review pass.
Locked Label/Report/Field linked pane and fourth-session desktop wiring are
privately reviewed, then nine exact candidates are materialized with portable
test paths, optional flag type and explicit fourth-peer prior lifecycle tests.
61 focused+1453 full frontend cases/check0/0 and isolated enabled/default
builds pass with all17 accepted assets unchanged. Exact materialized frontend
review finds no significant issues and independently reruns61focused+72changed
prior cases; actual native/layout/lifecycle and full integration remain.
Bootstrap learns exact VPro64 sourcePath from validated owned GetReview rather
than inventing it from a project path. IncludeEnv draft controls staging/Save
intent only, never formatted environmental output. No default promotion.
Native default/client-off gates close and restore. Backend-off refusal is correct,
but a failed initial bootstrap retains a root-close hold with no available error
dismissal. That owner is rejected before field-list writes; forced specific-PID
cleanup and300-file failed seal are not normal/native acceptance. An explicit
readonly-bootstrap error recovery is required only when no authority/session/
draft/receipt exists and every read/validation dispatch has settled. Established
session errors/irreversible receipts must never be dismissed by that recovery.
The exact three-file correction passes67coupled+42prior cases/check0/0/private
build and distinct private review, then is byte-exact materialized.67focused+
1459full frontend cases/check0/0 and isolated enabled/default builds pass;
focused materialized follow-up finds no significant issues. The separate
corrected actual-main candidate accepts five normally closed owners/five once-only
commits: independent gates and explicit no-authority read dismissal, layouts/
headers/physical duplicate identity, hidden/NULL Report bulk mutation, exact replay/
collision/historical-drift receipts, unknown commit plus removed authority explicit
correction, four-session peer/root holds, cancellation/late remount/reload recovery
and IncludeEnv false draft holding mutation while Load/Undo remain available.
352 files/39 reviewed inputs seal with restored data/config/protected assets and
rename-proven no locks. The failed300-file predecessor is retained/rejected, never
normal-close acceptance. Frozen complete-source90m integration remains; no
environmental function/report formatting/XLSX/printing/default promotion.
The complete-source race is now active over1428 frozen inputs with90m budget.

### Environmental plot-count prerequisite

`SVnPlots` uses grouped `Count(PlotNumber)` over `USysSuTableDynamic_SU`, not
DISTINCT counts or joined Env membership; assigns the result to VBA Integer and
returns that Integer Variant. Eight exact original loaded/reexported cases pass:
duplicates count separately, NULL PlotNumber does not; a present group with only
NULL plots returns Integer0, a missing group fails3021, NULL String argument94,
and32768 overflows6 while32767 succeeds. Empty unit text remains distinct from
NULL. Source database case equivalence is observed, not adopted in byte-exact
SQLite literal scopes. A separate readonly reopen verifies65544 complete input
rows/physical spellings and query-group counts; eight files seal with normal
Access quit/no owner/locks/canonical writes. This is not a QC projection or
workbook retry, and does not supply an authoritative owned count source.

The private prepared-count typed leaf passes11 isolated race events with the
byte-exact production cell DTO, including source failures/range, cancellation/
retry, detached result pointers and complete tagged integer wire. A coupled root
overlay compile failed allocation while full integration was active; its failed
evidence remains, and isolation is not proxy integration acceptance.
Distinct exact private review finds no significant issues only within the
prepared-count value boundary. After the full1428-input integration passes
exit0 in4785.777927s, combined private count/category overlay race48.485s,
vet7.156s and production12.094s pass with unchanged inputs/candidates/discovery
and absent virtual targets. Distinct focused follow-up finds no significant
issues; it returned no separate hash/event receipt. Acceptance is private leaves
only, not source preparation or output.
Six reviewed primitive/test sources and two portable synthetic source receipts
are now materialized. Focused178-event race38.063s/vet3.625s/production5.625s
pass without environment fixture overrides. Exact eight-path independent review
verifies receipt hashes, unchanged application/preparation-test bytes and only
the portable test import/fallback adaptations. These do not enable a formatted
environmental consumer.

A separate private independently gated owned normal-SU count facade passes
182 coupled events/race39.25s/vet3.781s/production11s and independent exact
review of1436 baseline inputs/candidate hashes. It uses the existing context
operation lease and owned pinned read snapshot, reads the complete physical SU
table and returns detached provenance/physical witnesses/tagged count. Default/
external paths, zero database/config writes, stale/changed ownership, views/
shadow rowid/unsupported scopes, malformed correction, queued cancellation,
late-before-commit cancellation and failed snapshot cleanup followed by a
successful pinned retry are covered. Interruption during SQL row scanning
remains untested. The facade is now materialized but unregistered and has no bindings/UI/native
lifecycle/output acceptance.

### Direct TEXT environmental category prerequisites

Original `SVStructureStage`, `SVSuccessional`, `SVSurfaceTopographyType`, `SVSnr`
and `SVMesoSlopePosition` have equivalent ordered directly labeled TEXT paths.
Forty exact typed original cases pass: duplicate SU memberships multiply joined
counts; orphan Env, missing Env and NULL join keys disappear. Empty categories
render `(1)  `, unlike NULL suffixes. Category output retains two trailing spaces
unless followed by `(Null n)`. All-NULL rows render only that suffix; missing/
no-join units yield String empty, while a NULL String argument raises94.
Independent readonly reopen verifies all22 physical inputs, ten joined rows,
field types/sizes and unchanged database bytes. Ten files seal with normal owned
Access quit/no locks/canonical drift. A fixture-only predeclared `CurrProject`
getter returns Sample without registry access; only the five function bodies,
not the real project getter/startup, are byte-exact original source.

The private prepared renderer passes61 isolated actual-production-DTO race events:
all40 original cases, literal order/spelling/whitespace, complete tagged String
wire, detached output, explicit malformed-text/count/unsupported-function refusal,
cancellation during iteration/finalization and correction/retry. Exact independent
review finds no significant issues within that prepared-value boundary. Coupled
race/vet/production and focused follow-up pass as recorded above. The caller still
must prepare authoritative categories/order/membership/equivalence; this leaf
does not prove those decisions, source Integer loop limits, mutable reader
ownership or formatted report availability. `SVSmr` code conversion, concatenated
labels, dynamic layers and numeric criteria remain separate prerequisites.

Eleven additional directly labeled TEXT originals are calibrated with distinct
column values: HydroGeoSubSystem/System, SoilDrainage, HumusForm,
RootRestrictingType, SoilClassSubGroup/Group, RootZoneParticleSize,
SurficialMaterialSurf, Exposure1 and Zone. Eighty-eight exact typed cases,
complete22-input/ten-join readonly reopen and ten-file seal preserve original
bodies and physical TEXT sizes. A shared function/field lookup expands the
renderer/preparer to16 functions, with327 focused race events/vet/production
and exact independent five-path review of bindings/source properties/portable
fixtures. The map is a calculation boundary, not a substitute for physical
tables or `_table_metadata`. Literal join/group/sort semantics remain explicit
adaptations;32767 distinct categories remain unavailable because the original
Integer-loop boundary is not calibrated.

The first materialized categorical planner review exposed missing field-binding
witnesses: identical values in all columns could hide swapped mappings. Distinct
five-field NULL/empty/duplicate witnesses resolved the finding; focused review
verifies221 events and only that test changed. Sixteen additional independent
bindings and88 native cases now cover the expanded registry. An initial runner
incorrectly counted33 prefix passes against three top-level cases; retained
failed evidence and a separate exact gate correction preserve all successful
checks without rerunning native work.

A private selected-field matrix consumes complete Report=Yes selection, not the
child's positive-Order visible filter. It retains NULL/negative/zero Order and
stable rowid ties, complete seven-cell field witnesses and typed values per
prepared SV1 unit. Physical SU counts ignore cached NumPlots/Env join admission;
direct categorical values use Env/SU joins. The literal n Plots caption is
assigned column3, others column2. Unsupported selected functions or nonNULL
FieldNumber refuse the entire result. Focused private-overlay race37.625s/vet/
production and independent review pass; no separate reviewer hash/event receipt
was returned. The planner and an independently default-off combined facade are
now materialized. The existing Summary callback is extracted into an internal
snapshot reader returning the unchanged legacy DTO plus complete physical SU.
The combined facade reads selected fields and needed physical Env inside that
same transaction and operation lease, without nested service calls. Count-only
selection has no Env dependency. Unsupported functions/extra arguments refuse
the complete result, and detached typed witnesses survive strict JSON round-trip.
Private retry race61.562s/vet5.860s/production10.984s and actual-source
race42.813s/vet3.922s/production5.812s pass. Exact independent reviews verify
legacy callback equivalence and965 actual passing events/1447 current hashes,
with1442 predecessor files unchanged excluding the intentional extraction.
The first private test fixture replaced the complete Env schema and failed;
its receipt remains failed. Corrected fixtures preserve the original schema.
The facade is registered with independent backend/client flags
`VPRO_SUMMARY_VEGETATION_ENVIRONMENT`/`VITE_SUMMARY_VEGETATION_ENVIRONMENT`,
both default-off and requiring the base Summary feature. A separate readonly
inspector deliberately uses Layer/By n Plots/% Cover to establish its frame;
it does not consume saved IncludeEnv or the current Summary controls.
Strict production Go wire covers all16 categories/count; the client reuses
legacy yielding vegetation and dense physical field-list validators. A review
finding about sparse unused returned witness cells was fixed by reusing the
existing dense table guard; all seven independently missing cells and extra
array properties refuse. Focused follow-up accepts53 client cases/check0/0/
private build, exact hashes and unchanged accepted assets.

Registered race72.094s/vet4.266s/production11.906s, coupled Summary tests,
check and private default/enabled builds pass. The retained registry keeps
cancelled dispatch holds until response/validation settlement, independently of
the panel lifetime. Actual Svelte script/proxy tests cover remount, transport
errors, immutable results and actual root-derived holds. The exact generated
interface delta is index/models/new service only. Desktop review reports no
significant issues, with no separate reviewer hash/event receipt.

Four normally closed actual Wails owners prove gates, physical per-unit values
including exact trailing spaces/NULL/empty and hidden NULL-order selection,
three-width visibility/viewport/containment, result/error retention, live Cancel
and native-close/peer/actual guarded Home holds through settlement.312 files are
sealed and16 fixture originals/protected identities restore; zero commits.
Caption-whitespace and disabled-Home assumptions were corrected in the harness
on the same readonly owner; its first successful Preview was not replayed for
recovery. Initial test scope/reactivity/peer mock/runner failures remain failed
receipts. No application fix during native acceptance.
Complete-source1452-input integration passes exit0 with unchanged inputs and
ordinary/production discovery.312 sealed native files and protected fixture/
asset restoration verify separately without rewriting that seal. No environmental workbook,
printing, remaining function or default promotion is claimed. Independently
completed previews cannot establish a combined observation.

### Private environmental extensions

The accepted live inspector still supports sixteen direct TEXT categories and
plot count. Reviewed private candidates add `SVSmr`, `SVBioGeoClimaticUnit` and
integer `SVAspect`, with19/7/44 independently sealed original observations.
SMR groups literal codes before display mapping; unknown and empty codes do not
merge merely because their labels are empty. Aspect groups converted compass
labels, merging0 and360 into N. BGC admits distinct concatenations only where
Zone is nonNULL, but counts matching concatenations without repeating that
admission filter. Both-NULL contributes the NULL suffix; empty concatenation
remains distinct. Portable test-only physical rows/typed observations avoid
private machine-path requirements. Private production strict-Go wire, actual
proxy/controller/panel tests, check/build and independent review pass; these
extensions are now exactly materialized and separately Wails-accepted below.

Private Seepage/RootingDepth primitives preserve Empty Variant explicitly,
including all-NULL/missing/nojoin results. Original extrema apply VBA
`Val(Format(value,"#,##0.00"))`, so1000/1001 produce extrema1/1. The maximum
alone includes NULL as zero: independent review exposed the previously missing
negative-plus-NULL case, and a fresh original22-case source extension confirms
max0 for both functions. Mean uses raw values. A reviewed shared signed16
physical join/guard/aggregate boundary now supports private StandAge/Elevation
policies while retaining depth behavior. Eighteen fresh original age/elevation
observations and nine bounded SINGLE Slope observations pass independent
schema/readonly reopens and private implementation checks/review. Arbitrary
numeric domains and the dispatcher/wire representation remain unqualified.

Dynamic Bedrock layers1/2/3 and SiteDisturbance fields1/2 pass40 original typed
source observations and complete six-field physical/schema/join reopens.
Reviewed private dynamic primitives reuse the demonstrated equivalent literal
TEXT joined-count preparation; distinct per-field expectations supplement the
original identical-field fixture. Coupled race/vet/production and independent
review pass, retaining existing direct/derived category cases. FieldNumber
consumer/service/client integration and native acceptance remain separate.
The private selected-argument pipeline is now reviewed with25 strict-Go wire
rows and coupled backend/client/check/build success. FieldNumber is physically
TEXT255: canonical text1/2/3 coerces into original Integer helper arguments,
independently observed in40 fresh typed source cases. The initial private
integer-storage assumption failed production wire validation and was corrected,
not imposed on the source schema. Noncanonical argument text remains unavailable.
Exact23-path materialization passes actual-source race/vet/production, coupled
Summary tests/check/private build and independent review. Five normally closed
disposable Wails owners prove independent gates and eleven selected physical
rows: SMR unknown-code separation, BGC NULL-Zone admission/count distinction,
Aspect0/360 label merging and all five TEXT dynamic arguments. Three-width
actual visibility/viewport/containment, retained results/errors and cancellation/
native-close/peer/actual Home holds pass. Disturbance3/Bedrock01/empty arguments
refuse the entire matrix and retained errors survive remount; correction recovers.
The refusal fixture's physical seed changes restore byte exact after normal
close.315 files seal, all16 originals/protected hashes restore, zero app commits.
Expanded complete-source integration remains pending; default-off availability
and the inspector's fixed separate frame are unchanged.
SiteDisturbance3 is not enabled: its active loop reads SiteDisturbance2 even
though its query projects only the requested field. HumusThickness is absent
from the exported sample schema; no column or workflow is invented. Dynamic
source calibration is not implementation or native acceptance.

### Numeric environmental branch prerequisites

Original `SVStandAge`, `SVElevation` and `SVSlope` pass24 typed observations and
independent readonly complete20-input/nine-joined-row/schema reopens. Duplicate
SU memberships affect the means. Integer extrema and mean masks `00` versus `0`,
zero and negative results remain exact. All-NULL StandAge renders `(Null 2)`;
Elevation renders `  (Null 2)` with two leading spaces. Slope's ungrouped extrema
query has a row even without nonNULL values, then its absent grouped mean jumps
to cleanup before assigning the return. It therefore yields **Empty Variant**,
not String empty, for all-NULL, missing and no-join units. Other two functions
return String empty for missing/no-join units. NULL String arguments raise94.

The first fixture used DAO SQL `INTEGER`, which created Long4/four-byte fields,
not the intended Integer3/two-byte fields. Independent schema acceptance refused
that candidate despite24 passing outputs; its ten-file failed archive remains.
A separate absent-only `SMALLINT` correction proves actual Integer3 fields and
seals ten files. Both native owners normally quit without locks or canonical
drift. Original bodies are byte-exact; the project getter remains fixture-only.
No general SINGLE-to-text/noninteger decimal calibration, physical source
ownership, formatted dispatcher or report availability follows.

### Private typed environmental workbook preparation

The unexported [environmental workbook preparer](../summaryvegetationenvironmentworkbook.go)
reuses `prepareSummaryEnvironmentalHeader`, not another field dispatcher or
calculation implementation. Its complete selected-row witness, original unit
order, Source/Label grouping, special `n Plots` caption column and optional
English-name offset remain the accepted placement boundary.

Static `V7mdlReportsShortVeg` evidence separates this boundary from full output:
`AddEnvFields` selects Report=Yes in Order order and writes helper results into
the header; `NewShortVeg` starts that header at row5 when included; `BuildReport`
then inserts vegetation, deletes/rearranges rows and applies `FormatCells`.
The latter's Excel value assignment and multiply operation are not implemented
as implicit scalar conversion. The field-list form's positive-Order display is
not substituted for the full selected header. Its seven storage columns remain
those in `USysShortVegSummaryFieldList_CreateSQL`; source report queries are
inline VBA SQL, not a newly invented named query.

This private inspection writes text literally, including date-like,
formula-like and leading-zero values. Canonical integers within fifteen decimal
digits are numeric cells; larger integers refuse the entire output rather than
round. Numeric Empty and NULL are both visually blank, but remain distinct from
each other and empty text in complete hex-encoded input provenance on the
very-hidden `_VPRO_Source` worksheet. Unsupported XML/Unicode, overlength text
and non-roundtrippable OOXML escape literals refuse all bytes. Cancellation
during placement, after serialization or cleanup also returns no partial result.
No external file is opened or published.

The [shared value boundary](../reportworkbookvalues.go) reuses Summary
Environment's checked literal OOXML writer and the existing recursive
string/finite-number validation. Typed-cell policies stay explicit: existing
Lifeform callers still accept text/NULL identities only; Summary provenance
also accepts canonical integer/real/blob cells where the existing field-list
witness permits them. This does not broaden any existing reader.

There is no service registration, binding, UI action, runtime flag or default
change. This preparer does not compose the vegetation body, supply source
scientific/common-name projection, replace Excel macros, establish printing or
enable whole formatted Summary output. `SVHumusDepth` and the other currently
unsupported source branches remain unavailable at the existing preview boundary.
Reopened-workbook tests cover the complete53-row supported canonical selection
through both managed and external owned snapshots, without changing their data.
Focused coupled race150.725s, vet and production compilation pass. Distinct
review verifies the nine-path freeze and the byte-exact confirmation build;
no significant issue remains. Fresh full race passes root4124.177s with1876
managed inputs/11 artifacts, package/embed discovery, canonical/protected assets
and both closed native families unchanged. These are private preparation and
legacy-caller preservation checks, not new native/public report acceptance.

### Private named layer workbook composition

The [private composer](../summaryvegetationworkbook.go) consumes the existing
`SummaryVegetationPreview`; it does not look up names after aggregation.
`MakeSvQueries` groups by the ScientificName-or-Species expression before its
crosstab. The existing layer planner already performs that step with complete
reference-origin witnesses and literal NULL-first ordering. Its already accepted
display/average/class option shapes share the same literal transport; no formula
is recalculated by the composer.

Without environmental fields, original `BuildReport`/`AddNPlots` placement leaves
plot counts at row4, column headings at row5 and vegetation at row6. With `k`
selected environmental fields, unit headings remain at row4, fields occupy
rows5 through `k+4`, the `StartVeg` separator is at `k+5`, and vegetation starts
at `k+6` after the original row deletions. Layer, Spp and Order occupy A/B/C;
unit columns start at D. Common/guide names and alternate grouping remain
outside this boundary. Exact SV1 unit/pivot-column agreement is required:
subsets or reordering refuse all output, never silently shift counts or values.

The environmental and composed paths share one renderer and lifetime/provenance
boundary rather than serializing/reopening an intermediate workbook. It rejects
duplicate/out-of-frame placements, malformed value tags and unsupported visible
storage. Input witnesses remain unchanged and complete; text stays literal,
integers retain the checked precision bound and failed/cancelled cleanup clears
all returned bytes. Project worksheet identity must survive unchanged, including
the shared reserved-name/Unicode checks; sanitization/truncation is refused.

This is private in-memory preparation, not a new owned read or writer authority.
A future public coordinator must obtain the coupled preview in one owned
snapshot and separately qualify preferences, source/destination guards, binding,
client receipt/lifecycle and actual Wails acceptance. No service, binding, UI,
flag, publication, default promotion, Excel macro or printing is introduced.

Distinct read-only review finds no significant issues in the frozen private
composition and shared renderer. A focused evidence follow-up verifies exact
race/vet/build argv and exit0 markers, repeated coupled race155.760s and
byte-exact stripped SQLite-only production executables. Fresh full repository
race passes (root4172.686s), with1879 managed inputs/16 artifacts, discovery,
canonical/protected assets and both closed native families unchanged. This is
private preparation/caller-preservation acceptance, not a new public/native
report or source scalar-coercion/formatting claim.

### Private owned Summary workbook input

The private [owned reader](../summaryvegetationworkbooksource.go) builds the
existing compositor input inside an already owned context/read transaction.
It retains all five physical SU/vegetation/master/personal/layer tables, complete
selected/unselected field-list definitions and needed Env, plus literal source
attachment roles/paths. Unused personal and layer rows remain part of the source
even when their changes leave the visible workbook input unchanged. These are
observation-time inputs for future approval, not a file-publication guarantee.

The environmental reader and QuickVeg/lifeform composition are extracted from
the existing preview paths rather than duplicated. Existing public previews
retain their wire, values, independent gates and operation-specific errors.
Vegetation-only input does not require environmental tables or their gates;
combined input requires the existing environmental/numeric/lifeform gates.
Mismatched or unscoped ownership, stale context, malformed physical text and
failed/cancelled read cleanup return no partial source; retry preserves owned
attachments. Focused managed/external mixed and canonical source tests pass,
including reopened compositor bytes and invisible peer changes. Coupled caller
races59.019s, vet and stripped SQLite-only production compilation pass. Distinct
read-only review and focused identity/diff follow-up find no significant issues.
Fresh full-repository race passes (root4202.938s), retaining1882 managed inputs/13
artifacts, package/embed discovery, canonical/protected assets and both closed
native families.

This boundary does not interpret saved report preferences, authorize an output
file, add a service/binding/UI/flag, or enable common/guide names, hierarchy,
lumping, quality, constancy, source Excel coercion, macros or printing.

### Default-off accepted inspection workbook

The [inspection publisher](../summaryvegetationworkbookservice.go) uses
explicit title, average, display, class and environmental options, not saved
report callbacks. Review binds the entire physical input, exact scope/options,
worksheet/frame and deterministic workbook bytes. Duplicate/escaped request
keys, NULL options, raw malformed Unicode and unknown fields are refused before
typed decoding. Saved quality enforcement remains unavailable and cannot be
bypassed by disabling an older preview flag.

Publication reobserves complete owned inputs and uses the shared checked
no-replace artifact boundary. Invisible personal-source changes invalidate the
approval even if rendered bytes are identical. Precommit failure leaves no
destination and supports an explicit safe retry; committed drift/cancellation/
cleanup errors retain path/hash/outcome and warn against replay. Completed
snapshots prove observation-time equality, not absence of later external writes.

Coupled backend races, vet and production compile pass; exact-scope independent
backend review found no significant issues. Registration, interface bindings
and a separately gated client panel are implemented. The strict-Go wire fixture
normalizes machine paths and carries a zero approval placeholder: it proves
transport/validation, not authority to publish a real project. Client tests
exercise actual Svelte proxies, immutable review state, held asynchronous
validation, cancellation/remount, strict options, unknown/committed receipts and
the combined root lifecycle barrier. Existing environmental entrypoints retain
their fixed1/6/0 policy; only the explicit workbook validator accepts its selected
calculation options. Client review, private production build and actual Wails
acceptance and full integration pass. No public/default promotion is
accepted. It must be labelled as literal export of
read-only inspection, not the original `NewShortVeg` action. That entry actively
rebuilds master attribute descriptions and adds ancillary quality/SU output;
neither is silently reproduced here. Its `CreateSummary` local is unused, but
the original `USysShortVegOptions.btnViewReport_Click` sets
`sysCreateReportSummary` before calling `NewShortVeg`, and `BuildReport` consumes
that global. The same click selects the effective variant/lump table. These
active original-workflow effects remain required for full Report parity; the
inspection export deliberately does not dispatch them.
Group11 does not consume `ValueLimit`; irrelevant historical options
must not become new output-admission constraints.

Retained Show1/10/20 is not a class-calculation instruction for displays1/2/3/6.
The inspection adapter validates that retained selection and calls the existing
calculator with no class selection for those modes, without changing the
legacy class-only API. Review still returns and approval domainv2 binds the
original explicit Show value: different retained selections must not share
approval merely because their workbook bytes are equal.96 combinations cover
both environmental modes, both averages, every display and all retained Show
selections, including required rejection of missing class selection for4/5.
The first Wails candidate exposed this boundary and was rejected with zero
publications/all16 fixture files unchanged. Replacement formal checks,
independent reviewer follow-up and native acceptance pass. Actual production
Wails exports both environmental modes and independently reopens their bytes/
hashes/literal title. A private entry-only mount seam exercises real app remount
during held validation and committed publication without changing production
components/sessions; old completion cannot populate the new panel. Invalid
drafts and read errors persist across remount with correction/Undo available.
Each of four close refusals has a fresh actual CancelClose response and exact
closing-message witness before changing its barrier. A deliberately lost client
acknowledgement retains an independently inspected physical commit, blocks
replay/navigation/close and survives remount; acknowledgement preserves evidence.
Native backend-off refuses the enabled client.30 responsive observations measure
controls/labels, reviewed hashes and unknown destination receipt at three
widths with actual visibility, viewport intersection and containment. All16
physical data/config/audit files remain byte-identical across normally closed
phases. Fresh full race integration passes exit0 in4632.360s (root4580.720s),
preserving1893 managed inputs/23 artifacts, exact package/embed discovery,
canonical/protected assets and four closed native families. The portable
[evidence summary](MIGRATION_EVIDENCE.md#summary-inspection-workbook) identifies
the immutable private receipt; the delivered demo and default flags are unchanged.

### Private Summary ancillary source preparation

The original `USysShortVegOptions.btnViewReport_Click` sets
`sysCreateReportSummary` before `NewShortVeg`; its `BuildReport` consumes that
global. The original workflow therefore still requires `ReportSummary` when
selected and always adds `SU Table`. The unused local `CreateSummary` is not
proof that the form callback is dormant.

The new private ancillary boundary prepares those source inputs without
registering a service, adding flags/UI or changing the accepted inspection
workbook. Its additional Env, master site-unit names and actual AllSpecs
Description are read on the caller's existing owned transaction; the caller
reuses its complete already-read SU/Veg. All raw additional tables remain
available for later approval, including unused physical peers.

The literal normal-SU projection retains every physical membership, including
NULL/empty and plots without vegetation. `SnTable` unit admission instead
requires a real Veg join; duplicate Veg matches do not multiply its DISTINCT
units. Ancillary counts include every non-NULL SU plot in admitted units and
the full duplicate master-name join multiplicity. Missing master names omit
only that ancillary unit row. Conflicting `First` long-name candidates refuse
preparation rather than inventing a stable Access representative.

An explicit canonical reviewed RFC3339 time is retained for later deterministic
publication, not regenerated at export. Empty Env has a distinct unavailable
record-count state corresponding to the source's `MoveLast` error path.
AllSpecs version uses the existing physical Description boundary, retaining
missing/NULL/empty status separately; a future presentation adapter must account
for the original String function's display behavior without changing storage.
The model is not a rendered or publicly accepted original report. Attribute
preflight, temporary-table mutations, quality/lump/common-name callbacks,
Excel scalar coercion, formatting/printing and whole-workflow acceptance remain
separate. Focused races58.625s, vet4.625s and stripped SQLite-only production
10.625s pass. Independent review and focused follow-up find no significant
issues and verify source/runner/log/exit/binary associations. A reproduced
unavailable-Env regression now refuses a missing schema instead of confusing
it with an empty physical table. Fresh full race integration passes exit0
in4524.000s (root4509.951s), preserving1895 managed inputs/23 artifacts,
package/embed discovery, canonical/protected assets and the closed native
families. The rejected Excel/ADO control remains separately unaccepted with
clipboard restoration unknown; integration does not change that disposition.
Source preparation alone does not establish rendering or native/end-to-end
original Report acceptance; private literal composition is qualified below.

### Private ancillary worksheet composition

The private composition writes `SU Table` and optional `ReportSummary` in
the existing vegetation/environment workbook lifetime, without serializing and
reopening an intermediate XLSX. The legacy entry points and their typed source
wire remain unchanged. Shared physical-membership projection serves both the
source planner and the two membership blocks, retaining all rows and literal
NULL/empty identities rather than reducing them to vegetation-backed units.
The optional summary retains the source captions, D:F unit block, H:I membership
block and duplicate master-name count multiplicity. `SU Table` keeps D:G hidden.

This is explicitly normal-SU Layer inspection with zero value limit and no
lumping, quality, common-name, guide or constancy callbacks. B5 identifies the
actual selected normal-SU physical table, not an uncreated dynamic temporary
table. The reviewed RFC3339 timestamp is used literally instead of claiming
Access/Excel locale formatting. Missing/NULL AllSpecs Description displays
`Unknown`, literal empty stays empty, and empty Env displays `N/A`; their full
raw states remain distinct in very-hidden provenance alongside complete raw
body/reference inputs, source paths and options. Summary-only unavailable
tables or ambiguous `First` values do not block SU-only composition.

Every allocated/reserved worksheet name is compared with the XLSX library's
`strings.EqualFold` equivalence, including long-s aliases. Failure or cancellation
must return no partial workbook. Project-first sheet order and fixed readable
column widths are explicit presentation adaptations, not original Excel AutoFit,
scalar coercion, formatting/macros/printing or full Report parity. This boundary
adds no registration, flags, bindings or UI. Focused races66.640s, vet4.625s and
stripped SQLite-only production10.594s pass. Exact worksheet row boundaries,
including the post-write VBA Integer increment, are tested without constructing
an oversized workbook; actual oversized SU refuses before membership projection.
Independent review and substantive focused follow-up find no significant issues
and verify source/runner/log/exit/binary associations. Six isolated candidate/base
probes preserve byte-exact legacy vegetation/environment workbooks and the source
plan, with unchanged package/embed discovery; this is not universal input proof.
Fresh full race integration passes exit0 in4484.313s (root4467.898s),
preserving1897 managed inputs/36 artifacts, package/embed discovery,
canonical/protected assets and closed native families. Only directly related
status documentation is reconciled afterward; application/test inputs are
unchanged. No native/default/demo availability or original Report acceptance
is claimed.

### Default-off accepted ancillary inspection

This is an independently reviewed, native-accepted and full-race integrated
extension of the existing inspection
publisher, not another service/panel and not original `NewShortVeg` enablement.
It requires both existing workbook gates and independent
`VPRO_SUMMARY_VEGETATION_ANCILLARY_WORKBOOK` /
`VITE_SUMMARY_VEGETATION_ANCILLARY_WORKBOOK` gates, default-off separately.
The existing constructor still looks up only its original feature flag.

Legacy requests retain exactly five options and three export properties.
Absent `ancillary` retains the old review/source JSON, approval domainv2 and
workbook output; explicit NULL is rejected rather than treated as absence.
The optional mode is `ancillary: { createSummary: boolean }`, with the nested
boolean explicit and non-NULL. Exact keys, decoded-key duplicates, raw Unicode
and trailing JSON are checked. Legacy publication rejects `createdAt`, including
an explicitly supplied empty or NULL value.

In ancillary mode the server captures one canonical UTC RFC3339 time at review.
The review echoes it, and publication requires exactly that time under a
distinct ancillary approval domain. Export never regenerates it from the clock.
The same owned read snapshot captures the complete body, references and paths;
summary selection adds physical Env, master-unit names and AllSpecs Description
definitions. An already-read complete Env is reused for the environmental
header. SU-only selection does not read summary-only tables. Complete added
inputs, including unrendered peers, participate in approval, very-hidden
provenance and pre/postpublication source checks. The composer retains the added
tables/time once, not twice in nested source provenance.

The review exposes visible sheet order, physical membership count, optional
environment/non-NULL-selected counts, version status/display and literal
unit/count/nullable-long-name rows. Empty Env is unavailable count `N/A`, not
numeric0; NULL and empty long names remain distinguishable. Metadata validation
yields and checks request generation before installing results; the immutable
transport boundary remains `$state.raw`. Unit review uses50-row pages rather
than rendering the entire source catalogue. Draft selection, validation and
irreversible publication receipts remain separate in the existing retained
session and root navigation/native-close barrier. Checkbox edits do not erase
title/destination draft errors; Undo does not erase publication receipts.

No new/replayed Access, Excel or clipboard control is required or authorized by
this extension. Source adaptations and original preflight/quality/lumping/
guide/common/constancy/formatting/coercion/printing exclusions above remain.
Frozen validation passes202 related client cases,18 browser cases, check0/0,
private frontend build, coupled Go races159.422s, vet and production compilation.
Independent review finds no significant issues.26 isolated base/candidate vectors
retain legacy output, wire and approval identities.

Actual Wails accepts all four summary/environment output combinations with
independently reopened hashes/cells, numeric counts, formula-safe literal text,
hidden provenance and50/4-row metadata pages. Four actual remounts/four specific
native-close cancellation acknowledgements cover invalid drafts, actual held
metadata validation and a committed response withheld from the client.
Stale validation cannot install a result after remount. Unknown publication
receipts survive acknowledgement/Undo/remount without replay.36 observations
check visibility, viewport intersection, containment and labels at1400/760/420.
The frontend-enabled/backend-disabled profile refuses both ancillary modes while
legacy review still works. All16 fixture/config/audit files remain exact and
all owned processes close normally. A private main-entry-only seam enables
remounts; production components/session/validator remain unchanged.

Fresh full race passes exit0 in4545.281s, preserving1901 managed inputs/58
artifacts, discovery, protected data and five closed native families. See the
[evidence summary](MIGRATION_EVIDENCE.md#accepted-ancillary-desktop-inspection).
This acceptance does not enable defaults, replace the delivered demo or establish
original Report/Excel behavior. Original clipboard restoration remains unknown.

### Default-off accepted effective-reference inspection

This reviewed/native-accepted and full-race integrated extension reuses the inspection service, publisher,
owned snapshot, retained session and panel. Independently default-off
`VPRO_SUMMARY_VEGETATION_EFFECTIVE_REFERENCES` and
`VITE_SUMMARY_VEGETATION_EFFECTIVE_REFERENCES` require existing workbook gates.
Strict optional `references={showCommon:boolean,useGuideNames:boolean}` selects
read-only effective references; omission retains legacy wire/approval/output.
An independent approval domain binds complete physical inputs and both options.
Ancillary mode remains optional, with its own unchanged reviewed-time contract.

The projection excludes master IsAttribute=True (-1) and derives every metadata
row with non-NULL ShortName as Code/U/Definition, without filtering on metadata
IsAttribute or inventing physical master row IDs. Source origins retain the
physical description table/row. Master Code/Codetype255 and Lifeform LONG32 are
distinct from personal Code8/Codetype1/LifeForm INTEGER16 and staged Species8.
Guide selects master CombinedEnglishName before whole-row literal UNION;
personal EnglishName stays unchanged and derived guide names are NULL.
Guide forces effective common-name output, while requested options stay explicit.

Layer11 groups/sorts by common name when selected; its workbook column between
Spp and Order shifts environmental/header/body columns consistently. Complete
master/description peers remain in provenance even when not rendered. The
environmental lifeform calculation uses retained physical master rows, never
personal UNION rows or derived NULL-Lifeform attributes, with signed32 only in
this new mode. Ordinary accepted consumers remain signed16. Literal equality/
ordering is an adaptation, not general Access collation or UNION representative
parity. Group12 has the separately qualified composition below; Group13/14
First(common name) is not inferred.

Mode/common/guide drafts preserve independent title/destination errors and
irreversible receipts. Guide/common coupling is visible; name-group review uses
50-row pages and explicit NULL/empty labels. Eight strict-Go transport vectors,
actual client proxies, cancellation/generation, draft/receipt and consumed
rendering cases pass. Frozen validation accepts208 related client/24 browser
cases, check0/0/build, coupled race/vet/production and46 exact legacy comparisons.
Distinct exact-candidate review finds no significant issues. Actual Wails accepts
five independently reopened outputs,50/19-row name pages, four real remounts/
four exact native-close acknowledgements,54 responsive observations and four
independent backend refusals. Held metadata validation rejects stale results;
lost committed-publication acknowledgement stays unknown without replay or
receipt erasure. All16 fixture/config/audit files remain unchanged and owners
normally close. Private host-remount instrumentation changes no product component.
Fresh full race passes in4438.094s with1906 inputs/108 artifacts, discovery,
canonical/protected data and six closed native families unchanged. See the
[evidence summary](MIGRATION_EVIDENCE.md#accepted-effective-reference-inspection).
No physical reference mutation, original quality/lumping/callback/constancy/
Excel/printing, default or delivered-demo promotion is granted.

### Private accepted grouped workbook body

This private prerequisite reuses the same renderer for already-calibrated
normal-SU Group12/13/14 previews. Original query group names/types remain Layer/
TEXT1, LifeForm/TEXT3 and Group/NULL BINARY9. Display6 Order remains DOUBLE7/8;
the other supported displays retain their original TEXT10/0 Order, including
the late class override. The body preserves nullable groups, literal ordering,
leading-zero labels, TEXT covers, complete input/provenance and environmental
placement. Only an explicit grouped body Order range permits visible REAL cells;
all previous callers retain their real-value refusal. This is typed inspection,
not original Excel CopyFromRecordset coercion or formatting.

The existing original source fixtures supply independent expected fields/rows.
Owned managed/external previews, reopened cell/XML types, finite binary64
transport, cancellation/no partial output, input immutability, deterministic
bytes and exact legacy output pass112 source-layout/224 owned renders, four
source refusals and86 byte-exact baseline vectors. Coupled race152.562s,
vet9.625s and production12.906s pass. Distinct review finds no significant issues.
Full race accepts exit0 in4712.235s with1907 inputs/142 artifacts, discovery,
canonical/protected data and six closed native families unchanged. See the
[evidence summary](MIGRATION_EVIDENCE.md#accepted-private-grouped-workbook-body).
No new service/options/bindings/
flags/UI, grouped effective references/common/guide, ancillary routing, original
Report/callback/quality/lump/constancy/printing or default/demo change is included.
Group12/13 Display2 retains its source criteria-type failure. Group13/14 common
First selection remains unknown rather than synthesized.

### Accepted grouped inspection publication

This accepted default-off desktop extension reuses the existing workbook service, snapshot,
reviewed publisher, session and panel. Strict optional `groupAndOrder` accepts only
12/13/14; omission keeps the accepted Layer11 wire, approval and output. Explicit
11, NULL, wrong scalar/key case and duplicate decoded keys refuse. Independent
default-off backend/frontend `VPRO_`/`VITE_SUMMARY_VEGETATION_GROUPED_WORKBOOK`
flags govern availability. The existing Summary Group by control supplies the
selection, and the panel independently checks its flag.

Grouping uses a separate approval domain and exact typed inspection scope, with
full options/source provenance. Selected environmental rows remain supported.
Group13/14 effective references/common/guide and grouped ancillary sheets remain
unavailable; Group12 references have the separately qualified composition below.
Unavailable selections stay retained rather than automatically cleared.
Complete unit/pivot alignment remains required: zero-only source contributions
can omit a pivot column and must refuse, not gain an invented column. Group12/13
Display2 still refuses the calibrated source criteria failure.

Development backend race and eight production-Go wire vectors pass, including
strict authority, matrix/output/no-write, collision, drift, cancellation and
missing-pivot correction/retry coverage. Client tests exercise actual reactive
proxies, independent compiled panel flags, publication option identity, receipt/
draft retention, held-validation cancellation and consumed outer-panel routing.
Frozen validation accepts215 related client cases/31 browser cases, check0/0,
private build, coupled race787.157s, vet5.782s and production12.984s.86 exact
legacy vectors and package/embed discovery match; distinct review finds no
significant issues. Actual Wails accepts five reopened typed outputs, four real
remounts/four exact close acknowledgements,36 responsive observations, held
validation/stale-result refusal, retained unknown committed publication without
replay and four independent backend-off refusals. All16 fixture/config/audit
files remain unchanged, and all three owners close normally. Only a private
main-entry seam enables test remounts; product components/session stay unchanged.
Full race accepts exit0 in4588.062s, preserving1910 managed inputs/190 artifacts,
discovery, canonical/protected assets and seven closed native families.
No original Report or default/demo promotion.

### Accepted Strata effective-reference inspection

The existing inspection source/publisher/session/panel compose explicit
GroupAndOrder12 and effective references. Both existing independent
grouped-workbook and effective-reference backend/frontend gates are required;
no service, method, model, binding, dependency or flag is added. A distinct
combined scope and approval domain bind complete physical peers, requested
reference options and typed output. Original Report callbacks remain unavailable.

Original MakeSvQueries lines1241-1268 include EnglishName in Group12 grouping;
unlike13/14, this does not require choosing an arbitrary First(common name).
The private prerequisite reuses the effective-reference plan and existing
staging/aggregation kernel. Independent expected values cover common/guide
grouping, reference fanout, fixed denominators, NULL/empty names, whole-row UNION
origins, raw MAX provenance, cancellation, detached results and malformed-name
refusal/correction. Its affected race848.531s/vet/build and distinct review pass.

Common-name grouping shifts DOUBLE Order from column3 to4. The shared writer
requires the explicit paired row/column range; ordinary rendering still refuses
REAL cells, and either permitted Order column refuses the other. Existing
reference preparation, literal/NULL/empty distinctions, raw MAX origins and
retained drafts/receipts/stale-validation barriers are reused. Review and Publish
buttons share the selected-mode guard with their handlers, including retained
references when their independent build gate is off.

Eight new strict-Go vectors cover attributes/common/guide, both Average modes,
environment, class and distribution. Shared portable normalization removes
duplication while preserving both existing eight-vector files byte-exactly.
Managed/external tests reopen every body cell and hidden complete provenance,
checking independent gates, changed authority, an unrendered peer, cancellation,
collision and recovery. Frozen validation accepts220 related client/36 browser
cases, actual proxies/compiled handlers/consumed rendering, check0/0/isolated
build, coupled race1012.406s, vet5.984s and production12.531s.86 complete legacy
output/source/approval vectors match; distinct21-path review finds no significant
issues.

Actual Wails accepts five independently reopened outputs,50/19-row common-name
pages, four real remounts/four exact native-close acknowledgements,42 responsive
observations and stale held-validation rejection. Lost committed-publication
acknowledgement stays unknown without replay. Separate grouped-off/reference-off
profiles refuse combined inspection with zero publication while keeping the
unrelated Layer-reference/plain-Strata routes available. All16 fixture/config/
audit files remain unchanged and all four owners normally close/freshly disappear.
Private instrumentation changes only the isolated main entry.

Full race accepts exit0 in5153.016s, preserving1913 inputs/236 artifacts,
package/embed discovery, canonical/protected assets and eight closed families.
See the [evidence summary](MIGRATION_EVIDENCE.md#accepted-strata-effective-reference-inspection).
Lifeform/None common/guide names, grouped ancillary sheets and Strata Display2
remain unavailable; the attribute-only extension is qualified below.
No original quality/lumping/Report callbacks, Excel/formatting/
printing, default or delivered-demo promotion follows.

### Accepted Lifeform/None effective attribute inspection

Canonical NewShortVeg applies AddAttributeDescriptions before MakeSvQueries.
Its Group13/14 branches select First(common name) only when ShowCommon is
nonzero, so effective attribute references without common/guide names do not
require choosing that unqualified representative. The extension reuses existing
staging/aggregation, source snapshot, writer, publisher and retained panel for
explicit13/14 with both names off. A shared Go/TS availability policy covers
Layer/Strata and this exact new case. Both independent existing grouped/reference
gates remain required; unsupported names/ancillary selections remain retained,
not automatically cleared. No new service, model, binding, dependency or flag.

The new scope and approval domain are distinct; accepted Strata12 and all old
omission/request/output/approval paths match166 exact base/candidate vectors.
Common Name is absent, not an empty synthesized field. Group13 retains source
NULL groups from derived NULL-Lifeform staging and LEFT JOIN LayerCode
(MakeSvQueries1403-1409); Group14 retains its NULL BINARY group. Source13 Display2
still refuses while source14 Display2 remains supported. Reference fanout in
staging/aggregation, fixed denominators, whole-row UNION/derived origins and raw
MAX peers remain explicit. No new calculation or First-selection algorithm.

Independent values cover both groups/Average modes, including NULL and mapped
Lifeform groups. Existing calibrated display/class equivalence, common/guide
refusal, cancellation/detachment and managed/external reopened body/provenance,
authority drift, collision and exact recovery pass development race18.315s.
The shared body assertion still checks literal text, exact DOUBLE bits and no
formulas; its original wrapper retains complete private input/provenance checks.
Eight strict-Go vectors and41 actual proxy/compiled-handler/consumed-rendering
workbook cases pass both ordinary/browser runtimes; check reports0 errors/
warnings. A coupled table correction renders REAL Order instead of NULL and
uses original group captions rather than a fixed Layer header. Consumed rendering
first reproduces `(NULL)` instead of1.56, then checks all visible Order values
for ordinary Layer, Strata and both new groups. The first frozen attempt remains
stopped/unaccepted and hash-archived before any native launch; source/workbook
values were never changed to match the display. Corrected frozen validation
accepts225 related client/41 browser cases, check0/0/isolated build, coupled
race1182.656s, production vet/build and166 exact legacy comparisons.
Distinct19-path read-only review finds no significant issues.

Actual Wails accepts five independently reopened publications: Lifeform/None
attributes, Lifeform/class/environment, None frequency and None Average2/
environment. Every visible reference row checks original group caption,
scientific-name row header, NULL/empty group and typed Order, including paging.
Independent physical LF1 duplicate/membership arithmetic verifies None3.13/1.00
and LifeForm01 6.25/2.00 without application calculation/formatting helpers.
Four actual remounts/four exact native-close acknowledgements,42 responsive
observations, held-validation stale rejection and persistent unknown committed
publication without replay pass. Both independent backend denials preserve
unrelated authorized routes with zero publication. All16 fixture/config/audit
files remain unchanged and all four owners normally close/freshly disappear.
Only the private main entry is instrumented. Inherited native witnesses were
corrected for accessible th row headers, original LifeForm XML headings and
cleared post-ack review plans; the completed first publication was verified
physically before continuation, never replayed.

Full race passes exit0 in5781.860s, preserving1916 inputs/285 artifacts,
package/embed discovery,26 canonical inputs/21 protected assets and nine closed
native families. See the [portable evidence](MIGRATION_EVIDENCE.md#accepted-lifeformnone-effective-attribute-inspection).
Common/guide names, grouped ancillary, Strata/Lifeform Display2, original Report/
callback/formatting/printing and default/delivered-demo promotion are not granted.

### Accepted grouped ancillary inspection

This accepted successor composes the existing read-only SU Table and optional
ReportSummary with supported Strata12, Lifeform13 and None14 workbooks. Existing
effective-reference rules still apply: common/guide names are allowed for
Strata only;13/14 require both off. No original Report action is enabled.

The canonical active `MakeSvQueries` constructs SnTable from original project
Veg joined to selected SU before `CreateTempVeg`/`BuildDataTable` grouped staging.
`BuildReport` selects ReportSummary independently of grouping; `AddSuTab` copies
the selected SU with no grouping branch when quality enforcement is off.
Consequently the existing raw-SU/Veg admission, physical duplicate memberships,
master-name join multiplicity and ambiguous First-name refusal remain the
ancillary contract. Grouped pivot rows must not replace that raw admission.

Original `ShortVegOptionText` captions determine ReportSummary B7: Strata,
Lifeform or None. These differ from body DAO field captions Layer, LifeForm
or Group. B8 remains Species. B5 continues the explicit adaptation of displaying
the selected physical SU table, not inventing an Access scratch table.
Complete original source/options/time and the new combined scope are retained
in hidden provenance. The grouped ancillary approval domain is separate;
requests without this composition keep their existing scopes and domains.

The shared source/snapshot, body/ancillary writer and no-replace publication/
session/lifecycle are reused. Existing grouped, ancillary and optional reference
gates are independent; there is no new production file/API/binding/flag.
Tests include typed main bodies with optional common-name columns,
source worksheet identities/metadata,16 managed/external compositions, strict
Go transport, all eight independent gate combinations and explicit mismatched
authority/cancellation refusal. Guide-only API authority remains exact; the UI
also selects Common when Guide is selected.
Frozen validation accepts229 related client/45 browser cases, check0/0/isolated
build, coupled race768.297s/vet/production and206 exact legacy comparisons.
Distinct read-only review finds no significant issues.

Actual Wails accepts six independently reopened publications across all three
groups, common/guide/attribute references, summary, SU-only and environment.
Every main body cell, typed shifted Order, original caption and full hidden
provenance is inspected. Independent physical reads establish eight memberships,
two admitted units/seven selected plots, duplicate master-name count effects and
NULL metadata. Four remounts/four exact native-close acknowledgements,54 responsive
observations, held-validation stale refusal and unknown committed-publication
retention pass. Each of three independent backend-off profiles preserves an
unrelated authorized route. Four retained unsupported-name drafts dispatch
nothing; corrected reviews succeed. All16 source/config/audit files remain
unchanged and all six owners normally close. The retained first witness used
larger-fixture constants; the completed workbook and actual fixture were checked
before continuing, without replay.

Full root race passes in4861.766s, preserving1918 inputs/332 artifacts,
discovery,26 canonical inputs/21 protected assets and ten closed native families.
See [portable evidence](MIGRATION_EVIDENCE.md#accepted-grouped-ancillary-inspection).
No default/delivered-demo promotion or acceptance of13/14 common/guide,
Strata/Lifeform Display2, quality/lumping, original formatting/callbacks,
Excel coercion or printing. No Access/Excel/clipboard action occurred.

### Private accepted Constancy preparation

Original `IsConstancy` forces Group3/Display1/common0 through the existing
callbacks without clearing the historical limit. Existing coupled presentation
preference CAS already models that persistence boundary; this preparation adds
no competing preferences or automatic saves.

The later `MakeSvQueries` Constancy override selects numeric
`USysAllSpecies.Lifeform`, scientific name with NULL-only raw-code fallback and
lexical prefix Max Order. GROUP BY separately retains numeric lifeform, raw
Species and displayed name. Ordinary Group13's textual LayerCode label is not
equivalent. Group13/Display1 keeps empty MyWhere and LifeForm/Spp ordering.
Ten disposable staged-query cases across both Average branches establish
Lifeform DAO LONG4/size4, Order TEXT10/size0, NULL-first numeric ordering and
retained identical visible rows through SV3 DISTINCTROW. Distinct-valued rows
with equal displayed keys also remain separate. Stable raw-code tie order is
an explicit deterministic read-model adaptation, not source tie-order parity.

The private kernel reuses effective-reference preparation, original Group13
MAX/SINGLE staging, joins, denominators, aggregation, decimal formatting and
whole-row physical witnesses. Only the new Constancy caller uses explicit
LONG32 integer-reader variants on the already validated master32/personal16
union. Every legacy wrapper remains signed16. An unused master2147483647 does
not invalidate this new route; a selected formatted lifeform still must fit
the original TEXT3 stage or the operation refuses without partial output.
RawSpecies remains separate from display name and the numeric group. Derived
attribute NULL groups combine original stage layers without losing origins.

Independent tests retain the ten source-query vectors, cross-stage/attribute
arithmetic, duplicate MAX peers, helper/caller policies, bounds, detachment and
cancellation/retry. Frozen six-path/1919-input validation accepts focused
race15.688s, coupled race1507.859s, production vet/build and246 exact legacy
workbook/source/approval/scope/options comparisons. Distinct read-only review
finds no significant issues. Full root race passes in6823.438s, preserving1919
inputs/2904 protected artifact entries, discovery,38 canonical inputs/21 assets
and eleven closed native families. See [portable evidence](MIGRATION_EVIDENCE.md#accepted-private-constancy-preparation).

This is not a public Constancy route: no public DTO/service/transport/binding/
frontend/flag/prefs/default/demo change. Query calibration uses explicitly
projected staged inputs, not full Report/preflight/BuildLifeFormTable execution,
Excel/clipboard/printing or callback parity. The next consumer must separately
wire strict authority, numeric rendering, raw-code duplicate validation and
parent presentation/calculation ownership before desktop acceptance.

### Constancy workbook consumer

**Independently reviewed/native-verified and full-race integrated; default-off.**
The existing inspection panel consumes the loaded presentation Constancy draft:
effective Group13/Display1 with the retained Average and historical ShowClass
transport (calculation0). It does not rewrite basic calculation drafts, add a
checkbox/editor or persist preferences. Effective references are selected with
common/guide names off; separate manual reference/name drafts remain available
after deselection. Existing grouped, reference and presentation flags are
independent prerequisites. Unavailable intent remains selected, never an ordinary
workbook fallback.

The optional request marker must be true when present. It requires explicit
Group13/Display1 and effective names-off references. A distinct preview source,
scope and approval domain cover numeric nullable `Lifeform` LONG4/size4,
TEXT Order, raw-species grouping and supported environment/ancillary composition.
Raw identity is retained in provenance, not an invented worksheet column.
The first body column is numeric; the original Constancy caption yields to
`Site Unit Summary Table` when environmental rows are included. The actual
inspection table renders numeric groups and retains duplicate visible rows.
Ordinary validators and signed16 readers stay narrow; unused valid master LONG
values are not a reason to widen legacy consumers.

Development tests include eight strict production-Go wire vectors, independent
MAX/fixed-denominator arithmetic, actual Svelte proxies, all eight prerequisite
combinations, consumed panel/parent rendering, cancellation/remount, collision,
cleanup/retry and retained receipts. Frozen focused/coupled races,236 related
client/52 browser cases, check/build/vet/production and250 exact compatibility
comparisons pass. Numeric-leading ordering now initializes after the first row;
seven explicit cases cover valid negative/raw-tie starts and error-with-zero-bytes
duplicate/descending group/species/raw/NULL transitions. Independent review and
focused follow-up find no significant issues.

Five once-only Wails outputs independently reopen typed bodies, original captions
and hidden provenance, including environment and raw-SU/ReportSummary composition.
Actual paged tables show numeric/NULL/negative groups and equal-visible TWIN/TWN2
rows with separate raw identities. The pre-existing LF2 attribute-stage fanout
is retained, not flattened into the new pair. Presentation selection without Save
preserves basic/manual drafts and16 source/config/audit files. Four actual
remounts/exact close acknowledgements,54 three-width observations and independent
grouped/reference/presentation backend denials pass; ordinary reads remain usable.
Presentation-off source editing stays unavailable, not a fabricated loaded draft.
Full root race integration passes in5011.094s, preserving1923 managed inputs/
2964 artifacts,38 canonical inputs/21 assets and twelve closed native families.
No new service/flag/default/demo promotion or original Report/printing claim.

### Private unanimous common-name preparation

Group13/14 omit EnglishName from the original GROUP BY while selecting
`First(USysAllSpecies.EnglishName)`. The private successor therefore does not
reuse Group12's common-name grouping or invent a first reference representative.
Its explicit mode accepts only normal-SU13/14, Display6, showClass0 and an
effective common/guide reference plan. Ordinary/Constancy modes are separate
typed values; legacy wrappers and public availability guards remain unchanged.

Twenty-four original projected queries cover six equivalent scenarios across
both groupings/averages. Nonempty/NULL unanimity is stable in this bounded
projection. Uniform physical empty TEXT projects NULL through original First;
physical/UNION values and metadata remain empty. Mixed text, NULL/empty and
unit-separated choices are observations only. The failed initial expectation
and its normally closed family remain preserved; a fresh second family passes
all inputs, exact output metadata/arithmetic and reopened snapshot observations.
The Access database reopen is exclusive, not a database-level read-only mode.
Source review accepts the bounded invariant with that qualification.

The shared aggregation checks exact typed common values within the original
literal group across all contributing units, before returning any report.
Different values fail explicitly, including case/space differences and NULL
versus empty; they are not extra groups or a fallback name. An unused reference
or nonmember stage row does not create a conflict, but a contributing NULL-cover
row still participates in First. Uniform empty normalizes only the output cell.
The raw reference plan, complete UNION origins, raw MAX peers, fanout and
membership witnesses remain unchanged. Guide names consume the already accepted
effective EnglishName plan, not newly executed rename callbacks.

Independent24-case raw/stage combinations and additional grouping/guide/fanout/
provenance/cancellation/retry/detachment checks pass. Frozen focused/coupled races
pass in19.296s/763.687s; production vet/build and250 exact legacy comparisons pass.
Distinct application review finds no significant issues. Full root race
integration passes in5005.937s, preserving1924 managed inputs/3662 artifacts,
38 canonical inputs/21 assets and fourteen closed native families.
This is a private
prerequisite, not a new desktop consumer, original Report/preflight/formatting/
printing or whole-collation guarantee. No new production file/service/binding/
flag/preference/default/demo change follows.

### Class threshold neighbors

Thirty-two independent physical inputs yield192 typed original helper outcomes
across historical1/class10/value20 and readonly reopens. Prominence upper
bounds15/50/100/200 and Goldstream5/25/75/150/300/500 are inclusive on the
assigned SINGLE. Adjacent SINGLEs may round to the same seven-significant TEXT
(for example `15`) while requiring different classes; never classify the
formatted text. Historical1/class10 agree as INTEGER2 or NULL1 Variants.
Negative linear products produce NULL classes, but signed SINGLE values for20.
No negative square-root input is executed or accepted. Twelve source files seal
with unchanged bodies/canonical hashes and fresh owned PID/DAO-lock absence.

### Prominence and Goldstream function prerequisites

Two normally closed disposable owners calibrate12 physical operand cases for
each historical ShowClass1, class10 and value20 getter. A controlled isolated
getter supplies the original helper property boundary without original registry
writes. Six original loaded/reexported function bodies remain exact, including
RkFormula3/4, ProminenceClass, GoldstreamClass and vpRoundUp.

Historical1 and class10 yield the same INTEGER Variants. Value20 yields SINGLE
Variants: the ordinary operands produce21.650634765625/118.58541107177734,
while DAO query inference converts them to TEXT21.65063/118.5854. Preserve
this distinction; the numeric helper is not already the report's transport cell.
Seventy-two typed helper outcomes independently reopen, alongside36 DAO query
cases per owner. Twenty files seal, canonical hashes remain unchanged and both
fresh PIDs/DAO locks are absent.

RkFormula3/4 format Sum/nPlots and Count/nPlots before arithmetic and do not
consult AverageType. Their guards are non-NULL Sum and positive nPlots, not
positive Count. Source vpRoundUp performs formatting but ultimately returns
ValIn unchanged in both branches; it does not floor values below0.1. Tiny
formatted means can therefore produce numeric0. NULL Sum/nonpositive nPlots
yield NULL. Negative sqrt/error dialogs, universal VBA Sqr equivalence and full
crosstab output/Order remain outside this function-only proof; modes4/5 stay
unavailable until those report boundaries are established.

Eighteen original normal-SU/layer Display4/5 crosstab scenarios now independently
reopen with exact six function bodies: ordinary/reference fanout/all-NULL,
historical1/class10/value20. All observed Layer/Spp/Order/unit fields are TEXT.
Ordinary class4 A3/B2 and class5 A4/B4 preserve their lexical Order; numeric20
uses source shortened SINGLE strings (86.60254/50,237.1708/223.6068). Fanout
does not clamp presence at1. All-NULL groups yield zero final rows without the
Display1/3 criteria error. Twelve more files seal with source hashes unchanged
and normal owned Quit/fresh PID/lock absence. General SINGLE text formatting,
universal sqrt behavior and negative-domain dialogs remain separate boundaries
before implementing/enabling these modes.

Twenty-seven original `vpRoundUp(CSng(ValueIn))` identity-return witnesses now
independently reopen DOUBLE input/SINGLE output bits and inferred TEXT values.
Seven-significant decimal midpoint rounding is half-away:123456.25 becomes
123456.3, whereas its lower adjacent SINGLE becomes123456.2. Nearest-even
`FormatFloat(...,'g',7)` is not equivalent. SINGLE negative zero retains its
sign and yields TEXT "-0", not "0". One million stays fixed; ten million becomes
"1E+07" with uppercase E and padded exponent. Tested small values through1e-7
stay fixed; minimum normal/subnormal values are scientific. Class20's
representative shortened strings match this original SINGLE coercion boundary.

Twelve files seal, with original body/canonical hashes unchanged and normal
owned Quit/fresh PID/DAO lock absence. The precise small-exponent transition
below1e-7 and other machine locales remain unverified. Class4/5 arithmetic,
SINGLE assignment and text conversion are separate boundaries; restrict or
refuse any unsupported domain explicitly rather than using a generic formatter
or claiming these scalar witnesses alone enable a report.

Two closed disposable DAO copies verify24 assignment observations and actual
cloned USysV2Veg metadata (Cover SINGLE, Plotnumber TEXT8 Required, zero-length
text disallowed). Physical single-table DISTINCTROW retains duplicates. Adjacent
0.15 DOUBLE values assign SINGLE bits9a99193e; negative zero survives and tiny
positive values underflow to zero. Text length9 truncates8. Default DAO Execute
can omit invalid NULL/empty plots or empty species, or assign overflow cover
NULL; fail-on-error produces explicit errors and rolls back. The first SQL
parameter setup converted empty inputs to NULL before projection; independently
reopened AddNew physical-empty cases establish the actual distinction.
Canonical hashes remain unchanged and seven evidence files are sealed.
These observations neither grant a raw LongVegReport path nor require the
desktop to inherit silent truncation/omission/repair. DoCmd.RunSQL dialogs and
partial-action timing have not been accepted.

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

# Site unit detail / summary environment source boundary

## Status

An independently default-off normal-SU, layer-cover Summary Environment preview
now implements the39 source summary fields in SITE/VEGETATION/SOILS order,
with Mean and Interquartile choices. Its owned service, generated bindings and
responsive desktop panel pass focused tests and disposable Wails checks.
Full integration passes (fresh root794.606s/all packages). Species/lifeform summaries, hierarchy/field-derived
units, saved options and Excel/file publication remain unavailable. This is
distinct from the per-plot Long Environment preview; the consolidated client
presentation remains open in [client scope](CLIENT_SCOPE.md).

The local reference root is
`C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64_forAI`. Static exports were read
without opening Access or modifying source/data:

| Source | SHA256 |
| --- | --- |
| `Forms/USysSuDetailReport.txt` | `7b98c8c6a395b2352b842f52c435febe0afb5198b819e0c646d559dda8ba05b8` |
| `Modules/V7mdlReportsEnv.txt` | `8917bcd43c6e8001585795aa525562b77cb4ecd9d74b65350a01796ee69cb8a9` |
| `Modules/V7mdlReportsSiteUnitDetail.txt` | `af23d63350c845dc40a151135a147a312a6504c7de8c9d948ce7e1904b6bee98` |
| `Modules/V7mdlSetCurrent.txt` | `4f27c6021b480b4ddc56607b0a518801f7afc83b7f28d106bfae6790f3607d66` |
| `Modules/V7mdlShortCutToolBarCmds.txt` | `ce0c276fd78e43e6eb97ed36eceec988c6d26a4a06c837a1a2728dbf2e8feb9c` |
| `Tables_Def/USysSuTableDynamic_SU_CreateSQL.txt` | `6872acec48de25df984ebb68577df1257106f907fc2b84a5decc64e9837e65b1` |

The form export is UTF-16. Its commented event bodies are evidence of inactive
wiring, not permission to recreate presumed behavior.

## Live entry and options

`btnCreateReport_Click` calls `SiteUnitDetailReport`. The live `Form_Load`
assignments read only `SEOptValueMethod` and `SESuType`; their AfterUpdate
events persist those two properties. Both registry getters default to 1.

| Option | Source behavior |
| --- | --- |
| Value method 1, Mean | Helper-specific minimum / formatted average / maximum |
| Value method 2, Interquartile (25% - 50% - 75%) | Excel `Application.Quartile` at indices 1 / 2 / 3 |
| SU type 1 | Copy current selected SU PlotNumber/SiteUnit rows into the temporary dynamic SU table |
| SU type 2 | Build hierarchy break table, use `Mid(Reference,2)` as SiteUnit |
| SU type 3 | Open `USysSelectUnitFields`; report consumes the prepared dynamic SU table |

The cover/presence/species-option read/write assignments in this form are
commented. The report still reads `SECoverCalc`, `SEOptAndOr`, `SEOrderBy`,
`SESelectSppPresent`, `SESelectSppCover`, and `SEIncludeSppSummary` from
`clsRepOpt`. Do not infer that their visible-looking controls currently save
them, or substitute new defaults for historical stored values.

## Scope is not the Long Environment scope

The report requires a selected SU and sets `MyProject = "USysEnv"`.
The ordinary project setter creates this query with `SELECT DISTINCTROW
Env.*, Admin.* FROM Env INNER JOIN Admin ON Env.PlotNumber = Admin.Plot`.
Another toolbar path replaces it with `SELECT DISTINCTROW Filtered_Env.*`.
Current query provenance therefore matters; an unqualified whole-project Env
read is not a demonstrated replacement for every source entry path.

Normal SU preparation copies rows without `DISTINCT`. The temporary table has
nullable PlotNumber TEXT7, SiteUnit TEXT255, Group TEXT255 and Level INTEGER;
its PlotNumber/SiteUnit indexes are not unique. Preserve physical membership
and duplicate weights rather than deduplicating from field appearance.

The unit list joins USysEnv to the dynamic SU table, excludes NULL SiteUnit,
counts joined `USysEnv.PlotNumber`, and orders SiteUnit descending. Empty unit
text is not the same exclusion as NULL. Name lookup groups MasterSiteUnitList
with `First(SiteSeriesLongName)`; retain NULL/empty/duplicate candidates before
choosing an explicit deterministic desktop adaptation. Orphan Admin behavior,
DISTINCTROW identity and filtered-query provenance must not be inherited from
the different Long Environment planner.

Source temporary-table/query deletion and Excel sheet-name collision skipping
are implementation artifacts, not authority for canonical mutation, destructive
restoration or silently dropping report units.

## Numeric helpers must not be conflated

These source helpers share a triplet presentation, but not identical conversion,
rounding, NULL or empty-result behavior:

| Helper | Mean-path evidence |
| --- | --- |
| Elevation / StandAge | `Val(field)`, minimum/maximum; average formatted with `00` |
| Slope | Raw SlopeGradient; average formatted with `0` |
| GroundCover | Extrema use `Val(Format(field,'#,##0.00'))`; average uses `Avg(Val(field))`, formatted with `0.0` |

Interquartile helpers obtain a DAO `GetRows` array and call the shared Excel
application. The declared FieldName parameter is not used to select a column.
The `On Error GoTo errGetMedian` statements are commented, so the remaining
`"N/A"` labels do not establish an active success-shaped fallback.

Keep source `Val` conversion and source number formatting distinct from input
validation. Do not replace them with generic ParseFloat/trim/round logic or
claim that an R summary establishes Access parity. NULL suffix counting,
all-NULL early exits, duplicate membership weights, zero/negative values and
format-before-extrema differences need actual expected-output tests before
summary preparation can be accepted.

## Grouping and observed differences

Source sections are SITE, VEGETATION and SOILS, with an optional species summary.
Preserve meaningful labels/order and related grouping rather than spreadsheet
coordinates. Numeric and categorical summary helpers, lifeform versus strata,
reference names and underlying Humus/Mineral projections remain separate.

Both source labels `Site Disturbance 1` and `Site Disturbance 2` call
`SiteDisturbanceSummary(..., 2)`. Record that difference; do not silently correct
the first argument to 1 or present the duplicated source output as independently
verified domain intent.

The ready implementation boundary is a read-only normal-SU scope/provenance
preparation followed by batched, source-specific environmental summaries. It
does not authorize hierarchy/field-derived units, species aggregation, report
publication, canonical writes or enabling the summary entrypoint.

## Private normal-SU preparation

[The scope planner](../siteunitdetailscope.go) composes existing physical schema,
signed64 row identity, tagged cell and clone guards rather than introducing
another decoder or table reader. It accepts only explicit ordinary project
Env/Admin or current-selected-SU filtered provenance; unknown filters,
hierarchy/field-derived scopes and unselected SU fail without partial output.

Both accepted inputs join Env and Admin before weighting each normal-SU
membership. The selected-SU filter must not multiply this final join a second
time. Every physical SU/Env/Admin combination remains identifiable; missing Env,
missing Admin, NULL plot and NULL unit memberships remain explicit diagnostics.
Empty plot/unit strings can join and are not silently repaired. A positive
caller-provided joined-row budget succeeds at the exact limit and refuses the
next combination, without returning a truncated scope.

Ordering and key equality are exact literal SQLite-style comparisons, not a
claim about Access locale/collation. Physical row identifiers remain exact
strings and ordering is deterministic, not a numeric ranking. Outputs are
detached; cancellation, malformed tagged identities and schema/physical-ID
errors return the zero result. This preparation does not resolve names or
validate unused numeric summary fields.

Focused/shared race3.937s passes. An independent disposable SQLite join checks
actual 2 Env x 2 Admin x 2 SU weighting, the additional empty-unit membership,
and physical triples. Tests cover exact nine-row budget success/eight-row
refusal, excluded-row reasons, literal apostrophe/CRLF/NUL/Unicode identities,
permutations/output ownership, unknown provenance, malformed inputs and
deterministic mid-plan cancellation. These are private kernel receipts,
not Access/native report parity or full application integration.

The private owned reader additionally uses existing context leases, physical
table checks and checked read-snapshot cleanup. Project and external selected
SU paths are supported without TEMP-view substitution or persisted selection
changes. Shared race6.433s/vet and independent review (focused race2.276s) pass;
tests cover current paths, repeated zero-write reads, stale/None/unowned
contexts, view substitution, lease deadline, commit/rollback cleanup failure,
post-read cancellation and clean retry. Full406-file race integration passes
(fresh root979.552s; unchanged packages may be cached), with all reviewed code,
eight original exports and192 frontend/dependency identities unchanged.
The [private scope successor](../archives/site-unit-detail-scope-checkpoint/evidence-manifest.json)
retains source, review, focused/full receipts and the350-file accepted saved-title
predecessor identity. No service authorization, binding/frontend change, new
report/native claim or production/default promotion follows from that predecessor.

## Language references for the next arithmetic boundary

Microsoft's [Val reference](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/val-function)
documents period-only decimal recognition, stopping at commas/currency symbols,
stripping selected whitespace, radix prefixes and type-suffix errors.
[Format](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/format-function-visual-basic-for-applications)
is internationally aware. Their composition in GroundCover extrema is not
equivalent to either raw numeric extrema or a locale-aware generic parser.

The source uses legacy [Quartile](https://learn.microsoft.com/en-us/office/vba/api/excel.worksheetfunction.quartile),
not a chosen R quantile convention. Public documentation describes index/error
semantics; the [inclusive replacement's example](https://support.microsoft.com/en-us/excel/functions/quartile-inc-function)
is useful expected-vector evidence, not proof of installed legacy behavior,
DAO array handling, source formatting or a full report.

A minimal ordinary/filtered duplicate-weight ACE probe was prepared under the
sole disposable `native-site-unit-detail` fixture but did not execute:
this machine refused PowerShell script execution. No execution-policy change,
Access application, database creation or canonical/source mutation followed.
The weight evidence above therefore remains actual SQLite/private Go evidence,
not a successful ACE weight probe. Access collation remains unverified.

## Normal-SU summary desktop successor

[The summary planner](../siteunitdetailreport.go) reuses the accepted physical
scope and shared name-candidate resolver. It transports every physical
SU/Env/Admin triple and all memberships/exclusion reasons, including empty units
and duplicate weights. A one-million joined-row budget refuses overflow without
truncation. Missing Env/Admin memberships are excluded from the source inner
join, not displayed as invented observations. Name ambiguity is explicit rather
than inheriting arbitrary `First()`. The39 fields retain labels, section order,
Admin.HumusThickness and the duplicated SiteDisturbance2 binding.

The [numeric helper](../siteunitdetailnumeric.go) batches the three distinct
source arithmetic families. Elevation/StandAge use minimum/two-digit mean/maximum;
Slope uses raw numeric extrema and an integer-format mean; GroundCover uses
format-before-Val extrema and one-decimal mean. Its maximum query alone includes
NULL formatted to empty and converted to zero. All-NULL early returns remain
different: Mean Elevation/StandAge retain the NULL suffix, while Slope/GroundCover
and Interquartile return the source empty result. Inclusive quartiles preserve
physical weights and do not invent an `"N/A"` error fallback.

A fresh disposable DAO database measured half-away-from-zero Format ties,
English thousands commas terminating Val, Format(NULL)/Val zero and NULL
concatenation. A separately owned Excel instance confirmed legacy
WorksheetFunction.Quartile results3/6/9 for a DAO-shaped rank-two array0/4/8/12.
The first Application.Quartile dispatch attempt was unavailable in PowerShell;
the successful replacement proves the installed legacy calculation, not the
original Application dispatch or entire report. Normal Quit initially retained
the process at500ms; a later independent observation confirmed PID10852 absent.
No further oracle attempts were made. Exact primary observations are retained
in the disposable fixture, separate from the unexecuted scope probe.

Explicit adaptations/limits: English-locale formatting and deterministic Go
shortest numeric rendering, not arbitrary VBA CStr precision; numeric/NULL
storage only, with historical numeric text/blob explicitly refused; literal
SQLite key equality/category ordering, not Access collation. Moisture codes and
source Single aspect boundaries remain source-specific. BGC unit counting
retains the two-query asymmetry: only non-NULL Zone concatenations establish
categories, but all matching concatenations contribute their counts.

Runtime `VPRO_SITE_UNIT_SUMMARY=true` and build
`VITE_SITE_UNIT_SUMMARY=true` are independent opt-in gates. The panel explicitly
requests normal-SU/layer-cover/no-species scope. The initialization successor
maps active `USysSuDetailReport.Form_Load` to owned reads of retained
`ReportOptions.SEOptValueMethod` and `SESuType`, preserving saved Interquartile.
Missing, NULL, text or out-of-range saved values fail without inferred defaults.
Hierarchy/field-derived values are retained and displayed as unavailable;
Create Report is disabled rather than silently switching scope. Method edits
affect preview only; explicit Reload restores the saved method/scope.
No registry values or source temporary queries are read or written.
Hierarchy/field-derived units, lifeform covers and species/publication controls
are not enabled. The separately gated reviewed-preference successor below
implements an explicit desktop adaptation of the active AfterUpdate persistence;
loading saved options alone is not evidence of that write path.

Focused/shared Go race3.082s,20 frontend summary/report/read/close tests and
Svelte check0 errors/warnings pass; the full frontend suite passes441 tests
(13 pretest plus428 runner). Isolated enabled/default frontend and production
Wails builds leave protected assets unchanged. Independent review
`83df75df-a614-4d3e-998c-2a0e781da988` found no significant issues.
Four normally closed disposable Wails owners pass14 cases: independent UI/
backend/default denial, exact SQLite physical triples/weighted mean/quartiles,
actual1400/600px labels/values, held real read responses with cancellation/
remount/retry and native-close/navigation barriers, plus existing FS882 valid/
invalid draft Cancel/Discard/Undo behavior without replaying Save. All16
database/config hashes are unchanged after each close. These receipts accept
this bounded preview, not full forms/reports parity or production promotion.

One coherent full race integration passes with fresh root794.606s/all packages,
against611 unchanged pinned Go/frontend identities. The sealed desktop successor
is [the summary checkpoint](../archives/site-unit-summary-desktop-checkpoint/evidence-manifest.json).

The saved-initialization successor passes focused/shared Go race3.125s,
22 targeted frontend tests,444 complete frontend tests/check0/0 and isolated
enabled/default frontend/Wails builds. Twelve real Wails cases/three normally
closed owners verify actual saved quartile/full context dispatch, weighted
arithmetic, unsaved preview Mean/Reload, both unsupported scopes without repair,
malformed-method refusal, real held initialization with report/navigation/native
close barriers and cancellation/remount/retry, actual1400/600px labels and
independent default/backend denial. All16 database/config hashes restore.
Only the driver changes/restores fixture options; no application writes occur.
Full coherent race passes with fresh root797.541s/all packages against613
unchanged Go/frontend identities. The
[initialization successor](../archives/summary-options-desktop-checkpoint/evidence-manifest.json)
preserves the42-file None and48-file original summary predecessors. This
initialization does not implement AfterUpdate preference writes or overall parity.

## Reviewed summary-option persistence

Original active optValueMethod/optSiteUnitType AfterUpdate routes write
`clsRepOpt.SEOptValueMethod` and `SESuType`. The successor replaces immediate
registry writes with explicit reviewed Load/Save/Undo in retained YAML.
Runtime `VPRO_SITE_UNIT_SUMMARY_PREFERENCES` and build
`VITE_SITE_UNIT_SUMMARY_PREFERENCES` remain separately default off.
Only normal-SU proposals are accepted; retained hierarchy/field values remain
readable but require explicit normal-SU selection before Save. This does not
enable either unavailable report scope.

Shared typed report CAS retains the accepted Google Earth/Long Environment
string path while adding strict integer reads. Both keys are observed before
collision checks; unchanged assignments are omitted. Atomic replacement follows
owned snapshot cleanup and preserves a committed receipt outside callbacks.
Rollback/replacement failure, cancellation, stale context, invalid raw JSON,
unknown/duplicate/missing/NULL keys and concurrent preferences reject explicitly.
No database, audit, source temporary table or registry writes occur.

Persistent context/path-keyed drafts preserve invalid errors, unknown authority
and receipts through remount. Reads are cancellable/generation-checked; Save is
not. Busy/blocked/unknown/error state enters shared navigation/native-close gates.
Committed warnings do not authorize replay. Contradictory/missing receipts remain
unknown after acknowledgement; only explicit owned Load recovers authority,
without rewriting the unknown ledger.

Focused shared preferences/SIVI Go race9.649s,28 summary frontend tests and the
complete461-test frontend suite/check0/0 pass. Isolated enabled/default
frontend/Wails builds and13 actual Wails cases/three closed owners verify
four planned YAML writes, unchanged no-op, independent collision, valid draft
remount/Undo, held real committed write with lost acknowledgement, explicit
authority reload, retained unsupported scope correction and responsive labels.
All16 fixture hashes restore. The default owner closed normally before a
concurrent accidental frontend build tripped protected-asset verification.
Its cases were not replayed: all17 protected assets were restored exactly from
the accepted archive after preserving the overwritten build, and fresh fixture/
owner/protected checks sealed its cleanup. The incident does not establish a
production promotion or canonical-data change.
Coherent full race integration passes791.377s/all packages against626 unchanged
source identities. The combined immutable successor is
`archives\forms-reports-shared-checkpoint\evidence-manifest.json`.

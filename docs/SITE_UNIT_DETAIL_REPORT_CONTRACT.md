# Site unit detail / summary environment source boundary

## Status

Source-mapped only; no summary calculation, public service, native preview,
Excel automation or export is accepted. This is distinct from the accepted
per-plot Long Environment preview. The client's consolidated per-plot versus
unit-summary presentation remains open in [client scope](CLIENT_SCOPE.md).
Shared report options must not turn on this unavailable workflow.

The local reference root is
`C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64_forAI`. Static exports were read
without opening Access or modifying source/data:

| Source | SHA256 |
| --- | --- |
| `Forms/USysSuDetailReport.txt` | `7b98c8c6a395b2352b842f52c435febe0afb5198b819e0c646d559dda8ba05b8` |
| `Modules/V7mdlReportsEnv.txt` | `8917bcd43c6e8001585795aa525562b77cb4ecd9d74b65350a01796ee69cb8a9` |
| `Modules/V7mdlReportsSiteUnitDetail.txt` | `af23d63350c845dc40a151135a147a312a6504c7de8c9d948ce7e1904b6bee98` |
| `Modules/V7mdlSetCurrent.txt` | `4f27c6021b480b4ddc56607b0a518801f7afc83b7f28d106bfae6790f3607d66` |
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

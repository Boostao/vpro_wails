# Standalone Lifeform Summary preview

## Status and scope

The read-only normal-SU planner/service and desktop route are implemented behind
independent default-off `VPRO_LIFEFORM_SUMMARY` and `VITE_LIFEFORM_SUMMARY` gates.
Focused race12.661s, complete frontend tests (554), type-check (0 errors/warnings),
interface-mode bindings and isolated production builds pass. Eleven native cases/
four normally closed owners pass. Full coherent race passes fresh
root1122.770s/all packages against644 unchanged source identities; immutable
successor `archives/lifeform-summary-desktop-checkpoint/evidence-manifest.json`.

This is the standalone `Modules/V7mdlReportsLifeform.txt` workflow, not the
SiteUnitDetailReport `SuDetailStep1.MyCover` helper. Its source wrapper uses
argument 1 for normal SU and 0 for dynamic hierarchy preparation. The desktop
does not invoke that wrapper or build dynamic tables: it explicitly selects the
owned ordinary SU. Neither the generic Summary Vegetation menu item nor hierarchy
breaks, saved report options or publication becomes available through this slice.
The separate [attribute-count successor](SPECIES_ATTRIBUTE_SUMMARY_CONTRACT.md)
adds six independently gated raw-vegetation query families in the same host;
its native acceptance does not reinterpret the Lifeform calculation below.

Canonical Access and imported databases remain unchanged. Access is not opened.
The implementation uses existing owned SQLite readers, not a second Access
importer, registry mutation, source temporary tables or Excel automation.

## Source preparation and physical weights

Local read-only source root:
`C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64_forAI`.

| Export | SHA256 |
| --- | --- |
| `Modules/V7mdlReportsLifeform.txt` | `d4ac41b542f80de07b6dd2a8138d702fedcf10431fed41cffc3526e0794025c0` |
| `Tables_Def/USysVegTable_CreateSQL.txt` | `fbe375ede145943e6b61c64e4382843813b859559023830dfb22ab3e85265954` |
| `Tables_Def/USysV2VegLF_CreateSQL.txt` | `388060a6295b8a221542259e09ddbe95005991c5faf6c44810de09f7111b4a61` |
| `Tables_Def/LifeformCodes_CreateSQL.txt` | `c28bfc65203e07855eacf84cfec76660179f40db5c42b05c737b89fc1670d9e4` |

Module lines13-16/27-30 establish wrapper selection. Lines320-324 plus copied
vegetation schema lines5-30 establish the first SINGLE assignment; lines287-291/
304 and intermediate schema line6 establish the second. Lines503-507 contain
the per-unit physical INNER JOIN before grouped cover COUNT/SUM. Lines509-513
left-join the catalogue; lines515-524 use the same unit join for species counts.
Source output lines645-653 display numeric Lifeform/Presence/Mean Cover:
catalogue label, definition and short name are supplemental desktop metadata.

`CreateSmallVeg` joins the selected SU to vegetation and groups by PlotNumber
and Species. It holds MAX for Cover1 through Cover10 and TotalA/TotalB rather
than expanding physical LayerCode rows. The source lifeform insert joins
USysAllSpecs, not the different USysAllSpecies union or user catalogue. It loops
lifeforms 0 through 12, excludes CodeType S and sums ten Nz-wrapped cover values.
Both source SINGLE assignments are represented; inserted sums round to finite
float32 before report aggregation. TotalA/TotalB are not substitutes for that
sum.

Species codes are not assumed unique. Global inserted entries retain insertion
SU row, species-definition row and ProjectID evidence. Later per-unit reads
rejoin by PlotNumber; they do not filter global rows by their insertion ProjectID.
Duplicate and cross-unit memberships must therefore not be flattened.

The denominator is COUNT(PlotNumber) grouped by SiteUnit, counting physical
non-NULL membership values. An orphan PlotNumber still contributes to this
denominator; NULL PlotNumber does not. NULL SiteUnit and empty text remain
different identities. Unique species and occurrence counts derive from the
per-unit entry rejoin without a second species-catalogue join.

The final query left-joins LifeformCodes, excludes only lifeform 13 and retains
catalogue rows without observations. Presence is a fractional grouped-plot count
divided by the physical denominator. Mean cover is fractional summed cover divided
by the denominator and 100. For a positive denominator and no entries, presence
is zero but mean remains NULL. Zero denominators are displayed as NULL ratios,
an explicit safe desktop adaptation rather than an Access division-by-zero claim.

## Owned API and desktop lifecycle

`LifeformSummaryService.Preview(ctx, contextID)` borrows the ContextService owned
read lease. It returns an envelope with `contextId`, `projectPath`, `suPath` and
`report`. The report contains `project`, `su`, `querySource`, `ordering`,
`memberships`, `entries`, `catalogue` and `units`.

Units retain typed nullable `code`, `nPlots`, physical `suRowIds`, `uniqueSpecies`,
`occurrences` and `rows`. Each row retains `catalogueRowId`, `lifeform`,
`plotGroups`, `coverCount`, nullable `presence` and nullable `meanCover`.
The frontend validates transport and owned identity before publishing. Report
reads are explicitly cancellable; busy state blocks host navigation and native
close. Disposed/stale requests cannot publish into a new context.

The Reports / Vegetation / Lifeform Summary route is separately gated. It does
not change a preference, filter by the profile browser, save an editor draft,
write report tables or export a workbook.

## Disclosed adaptations and remaining gaps

- Literal binary identities do not establish Access Unicode collation parity.
- ASCII S/s exclusion is explicit; NULL CodeType remains excluded by SQL
  semantics. No Unicode recasing or inferred catalogue creation occurs.
- Catalogue/unit presentation follows deterministic physical evidence; source
  SQL without ORDER BY is not a promise of Access iteration order.
- Exact aggregation makes the desktop arithmetic deterministic. Source SINGLE
  boundaries remain represented rather than silently promoted away.
- Percent formatting is display-only; emitted ratios retain their numeric
  precision and distinguish NULL from a very small nonzero value.
- Hierarchy/field-derived units, other species modes, source workbook layout and
  publication remain unavailable. Attribute counts have a separate contract,
  gate and acceptance scope; they do not inherit this cover planner.
- The separate SiteUnitDetailReport MyCover path is unresolved and must not reuse
  this planner as parity proof.

### Not the Long Vegetation lifeform conversion

Static continuation traces a third, different path:
`V7mdlReportsLongVeg.txt` lines158-174 selects GroupAndOrder13/23 and calls
`V7mdlTableConversions.BuildLifeFormTable`. Its active lines145-174 end with
`GoTo MyExit`; the lengthy later alternative inserts are unreachable and are
not requirements for the active branch.

That conversion LEFT JOINs **USysAllSpecies**, the five-column UNION of
USysAllSpecs and USysUserSpp with CodeType non-S predicates, rather than this
standalone report's inner join to USysAllSpecs. Existing desktop
`vegetationSpeciesUnion` is precedent for that exact source query. UNION equality/
provenance and NULL CodeType exclusion remain separate from later left-join
missing-species behavior.

The active conversion includes Cover5a/5b/5c but **omits Cover10**, unlike this
standalone ten-cover sum. It applies `SetTo99` before the grouped SUM; utility
lines433-441 show that the cap is **99.9**, not the function name's implied99.
The parameter is SINGLE. It groups PlotNumber, formatted Lifeform insertion
code and Species before writing the SINGLE intermediate. Source
`Null2Question` (utility lines451-459) tests empty text, not `IsNull`.

Long Vegetation's later report query joins the inserted data to physical SU,
then LEFT JOINs USysAllSpecs **again** and LayerCode. It groups and displays the
later USysAllSpecs.Lifeform, not necessarily the insertion code. Constant lists
match lifeform/species rather than layer/species. These distinct catalogue and
cover boundaries must be implemented/tested explicitly before enabling that
mode; the accepted standalone planner is not a shortcut or parity proof.

## Disposable desktop acceptance

Fixture `evidence/private/native-lifeform-summary` was copied from the normally
closed reference predecessor without database seed writes. Independent physical
SQLite reads match every membership, inserted entry and catalogue row and each
unit's join arithmetic. The retained Summary fixture has denominator3/unique41/
occurrences106 and empty unit1/38/76; its missing/NULL plots prove zero versus
NULL ratios. Pinned ordinary Sample tests also prove BWBSdk 1 /08 (3/42/80) and
BWBSmw 2 /05 (9/106/288).

Four closed owners cover eleven cases: arithmetic,1400/600px actual visibility/
scoped table and row headers, held real SDK read with navigation/native-close
denial/cancel/remount/retry, independent default/backend gates and existing
FS882 valid/invalid/discard navigation drafts. All16 fixture hashes and protected
source/default identities are unchanged. A preliminary candidate was closed and
its36-file build archive retained before the descriptive candidate was rebuilt.
No business operation is replayed and no Access/Excel oracle is reopened.

See [migration plan](../MIGRATION_PLAN.md) and
[Site Unit Detail source boundary](SITE_UNIT_DETAIL_REPORT_CONTRACT.md).

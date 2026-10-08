# Standalone species attribute summaries

## Status

Six read-only attribute count families are implemented in the Lifeform Summary
host behind independent default-off `VPRO_SPECIES_ATTRIBUTE_SUMMARY` and
`VITE_SPECIES_ATTRIBUTE_SUMMARY`. The existing Lifeform route remains separately
gated. Attribute reads do not enable hierarchy, generic Summary Vegetation,
Long-report selected attributes, preferences editing or workbook publication.

Coupled race18.691s,564 complete frontend tests (126+438), check0 errors/warnings,
interface-mode bindings (261 packages/25 services/207 methods/190 models) and
isolated enabled/default frontend/embedded production Wails builds pass.
Nine native cases/three normally closed owners pass. All16 owned/protected
identities restore. Full coherent race passes fresh root1166.610s/all packages
against660 unchanged source identities. Immutable successor:
`archives/species-attribute-summary-desktop-checkpoint/evidence-manifest.json`.

## Original source

Canonical exports were read without opening Access or changing its data:
`C:\Users\BrunoTremblay\Work\VPRO_ACCESS`.

- VPro64_forAI/Forms/USysSppAttributeReportOptions: SU Table versus Hierarchy
  Breakpoints, Include Details controls and captions SRank Detail, Wetland
  Indicator, Weed Status, Red Blue List, Est. ASMR and Climate. Form_Load reads
  retained settings; each AfterUpdate writes its corresponding clsRepOpt flag.
  Create Report calls BuildLifeformSummary with the selected unit mode.
- VPro64_forAI/Modules/V7mdlReportsLifeform: BuildQueries calls all six count
  query families. Lines332-499 and689-766 establish their raw Veg/SU/attribute
  joins, non-S CodeType and non-NULL attribute predicates, fixed pivots,
  grouped-plot counts and grouped-count sums. Lines528-629 display all six
  totals regardless of the optional detail flags.
- VLists/Tables_Def/USysSppAttributes_CreateSQL: Code is TEXT8, Codetype TEXT1,
  SRank TEXT50 and the five other selected attributes TEXT1.

These queries join the selected normal SU directly to physical project Veg and
then USysSppAttributes. They do **not** read TempReportVegLF, a MAX reduction,
EntryDatVegLF, USysAllSpecs or LifeformCodes. A repeated physical vegetation
observation remains repeated even when its covers are NULL or historical text.
Cover arithmetic and the separate Lifeform report's SINGLE assignments are
irrelevant to these count queries.

## Shared six-family count contract

One typed kernel applies the same physical join/count behavior to these source
fields, preserving source family order and fixed pivot category order:

| Field | Fixed detail categories |
| --- | --- |
| SRank | S1, S2, S2S3, S3, S3S4, S4, S4S5, S5, SE1, SE1SE2, SE2, SE3, SE3SE4, SE4, SE5, SEH, SEX, SH, SU, SX |
| Wetland_Ind | 1, 2, 3, 4 |
| WeedStatus | I, P, R |
| RedBlueList | R, B |
| Est_ASMR | 0, 1, 2, 3, 4, 5, 6 |
| Climate | 0, 1, 2, 3, 4, 5, 6 |

Duplicate SU memberships, raw Veg rows and attribute definitions multiply count
weights. Plot occurrences count distinct non-NULL PlotNumber values after the
same attribute-specific joins/predicates. Unit nPlots separately counts physical
non-NULL SU PlotNumber cells, including orphans.

NULL Code/Codetype cannot match. Explicit ASCII S/s exclusion is shared with the
accepted Lifeform report; empty and other literal non-NULL CodeType values are
not silently classified as synonyms. Attribute NULL is excluded separately for
each family. Empty, case-distinct, whitespace and non-pivot attribute values
remain literal: they contribute to totals/plot counts but are not invented
fixed-pivot columns. An absent grouped SUM remains NULL; distinct plot count
is zero. A missing pivot observation remains NULL, not zero.

Report provenance retains every physical membership and every eligible
SU/Veg/attribute row triple with six original text/NULL cells. No source row,
species definition or unit is deduplicated. NULL/empty SiteUnit remain distinct;
NULL unit memberships are retained without inventing an equality predicate that
would match the source's per-unit string filter.

## Ownership and presentation

SpeciesAttributeSummaryService.Preview borrows the existing cancellable owned
context/snapshot. The selected project, internal/external SU and configured
VLists attachment must still be the original owned files. Missing physical
tables/columns, malformed storage, stale ownership, cancellation and failed
read completion/cleanup refuse the entire result explicitly.

The Lifeform host uses one request generation, cancel tracker and busy boundary
for both report types. Neither report can overlap the other or clear native-close
and navigation protection prematurely. Cancel/remount suppresses stale reads.
The frontend validates source definitions, exact metadata, physical triples,
membership partitions, counts, distinct plots and pivots before publishing.

All six detail families are displayed explicitly in responsive, labelled,
horizontally scrollable tables. This is a read-only presentation adaptation,
not application of retained Include Details preferences or an AfterUpdate write.
Counts remain integer/NULL. Literal binary joins/pivots and deterministic
physical unit order do not establish Access locale/collation/iteration parity.
No numeric/text coercion, case completion, trimming or catalogue fallback occurs.

## Disposable desktop evidence

Fixture `evidence/private/native-species-attribute-summary` derives from the
sealed, normally closed supplementary Code successor. It has no database seed
writes; only owned configuration paths are rebound before its baseline.

Independent physical SQLite queries verify all72 raw match triples across five
units, each family's total/distinct plot count and every fixed pivot cell.
Five enabled cases cover these calculations,1400/600px actual visibility and
labels, unchanged Lifeform reads and actual held-read navigation/native-close
denial, cancellation/remount/stale suppression/retry. Two build-off cases retain
the Lifeform route while denying the attribute button and accepting the
independently authorized backend. Two backend-off cases show an explicit alert
and reject the actual binding. All three owners close normally and all16
fixture/protected hashes restore. No Access/Excel oracle or source mutation occurs.

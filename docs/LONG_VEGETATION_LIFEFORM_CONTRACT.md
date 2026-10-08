# Long Vegetation Lifeform grouping

## Status

Source grouping3 (GroupAndOrder13/23) is implemented in the existing owned Long
Vegetation preview behind independent default-off runtime/build gates
`VPRO_LONG_VEGETATION_LIFEFORM` / `VITE_LONG_VEGETATION_LIFEFORM`. The ordinary
Long Vegetation presentation gate remains required. Saved LVGroupBy3 is not
rewritten when either gate denies it; no Layer fallback is allowed.

Focused coupled Go race46.207s,556 complete frontend tests/check0 errors/warnings
and exclusively isolated frontend/production builds pass. Eleven native cases/
three normally closed owners pass, with all16 fixture and protected identities
unchanged. Full race passes fresh root1038.681s/all packages against650 unchanged
source identities. Immutable successor:
`archives/long-lifeform-desktop-checkpoint/evidence-manifest.json`.
Public API/DTO/bindings shapes are unchanged: the existing row `layer` cell holds
the typed nullable INTEGER Lifeform when `settings.grouping` is `lifeform`.

This is not the separate [standalone Lifeform Summary](LIFEFORM_SUMMARY_CONTRACT.md)
and does not resolve SiteUnitDetailReport MyCover, Strata, taxon lumping, additional
summaries, saved report preferences, workbook layout or publication.

## Static source boundary

Canonical source was read without opening Access or writing data:
`<private-Access-root>\VPro64_forAI`.

| Export | SHA256 |
| --- | --- |
| `Modules/V7mdlReportsLongVeg.txt` | `db9f146435895d23c6191cc97027dd6d330a488d6db1c61eb4abd811da472bcd` |
| `Modules/V7mdlTableConversions.txt` | `af18a926d99fe650c7f62b51ed24fecc5dd261b4f83ac9ca1e432118610fe566` |
| `Modules/V7mdlUtility.txt` | `aede7a19144313c75b5fe7a174f81f11648dfdd0492c970c174ab2967b50a1bf` |
| `Queries/USysAllSpecies.txt` | `965069c64db5ad808dcf3ccd5fca2ef1313d638225666df77722ddf5671dc1a5` |
| `Tables_Def/USysV2Veg_CreateSQL.txt` | `8124ca616a6b95833f13f1da0e670b941bbc89816639bdee61f39a4c28c9860f` |

ReportsLongVeg159-174 selects Lifeform ordering/conversion; TableConversions145-174
is the active BuildLifeFormTable branch. Its GoTo MyExit makes the later lengthy
alternative inserts unreachable. Do not migrate those alternatives as active
requirements. ReportsLongVeg257/269/284 establishes the later separate joins
and Lifeform/species constant-list match.

## Preparation and reference identities

The existing selected-SU MAX-per-plot/species reduction supplies original
USysVegTable fields. Conversion uses Cover1 through Cover9 plus Cover5a/5b/5c:
**Cover10 and TotalA/TotalB are not part of this sum**. This differs from the
standalone summary's ten ordinary cover columns.

The conversion LEFT JOINs USysAllSpecies, the five-column UNION:
Code, ScientificName, Lifeform, EnglishName, Codetype. Master data comes from
VLists.USysAllSpecs; personal data comes from VUser.USysUserSpp. The physical
personal column is `LifeForm`, explicitly mapped to union `Lifeform`. Source
columns/cells are not recased. Ambiguous or other guessed aliases reject.

SQL UNION removes identical complete five-cell definitions, not duplicate codes.
NULL CodeType is excluded by its predicates; literal ASCII S/s exclusion is
explicit. NULL/empty names and definitions differing in any retained column stay
distinct. Private reference preparation retains source-table/row origins for
merged definitions. The public report retains existing multiplicity diagnostics,
not a new complete catalogue-provenance API.

The source applies SetTo99 before grouped SUM. Utility433-441 shows its actual
cap is99.9, not99, and its parameter is SINGLE. The first MAX-field assignment,
SetTo99 parameter and final USysV2Veg.Cover assignment each explicitly represent
finite float32 rounding. Negative and zero covers are not silently removed or
clamped upward. Grouped insertion retains PlotNumber, formatted lifeform insertion
code and Species. Source Layer TEXT3 overflow is refused, not truncated.
Utility451-459 Null2Question tests empty text rather than IsNull.

Historical malformed numeric text and invalid physical reference storage fail
explicitly. Typed refusal, exact deterministic addition and literal binary
identities are disclosed adaptations, not a new Access Nz/Val or Unicode
collation oracle. Closed calibration attempts are not reopened.

## Later report joins and statistics

Inserted rows rejoin physical selected-SU memberships, then LEFT JOIN
USysAllSpecs **again** and LayerCode using their insertion code. Display/group
Lifeform comes from the later master definition, not automatically from the
earlier master/personal UNION. Missing master definitions retain NULL group and
raw species; duplicate definitions and matching LayerCode rows multiply joins.

The proven layer kernel is reused through a private bridge; final row grouping
is restored to exact signed16 INTEGER/NULL. Species and name cells remain
text/NULL. Numeric Lifeform order precedes species or descending presence in
13/23, while constant lists retain Lifeform/species order regardless of the
nonconstant order option.

Existing average choices, named/unassigned physical denominators, plot pivots,
quality qualification, strict thresholds, names and NULL constant-list semantics
remain unchanged. EnglishName participates in source grouping but not constant
list matching. NULL Lifeform/species keys do not become invented matches.
Source row weights and selected-SU scope are not replaced by profile filtering.

Owned reads acquire the existing cancellable context/snapshot lease. Personal
metadata is read only for Lifeform mode and within the same snapshot. Quality
source indices remain stable when the personal table is appended. Errors, stale
contexts, cancellation and rollback failures publish no partial report. Rejected
personal-file contexts preserve the existing owner.

## Disposable desktop evidence

Fixture `evidence/private/native-long-vegetation-lifeform` derives from the sealed,
normally closed standalone successor. It has no database seed writes; driver
configuration selects grouping3 explicitly before the baseline is recorded.
Only the driver changes/restores order, average and constant-list options.

Independent physical SQLite/Python checks verify51 held rows,10,427 complete
UNION definitions and51 inserted rows, including SINGLE boundaries/cap, separate
master/LayerCode joins, pivots, statistics and numeric ordering. The retained
fixture has unassigned denominator1/8 rows, empty unit1/42 rows and Summary3/47
rows. The same independent checks verify source13 ordering, characteristic
average and constant-list NULL/name matching, not just enabled UI appearance.

Eleven cases/three closed owners additionally cover1400/600px actual visibility,
Lifeform/settings/column/row labels, held SDK read with navigation/native-close
denial/cancel/remount/retry, build-off with independently authorized runtime and
backend-off with enabled build. All16 file hashes and protected source/default
identities remain unchanged. No Access/Excel, database/audit/preference mutation,
temporary source table or export is performed by the application.

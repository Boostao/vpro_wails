# Plot location review

## Source evidence and scope

Read from the original SaveAsText `V7mdlReportLocation`, `V7mdlGoogleEarth`,
`frmGoogleEarth` and `V7mdlShortCutToolBarCmds` exports, and the `SetCurrentSu`
query construction. Ribbon `sb12btn06` invokes ReportLocation; `sb12btn07`
opens Google Earth options. These are distinct workflows.

ReportLocation selects PlotNumber, Zone, SubZone, SiteSeries,
LocationAccuracy, negated Longitude, Latitude and Elevation from USysEnv,
requiring both coordinates non-NULL. Worksheet presentation orders latitude
before longitude. Labels are Plot Number, Zone, Subzone, Site Series, Accuracy,
Latitude, Longitude and Elevation. Excel apostrophe prefixes preserve text
formatting; they are not literal plot/site-series data.

USysEnv originates from SetCurrentSu's DISTINCTROW Env/Admin join, with
selected-SU membership where applicable. The desktop reader uses unique
physical Env/Admin text links and retains every matching physical SU membership
ID without multiplying report rows. An external SU database is allowed.
Unclassified NULL/empty SiteUnit values do not exclude coordinates. No Admin
match excludes Env. SU None is project-wide eligible Env/Admin, not all Env.
Current plot, profile, quality and location-range filters are not introduced.
Ambiguous physical Env/Admin links fail explicitly. Desktop exact-text joins
are an adaptation, not proof of full Access Unicode collation.

## Implemented read-only boundary

`ContextService.GetPlotLocationReview` is independently default-off under
`VPRO_PLOT_LOCATION_REVIEW`; navigation also requires
`VITE_PLOT_LOCATION_REVIEW`. The owned reader uses the existing context lease,
cancellation-aware snapshot coordinator, read-only transaction and before/after
owned-file checks. Foreign/stale contexts, cancellation, ownership failure,
malformed snapshots and cleanup errors return no partial result.

The eight-field report retains exact signed64 physical Env/Admin IDs,
membership IDs, tagged SQLite values and the original stored longitude.
Longitude is numeric negation, never absolute value, hemisphere inference,
clamping or UTM conversion. Negative originals produce positive output.
Real signed zero is preserved. Minimum signed64 longitude cannot be negated
exactly and is rejected, not rounded. Nonnumeric included coordinates fail
explicitly; historical numeric out-of-range values remain readable.

Frontend validation detaches input, checks owned scope, exact field
order/labels, complete tagged cells, physical identities/membership provenance
and exact derived longitude. Large validation yields to the event loop.
Fifty-row pages bound rendered DOM, not the report's eligible dataset.
Generation checks suppress cancelled/remounted results; reads reuse existing
draft-navigation and busy native-close barriers. No database/audit/configuration
or output file write is permitted.

## Not implemented or proven

The original Google Earth multi-plot builder reads direct project Env,
optionally joins SU, and differs from ReportLocation's Env/Admin scope.
That INNER JOIN multiplies placemarks for duplicate SU membership; ReportLocation
DISTINCTROW behavior must not be reused as an unqualified Google Earth rule.
Single-plot building bypasses SU. Its literal `"-" & Longitude` concatenation
can produce double-minus output and is not inherited as valid numeric behavior.
Dynamic description choices, form preferences, XML/CDATA escaping, overwrite
and Build versus Build-and-Launch require a separate publication contract.
Hardcoded output names and executable launching are not automatically retained.

The original form's `PlaceName_AfterUpdate` and `optDescField_AfterUpdate` write
report preferences. Description choices are an Access Field List whose exported
RowSource is Sample_Env; this is not authorization to hardcode Sample in an owned
current-project reader. Form_Load displays report preferences, the configured
output directory and executable. Output-directory/executable KeyPress handlers
show a not-editable warning; that alone is not proof of a locked control.
Build File invokes `PlotPlotsInGE False`; Build File and Open invokes True.
The Build File click then reports success unconditionally even if the module's
error path already returned. That success-shaped error behavior is not retained.

The builder uses KML2.1, a remote HTTP red-dot icon, a zero altitude and a trailing
period in description CDATA. It concatenates raw names/descriptions and uses
unescaped configured shell arguments. A successor must distinguish raw source
values, output escaping/Unicode validity, nullable descriptions, coordinate
validity, remote-resource/privacy policy, explicit destination collisions and
publication success from viewer-launch success. These remain an unenabled
contract, not silently selected desktop requirements or an XML/launch promise.

File publication, Excel/KML production, Google Earth launching and map readiness
remain unavailable. Focused backend race2.266s, all381 frontend tests,
Svelte check0/0 and isolated enabled/default production Wails builds pass.
Independent review found no significant defects. Twelve native cases on a fresh
location-only fixture verify independent gates, all500 eligible rows against
independent SQLite/source-order projections, actual1400/600px visibility and
column/cell accessible names, pagination, cancellation/remount/retry,
busy navigation/WM_CLOSE and FS882 valid/invalid draft barriers without Save.
All16 owned hashes restore after each of four owners closes.

Native acceptance is project-wide SU None. External selected-SU and duplicate
membership provenance are covered by fast owned-reader/transport tests, not a
new native SU claim. Initial fixture setup rolled back an absent assumed source
row; copied databases were independently hash-verified before recovery. One
same-owner read-only harness retry corrected the expected busy-guard text,
preserving first observations/visuals and restoring hooks. No business write,
successful creation or Save was replayed. Fresh full Go race integration passes
root775.988s/all packages with the established explicit long-suite timeout.
The initial default10-minute timeout is preserved as failed runner evidence,
not acceptance. The [122-file exact successor](../archives/plot-location-owned-review-checkpoint/evidence-manifest.json)
verifies the immutable18-file template and123-file CSV predecessors and records
354 Go integration source hashes. None of this is native Access/Excel
publication parity.

## Private direct-Env Google Earth preparation

The pure `planGoogleEarthLocations` candidate implements only the multi-plot
data projection from `GetPlotLocations`, not `GetSinglePlot`, KML serialization,
an owned reader or a public workflow. With SU None it reads direct project Env,
including eligible plots without Admin; with a selected SU it emits one entry
per physical Env/membership pair. Duplicate memberships and duplicate logical
Env plot names therefore retain source INNER JOIN fanout instead of inheriting
ReportLocation's DISTINCTROW reduction. Provided physical row order is retained;
the original query has no explicit ORDER BY.

Description selection is an exact existing Env column, never executable SQL,
an inferred Sample binding or case repair. Raw tagged descriptions and plot
names preserve NULL/empty/BLOB/integer/text distinctions without XML/CDATA
escaping, a trailing period or locale/string coercion. Selected-SU membership
uses the established exact-text desktop adaptation. Longitude reuses the
proven numeric negation guard, avoiding double-minus concatenation, preserving
signed zero and rejecting signed64 minimum overflow. Historical out-of-range
numeric coordinates remain raw; they are not a KML-validity claim.

Caller inputs are detached before fallible context checks, each emitted pair
owns its values, and cancellation returns no partial projection. Focused
location/projection race1.176s and package vet pass, including independent
SQLite join-fanout comparison, raw nullable names/descriptions, alias mutation,
malformed storage and cancellation. Independent reviewer reports no significant
issues; detailed scope/runtime disposition was not supplied, so independent
runtime/source-comparison proof is not inferred. Fresh full integration passes
root937.495s/all packages against370 captured Go source hashes. Remote-icon/privacy, document
naming, XML Unicode rules, destination
publication, preferences, single-plot ambiguity and viewer launching remain
separate unavailable boundaries.

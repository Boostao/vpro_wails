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
Single-plot building bypasses SU. Its literal `"-" & Longitude` concatenation
can produce double-minus output and is not inherited as valid numeric behavior.
Dynamic description choices, form preferences, XML/CDATA escaping, overwrite
and Build versus Build-and-Launch require a separate publication contract.
Hardcoded output names and executable launching are not automatically retained.

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
not acceptance. Exact successor sealing is in progress. None of this is native
Access/Excel publication parity.

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
The [19-file exact pure projection successor](../archives/google-earth-projection-checkpoint/evidence-manifest.json)
verifies all76 timestamp predecessor files/protected hashes and records370
unchanged integration source hashes. It is not an owned/native KML acceptance.

## Private owned Google Earth read boundary

The unexported `readGoogleEarthLocations` holds one current-context
operation lease and the existing owned read snapshot. It reads the current
project's literal physical Env and, when selected, its physical SU table,
including an owned external SU path. Env rows, memberships and the chosen raw
description field share one snapshot. No Admin data is required; no caller SQL,
Sample binding or inferred field selection is used.

The physical-table reader is shared with the existing ReportLocation reader
without changing its error or Env/Admin scope. Ownership/stale-context,
malformed schema/values, cancellation and transaction-cleanup failures return
no partial result. Reads leave project/support/SU/config bytes and selection
unchanged. Focused owned/pure location suites race2.297s and package vet pass:
independent direct-Env SQL with empty Admin, external SU three-pair fanout,
literal fields, detached outputs, physical view impostors and queued
cancellation/retry are covered. Independent review found no significant issues,
executed17 focused tests1.545s and owned-location race2.290s after correcting its
initial system-toolchain link failure. Fresh full integration925.947s/all
packages passes with372 pinned Go hashes unchanged throughout validation.
No public method, generated bindings, feature gate, UI, preferences,
KML serialization/publication or viewer launch is added.

## Independently gated Google Earth preparation preview

The public `GoogleEarthReviewService` is separately default-off under
`VPRO_GOOGLE_EARTH_REVIEW`; its Svelte preparation route requires
`VITE_GOOGLE_EARTH_REVIEW`. Original file/launch menu items remain disabled.
`GetGoogleEarthDescriptionFields` observes physical current Env columns in
literal order under an owned snapshot, retaining declared types without
hardcoded Sample fields. `GetGoogleEarthReview` takes string JSON with the
existing raw-Unicode/lifecycle decoder guard and an explicit literal field,
offset and limit1..500. It exposes direct Env rows with selected SU fanout,
exact physical decimal IDs and raw tagged description/stored-longitude cells.
Output fields are Plot Number, Longitude, Latitude and the selected literal
description field; selecting a repeated source field does not remove it.
NULL/empty/BLOB descriptions remain distinct. There is no saved field/title
preference, XML/CDATA conversion, new coordinate range rule or map-ready claim.
Frontend checks exact scope/field labels/page shape, numeric longitude
negation, distinct physical pairs and consistent raw Env values across
memberships (including signed zero). It detaches before yielding and uses
existing cancellable read requests, keyed context remounts and draft/close
barriers. Routine guidance stays below fields; errors remain above them.

Coupled race4.622s/vet, independent reviewer13tests/race3.086s and11 frontend
tests, primary387 frontend tests/check0/0 and both production builds pass.
Four actual disposable Wails owners prove16 cases, with independent schema/
SQL/RPC assertions including an Admin orphan, raw NULL/empty/BLOB descriptions,
real backend pagination,1400/600 actual visibility/accessibility, remount,
separate backend denial/default UI disabled and6 valid/invalid draft transitions.
Every owner closes normally and restores16 DB/config hashes; original
executable/default assets/canonical data remain unchanged. A transient absent
row in the proof's pagination wait was corrected without application changes;
the first observation is retained and compared on read-only retry.
Queued/cancelled backend requests are fast-tested; native coverage proves
remount/draft lifecycle, not every in-flight cancellation schedule.
Fresh full integration899.304s/all packages passes with374 Go hashes unchanged
throughout validation. The
[147-file exact successor](../archives/google-earth-owned-review-checkpoint/evidence-manifest.json)
verifies81 reader-column predecessor files, preserves closed native builds and
captures source/form evidence. This completes the bounded default-off preview,
not Google Earth export/launch or overall replacement acceptance.

## Private finalized-text KML byte preparation

`prepareGoogleEarthKML` accepts an explicit title and ordered detached
`googleEarthKMLPoint` values: finalized name/description text and already
east-positive numeric longitude/latitude. It does not read raw database cells,
apply preferences, negate longitude again or silently select text defaults.
Empty title/name/description and duplicate names remain explicit inputs.
Zero points produces an empty document, not an error-shaped successful export.

The original `BuildKmlFile`/`GetPlotLocations` supply the KML2.1 namespace,
zero altitude and appended description period. These are retained. The original
remote HTTP red-dot style is omitted deliberately, not claimed as presentation
parity. Description text is escaped for HTML before XML encoding because KML
balloons can interpret HTML after XML parsing; XML escaping or CDATA alone does
not make arbitrary markup literal. Names/title use XML text encoding. Whitespace,
CRLF, Unicode, markup/entity-like strings and `]]>` remain literal after the
appropriate XML/HTML decoding. No explicit network/style/icon/link elements are
generated. This does not guarantee zero-network viewer behavior: Google Earth's
[KML tutorial](https://developers.google.com/kml/documentation/kml_tut)
describes automatic URL linkification, and no viewer has been exercised.

Malformed UTF8 or XML1.0-disallowed characters fail without repair. Final map
coordinates must be finite, longitude within[-180,180] and latitude within[-90,90].
Locale-neutral shortest round-trip numeric text retains signed zero. These
KML-only rules neither clamp nor reject historical raw preview/storage values.
The caller's points are copied before fallible context callbacks; initial,
per-point and final cancellation return no bytes, and retry is deterministic.
Encoder errors are propagated; no partial output is exposed.

Focused coupled Go race1.763s/package vet pass, including exact XML namespace/
token inventory, literal description roundtrips, duplicate/order preservation,
coordinate boundaries/nonfinite inputs, invalid text, detached output and
snapshot-before-callback/cancellation/retry. Primary inspection found no
significant issues; no independent reviewer execution/disposition is claimed.
Fresh full application race943.664s/all packages passes against376 pinned Go
hashes unchanged during validation. The
[private KML successor](../archives/google-earth-private-kml-checkpoint/evidence-manifest.json)
verifies147 predecessor evidence files/protected hashes. Parsed
XML shape is not formal KML2.1 schema validity, Google Earth rendering or native
Access parity. Raw-cell adaptation (see below), unverified non-text/Variant
coercion, decimal conversion policy,
source icon presentation, title/field preferences, destination ownership/
collisions/publication, viewer launch and single-plot behavior remain separate,
unavailable boundaries. No public facade, binding, UI or native lifecycle changes
are introduced by this private kernel.

### Source text-adapter investigation

The active multi-plot path is `PlotPlotsInGE` -> `GetPlotLocations` ->
`BuildKmlFile` -> `WriteKML`; the separate `GetPlotLocationsNew` is not invoked.
Each name/description is concatenated with existing non-NULL string literals
using VBA `&`, not `+`. Microsoft's
[ampersand operator reference](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/ampersand-operator)
specifies that a single NULL operand is a zero-length string. Consequently a
raw NULL description contributes no text and still receives the literal period;
NULL and empty descriptions can share output text without becoming identical
stored metadata. This is static source/language evidence, not a native oracle
observation. The raw preview must retain their distinct tagged values.

`Sample_Env_CreateSQL` declares PlotNumber TEXT(7) NOT NULL, PlotRepresenting
TEXT(255) and Longitude/Latitude DOUBLE. The detailed Sample_Env design records
PlotRepresenting as optional short text, Allow Zero Length No, with the original
dominant-species/site/soil description. This is source-template evidence, not a
claim that current SQLite data satisfies every historical constraint or that
the incomplete exported DEFAULT clauses supply defaults. Dynamic Field List
selection also allows numeric/date/boolean fields; VBA converts non-string
operands to string variants, but exact locale/date/boolean formatting and
malformed/BLOB behavior remain unverified. An initial adapter should therefore
explicitly support original TEXT/NULL names/descriptions, reject other storage
with a field/physical-row error rather than guess display text, and preserve
order/fanout and the already-negated numeric coordinates. Neither default preference (`VPro Plot Locations`,
`PlotRepresenting`) may be silently persisted or substituted.

### Implemented private TEXT/NULL point adapter

`planGoogleEarthKMLPoints` now implements that restricted adapter over the
private direct-Env projection, not a new database reader or ownership authority.
It snapshots row values and the four used tagged-cell pointers before fallible
context callbacks, reuses canonical cell and XML text validation, and preserves
every supplied row in order without grouping/deduplicating physical pairs.
NULL name/description becomes empty output text only; original NULL/empty
metadata is never changed. Integer/real/date/boolean/BLOB descriptions and names
remain explicitly unavailable, even if the source Field List offers them.
Malformed tagged cells and invalid text fail with point/Env/membership/field
context. Missing/nonnumeric, nonfinite or out-of-bounds coordinates return no
points; no rounded out-of-range integer, repair, clamping or partial result is
exposed. Already-negated longitude is used directly, preserving signed zero.
The serializer and adapter share one coordinate-validation helper.

Coupled race1.913s/vet and primary inspection pass. Tests cover exact integer
bounds, all signed64 extremes rejected from map preparation, NULL/empty output
with unchanged tagged storage, raw text/entity/markup roundtrips, source order/
fanout, malformed/unsupported cells, detachment before context callbacks and
initial/mid/final cancellation plus retry. The actual direct-Env projector
feeds the adapter and serializer: eligible Admin orphan/NULL descriptions and
six physical selected-SU Env/membership placemarks survive end to end. This is
fast source-projection/XML evidence, not a new native Wails/Access or viewer
claim. An added test initially landed inside another test through an ambiguous
patch anchor; compilation failure is retained and package-scope correction passes.
Fresh full integration995.902s/all packages passes against378 Go hashes unchanged
during validation. The
[TEXT/NULL KML successor](../archives/google-earth-text-kml-checkpoint/evidence-manifest.json)
verifies14 private KML predecessor files/protected hashes. No public method,
binding, UI, files, preferences, icon/network or launch action is enabled.

## Private owned KML byte preparation

`ContextService.readGoogleEarthKML` now shares
`readGoogleEarthLocationsSnapshot` with the accepted raw reader. The latter
extracts only the original physical Env/optional external SU and source projector;
it does not nest an operation lease. The new reader holds one context lease
and owned read snapshot through TEXT/NULL point planning and XML serialization,
then lets the existing coordinator verify owned files, cancellation and
transaction cleanup before exposing any bytes. Cleanup/ownership/cancellation
errors discard the result. Explicit title validation precedes data work.
Returned scope, selected field, title and placemark count belong to the same
complete source snapshot; bytes are detached and never written to a destination.

Coupled location/Earth race4.975s/package vet and primary inspection pass.
Independent direct SQLite/source-order comparisons verify every generated
name/coordinate/count without requiring Admin, including NULL description's
literal period. External-SU duplicates yield three marks; selected PlotNumber
description aliases remain literal. Caller-byte mutation cannot affect retry.
Database and preferences bytes remain unchanged. Invalid/stale contexts,
description case inference, malformed title, unowned attachments and physical
view impostors fail without output. Initial and mutex-queued cancellation
release the lease and allow retry. BLOB description and historical longitude181
reject KML output while the accepted raw reader still returns original values.
Pure tests cover later point/encoding cancellation checkpoints; this is not a
claim that every such checkpoint was triggered inside the owned wrapper.

Fresh full integration987.040s/all packages passes against380 Go hashes unchanged
during validation. The
[owned KML successor](../archives/google-earth-owned-kml-checkpoint/evidence-manifest.json)
verifies22 TEXT/NULL predecessor evidence files/protected hashes.
This adds no public
service/JSON transport, startup gate, binding, UI or native proof. It neither
saves preferences nor publishes files, audits writes, loads remote icons or
launches a viewer. Formal KML schema, viewer rendering/privacy and native Access
coercion/parity remain unverified.

## Independently gated KML text preview

`GoogleEarthKMLService.GetGoogleEarthKMLReview` is a separately authorized,
read-only text preparation facade over the accepted owned byte reader. Both
`VPRO_GOOGLE_EARTH_KML_PREVIEW=true` and
`VITE_GOOGLE_EARTH_KML_PREVIEW=true` are needed within the opted-in raw-review
panel; neither follows the raw-review feature flags. Disabled authorization
fails before data work.
The strict string-JSON request requires explicit `descriptionField` and `title`.
Raw malformed Unicode, unknown properties and exact/case-folded/escaped
duplicate keys are rejected rather than decoder-repaired or silently overwritten.
Transport carries exact context/project/SU paths, title/field, complete physical
placemark count, UTF-8 byte count and detached XML from one source snapshot.
TEXT/NULL names/descriptions retain the restricted source adapter; numeric,
date, BOOLEAN and BLOB string formatting remain explicitly unavailable.

The panel retains raw review and uses one live labelled Place Name textarea,
the literal Description field selection and a separate Prepare KML text action.
Source default `"VPro Plot Locations"` is visible initial text, never persisted.
An explicit empty title is supported; pasted line breaks are not trimmed.
XML is shown only in a labelled read-only textarea, never as live HTML.
Scope/count/byte/text checks precede actual browser DOMParser checks of the
KML2.1 namespace, hierarchy and scalar coordinates. Asynchronous validation
detaches first, yields initially and every100 marks, then checks the request
generation again. Cancellation/tab remount cannot publish a late result.
Shared busy navigation/native close and valid/invalid editor draft barriers
stay active; no completed Save is replayed.

Coupled race5.169s/package vet,391 frontend tests/check0/0 and isolated enabled/
KML-default-off production builds pass. Four new unit tests use a DOMParser
stub; actual parsing is separate disposable Wails evidence. Independent scoped
review found no significant issue and ran focused Go0.270s (not race) and
15 frontend tests0.673s. Its exact12-file ordered identity is
`8c7b5c46c591a17292ea504b57875f3cdad33e5d27819904eac9ebd433af2c64`;
the reviewer did not run native/browser/build/full integration.

Five native owners prove19 zero-write cases and close normally, each restoring
16 data/config hashes to its applicable baseline. The first correctly denies
all KML on inherited LOC0500 Latitude190, without clamping or partial bytes.
After that owner closed/restored, only disposable Sample_Env rowid553 Latitude
190->90 changed; independent comparison retained every other table/schema/row/
cell/audit. Original database SHA256
`8cfb08cf238b6384fa0ffdccd81d4ae49f3ccc9d86c4b49c070bc0ab41d5dc68`
and initial preparation/restoration remain retained.
Subsequent observations prove all500 SQL/XML marks, exact UTF-8 bytes, browser
parsing, literal markup, actual1400/600 visibility, BLOB denial/raw availability,
field correction/retry/remount and independent gates.
Initial4owners/12cases precede async scheduling; their36 build files were
archived with manifest
`6834673171df50c7fadb350fcbee5ad66abd7f9073027bd1a9352e9429967f35`
before rebuilding. A fresh final-source owner proves seven additional busy/
WM_CLOSE/cancel/remount/retry/valid-invalid-draft cases with the timer hook
restored. Fresh full race926.536s/all packages passes against382 unchanged Go
hashes. The [exact preview successor](../archives/google-earth-kml-preview-checkpoint/evidence-manifest.json)
verifies18 owned-byte predecessor files/protected hashes and exact review identity.

This capability publishes no file, changes no data/audit/preferences, loads no
remote icon and launches no viewer. XML shape validation is not formal KML2.1
schema or Google Earth rendering/privacy/native Access acceptance. Original
Build File and Build File and Open Google Earth remain disabled.

## Private shared KML artifact publication preparation

Static `V7mdlGoogleEarth.WriteKML` concatenates registry `GELocation` and
`"VProPlotLocations.kml"`, then uses `Open ... For Output`, `Print` and `Close`.
The form separates Build File (`PlotPlotsInGE False`) from Build File and Open
Google Earth (`True`); output-directory/executable controls follow setup paths,
while title/description AfterUpdate writes preferences. None of those side
effects becomes implicit permission for the desktop text preview.

`publishGoogleEarthKML` is a private finalized-title/map-point caller, not an
owned context service or public API. It detaches the point slice before any
callback and invokes the accepted strict serializer; staged bytes are compared
against deterministic regeneration from those detached values. Output uses the
declared UTF-8 encoding, not a claim of historical VBA Print/code-page/file-byte
parity. The requested destination is explicit absolute literal text, with no
inferred extension, registry directory, fixed name or replacement.

The caller reuses `publishArtifactChecked`, extracted from the existing CSV
publisher. Only operation labels/temp prefix/encoder/staged validator differ.
CSV aliases retain original bundle serialization/decoding, errors, hooks and
source-authority ordering. The identical context-cancel/32KB reader is shared
through an alias rather than duplicated. Encoded bytes are detached before
staging callbacks. Existing Win32 aliases, opened-handle parent/stage identity,
exact staged-byte reobservations, atomic no-replace link, explicit cancellation
and owned-only cleanup remain. The irreversible link sets Published before any
fallible callback; subsequent errors retain path/hash/Published and an explicit
do-not-replay message. This is observation/identity protection, not a blanket
guarantee against every external filesystem race or crash.

Unchanged CSV bundle/publication/owned-source fault suites plus three generic
tests pass race11.785s/vet. Coupled CSV/shared/KML/owned-KML race14.101s/vet passes.
New KML tests independently parse names/title/coordinates/periods/duplicates,
check literal HTML/XML handling and exact encoded hash/header, mutate caller
points before encoding, reject invalid text/ranges, exercise cancellation before
and after commit/retry, late collision/short write and committed cleanup errors.
Independent scoped review finds no significant issue and runs uncached coupled
race12.821s after correcting an Rtools GCC `_snprintf` link failure (no tests
executed in that attempt). Exact6-file ordered identity, SHA256 followed by two
spaces and absolute Windows path, UTF-8 LF including final LF:
`c69424a9a675528c6f796f10a1d3657de6ab9c76d2fd9e70a6936285567763ba`.
Fresh full race881.906s/all packages passes against386 Go hashes unchanged
throughout validation. The
[private publication successor](../archives/google-earth-private-publication-checkpoint/evidence-manifest.json)
verifies139 predecessor files,36 closed initial build files and protected hashes.

No owned source snapshot revalidation, public gate/action, preference persistence,
audit/data mutation, remote resource or viewer launch is added. Existing native
preview19-case evidence validates its accepted predecessor, not new file output.
The next owned boundary must compare independently completed fresh snapshots,
compare original tagged cells and physical Env/membership identities, not only
lossy XML bytes: NULL and empty descriptions can render identically. Close read
transactions before file commit and preserve committed results after postcommit
source drift/read-cleanup errors. Build File/launch stay disabled.

## Private owned raw-source KML publication

`readGoogleEarthKMLPublicationReview` returns original tagged projected source
cells, physical Env/membership row IDs and scope alongside the owned KML
title/field/count/bytes. The shared preparation extraction leaves the existing
byte preview's serialization and owned-read lifecycle unchanged. The new
publisher clones every tagged-value pointer, row slice and byte buffer before
callbacks or lease acquisition. Caller mutation cannot alter authority.

`publishOwnedGoogleEarthKML` holds one context operation lease and owner mutex,
then compares the full preparation against an independently completed fresh
read. Another fresh transaction validates before irreversible linking, with a
third after publication. This is exact projected raw-source/physical provenance
comparison, not merely XML equality or a claim of complete database-schema
equality. NULL and empty descriptions, or moved duplicate-SU physical row IDs,
can leave KML unchanged and still reject an old review.
The owner mutex does not exclude already-borrowed or external writers; these
observations do not promise source stability at every instant around file commit.

`withPublicationReadSnapshot` extracts the CSV boundary's equivalent profile/
pinned-read/commit/cancel/rollback cleanup, including its post-read-commit
cancellation guard. Original CSV fault tests remain byte-identical. Every
source observation is fully closed before file commit; publication never runs
inside a helper that zeroes results on read cleanup failure.
Precommit faults expose no published file and permit a fresh retry.
After commit, source drift, cancellation or rollback-cleanup failure retains
Published/path/hash and explicit do-not-replay feedback rather than pretending
nothing happened. The private checked KML adapter reuses the same no-replace
core/source-authority ordering.

Coupled shared/CSV/KML race15.428s/vet and independent review pass. The reviewer
finds no significant issue, verifies original CSV tests, runs uncached coupled
race15.344s/vet and records exact six-file identity
`6055ad402c8d587a57ccf044fe572b52655fa42491225915fc7f446a3f00addf`.
Seven new tests cover independent SQL/XML and unchanged databases/preferences,
deep detachment, scope/raw/provenance/bytes tampering, raw NULL-to-empty before/
prelink/postlink with independently proven identical XML, cleanup/cancel/retry
and pinned-coordinator reuse, operation-lease queued cancellation, and external
SU3-mark fanout plus same-XML membership-row-ID drift.
Membership-only drift is directly injected before publication, not separately
prelink/postlink; those phases use the same comparison, with NULL/empty injected
at all three. Cancellation exactly after the actual read transaction commit is
guarded but not directly injected there. Source edit/test-scope and missing
SiteUnit fixture failures are retained with corrections; no successful write
or native observation was replayed to recover them.
Fresh full race962.046s/all packages passes against389 Go hashes unchanged
throughout validation. The
[owned publication successor](../archives/google-earth-owned-publication-checkpoint/evidence-manifest.json)
verifies26 predecessor files/protected hashes and exact review identity.

This is private file preparation, not a public export service, startup gate,
binding, UI destination/approval flow, preference write, audit/data mutation,
remote icon or viewer launch. No new native Access/Wails/file/schema/viewer/
legacy-VBA-byte/privacy acceptance is claimed. Build File/launch remain disabled
until the distinct desktop contract and disposable native checks are completed.

## Independently gated desktop KML file export

`GoogleEarthKMLExportService` has a separate immutable default-off backend gate,
`VPRO_GOOGLE_EARTH_KML_EXPORT=true`; its panel controls require the separate
`VITE_GOOGLE_EARTH_KML_EXPORT=true` inside the opted-in raw-review panel. The
accepted text-only preview remains independent. Disabled/nil/cancelled/unavailable
authorization is rejected before data/file work. Exact review/publication JSON
property sets reuse strict raw Unicode/property validation, including duplicate
and escaped/case-folded property rejection.

Review returns the complete owned text review, exact UTF-8 KML SHA256 and a
domain-separated SHA256 of the canonical preparation. That preparation includes
original tagged projected cells, physical Env/SU identities, scope, literal
title/description field and generated KML metadata/bytes. NULL versus empty TEXT
cannot be collapsed just because XML is identical. This is a content fingerprint,
not a signature, authentication token or promise of external-writer exclusion.
Publication re-reads approval, then uses the existing independently completed
initial/prelink/postlink owned publisher snapshots.

The explicit new absolute destination is never replaced or implicitly extended;
source registry/fixed-name overwrite is deliberately not inherited. Public
outcomes are `published`, `published-with-errors` or `not-published`, with requested
destination, observed path/hash and explicit errors. A postcommit error is a
typed committed outcome, not a thrown RPC error that erases the file result.
The actual facade read-cleanup failure test proves this boundary.

One live labelled Place Name/Description pair serves review and export. The
frontend checks the displayed XML bytes with real browser crypto/DOM parsing.
Publication promises are not sent through cancellable preparation reads.
Busy/acknowledgement barriers block ordinary controls, navigation and native close.
Lost/rejected/malformed transport acknowledgements mean unknown outcome: never
claim no write or replay the destination automatically. Every outcome requires
acknowledgement and a new review before another publication; receipt/barrier state
survives component/tab remount within the running app, not reload/restart.
Safety feedback stays above fields and routine guidance.

Focused backend race12.627s/vet,398 frontend tests/check0 errors/warnings, isolated
enabled/export-default-off builds and independent eleven-file review pass.
The reviewer runs seven uncached Go tests (not race) and22 frontend tests.
Fifteen actual Wails cases across four normally closed owners include full500-row
independent SQL/XML bytes/hashes, secure-context crypto/DOMParser,1400/600px visible
associated labels, exact no-replace output, held committed-response navigation/
native-close refusal, acknowledgement/new review/remount receipts, disabled gates
and valid/invalid parent drafts. Warning/unknown transport is intentionally
injected after actual successful backend commits; it is not native backend cleanup
fault evidence. External NULL-to-empty drift changes approval despite identical
XML and refuses the stale file. All16 fixture data/config hashes are restored after
closing; three KML outputs and one byte-identical collision sentinel are retained.
Fresh full Go race920.120s/all packages passes against391 source hashes unchanged
throughout integration. The
[desktop export successor](../archives/google-earth-desktop-export-checkpoint/evidence-manifest.json)
preserves exact reviewed source/generated interfaces/isolated builds/native
output/restoration and validation evidence, verifying31 predecessor files and
protected hashes.

No canonical Access/data writes, audit/preferences mutation, default promotion,
remote icon, viewer launch or single-plot action occurs. Formal KML schema,
viewer/network behavior, native Access/VBA file-byte parity and overall replacement
acceptance remain outside this claim.

### Saved-preference source and storage boundary

`frmGoogleEarth.Form_Load` reads `clsRepOpt.GoogleEarthPlaceName` and
`GoogleEarthDescField`; their separate `AfterUpdate` events call the corresponding
property setters. `clsRepOpt` uses `GetSetting`/`SaveSetting` in VPro64's
`ReportOptions`, with literal defaults `VPro Plot Locations` and `PlotRepresenting`.
The retained runtime YAML already maps both exact names under `ReportOptions`.
Reuse strict `configString`, the owned runtime configuration and its context-aware
compare-and-set/atomic-commit precedent; do not create another preference store or
touch Access's registry. Missing, NULL and wrong-type values must not silently turn
into defaults; unchanged historical values must not be repaired.

Saved field names are global source preferences, not proof that a field is
physically available or KML-supported in the current project. Preserve and explain
unavailable selections rather than silently choosing another field. Explicit
preference persistence must remain separate from review/publication and preserve
unrelated settings, cancellation, stale-review refusal and retry/error identity.
The accepted private adapter reads the two strings strictly and compares
their original values under the existing configuration mutex before updating.
Only changed assignments are made; unchanged historical invalid strings survive.
The original `commit(data)` behavior is a compatibility wrapper around shared
context-aware staging/replacement, whose typed committed result survives
postcommit cancellation/cleanup errors. Focused configuration/preferences/KML/
CSV race14.712s/vet passes on disposable configs, including stale/no-op, bad new
values, retained historical values, queued cancellation, replacement failure/retry
and irreversible cancellation outcome. These private checks are complemented by
the coherent independently gated desktop acceptance below.

### Accepted independently gated saved-preference desktop

`GoogleEarthPreferencesService` independently authorizes live context-scoped
reads/changes with `VPRO_GOOGLE_EARTH_PREFERENCES=true`; panel controls require
`VITE_GOOGLE_EARTH_PREFERENCES=true`. No other KML capability is implicitly enabled.
The existing runtime YAML is the explicit registry-storage adaptation.
Literal source defaults are already retained there, not guessed by this service.
Exact nested raw-JSON property sets reject aliases/duplicates/malformed Unicode.
Get preserves missing/NULL/wrong-type errors and readable historical strings.

Explicit Load, not mounting/reloading fields, applies saved values to the one
shared title/description pair. Unavailable literal selections remain visible and
are not silently completed. A new description choice requires an independently
completed owned physical Env observation; unchanged unavailable/historical choices
may remain while correcting the other preference. Physical membership does not
establish KML cell support. Config CAS happens after read cleanup, compares original
values, omits unchanged assignments and preserves unrelated configuration.
Observation-time checks do not exclude external configuration writers.

Save deliberately replaces automatic source `AfterUpdate` persistence with
explicit reviewed CAS. It is noncancellable, returns a typed changed/committed/error
receipt, and never loses the committed bit through cleanup/cancellation or RPC
errors. Unknown transport requires acknowledgement plus explicit saved-value
reload before another Save. Per-context drafts, validation errors and receipts
are distinct: new draft errors survive remount and block Save/native close,
without disabling correction controls. Correction/Load/Undo clears only the
appropriate draft validation; Undo restores owned originals without writes and
invalidates prepared KML/source approval. Receipts survive acknowledgement/remount.

Coupled config/preferences/KML/CSV race15.744s/vet,412 frontend tests/check0/0,
isolated production/native builds and independent thirteen-file review pass.
Reviewer uncached preference/config and coordinate/working-unit races also pass.
Twenty-one actual Wails cases across five normally closed owners verify all-map
YAML differences for five planned app saves, no SQLite/audit writes, responsive
visible labels, cancelled late-read suppression, stale originals, busy/native
close, ack/reload/remount, correction/Undo and existing parent draft barriers.
Historical XML-invalid title and NUL unavailable field survive actual Load and
title-only correction; explicit field correction permits text review without
another config/file write. Warning/unknown response delivery is deliberately
injected after real commits; actual postcommit cancellation is separately tested
through the Go facade. All16 fixture data/config hashes are restored.
Fresh full integration916.340s/all packages passes against395 unchanged Go
sources. The [saved-preference successor](../archives/google-earth-preferences-checkpoint/evidence-manifest.json)
retains exact reviewed source/bindings/builds/native/restoration/integration
evidence and verifies the357-file export predecessor and protected defaults.
No default promotion, registry/canonical
Access/data writes, viewer/icons/single-plot or overall replacement claim occurs.
`clsVProReg.GELocation`/`GEProgram` are separate System properties; the form denies
direct keyboard editing and links to setup dialogs. Their directory/executable
defaults do not authorize filename overwrite, executable inference or viewer
launch in the desktop adaptation.

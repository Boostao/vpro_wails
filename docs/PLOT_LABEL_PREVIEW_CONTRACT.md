# Saved plot label preview

## Source and scope

This is a verified, independently default-off read-only saved-label preview,
not printer/report-layout parity. Canonical sources were read first:
`Forms/USysBecLabels.txt`, `Modules/V7mdlPrintPlotLabels.txt`,
`Modules/V7mdlUtility.txt`, `Modules/V7mdlRibbonOnAction.txt` and
`Tables_Def/BecLabels_CreateSQL.txt`. No label-specific query export was found.

The Reports ribbon's `sb12btn05` opens `USysBecLabels`, caption `VPro Plot Labels`,
bound to `BecLabels`. Form_Open requires an already loaded FS882 XL form;
Preview Current Plot calls SetLabelRecord and requeries. That routine deletes the
global staging table, then creates one row from the live form:

| Assignment | Source |
| --- | --- |
| PlotNumber1 | PlotNumber |
| Zone1 | Zone + Space(4 - Len(Zone)) + SubZone + "/" + SiteSeries |
| Representing1 | NullToNothing(PlotRepresenting) |
| ProjectID1 | NullToNothing(ProjectID) |

`NullToNothing` actually returns NULL for NULL; its empty-string assignment is
commented out. It is not a general NULL-to-empty helper. VBA concatenation treats
the nullable SubZone/SiteSeries operands as empty when combining these non-NULL
strings. A NULL Zone cannot establish the Len/Space padding count; the draft
rejects it explicitly rather than guessing padding or reusing an earlier label.

BecLabels stores twelve repeated slots. Relevant single-slot text bounds are
PlotNumber1=7, Zone1=50, Representing1=255 and ProjectID1=50 UTF-16 units. The
preview validates these new label assignments without truncating or changing
historical project data. Zone must fit the four-unit padding expression.
Malformed UTF-16/NUL, missing header fields and foreign literal identity reject.
NULL/empty metadata remains distinguishable.

The form displays Trim(PlotNumber1), Trim(Representing1), raw Zone1/ProjectID1,
`Label Date: ` plus the current Long Date and `(VPro)`. Display Trim removes ASCII
spaces at absolute string ends only, not tabs or spaces before a final newline.
Raw assignments remain available separately. The original desktop path uses the
browser's local clock/locale without exact Access regional-format calibration.
The opt-in native successor below has a separately bounded source calibration.

### Native regional date successor

An independent `VITE_PLOT_LABEL_REGIONAL_DATE=true` build-time gate and
`VPRO_PLOT_LABEL_REGIONAL_DATE=true` backend gate add a Windows Long Date path
without changing the original browser-locale preview when off. Both remain
default-off. When enabled, failure or an unsupported calendar refuses the whole
card; it never falls back to browser formatting.

The original form expression, `"Label Date: " & Format(Now(),"Long Date")`,
is now independently observed on twelve fixed-clock inputs, including leap
days, daylight-saving dates, year boundaries and year9999. Each String result
agrees after readonly reopen. The source owner normally closed, is independently
absent, has no DAO lock, and canonical source hashes are unchanged. This proves
the recorded en-US/Gregorian `dddd, MMMM d, yyyy` cases, not all regional settings
or physical printing. The portable [source fixture](../testdata/plot-label-regional-date.json)
retains independent expected strings.

The new owned RPC holds the existing context operation lease through reading
and formatting. One physical Env/Admin query requires byte-exact original text
parents; duplicates, numeric identities, substituted views, BLOB/nontext label
fields and malformed UTF-8 refuse before JSON can repair them. NULL and empty
label fields remain distinct. This stricter projection is an explicit opt-in
adaptation; the legacy `GetPlot` reader is unchanged.

Windows locale, Long Date pattern and Gregorian calendar are captured explicitly.
The fixed captured pattern is passed to `GetDateFormatEx`; every native result
checks full capacity/NUL termination and UTF-16 pairs before conversion.
Alternative calendars and API failures are explicit errors. The client validates
the exact strict-Go wire, source/owner, calendar date and text bounds, creates
detached immutable values from actual Svelte proxies, and rechecks generation
after validation before assigning a card. Existing Cancel/navigation/native-close
barriers and default-off report entries remain unchanged.

Focused Go race/native source vectors, sixteen coupled client cases,
warning-free interface bindings, Svelte check0/0, frontend build and production
compile pass. A mixed browser-condition SSR invocation is retained as a failed
runner; repository-standard execution passes without an application workaround.
Independent application review found no significant issues. Its native failure
coverage gap is resolved with typed API seams, all seven API failure phases,
unsupported calendars, zero/oversized capacities, the 4095/4096 UTF-16-unit
boundary and actual Windows export/API refusals. Focused race passes in2.909s;
distinct follow-up review closes the gap with no significant issues.
Isolated enabled and legacy Wails candidates compile with exact embedded-byte
verification, without replacing default frontend assets. Three normally closed
Wails owners now verify current native date/metadata, source NULL/empty assignments,
literal/source refusal and correction, three-width actual visibility/viewport
containment, held successful response cancellation/remount/root/native-close
barriers, backend-off refusal and legacy compatibility. Each preserves all16
fixture files. The first report-only seed lacked physical Admin parents: its
explicit refusal and zero-write normal close remain failed harness evidence;
corrected fixtures create those parents before launch, not through the app.

A separate current-candidate parent smoke verifies text Save, unchanged numeric
omission, a later real numeric Save, exact audits/all physical tables, tab draft
retention/Undo and P1/P2/P1 reopen. Its report-only NULL child identities are
assigned disposable fixture IDs before launch; no child repair/creation parity
is inferred. The first Save completed despite an incorrect two-audit runner
expectation; independent inspection seals its one real audit and continuation
never replays it. Full frontend1506 cases pass after five stale root lifecycle
test fixtures add the existing environmental activity dependency, without changing
application guards. Corrected complete1538-input race integration passes exit0
with unchanged inputs/discovery. Final association rehashes all1538inputs,
1002focused/native files,204supplement/source files and17reviewed application
paths; the application freeze is released. A separate zero-write native supplement
measures all eight disclosed assignment labels/values and live pending Cancel,
disabled input/Preview at all three widths, not screenshots alone. No printer job
or file publication is introduced.

## Desktop adaptation

`VITE_PLOT_LABEL_PREVIEW=true` adds a separate read-only Reports entry; the original
Print a Plot Label entry remains disabled. An exact saved-plot lookup replaces
source live-form controls. Existing `ContextService.GetPlot` supplies the owned,
cancellable header read; no new backend API, database family or write grant is
introduced. Source SU/profile recordset filters are not applied to this explicit
lookup. Existing Save/Discard/Cancel navigation resolves any editor draft before
leaving it. Pending reads aggregate root context/navigation/native-close barriers;
explicit cancellation and generations reject late/superseded results.

No BecLabels table, database, audit, configuration, output file or printer job is
modified. Read errors are not write drafts. Errors are visible and no partial
card is assigned if validation/date display fails. Guidance is below the fields.

## Unavailable source actions

The six print buttons open `USysBecLabels`; positions2-6 temporarily offset six
report controls by0.6666/1.3333/2/2.6666/3.3333 inches, then close without saving.
The missing report designs have now been recovered by selective static export on
a disposable exact application copy; see the recovered-design boundary below.
Physical label output is not inferred from the form's preview or report metadata.

Print All clones `Forms("FS882-6x4").RecordsetClone`, fills twelve-slot batches and
uses ProjectName where single preview uses ProjectID. Its named form differs
from the XL Form_Open requirement. Exact batch scope/name behavior, report
general pagination of `USysBecLabelsAll`, printer ownership and output remain unavailable. The
selected-device report projection below is not this callback's acceptance. No automatic
print, destructive staging-table behavior or speculative ProjectName lookup is
inherited.

## Validation

26 combined frontend tests (10 label/16 shared picture), all798 frontend tests and
`npm run check` pass with0 errors/warnings. Independent code review is clean.
Isolated enabled/default frontend and actual-main production Wails builds pass.
Four normally closed zero-write native owners verify two source labels against
independent raw SQLite rows, including NULL SiteSeries; source display/marker,
current local label date,1400/600/320px associated labels/card fields with actual
visibility/viewport intersection/containment, literal nonmember denial/no stale
card/retry, disabled default entries and held actual GetPlot response cancellation.
Input/navigation/native close are blocked while pending; explicit cancellation
discards late cards and retry succeeds. All16 original/13 retained picture/88
protected identities remain unchanged. No SQL-in-flight cancellation or Access
regional-date-format calibration is claimed. Initial native review found omitted Preview/disclosure
measurements despite their presence in screenshots. A distinct supplemental
layout owner preserves original evidence and records twelve input/card/closed
disclosure/opened-assignment segments at all three widths. Every expected selector
must resolve; each control/field has explicit visibility/intersection/containment
measurements. Focused independent follow-up is clean. The distinct
[327-file seal](../archives/plot-label-preview-checkpoint/evidence-manifest.json)
is independently rehashed twice; manifest SHA256
`caa21fc7e5558bdd018eefdf128c08058c92531ff1a5b4c1aaeeed76c54b930c`.
Accepted picture-manager
predecessor [322-file seal](../archives/picture-manager-desktop-checkpoint/evidence-manifest.json)
is unchanged; no canonical Access/data writes or new Access launch occurred.

## Recovered report designs (static evidence, not printing)

One macro-disabled Access owner exported only `USysBecLabels` and
`USysBecLabelsAll` through SaveAsText without opening forms/reports or executing
VBA/print actions. Its disposable copy had only the StartupForm property removed
before opening and restored after normal Quit. An initial DAO empty-string
assignment failed before launching Access; the second preparation succeeded.
The reader's post-Quit Get-Process check returned exit1 for an absent process;
independent closure verification confirmed no Access process, unchanged
canonical SHA256/registry and restored fixture setting without relaunching Access.
Linked-table inventory was not collected; no data/import boundary is claimed.

Canonical application SHA256:
`01481b94569c7172f32a812e65365f78cea5e440dd63220d91a37dade5778423`.
Private original UTF-16 exports retain opaque printer properties; the reused
inventory/layout parser publishes only source-hashed UI facts:

| Report | Source SHA256 | Packaged facts |
| --- | --- | --- |
| USysBecLabels | b5779398241ed783a9a3c5011861cb695605c05d5201e21ec2af009c7113d94f | [Single-label layout](../resources/plot-label-report-layout.json) |
| USysBecLabelsAll | ecbc1ffbe01e0cd2d66836c4301ed1e173de7e468cd4743d27ef9b4ac038f1c2 | [Twelve-slot layout](../resources/plot-label-all-report-layout.json) |

Both reports use BecLabels and have no exported event properties or executable
report procedures; their code-behind contains declarations only. Source
coordinates are twips, not an application desktop pixel-layout requirement.
Single report Width=4954, Detail Height=7260; batch Width=4833, Detail Height=14760.
Each slot contains:

| Control role | Left | Top relative to slot | Binding/caption |
| --- | ---: | ---: | --- |
| Plot number | 72 | 0 | Trim(PlotNumberN) |
| Project | 1296 | 0 | ProjectIDN |
| Zone | 2448 | 0 | ZoneN |
| Representing | 72 | 240 | Trim(RepresentingN) |
| VPro marker | 75 | 600 | (VPro) |
| Date | 507 | 600 | "Label Date: " & Format(Now(), "Long Date") |

The single base Top is60. The twelve batch Tops are
60/1200/2340/3480/4620/5760/8280/9420/10560/11700/12840/13980.
The2520-twip gap between slots6/7 is not the1140-twip spacing of neighboring slots;
do not substitute a uniform twelve-row grid or assume a page break from this
metadata alone. All72 batch controls are retained.

## Native report PDF projection (not source print callbacks)

Two fresh disposable Access databases now import the original single and
twelve-slot report exports, without importing/opening FS882 or invoking label
form callbacks. Synthetic BecLabels staging assigns all49 fields explicitly.
The source DDL's exported empty DEFAULT clauses are omitted only in these
fixtures; this does not qualify default/index reconstruction.

Each disposable report selects Microsoft Print to PDF locally. No default
printer is changed, `acViewNormal` is never invoked, and no job is spooled.
Original control bindings, twip positions, fonts and sizes remain identical
before/after that selection. Native preview/OutputTo produces four PDFs: single
and twelve-slot reports with complete values, then NULL/empty/nonempty metadata.
Independent PDF reopen verifies26 label slots,129 exact printed lines, source
ASCII-space Trim, unchanged raw zone/project display, original current Long Date,
and no text span outside the page. NULL and empty presentation both remain blank
while the49-field source staging retains their typed distinction and is unchanged.

All four outputs are one approximately612.16-by792.06-point page on this selected
virtual device. The recovered nonuniform twelve-slot spacing is visible in the
native renders; it is not replaced by a guessed grid. PDF span geometry/fonts are
retained separately from design properties: neither report metadata nor this
renderer observation proves other-device scale, stock alignment or pagination.
Both source owners quit normally, are independently absent, and leave no DAO
locks or canonical source changes. The27-file private source/projection seal has
SHA256 `55cf99df652cbc40121f46b8854dfc9c1ad2c9555b978fbf272d3cecc24440d7`.

This is a report-rendering contract, not a new application PDF export, source
Print1/position-button/PrintAll callback grant or physical printing capability.
In particular, Print All's ordinary FS882 recordset and ProjectName discrepancy,
live-form staging/editing and six-position click behavior remain unresolved.

### Portable single-label PDF preparation draft

A private Go preparation kernel now uses `go-pdf/fpdf` with independently parsed
runtime Arial/Arial Bold TrueType bytes, never the library's Helvetica alias.
No Windows font files are bundled. Font identity, embedding/subsetting permission
and required glyph coverage are checked before registration. Only installable
or editable embedding permissions are supported; preview/print-only rights are
not treated as permission for an unrestricted editable PDF. A separate private
font-only experiment independently reopens four candidates containing all129
original printed spans. Actual embedded font identities and exact text agree,
but native glyph advances differ by up to0.834595 points. This is explicitly
**not** accepted Access printer geometry; replaying observed text origins is not
a layout implementation.

The draft kernel instead prepares one portable Letter page with36-point margins,
the six original relative control rectangles and measured font sizes/weights.
It retains source ASCII-space Trim only for plot/representing, raw project/zone
display, NULL/empty source distinction, the marker and calibrated en-US Long
Date. Fixed spacing is an explicit adaptation, not CanGrow/CanShrink, driver or
stock parity. The supported preparation boundary is printable ASCII with no
field overflow; Unicode, multiline/clipped content, unsupported regional/date
provenance, mismatched saved-plot ownership and missing/substituted/restricted
fonts refuse with zero output. These are output availability decisions, not new
constraints or repairs on historical database values or the existing preview.

This preparation kernel alone grants no destination publication,
Print All/position callback or printer output. Focused PDF/regional race passes
in3.224s; ordinary and SQLite-only production builds pass. Independently reopened
actual-kernel full/NULL examples verify10 exact printed spans, embedded Arial,
a612-by792-point page, and every field's original relative rectangle/alignment.
Input/font immutability, all twelve original date vectors, identity/text/glyph/
embedding/overflow refusals and cancellation after registration/encoding pass.
Distinct read-only review finds no significant issues. This accepts only the
preparation boundary: owned-source publication, client lifecycle
and integrated desktop acceptance remain unfinished; defaults are unchanged.

A private runtime font loader now resolves the actual Windows directory rather
than trusting environment paths. Bounded regular-file reads own their handles,
surface open/stat/read/close failures and cancellation (including close-time
cancellation), validate the captured font bytes and return no partial font pair.
The existing cancellable chunk reader is reused. Missing or unsupported fonts
never trigger substitution. Focused combined race3.206s and SQLite-only production
compile pass; distinct loader review finds no significant issues. The preparation
permission tightening also passes focused follow-up review/race3.119s and an
ordinary production rebuild. No public output is enabled.

### Owned PDF publication draft

The private publication boundary reads complete physical Env/Admin rows and
column metadata through caller-owned pinned snapshots, with literal TEXT/byte
membership and exact selection/path/owner provenance. It retains raw NULL/empty
cells and physical identities, native date and runtime-font hashes alongside
internally generated deterministic PDF bytes. Existing legacy readers are
unchanged. Client-supplied PDF bytes never establish authority: full approved
bytes must agree with a fresh internal preparation.

The shared artifact publisher owns staging/parent identities and never overwrites
a destination. Source/date/font observations are repeated before the link and
after publication. Completed observations establish equality only at their
observation times: the owner mutex does not prevent writes through already
borrowed pools, and no through-link source lock is claimed. Postcommit drift or
cleanup errors retain the published path/hash with an explicit do-not-replay
error, rather than returning an empty failure result.

Focused coupled race8.480s and ordinary production compile pass. Actual published
output independently reopens with six exact source strings (including literal
project spaces), real embedded Arial and one612-by792-point page. Local/external
paths, complete-peer automatic rowid reuse, stale source/date/fonts, forged
headers/bytes/ownership, malformed storage, duplicate parents, cancellation and
pinned-coordinator retry, no-overwrite collision, corruption/short-write and
postcommit snapshot/stage-cleanup failures are tested. Distinct publication review
finds no significant issues. Direct Windows destination-parent replacement now
also refuses publication and retains the stage under its original moved parent;
affected coupled race8.627s and focused review pass. The publisher alone does not
establish retained desktop, printer or whole-migration acceptance.

### Strict service and attempt recovery draft

The new service has independently gated review/export methods and strict
required-key JSON requests, including raw-Unicode and decoded duplicate-key
checks. A review binds complete physical source, captured date/font provenance,
internally generated bytes and ownership to source/PDF hashes. Its service-owner
token is independent of the per-request lowercase UUIDv4.

Per-service attempt records bind that token, the original context and exact
canonical payload. A running request cannot replay. Completed calls return
detached immutable outcomes, including committed cleanup errors. Readonly
recovery remains available when exporting is disabled; it reports running work
or seals a not-yet-arrived exact request before certifying no publication can
begin. Recovery never republishes and may read a prior context's known ledger
receipt without granting old-context writes.

These receipts are process/service-owner scoped, not durable database history.
A fresh service rejects retired-owner requests with explicit unknown outcome;
an empty new ledger never certifies that a previous service did not publish.
Source-preparation failures and pre/post-publication cancellation retain typed
recoverable outcomes without claiming rollback of a committed file.

Focused coupled race11.932s and ordinary production compile pass. Strict-wire,
disabled/nil/canceled service, stale source approval, UUID payload/context
ownership, held concurrent export/running recovery, delayed-arrival sealing,
immutable receipts, service-owner replacement and committed-error/cancellation
cases are covered. Distinct service review finds no significant issues.

### Registered portable PDF desktop

The service is now registered behind independent default-off
`VPRO_PLOT_LABEL_PDF_EXPORT`; interface-mode Wails bindings preserve nullable
contracts. Independent `VITE_PLOT_LABEL_PDF_EXPORT` enables the saved-label route
and portable PDF controls, not original physical Print. With PDF off, existing
browser/regional previews retain their prior behavior. Full project/SU paths and
context identity accompany immutable raw reviewed DTOs; no Svelte deep proxy is
cloned at the export boundary.

The retained per-context publication session owns the exact lowercase UUIDv4,
approved hashes, destination and service token. Export is neither cancelled nor
replayed. Busy/unknown/unacknowledged outcomes hold root lifecycle barriers;
readonly recovery checks exact context/UUID/destination/bytes. Unknown outcomes
cannot be acknowledged as success. Known immutable receipts survive acknowledgement
and remount independently of source preview/card visibility. A retired service
owner stays explicitly unknown, never inferred as no publication.

Coupled race12.108s includes strict Go/client wire and independent default-off
registration. Thirty focused and1545 complete frontend case executions, check0/0,
default frontend and two exact-embed isolated Wails builds pass. Distinct15-file
desktop review finds no significant issues. Four normally closed disposable
owners retain all16 database/config files and verify one once-only PDF, six
independently reopened exact strings/embedded Arial/Letter page, actual successful
chunk-aware response hold/loss, readonly recovery without replay, card-independent
receipt remount, actual read Cancel/late-result rejection, NULL/empty/refusal
correction, backend/client gates and51 control/receipt measurements at1400/600/320.
The rejected read-only predecessor used an assignment selector that also included
new PDF hash fields; corrected scoping preserves its failure/cleanup evidence and
all application guards. Complete1578-input race integration passes exit0 in
5401.705149s with unchanged inputs/discovery. Final association rehashes1031
native/build/review artifacts, preparation/service predecessors and27 source
projection files, verifies five canonical label sources and seven original
native/source owners absent, and releases the freeze. A failed missing-process
exit-status assumption is corrected to accept only the explicit no-process error.
This accepts the bounded portable workflow, not whole-migration completion.
No defaults, canonical data, physical printing or unresolved callbacks change.

Single print positions2-6 **add**, not subtract,
0.6666/1.3333/2/2.6666/3.3333 *1440 to all six original control Tops.
The form explicitly names Avery #02181. These decimal offsets differ from the
batch spacing and are not a license to guess driver rounding, pagination or
paper calibration. A bounded original-VBA-leaf probe currently exhausted two
attempts without runtime qualification of positions2-6: unsupported Section COM
access, then Access.Run argument dispatch. Position1 produced an original PDF
with unchanged staging and restored controls, but is not additional-position
acceptance. Both failed owners normally quit, are independently absent and
DAO-unlocked; five canonical label sources remain unchanged and no job was
spooled. Additional positions remain unavailable rather than inheriting
arithmetic-only evidence.
Built-in properties omitted by SaveAsText (including some
textbox Heights and ProjectID FontSize) remain absent in the export; the separate
measured-default boundary below now resolves these properties. Exported FontName=Arial
defaults retain their inherited provenance. Opaque printer blobs are not copied
into public UI facts, decoded into a printer grant or treated as portable output.

Reproduction uses the existing guarded static extractor, now with mutually
exclusive form/report roots:

```powershell
go run .\cmd\fs882layout -source DISPOSABLE_EXPORT_ROOT -report USysBecLabels -out resources\plot-label-report-layout.json
go run .\cmd\fs882layout -source DISPOSABLE_EXPORT_ROOT -report USysBecLabelsAll -out resources\plot-label-all-report-layout.json
```

The export root may contain Reports only. No export folder is synthesized inside
canonical source, and destination/source-alias guards remain shared with form
extraction. Report metadata alone does not resolve Print All's named
FS882-6x4 recordset or ProjectName assignment. Physical printing remains disabled.

### Omitted built-in property measurements

A separate fresh macro-disabled application copy now resolves the previously
omitted control defaults. The two recovered reports have declarations-only code
and no exported event paths; the copy also has zero linked tables. Only Design
view was opened (independently observed CurrentView=0), never report preview,
printing or data actions. One fresh PID/start/executable/window owner measured
every6/72 control, closed each without saving, normally quit and restored copied
StartupForm. Canonical application/registry and all16 original/13 picture/88
protected identities are unchanged.

[Measured defaults](../resources/plot-label-report-measured-defaults.json) remain
separate from the raw exported facts:546 exported/inherited properties match the
observations exactly;78 omitted properties are supplied for all78 controls.
PlotNumber/Zone Height=240; ProjectID Height=240, FontSize=8 and TextAlign=0
(General); marker FontWeight=400. No guessed Height/FontSize is substituted into
the export. The source-compatible renderer must still account for exported
CanGrow/CanShrink and wrapping; a Design rectangle is not print-output proof.

Complete frontend validation now passes808 tests/check0/0 with isolated builds.
Full Go integration passes root2207.039s/all packages. A subsequent one-line
static-extractor correction prevents a same-named report from inheriting the
FS882 form coordinate inference; its full three-package race/vet passes
1.342/1.333/1.407s. This correction is not another root all-package run. No runtime
service, binding, native write lifecycle or printer grant changed.

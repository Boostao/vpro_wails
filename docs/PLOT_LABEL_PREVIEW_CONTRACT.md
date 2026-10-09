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
Raw assignments remain available separately. Desktop date presentation uses the
local clock/locale; exact Access regional formatting has not been calibrated.

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
pagination of `USysBecLabelsAll`, printer ownership and output remain unavailable. No automatic
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
[327-file seal](MIGRATION_EVIDENCE.md#snapshot-plot-label-preview-checkpoint)
is independently rehashed twice; manifest SHA256
`caa21fc7e5558bdd018eefdf128c08058c92531ff1a5b4c1aaeeed76c54b930c`.
Accepted picture-manager
predecessor [322-file seal](MIGRATION_EVIDENCE.md#snapshot-picture-manager-desktop-checkpoint)
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

Single print positions2-6 **add**, not subtract,
0.6666/1.3333/2/2.6666/3.3333 *1440 to all six original control Tops.
The form explicitly names Avery #02181. These decimal offsets differ from the
batch spacing and are not a license to guess driver rounding, pagination or
paper calibration. Built-in properties omitted by SaveAsText (including some
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

# VPRO Desktop

Access-to-Go/Wails v3 beta.26/Svelte5 migration with SQLite canonical.
**Incomplete, experimental; not a production Access replacement.**
Use disposable projects for writes. The companion R package is planned as
UI-free core automation, not a second data-entry UI.

## Demo readiness and remaining Access work

The usable core is project/Site Unit/hierarchy selection, offline SQLite data
management and the bounded FS882 editor. The 98 mapped parent fields and reviewed
child workflows are not proof of every original form event. Report previews,
qualified XLSX/KML output, additional forms and retained preference editors have
individual opt-in flags; the normal build intentionally leaves them default-off.
A disabled menu item means unavailable in that build, not permission to bypass
its guard. Use a disposable project/configuration for a demo with writes.

| Area | Ready within documented boundaries | Still missing for replacement parity |
| --- | --- | --- |
| Forms | FS882 mapped parent/children; bounded SIVI, two-page, picture and restoration workflows | Whole-form linked callbacks, remaining calculations/navigation, picture timing and printer-dependent events |
| Reports | Long Environment/Vegetation, Site Unit/Lifeform summaries, raw Summary inspection and qualified exports; independently gated bounded numeric environmental inspection | Complete combined/formatted Summary output, broader lifeform/numeric environmental cases, remaining grouping/lumping variants, printing |
| Data management | Original SQLite family, physical project names/descriptions, YAML configuration, external contexts, draft-safe switching and reviewed transfers | Production project creation/import/conversion, merge/splinter/compare and broader administration |
| Other workflows | Hierarchy browsing, bounded species checking, location preparation, read-only label preview and integrated saved User log inspection | Hierarchy editing/diagram, Herbarium, broader standards compliance, user/session administration, theme and TurboVeg |

UI-free R automation, cloud/public BEC products and wider publication/analysis
remain separate scope tracks, not delivered desktop functionality. See
[client scope](docs/CLIENT_SCOPE.md), the
[ordered migration plan](MIGRATION_PLAN.md) and the detailed capability table below.
No overall Access-parity percentage or completion claim is made.

The Access ribbon is not fully migrated. Its project/Site Unit/hierarchy
administration, import/export tools, auxiliary forms and Help actions are not
covered merely because the desktop displays Forms and Reports menus.
Normal **Forms > Enter/Edit SIVI Data** now delegates to the existing guarded
standalone opener when `VITE_SIVI_STANDALONE=true` and
`VITE_SIVI_PARENT_REVIEW=true`, with a selected owned plot and known context.
It preserves draft/close decisions and independent backend review denial;
The separate experimental CHARS parent entry requires its own literal
`VITE_SIVI_CHARS_PARENT_ENTRY=true` and independent backend opt-in; unsupported
actions remain disabled. Its linked covers/heights additionally require the
existing combined-editor frontend/backend gates. This does not complete SIVI callbacks or the original
ribbon's other functionality.

**Forms > Metadata** now delegates to the existing project-metadata editor for
the currently open FS882/SIVI plot. It is unavailable until the current editor
is clean, unlocked and available under `VITE_PROJECT_METADATA_EDITING=true`.
It shares the existing metadata guards rather than adding a separate controller
or silently choosing a physical record. Global metadata administration and
implicit Access Save behavior are not granted; other metadata operation flags
and backend ownership checks remain unchanged.

**Forms > User log - Read-only** has a reviewed, disposable-native-verified
integrated successor. It requires `VITE_USER_LOG_REVIEW=true` when
building the frontend and independent `VPRO_USER_LOG_REVIEW=true` at runtime.
The explicit Review reads the current context's preserved VPro64
`USysUserLog`, not project audit history or live Wails sessions. Original User,
Login, OutTime and Local Machine captions remain; Show Current defaults to2 and
means only stored NULL OutTime, while Show All is1. NULL/empty text, duplicate
physical rows and raw temporal values remain distinct. Technical rowid ordering,
50-row pages and explicit refusal above10,000 physical rows are desktop
adaptations. No login/logout recording, source editing/deletion, datetime or
timezone repair, new storage or default enablement is granted. This successor
is not in the qualified Friday ZIP below. Full Go race integration passes with
all1697 core inputs, separately pinned HTML,209 artifacts and package/embed
discovery unchanged; all seven native owners are freshly absent.

**Reports > Subzone Matrix of Units** has a default-off desktop successor,
reviewed, disposable-native-accepted and full-race integrated. It requires
`VITE_SUBZONE_MATRIX_WORKBOOK=true` at frontend build time and independent
`VPRO_SUBZONE_MATRIX_WORKBOOK=true` at runtime. It reads the selected physical
hierarchy and preserved VPro64 labels, not the project/SU implied by the old
ribbon description. Independently reviewed planner, reader, streamed XLSX
preparation and publication service retain literal text, NULL/empty values and
complete-original source witnesses. Differing unordered First candidates and
unsupported comparisons refuse rather than guess. Preview paging avoids XLSX
serialization; full review and no-overwrite publication are separate operations
with retained irreversible receipts. Technical ordering, heuristic widths and
portable `.xlsx` are explicit adaptations, not original Excel-host or `.xls`
parity. See [the report contract](docs/SUBZONE_MATRIX_CONTRACT.md). Both flags
remain default-off; the qualified demo and accepted frontend assets are unchanged.
Actual Wails default/frontend-off/backend-off/enabled modes pass, along with
literal paging, exact XLSX publication/no-overwrite, retained receipts,
Cancel/remount and exact native-close cancellation acknowledgements.
All1,963 registered frontend executions pass. Seven coupled root harnesses
explicitly preserve the new default-off publication peer; independent follow-up
review finds no significant issues, with application/bindings/assets unchanged.
Full default-profile Go race integration passes with all1,814 inputs,
2,439 artifacts and package/embed discovery unchanged. All five original native
owners are freshly absent; the source freeze is released.

**Reports > PC-ORD Break Report** now has an independently reviewed and
disposable-native-accepted read-only successor. It requires the independent
literal `VITE_BREAK_REPORT_REVIEW=true` frontend build and
`VPRO_BREAK_REPORT_REVIEW=true` runtime flags, both default-off. It reviews
saved global VPro64 `USysBreakTable` DISTINCT pairs, not a new PC-ORD export.
Original caption, code/reference column order, NULL/empty and literal values
remain visible. Whole-source bounds/hash, strict transport and source-stable
paging refuse unsupported/ambiguous values rather than repair them.
Actual Wails gate isolation, first/tail pages, three-width layout, held
read/native-close cancellation/remount and off-page malformed-text
refusal/correction/retry pass, with all sixteen fixture files restored.
All 2,110 registered frontend executions and complete-source Go race integration
pass: 1,833 inputs, 2,226 artifacts and package/embed discovery unchanged, with
all four original native owners freshly absent. Original timestamp/page footers,
physical printing and export generation remain unavailable.
See [the Break Report contract](docs/BREAK_REPORT_CONTRACT.md).
The qualified demo and accepted frontend assets are unchanged.

CHARS SIVI now has an explicit internal 71-binding source projection and owned
physical reader, separate from the unchanged normal 77-binding owner.
The source-specific particle-size TEXT6 and water-depth SINGLE fields are not
aliases. This is read-model groundwork, not a newly available form or writer.
All136 coupled parent race cases, source-tool races and production SQLite-only
compilation pass; distinct read-only review accepts the source-owner scope.
Complete-source Go race integration passes with all1,836 inputs,57 artifacts
and package/embed discovery unchanged. Four predecessor native owners are
freshly absent; this is not CHARS native-form acceptance.
See [the SIVI source boundary](docs/FS1333-parent-access-contract.md).
The internal CHARS direct-field planner now covers68 editable domains while
excluding identity/ProjectID/PlotType/action paths. It preserves unchanged
historical values and prepares new SINGLE values without changing normal
writers. All188 coupled race cases and production vet/build pass, and distinct
read-only review accepts this internal scope. This is not write or reference-
membership authority; complete-source integration now covers the internal successor below.

The internal CHARS reference successor covers31 non-ProjectID ComboBoxes within
one owned parent/reference snapshot, reusing28 source-equivalent normal policies.
RootZoneParticleSize, SnowCoverRegime and Ah/Ae/blank retain their distinct sources.
Original CHARS SubZone SQL points to the normal form's cmbZone, so its choices
remain explicitly unavailable pending runtime qualification; blank Ahorizon
assignment likewise remains unqualified. This does not enable CHARS editing.
All195 coupled race cases plus production vet/build pass, with distinct
read-only acceptance of this internal reference scope. Complete-source integration
covers this read model; native entry acceptance remains outstanding.

The internal CHARS typed writer/restorer reuses the owned source-parent
transaction machinery and requires independent approval. Its isolated history
keeps the full prepared plan separate from the audited subset; SubZone changes
and new blank Ahorizon assignments remain refused. All67 qualified domains pass
focused physical write/restoration tests. All201 coupled race cases and
production vet/build pass, with distinct acceptance of this internal scope.
Complete-source Go race integration passes (root4308.862s), with all1,846 inputs,
109 sealed artifacts and package/embed discovery unchanged. All12 canonical
sources/19 protected assets and the accepted frontend/demo remain unchanged.
CHARS desktop acceptance stays outstanding; internal writer acceptance alone
does not grant public form parity.

The CHARS service supplies strict review/save/restore
transport, owned reference-file identity checks and the four source-mandatory
catalogues inside the writer transaction. Optional codes do not inherit
two-page acknowledgements. All257 coupled races and production vet/build pass,
with distinct fourteen-path read-only acceptance. Its source-policy/client
boundary now has259 coupled race cases,68 frontend cases, zero-diagnostic
check, isolated build and distinct nine-path acceptance. CHARS keeps its own
finite DOUBLE coordinates, SINGLE narrowing and source year bounds; the shared
intent/text/date/Boolean parser does not change two-page numeric rules.
The independently reviewed desktop candidate wires the existing close/navigation owner,
source-grouped71-binding presentation, reference-safe drafts and explicit
Save/Undo/history behind independent `VPRO_SIVI_CHARS_PARENT_ENTRY` and
`VITE_SIVI_CHARS_PARENT_ENTRY` literal flags (both default off). The Forms item
retains a parent-only profile when the combined-editor gate is off. The linked
cover/height scope below adds only the existing extended editor; child Species,
Collected, identity/lifecycle and ProjectID/PlotType remain unavailable, as do
changed SubZone and blank Ahorizon. The separately qualified
Spp List action below adds only Comp./Part. to this same parent owner.
All200 affected frontend cases,2310 standard frontend lifecycle executions,
zero-diagnostic check, production vet and isolated default/enabled Wails builds
pass. Actual Go/client fixtures cover all31 references, including Ahorizon
literal ordinals and BEC Zone value identities without fabricated physical IDs.
Owned Wails acceptance covers independent gate combinations,71 labels/fields,
67 unlocked controls, validation/Lock/navigation/native-close barriers, held-read
cancellation and stale-remount refusal, a five-field mixed physical Save with
five exact audits, actual prune restoration and distinct P1/P2/P1 saved reload.
All original physical non-audit rows and389 pre-existing audits remain unchanged;
three consumed histories and one deliberately retained restored audit remain.
Remount ends this editor's history selection; explicit owned native restoration
was separately verified, not represented as retained UI selection.
Layout is verified at actual client widths1384/744/544px; the560px minimum
native window clamps a requested420px window, so420px client coverage is not
claimed. Full Go race integration was stopped before acceptance when exact DAO
calibration exposed a new Boolean NULL-write mismatch. Source SV_FloodPlain
converts NULL to No at assignment; the corrected candidate explicitly refuses
changed SQLite NULL and offers No (0), without silently normalizing a request.
Unchanged historical NULL and typed-history restoration remain supported.
The immutable corrective candidate passes2311 standard frontend executions,
201 affected client cases,262 fresh coupled Go race cases, check0/0,
production vet and default/enabled Wails builds. Distinct12-path review accepts
without significant findings. Current-binary default/client-off/backend-off/
read and Boolean write acceptance pass: real Yes/No Saves and exact audits,
unsupported NULL client/backend refusal, exact native-close cancellation before
correction, historical NULL omission and actual UI restoration. One explicitly
prepared disposable P3 NULL is the historical witness, not an Access new-write
claim. Two harness postcommit expectations were recovered from closed physical
history without replaying any Save/restoration. Three additional consumed
technical histories remain; all baseline source/audit/history rows are exact.
Fresh complete Go race integration passes exit0 (UTC2026-10-15T16:50:47 to
18:16:18):1863 inputs,149 sealed artifacts, package/embed discovery,12 canonical
sources/19 protected assets and the closed native family remain unchanged;
all18 original native owners are freshly absent. The source freeze is released.
The earlier stopped integration remains unaccepted. No default/demo availability
or whole-form/ribbon parity follows this bounded parent acceptance.
No default or qualified demo availability has changed.

CHARS **Spp List** now retains the source **Comp.**/**Part.** options through the
same owned parent draft and atomic Save as direct fields. Comp. stores true=-1;
Part. stores0. There is no new NULL command: unchanged historical NULL omits,
and typed restoration preserves it. Source callback timing is an explicit
Save/reload adaptation, not an automatic Access-event claim. PlotType's unwired
private procedure grants no writer. Strict optional action transport and complete
plan/audited-subset history preserve legacy direct-only requests and receipts.
All2320 frontend executions,205 affected cases,841 coupled Go test/subtest
events, check0/0, vet, isolated default/enabled production builds and distinct
20-path review pass. Actual Wails verifies independent gates, five Save/prune
pairs with six exact changed/audited cells, historical NULL, same-option zero
backend Saves, retained invalid mixed drafts and the exact native-close
cancellation acknowledgement before correction. One interrupted UI Save was
sealed and restored once through the scoped binding after restart; four later
restorations use the actual UI. No mutation was replayed. Original source,
audits and old histories remain exact with only five consumed new receipts;
fifteen other files are unchanged and all six owners are freshly absent.
Fresh full race passes (root4453.093s), preserving1869 inputs/98 artifacts and
package/embed discovery. This action alone does not enable children or complete
CHARS/ribbon parity; defaults/demo assets remain unchanged.

CHARS now composes the existing extended **17-column cover/height editor**
through its original Tree/Shrubs, Herb and Moss/Lichen PlotNumber links.
Both literal frontend gates (`VITE_SIVI_CHARS_PARENT_ENTRY`,
`VITE_SIVI_COMBINED_EDITING`) and their independent backend gates are required.
No backend writer, binding or flag was added. Parent and child retain separate
drafts, errors and typed history receipts; lock, navigation and native-close
guards cover both. Explicit read-only recovery remains reachable when a
postcommit parent read fails, without replaying the completed Save.
All2350 frontend executions, check0/0, focused Go races/vet and three isolated
frontend/Wails profiles pass. Independent review found and then verified the
parent-read recovery correction. Actual Wails proves all17 columns, cover-driven
A/C/D membership with height-only omission, P1/P2/P1, two Saves/four exact audits,
separate UI pruning, held read/Cancel/stale-remount refusal and three-width
visible labelled controls. Business/audit/old-history rows and fifteen other
files restore exactly; only two consumed receipts remain. All five owners are
freshly absent. Full race passes (root4345.973s), preserving1871 inputs,
89 artifacts and package/embed discovery. Original callback timing, child
Species/Collected/identity/lifecycle, whole-form/ribbon completion and
default/demo promotion remain excluded.

The same qualified Yes/No correction is now implemented across normal SIVI
FloodPlain/source Species actions, two-page common/complete entry and the ordinary
Species flag. Changed NULL refuses explicitly; Yes=-1/No=0, unchanged historical
NULL and old NULL-after restoration remain supported. Complete-entry requests
independently re-prepare these two Boolean drafts against their owned originals.
UpdatedFromCards, generic metadata/import and creation defaults are not changed.
All2318 standard frontend executions,167 affected cases,264 coupled Go races,
check0/0, vet and three private actual-main builds pass. Independent review
finds no significant issues. Fresh disposable Wails default/backend-off modes,
eighteen typed Yes/No Save/prune pairs and one ordinary Save/audit-restore pair
pass. Specific native-close acknowledgements precede correction/Undo; source,
original audits/history and fifteen nonproject files remain exact. An ordinary
postcommit harness expectation was completed from its sealed checkpoint without
replaying a mutation. Old NULL-after receipt coverage is Go, not a new native
legacy-receipt claim. The test-only ordinary-header follow-up passes21 focused
races and independent review, retaining all mapped-field legacy restoration.
Fresh full repository race passes (root4499.991s); all1866 inputs/296 artifacts,
package/embed discovery, canonical/protected assets and closed native family
remain unchanged, and all15 original owners are freshly absent. Earlier failed
runs remain rejected. No accepted demo/assets or availability defaults are promoted.

The qualified private Windows demo is
`build\bin\VPRO-Friday-Demo-2026-10-16-menu.zip` (17,273,336 bytes;
SHA256 `450313ae2b111312190f939f75d4b69e3d2cc57695b0b92615e907eb0a7ac013`).
It includes the normal SIVI and Metadata menu entries, actual parent/metadata
Saves with exact physical-data/audit checks, P1/P2/P1 saved-value reload and the
bounded thirty-row Summary inspector. The packaged launcher passes three-width
layout, cancellation/native-close/remount and sixteen-file read-only constancy
checks. Each launch creates a new disposable data/configuration copy.
The current `sqliteonly` executable is 41,276,928 bytes (39.4 MiB); a predecessor
same-assets comparison measured 86.4 MiB with optional DuckDB. The exact current
binary, tested with default backend gates, measured62-78ms for ten synthetic
10,052-plot first/deep-page reads against a250ms bound; fresh-install
launch-to-Home measured1.656s on this VM, including
harness overhead. These are bounded observations, not a general latency or
source-parity guarantee.

Full1687-input Go race integration passes with unchanged source/package/embed
discovery. Its strict wrapper remains rejected for one evidence-only context-log
footer that completed before actual Go startup; a distinct independent review
accepts the separate completed integration association while retaining the
original freeze and rejected receipt. ZIP integrity, all twenty member hashes
and a fresh prepare-only launch pass. The original demo ZIP is preserved
unchanged. Normal defaults remain unchanged; this is private demo acceptance,
not production release or whole-migration acceptance.

The combined Summary numeric environmental inspection requires both
`VITE_SUMMARY_VEGETATION_ENVIRONMENT_NUMERIC=true` at frontend build time and
`VPRO_SUMMARY_VEGETATION_ENVIRONMENT_NUMERIC=true` at runtime, alongside the existing
Summary and environmental gates. It supports five direct functions (Seepage,
Rooting Depth, Stand Age, Elevation and Slope), six substrate suffixes and four
stratum-cover suffixes within the calibrated source domain. Empty numeric results
are explicitly labelled, never converted to zero or an empty string. Unsupported
selected fields refuse the complete preview. Lifeform environmental rows, grouped
environmental output, formatting and printing are not enabled by this flag.
The Home capability panel reports build availability, not runtime authorization.

Lifeform environmental inspection additionally requires
`VITE_SUMMARY_VEGETATION_ENVIRONMENT_LIFEFORM=true` at frontend build time and
`VPRO_SUMMARY_VEGETATION_ENVIRONMENT_LIFEFORM=true` at runtime; all existing
Summary/environmental/numeric gates must also be enabled. The separate service
supports the thirteen literal source lifeform arguments (`1` through `13`) in the
same owned snapshot as numeric/category/count rows and displays master-source
provenance. Lifeform empty strings remain distinct from numeric Empty results.
The native read-only boundary is verified; whole formatted Summary output,
workbook scalar conversion and printing remain unavailable. Defaults stay off.
All 53 supported definitions from the 54-row canonical field list have also
been exercised together through owned managed/external snapshots, strict client
proxies and Wails in a bounded missing-Env/lifeform fixture. `SVHumusDepth` remains
unavailable: its source Env lookup disagrees with both exported Access and
current SQLite Admin storage, and no replacement join is invented. This static
mismatch is not a verified source runtime outcome. Selecting it refuses the
entire preview.
Original Excel output coercion is not a safe proxy for typed inspection: for
example, a source string `1 - 2 - 3` becomes a date serial. The inspection keeps
the original tagged value instead of claiming workbook-formatting parity.

The explicit [SQLite-only Windows build profile](duckdbdriver_sqliteonly.go) removes
unused optional DuckDB linkage rather than removing features: about 38.4 MB versus
87.8 MB for the same stripped frontend build. Sample native startup and first
editor opening measured about 1.5 and 0.6 seconds respectively, with harness
overhead. Ten first/late page reads on a disposable synthetic10,052-plot project
measured47-63ms, with unchanged data and normal native closure. The approximately
4 MB initial JavaScript bundle still needs separate optimization; these bounded
measurements do not prove that every reported performance issue is fixed.

## Current capabilities

| Workflow | Delivered boundary | Availability/gaps |
| --- | --- | --- |
| Database/context/configuration | Original SQLite family, YAML defaults/runtime and retained JSON migration; non-overwrite installation, external attachment/compatibility, offline TEMP views, draft-safe switching and visible recovery | Active; conversion/administration/multiwindow not complete |
| Project selection and browsing | Managed/external project paths, Working Unit/SU filtering and read-only hierarchy browsing | Attachment is not copying/conversion or full SU/hierarchy editing |
| FS882 parent |98/98 mapped parent fields native-verified writable, responsive source labels/groups/links and shared draft/audit/Lock/close behavior | Bounded baseline, not all forms/events/pictures/projection |
| FS882 children | Other8/Humus12/Mineral18 CRUD; Veg Other12 attributes, Collected, species decisions,11 ordinary cover/total fields and6 heights; shared three-field CHARS option | Ordinary editors default-on; extended shrubs independently opt-in; remaining calculations/effective source defaults and full restoration incomplete |
| Guarded vegetation lifecycle | Reviewed full-row deletion/reserved IDs; creation with explicit species/numeric/NULL decisions; independent personal definitions | Opt-in adaptations; no guessed defaults, invented U/X membership or hidden VUser write |
| Standards checking | Whole-project/selected-WU reviewed species-code replacements; exact source membership and all physical rows | Opt-in; broader standards/environment compliance remains open |
| Project metadata | Review/all-field drafts, blank/master-template creation and dedicated typed provenance restoration | Opt-in, distinct project/master/user storage; generic metadata restore unavailable |
| Saved User log | Reviewed/native-verified and full-race-integrated read-only four-column VPro64 inspection, NULL-only Current/All, physical duplicates and paging | Independent default-off frontend/backend gates; not session recording, editing/deletion or live-user presence |
| Profiles and navigation | Ordered preview, rule edit/create/delete, explicit result navigation, independent selection/writer ownership, usable blank files/tables and reviewed Save as SU | Opt-in; arbitrary-file administration, broad history restoration and some criterion semantics open |
| Find/Enter | Source-bound explicit Enter navigation and exact scoped Find with safe Open lifecycle | Opt-in adaptations; no Access Find-dialog emulation |
| Env Into SU / SU Into Env | Independent reviews of all matching selected-SU plots; owned-project file only; atomic typed provenance, drift/trigger rollback/retry and history replay guards | Independently opt-in; separate-file SU and personal-definition writes unavailable |
| Long Environment | Owned native preview: exact72 source rows, selected-SU membership, transposed per-plot values and explicit orphan/name diagnostics; separately gated saved-title preferences and reviewed no-replace XLSX publication | Independently opt-in; external SU reads supported; preparation/input never publish or save automatically; workbook Wails/full-race verified and sealed; Excel automation unavailable |
| Summary Environment | Independently gated normal-SU/layer-cover read-only preview:39 source fields, saved method/scope initialization, separately gated reviewed preference Save, Mean/inclusive quartiles and physical provenance; separate default-off reviewed XLSX publication. Separately gated lifeform preview projects49 fields with14 source captions/cover strings | Layer workbook [181-file successor sealed](archives/summary-environment-workbook-desktop-checkpoint/evidence-manifest.json). [233-file lifeform/decimal/private-species successor sealed](archives/summary-lifeform-desktop-checkpoint/evidence-manifest.json):638 frontend/check/builds, coupled race79.117s, fresh5 corrected native cases/no data/config writes, review and root full race1605.911s; archive-discovery issue fixed/verified, full retry passes with cached unchanged727 inputs. Initial9 native cases preserved separately. [Default-off public species preview successor](archives/summary-species-desktop-checkpoint/evidence-manifest.json) passes642 frontend/check/builds, independent review,57 native cases/six closed owners/no writes and fresh full race1445.861s/all packages against732 unchanged identities. Hierarchy and lifeform/species workbook unavailable; not full Access report parity |
| Summary Vegetation | Default-off owned normal-SU/layer preview supports1/2/3/6 and class4/5 historical1/Class10/Value20 drafts. Strata/Lifeform/None grouped drafts support1/3/4/5/6, plus2 for None only; source raw builders and typed DAO group/Order metadata remain explicit. Independently default-off saved options use explicit Load/Save/Undo and the same controls | Class/grouped/additional full integrations pass; additional3956.989s/1373 unchanged inputs/discovery verifies304 native files/39 reviewed inputs/76 branches. Additional modes preserve36 original outcomes/four explicit refusals and50 production wires. Saved options pass private and exact desktop/focused reviews, materialized coupled race36.091s/vet/production, strict-Go wire, warning-free interface bindings and1364 frontend tests/check/private build. Five normally closed Wails owners prove five once-only preference commits, independent gates, historical-limit retention and lifecycle/recovery;291 files/31 reviewed inputs seal with originals restored. Saved-option complete-source integration passes4138.438s/1382 unchanged inputs/discovery and independently verifies sealed native hashes/restoration. No asset/default promotion. Strata/Lifeform Display2 source failures, other grouping/report-unit branches, workbook and printing unavailable. [Contract](docs/LONG_VEGETATION_WORKBOOK_CONTRACT.md) |
| Standalone Lifeform Summary | Independently gated normal-SU read-only preview with physical membership/species-definition weights, source SINGLE assignments, presence, nullable mean cover and species counts | Eleven native cases/four closed owners and full race1122.770s/all packages pass. Distinct from unavailable Long Vegetation lifeform and SiteUnitDetailReport MyCover modes |
| Combined Lifeform workbook | Independently gated owned review/no-replace XLSX publication; both accepted planners, six always-rendered attribute totals and six explicit detail choices |617 frontend tests/check0/0, independent code review, four closed Wails owners and full race1692.501s against703 unchanged source identities pass. [133-file successor](archives/lifeform-workbook-desktop-checkpoint/evidence-manifest.json); hierarchy and broader reports remain unavailable |
| Long Vegetation preparation | Pure Go selected-SU MAX-per-plot/species reduction and original LayerCode-driven layer observations; exact typed memberships/source-row provenance | Calculation input, not a standalone report; quality filtering, strata/lifeform modes and export unavailable |
| Long Vegetation crosstab | ACE-probed plot-column presence, duplicate-weighted sums/observation averages and strict thresholds/constant-list bypass | Pure calculation kernel for already joined/grouped inputs; reference/quality/grouping integration and owned report UI remain unavailable |
| Long Vegetation preview | Owned cancellable native read-only preview: responsive default-off UI, selected-SU/external paths, physical joins, strict YAML, names/quality, independently gated None/Lifeform/typed Strata and supplementary Code | Group modes and Code Wails/full-race accepted; selected attributes, Access Val calibration and wider/lumped report modes unavailable |
| Long Vegetation XLSX | Separately gated explicit unlumped review/no-replace publication, source unit sheets and ReportSummary, typed metadata and quality-filtered membership weights | Twelve Wails cases/four normally closed owners,580 frontend/check/builds, separate kernel/owned/desktop reviews and full race1508.347s/all packages pass. [Exact101-file seal](archives/vegetation-workbook-desktop-checkpoint/evidence-manifest.json). Source4/lumping/combined variants/empty pivots unavailable. [Contract](docs/LONG_VEGETATION_WORKBOOK_CONTRACT.md) |
| Long Vegetation title preferences | Independently default-off explicit owned title Load/Save/Undo, literal YAML CAS and retained validation/irreversible receipts; stale workbook approvals refuse publication | Reviewed and bounded native-accepted:16 cases/four normally closed owners, actual fresh XLSX and historical NUL/CRLF/shared Environment preservation;306 sealed files/all16 restored/22 protected identities. Race30.483s,47 client/1126 full frontend cases/check/private builds and production/vet pass; complete-source1138-input full race passes exit0 in3641.968s with unchanged inputs/bindings/discovery. Not original AfterUpdate/registry parity. [Boundary](docs/LONG_VEGETATION_WORKBOOK_CONTRACT.md#saved-title-preference-boundary) |
| Long Vegetation Average/Order preferences | Independent default-off explicit owned Load/Save/Undo for source10/20 choices; strict two-key YAML CAS and shared retained draft/receipt lifecycle, without new arithmetic or unrelated options | Reviewed and bounded native-accepted:20 cases/four closed owners, all four combinations, retained draft/receipt/lifecycle behavior, historical/shared title preservation and once-only exact-SHA XLSX;339 sealed files/all16 restored/22 protected identities. Coupled race421.149s/final statistics race133.783s,57 client/1136 frontend cases/check/private builds/production/vet and exact three-file interface binding delta pass. Complete-source1146-input full race passes exit0 in3525.956s with unchanged inputs/bindings/discovery. [Boundary](docs/LONG_VEGETATION_WORKBOOK_CONTRACT.md#saved-averageorder-preference-boundary) |
| Long Vegetation report switch preferences | Independent default-off owned Load/Save/Undo for Screen report, Space between groups, Create report summary and Use constant species list; strict four-key YAML CAS preserving unchanged historical -1/0 storage | Reviewed and bounded native-accepted:24 cases/four closed owners, six once-only commits, historical invalid/correction/no-op/omission, retained drafts/receipts/lifecycle and fresh exact-SHA XLSX;378 sealed files/all16 originals/22 protected identities restore. Coupled Go race282.577s,67 client/1146 full frontend/check0/0/private builds/production/vet and exact three-file interface binding delta pass. Complete-source1155-input full race passes exit0 in3728.741s with unchanged inputs/bindings/discovery; not complete report/printing parity. [Boundary](docs/LONG_VEGETATION_WORKBOOK_CONTRACT.md#saved-report-switch-preference-boundary) |
| Long Vegetation threshold preferences | Independent default-off owned Load/Save/Undo for the two source percent thresholds; strict finite-number YAML CAS, raw drafts and unchanged integer assignments, without new bounds/arithmetic | Reviewed and bounded native-accepted:21 cases/four closed owners, five once-only commits, independent filtering/provenance, historical precision refusal/correction/no-op/omission, retained lifecycle and fresh exact-SHA XLSX;325 sealed files/all16 originals/22 protected identities restore. Focused Go race33.176s/coupled261.813s,76 client/1155 full frontend/check0/0/private builds/production/vet and exact three-file interface binding delta pass. Complete-source1164-input full race passes exit0 in3701.860s; frozen inputs and ordinary/production discovery remain unchanged. [Boundary](docs/LONG_VEGETATION_WORKBOOK_CONTRACT.md#saved-threshold-preference-boundary) |
| Long Vegetation grouping preferences | Independent default-off owned Load/Save/Undo for source Layer/Strata/Lifeform/None; strict one-key YAML CAS separates saved validity from backend/client availability, preserving unchanged unavailable no-ops; source-correct numeric Lifeform workbook groups | Reviewed/bounded native-accepted:23 cases/five closed owners/five once-only commits, exact labels/three widths/retained lifecycle/all four modes/unavailable historical no-op, zero-output stale refusal and one independently reopened exact-SHA numeric-Lifeform XLSX;430 files sealed/all16 originals/22 protected identities restore. Focused4.777s/integer-cell3.127s/coupled507.713s,85 client/1164 frontend/check0/0/private builds/production/vet/exact bindings pass; initial failure and harness continuations retained without replay. Complete-source1174-input full race passes exit0 in3658.827s; frozen inputs and ordinary/production discovery remain unchanged. [Boundary](docs/LONG_VEGETATION_WORKBOOK_CONTRACT.md#saved-grouping-preference-boundary) |
| Long Vegetation quality Include Nulls preferences | Independently default-off owned three-key Load/Save/Undo; quality-specific bool/-1/0/exact legacy True/False reader and omission of unchanged assignments; editing does not enable enforcement | Reviewed/bounded native-accepted:21 cases/four closed owners/five once-only commits, three-width labels/retained lifecycle, historical mixed storage/no-op and active/inactive qualification; stale zero-output refusal and one reopened exact-SHA XLSX.338 files sealed/all16 originals/22 protected identities restore; no native fixes/continuations. Focused4.947s/coupled218.679s race,84 client/1173 frontend/check0/0/private builds/production/vet/exact bindings pass. Complete-source1183-input full race passes exit0 in4201.292s; frozen inputs/ordinary/production discovery and all338 sealed hashes verify, with separate final acceptance. [Boundary](docs/LONG_VEGETATION_WORKBOOK_CONTRACT.md#saved-quality-include-nulls-boundary) |
| Long Vegetation quality minimum preferences | Independently default-off literal three-key YAML CAS, owned optional PlotQualitySite suggestions and retained Load/Save/Undo with root/sibling/workbook holds; existing DataQuality qualification unchanged | Reviewed/bounded native-accepted:25 cases/five closed accepted owners/six once-only commits/two reopened exact-SHA XLSX outputs; labels/three widths/retained lifecycle/unrestricted literals/physical duplicate-NULL-empty suggestions/malformed-source retry/historical omission and independent active qualification.184 files sealed/all16 originals/22 protected identities restore; wrong-gate zero-mutation build is separately archived. Race254.947s/independent6.702s,93 client/1182 frontend/check0/0/private builds/production/vet/exact bindings pass. Complete-source1191-input full race passes exit0 in4350.937s; frozen inputs/discovery and sealed hashes verify with separate final acceptance. Automatic Access Change timing/default promotion remain excluded. [Boundary](docs/LONG_VEGETATION_WORKBOOK_CONTRACT.md#saved-quality-minimum-boundary) |
| Summary Vegetation quality preferences | Independently default-off reviewed/bounded native-accepted: four literal minima/four Include Nulls flags, shared BEC key, owned typed suggestions and explicit Load/Save/Undo; enforcement read-only | Exact23-file desktop review finds no significant issues; focused race3.056s/vet/production,40 coupled client cases and1377 frontend tests/check0/0/private build pass. Five normally closed Wails owners/four once-only YAML commits prove independent gates, three-width labels, retained receipts/invalid Undo/read cancellation/failed reload recovery and bidirectional peer/root holds with both Summary sessions enabled.313 native files/31 reviewed inputs seal; fixtures/configuration/protected assets restore. New opt-in refuses enforced unqualified preview client/backend; existing feature-off behavior unchanged. Original BEC14 predicate/QC16 join cases independently reopen readonly; not desktop qualification proof. Complete-source1394-input race passes exit0 in3966.657s; inputs/discovery/native hashes/restored identities verify with separate final acceptance. No default/asset promotion. [Boundary](docs/LONG_VEGETATION_WORKBOOK_CONTRACT.md#remaining-summary-quality-and-naming-source-boundary) |
| Summary Vegetation stored presentation drafts | Independently default-off reviewed/bounded native-accepted stored title/seven flags with source constancy-on group3/display1/common0 plan and historical limit retention; explicit Load/Save/Undo and retained full callback receipts; hierarchy header read-only | Exact20-file desktop review finds no significant issues. Focused race9.865s/vet/production,46 coupled client cases and1392 frontend tests/check0/0/private build pass. Five normally closed Wails owners/four once-only commits prove independent gates, three-session root/peer holds, known/unknown callback receipts, cancellation/remount/read recovery, actual hierarchy RPC refusal and same-final constancy on/off coupled commit.301 native files/29 reviewed inputs seal; fixtures/config/protected assets restore. Raw crosstab preview does not consume presentation drafts; no guide renames, field-list editing, hierarchy/constancy report output, printing or workbooks enabled. Complete-source1404-input Windows race passes with unchanged inputs/discovery; final native/review/restoration acceptance is sealed. [Boundary](docs/LONG_VEGETATION_WORKBOOK_CONTRACT.md#stored-summary-presentation-draft-boundary) |
| Summary environmental field-list selections | Independently default-off reviewed/bounded native-accepted Report-only reader/planner/transaction/retry receipt/service and retained responsive locked-Label pane; no environmental report evaluation/output | Exact backend/private client/pane/wiring/materialized and bootstrap-recovery reviews pass.67 focused+1459 full frontend cases/check0/0 and isolated enabled/default builds preserve17 accepted assets. Five normally closed actual-main owners/five once-only commits prove independent gates, explicit initial-read dismissal, layouts/duplicate identity/hidden-NULL bulk rows, retry/unknown correction receipts, four-session holds and cancellation/remount/read recovery.352 files/39 reviewed inputs seal with restored data/config/protected assets/no locks; earlier300-file failed owner archive remains rejected. IncludeEnv draft gates mutation, not formatted-report consumption. Complete-source1428-input race passes exit0 in4785.777927s with unchanged inputs/discovery and final native/restoration association. [Boundary](docs/LONG_VEGETATION_WORKBOOK_CONTRACT.md#stored-summary-presentation-draft-boundary) |
| Summary environmental inspection | Independently default-off reviewed/bounded native-accepted readonly inspector, fixed Layer/By n Plots/% Cover unit frame, physical count, sixteen direct TEXT categories, SMR/BGC/integer Aspect and Bedrock1/2/3 plus Disturbance1/2 in one owned snapshot | Original inspector full integration passes1452 unchanged inputs/discovery in3933.400s. Expanded exact23-path implementation passes actual-source race/vet/production, coupled Summary tests/check/private build and independent review. Five normally closed Wails owners prove derived grouping/NULL distinctions, literal TEXT FieldNumber arguments, whole-result refusal/correction, exact trailing spaces, hidden selection, three-width containment and retained cancellation/native-close/peer/navigation holds.315 files seal;16 originals/protected hashes restore with zero app commits. Expanded integration remains pending. Saved IncludeEnv/controls are not consumed; unsupported/noncanonical arguments refuse all output. Numeric/cover/lifeform functions, source collation and formatted reports/workbooks/printing remain pending. [Boundary](docs/LONG_VEGETATION_WORKBOOK_CONTRACT.md#environmental-plot-count-prerequisite) |
| Long Vegetation Show preferences | Independently default-off reviewed/bounded native-accepted: three source choices, atomic integer/string YAML CAS and source-coupled selected-field clear; retained Load/Save/Undo and symmetric report/sibling/workbook holds | Naming/independent physical-source/reopened-workbook race106.301s; coupled race455.825s,102 coupled/1191 full frontend cases, check0/0/private builds/production/vet/exact bindings pass. Independent119-file-predecessor review finds no significant issues. Seven closed accepted Wails owners prove27 cases/seven once-only commits/one reopened exact-SHA workbook;225 files sealed/all16 originals/22 protected identities restore. Native covers independent naming/backend/client Code gates, source captions/three widths/lifecycle/receipts and historical mode4/unavailable Code no-op/correction, absent-field refusal and changed-back field-only clear. Complete-source1200-input Windows race90m passes exit0 in4399.773s; frozen inputs/discovery and225 sealed hashes verify with separate final acceptance. Custom-field editing/automatic Access timing/printing/default promotion remain excluded. [Boundary](docs/LONG_VEGETATION_WORKBOOK_CONTRACT.md#saved-show-naming-boundary) |
| Google Earth location preparation | Owned raw multi-plot review, separately gated KML text preparation/no-replace export, and separately gated explicit saved preferences | Export accepted with full race920.120s. Preferences accepted with full race916.340s,412 frontend tests/check0/0, isolated builds, independent review and21 native cases. All gates remain independently default-off; remote icons, launch and single-plot unavailable |
| SIVI / FS1333 parent review/editing | Source-labelled77-field panel, bounded ASCII join review and Env/Master preference; fourteen direct editors, sixty shared controls, two source actions and existing ProjectID/metadata adaptations with typed restoration | Independent gates default off. Lost source-preference authority blocks new drafts, profile mutations and owner disposal until explicit observation:636 frontend/check/builds, two closed native owners, independent reviews and race1524.592s pass. Initial five postcommit cases and four corrected pre-backend surface cases remain separate. Shared-field native acceptance is preserved; explicit audited desktop Save/reload is not literal Access callback timing. [Private child-cover storage](docs/FS1333-parent-access-contract.md#private-child-cover-planning) and the [desktop successor](docs/FS1333-parent-access-contract.md#gated-child-cover-desktop-successor) remain separately sealed. Full Unicode collation, new-ID/NotInList and complete form entrypoints remain unavailable |
| Long Vegetation enforcement preferences | Independently default-off owned paired enforcement/unit-group1-reset CAS and retained Load/Save/Undo, preserving changed-away/back provenance and unrelated drafts; Windows installed/default printer observation is read-only and separate from persistence | Lifecycle/wire race5.270s includes actual read-only Windows API; qualification/workbook race186.295s,122 coupled/1211 full frontend cases, check0/0/private builds/production/vet pass. Exact35-path independent review finds no significant issues. Five normally closed Wails owners prove15 cases/three once-only commits/one historical no-op/one exact-SHA reopened qualified XLSX;123 files sealed/all16 originals/22 protected identities restore. Native verifies independent gates, three widths, historical integer omission/group3 reset, retained sibling drafts, warning/unknown receipts, symmetric holds, Cancel/remount/retry and active/off reports. Full integration passes exit0/root4765.021s with1228 unchanged inputs/fourteen-package ordinary and production discovery; all123 native hashes verify with separate acceptance association. No printer setter, printing, unit-group2/3 consumer or full Form_Load timing parity |
| Long Vegetation None report units | Default-off selected-SU source3 and Unit Groups editor, distinct from None species grouping; explicit Env/current-list join witnesses retain original physical SU IDs and quality occurrence indices | Source DAO calibration, broader report/quality/workbook race497.389s, source/CAS/workbook race86.878s,1222 full frontend cases/check/private builds/production/vet pass. Exact31-path core and33-path preference reviews find no significant issues. Seven normally closed actual-main Wails owners prove17 cases/four once-only commits/two exact-byte historical no-ops/one independently reopened exact-SHA All plots workbook, including independent gates, lifecycle/receipts/symmetric holds, source1 recovery, QCLV provenance and enforcement-reset stale-CAS refusal. All16 originals/22 protected identities restore;149 files seal/no owner remains. Complete-source1249-input integration passes exit0/root4919.559s with unchanged inputs/discovery/native hashes. Source2 selected fields, all-project fallback and broader duplicate-parent quality joins remain unavailable. [Boundary](docs/LONG_VEGETATION_WORKBOOK_CONTRACT.md#none-report-unit-core-boundary) |
| Long Vegetation selected-field report units | Independently default-off selected-SU selector/Build/CAS/report and qualified workbook native/integration-verified | Ordered no-separator Env/Admin fields retain original nullable SU/parent tuples and quality plot-existence rather than fanout weighting. New empty/NUL/overlength destination labels refuse rather than Access's silent omission/truncation. Build is read-only, hashes complete physical rows/settings/context/order, and does not save YAML or source scratch. Selector/recovery/root-consumer/provenance reviews report no significant issues;1230 frontend cases/check/private builds and focused race30.224s/production/vet pass. Seven normally closed native owners prove13 cases/four once-only commits/one historical2 exact-byte no-op/one once-only UI workbook reopened to exact reviewed SHA;236-file seal restores16 originals/22 protected identities. Independent gates, real proxy/cancellation/remount/close/sibling/layout, physical joins/QCLV existence and enforcement-reset/stale-CAS/source1 recovery are verified. Complete-source1274-input Windows race90m passes exit0 in4757.152s with unchanged inputs/discovery;236 sealed files verify with separate final association. Empty derived-unit workbook output still refuses; historical saved2 without session Build stays unavailable. Broader all-project/combined/lumped/printing/replacement acceptance remains incomplete. [Contract](docs/LONG_VEGETATION_WORKBOOK_CONTRACT.md) |
| Long Vegetation whole-project report scope | Independently default-off owned original project scope/Build/CAS, retained workspace/panels, distinct preview/XLSX/summary/publication and coherent report/root UI independently reviewed and bounded native-accepted | Source2 preserves original Env/Admin tuples; source3 preserves original Env rows, never synthetic SU IDs. LongVegReport uses TempReportVeg/MAX even without an SU. QCLV requires actual originalSU and prunes by existence, not denominator weighting. Derived-project summary D:E visibly records a desktop adaptation, not literal current-SU Access parity. Scope/workbook services share one immutable default-off `VPRO_LONG_VEGETATION_PROJECT_SCOPE` flag plus the independent workbook gate; actual-source nullable interface bindings match61 services/324 methods/341 models. Retained workspace review resolves four lifecycle findings; exact22-path root review/focused ordinary holds/background reads/unavailable close/getter Cancel follow-ups report no significant issues. All1321 frontend cases/check0/0/private actual-main builds pass. Eight normally closed accepted native owners prove23 cases/four once-only Unit commits/four once-only independently reopened exact-SHA workbooks: original52-row joins, actual None/Env-only/saved3, original-SU QCLV existence, three-width controls and actual scope/getter/remount/unknown Save/warning publication/root-close/sibling safety. All16 originals/22 protected identities restore;289 files seal, with failed predecessor/runner evidence excluded and closed builds hashed. Fresh corrected1333-input Windows race passes exit0 in4069.67047s with unchanged inputs/package/embed discovery;289 sealed files and22 reviewed inputs reverify with separately associated final acceptance. Legacy consumers still refuse project provenance. Combined/lumped/empty-unit output/printing and whole replacement acceptance remain incomplete; no default availability is claimed. [Boundary](docs/LONG_VEGETATION_WORKBOOK_CONTRACT.md#whole-project-projection-boundary) |
| Other forms/reports/interchange | Navigation inventory and original resources provide precedents | FS1333/SIVI/two-page/CHARS equivalence, usable client-priority reports/locations and complete import/export not delivered |

Two-page FS882 variants now have [source-exact readers and a gated
additional-field editor](docs/FS882-two-page-access-contract.md):
nine normal/eight CHARS fields, unchanged-history
omission, atomic data/audits and separate typed restoration histories. Focused
race tests cover ownership, cancellation/retry, collision/rollback and restoration
aliases/replay and strict typed transport. The independently gated Wails service
and mounted source-labelled editor remain default-off.670 frontend tests/check0/0,
isolated production builds and five normally closed native owners verify the
bounded nine/eight-field workflow, audited Save/prune, remount history,
independent denial and read cancellation/retry. A directly reviewed common-input
exclusion repair adds two current guard-only native owners without replaying
mutations:20 total cases/seven closed owners,783 frozen source identities.
The [1012-file additional-field seal](archives/two-page-extra-desktop-checkpoint/evidence-manifest.json)
preserves that accepted boundary (manifest SHA256
`df8efcd0362ffc965ff5ec047510561dd5a402e0f38427fcf19d697594fac4cf`).
Its full race integration passes root2091.731s/all packages. The separately gated
13 normal/14 CHARS common-field successor now has registered strict Wails
transport, durable draft/history ownership and source-labelled controls.
PlotType uses the bound TEXT10/list workflow, not the SIVI action mapping.
685 frontend tests/check0/0, isolated enabled/default production builds and
27 native cases/seven closed owners pass; exactly one literal PlotType Save and
typed prune restore all16 fixture/22 protected identities. Native layout checks
measure3/2/1 columns at1400/800/600px and1 at320px, with related Stand controls
kept together. Fresh all-package race passes root1853.589s; Go is unchanged
across the frontend-only layout repair.792 source identities and embedded
assets are frozen in that accepted checkpoint. The [1052-file common-field seal](archives/two-page-common-desktop-checkpoint/evidence-manifest.json)
is independently verified; complete two-page
presentation of the existing96/98 XL fields, source events/children and pictures
remain unfinished.
The normal source's duplicate
age binding is preserved rather than silently replaced with a height binding.

Two-page linked soil editing is still unavailable. The
[private Humus/Mineral prerequisites](docs/FS882-two-page-access-contract.md#linked-soil-physical-read-prerequisite)
now include a reviewed complete tagged physical reader and existing-row planner
for all30 visible soil data bindings. The separately contracted atomic
stable-request writer passes coupled Go race/vet/production checks and independent
review. Independently default-off review/writing services and actual-main
interface bindings are also reviewed. Typed client drafts and the shared
strict-Go wire pass1009 frontend tests, check/build and focused validation.
Independent review's numeric-peer collision finding is closed with shared
Go/client witnesses. These private boundaries do not enable the child host, replace
ordinary editor guards or establish source ordering/focus and whole-form parity.
Retained mutation sessions and complete typed receipt validation are reviewed;
unknown requests cannot be discarded or replayed on missing history.
The independently gated persistent registry and aggregate lifecycle activity are
also reviewed;1029 frontend tests/check and isolated build pass. Experimental
linked-soil panels and parent/root ownership barriers pass1041 frontend tests,
check/private builds and independent review. Actual Wails lifecycle acceptance
remains outstanding. Exact numeric/NULL depth direction and existing-record
Enter are implemented as scoped explicit-Save adaptations;1047 frontend cases,
check/private builds and independent corrected review pass. Native proof remains
pending.
The experimental complete-entry soil profile now separates physical review
from legacy numeric-ID soil readers: ordinary soil rows/actions/counts explicitly
remain unavailable rather than blocking historical NULL-ID rows or reporting
zero layers. Both soil and complete-entry review flags are required; default
legacy behavior and unrelated read errors are preserved. This frontend-only
adaptation passes1051 frontend tests/check/private builds and independent review.
Two normally closed disposable Wails read owners prove preserved default
refusal and enabled all30-field/NULL-ID read-only reachability, with owned and
protected identities unchanged. Responsive layout and mutation/lifecycle
acceptance remain outstanding; no default editing or whole-form grant follows.
Further native layout/nonchunked mutation/recovery cases pass, but a large
Mineral request exposed a self-locking final SQLite scope read. Both failed
requests were certified no-commit and all disposable owners closed/restored.
A transaction-local scope correction passes focused race tests; external SU
WAL membership stays unavailable without through-commit read reservations.
Independent exact-five-file review reports no significant issues; shared
journal-mode/scope race checks and production/vet pass. Fresh690-input full
integration passes with unchanged inputs/discovery. A distinct corrected Wails
candidate passes one actual chunked Mineral Memo commit with one source audit
and complete typed history, held-response cancellation/native-close refusal,
read-only recovery without replay and retained receipt acknowledgement. The
fifth owner closes normally; the committed DB is retained and all16 identities/
protected originals restored. This accepts the bounded existing-row soil host,
not default editing, source combo lists, create/delete/restore or whole-form parity.
The next source-choice candidate implements independently default-off readonly
references, strict tagged Go/client transport and individually distinguishable
choice/metadata disclosures. Choosing observes the current physical row/form/
definition before existing draft validation, with no implicit Save or new
membership/coercion rules. Independent review,1058 frontend tests/check/private
builds and focused Go/production/vet pass. The695-input full race passes with
unchanged inputs/discovery. A sixth actual-main Wails owner accepts six normal/
CHARS layouts at1400/600/320, duplicate NULL/empty/BOOLEAN source metadata,
typed staging, invalid Save/disposal/native-close guards and correction/Undo.
Held-read cancellation/actual parent remount refuses stale definitions; malformed
fixture UTF8 fails completely and correction/retry succeeds. One planned
two-field choice Save commits exact TEXT/INTEGER cells, two strength3 source
audits and complete durable history; read-only recovery and acknowledged
receipt remount prove zero replay. Normal close restores all16 identities and
protected sources; committed evidence is retained/sealed with a fresh profile,
leaving preceding sealed files unchanged. Mixed-storage calibration,
create/delete/restore and whole-form/default editing remain unavailable.
Server-owned attempt sealing now provides reviewed no-commit certification
without inferring it from cancelled promises or absent history; late matching
UUIDs cannot mutate after certification. Coupled soil/SQLite races and1033
frontend tests/check/isolated build pass; the frozen687-input full integration
race passes with unchanged inputs and package/embed discovery. This freeze
does not cover the newly built candidate UI's native acceptance.

The current default-off complete-source presentation in
[twoPageEntryProjection.ts](frontend/src/twoPageEntryProjection.ts) accounts for
all118 normal/120 CHARS logical bindings:96/98 XL,13/14 Common and9/8 Extra.
It retains three PlotNumber source instances under one header owner, exact
Site/Veg and Soil/Terrain placement and all six PlotNumber child links.
[SourcePage](frontend/src/SourcePage.svelte) now accepts the exact projection;
bound Est/Meas groups retain typed editor slots without duplicate source buttons.
All source controls have semantic groups in every coordinate mode, and supplied
source captions/binding aliases cannot inherit misleading XL control-name labels.
[TwoPageEntryLayout](frontend/src/TwoPageEntryLayout.svelte) composes the header,
source pages and disjoint scoped editor slots, rejects wrong-scope/duplicate
owners and dispatches the five supported child links without dispatching pictures.
Explicit-source readonly BOOLEAN controls normalize Access true=-1 and disclose
historical invalid storage without repairing it.
The independently default-off `VITE_TWO_PAGE_PARENT_SOURCE_LAYOUT` now mounts
this layout in the real Common/Extra reviews using their original input snippets,
strict original storage, callbacks, draft/history caches and independent write
gates. Only the current scope is editable; other parent fields and five linked
child tables are read-only. The ordinary XL host is hidden without unmounting and
returns intact when the review closes. Pictures and unsupported source actions
stay unavailable.
697 frontend tests/check0/0, coupled two-page Go race20.167s and isolated
enabled/default production Wails builds pass. Fifteen zero-write native cases/two
normally closed owners verify both variants/scopes at1400/600/320px, source labels,
unique IDs/header, readonly child dispatch, ordinary-host restoration and default
denial. All16 fixture/22 protected identities remain unchanged; no sealed Save/
prune is replayed. Live XL editing inside the source layout, remaining source
events and pictures remain unfinished. This is not whole-form/forms-reports
completion or default promotion; the792-source seal remains a predecessor.
The current [890-file source-layout checkpoint](archives/two-page-entry-layout-desktop-checkpoint/evidence-manifest.json)
preserves794 application-source identities, exact enabled/default embedded
builds and all fifteen closed zero-write native cases. Every archived file was
verified; backend/bindings match the accepted Common pins. Its fresh Go race is
the focused two-page suite; the predecessor's full race is not a new run.

The next source-layout preparation reuses [HeaderEditor](frontend/src/HeaderEditor.svelte)
and its seven existing typed field families through labelled input slots.
Exact source identities guard field ownership, including unbound coordinate
parts/options in DD/DM/DMS; callers cannot override the source lock flags.
Both variants/all three modes compose in tests without duplicate labels or a
second PlotNumber owner.701 frontend tests/check0/0 and isolated frontend/Wails
builds pass. Five fresh zero-write native cases verify the default editor at
1400/600/320px, rejected Save, invalid-draft tab remount and correction/Undo.
All16 fixture/22 protected identities remain unchanged. Fresh all-package Go race
passes (root2041.922s) against the pinned794-source candidate. This XL composition
is preparation, not a mounted complete-entry workflow; source-specific working
unit actions, other XL fields and combined entry lifecycle remain unfinished.
The verified [860-file header preparation checkpoint](archives/two-page-header-composition-checkpoint/evidence-manifest.json)
preserves this exact candidate; neither source variant is promoted by the seal.

Private complete-entry physical planning now covers95 normal/97 CHARS XL page
fields and combines them with the accepted Common/Extra planners into one
117/119-assignment plan, excluding PlotNumber. It reuses accepted text/numeric
bounds, preserves unchanged historical storage by omission, requires new
BOOLEAN storage to be explicit0/-1, and rejects wrong owners, duplicates,
foreign-scope tails and unauthorized Master changes without partial plans.
Focused race tests pass3.057s; the preceding XL/shared-policy coupled suite
passes78.570s. A private complete-entry writer now applies that plan through one
shared audited transaction and separate normal/CHARS Entry history; typed
restoration reuses the same exact source ownership. Master editing intent is
intersected with source-user authority from the immutable owned context snapshot,
including restoration. Caller flags and approval callbacks cannot grant that
authority. Independent transaction-held reference/role approval is mandatory.
Final atomic/authority race passes11.020s; shared writer/restorer regression race
passes160.862s, isolated production compilation and fresh all-package race
(root2172.776s) pass. Independent bounded read-only review found no substantive
issues.800 application-source identities and17 exact embedded assets are pinned;
all16 closed fixture/22 protected identities remain unchanged. There is still no
registered service, reference-membership approval implementation, bindings or UI
write grant and no new native actions. The2041.922s full race/native
header seal predates these six new backend files and shared authorization hooks.
The [838-file private atomic-entry checkpoint](archives/two-page-entry-atomic-checkpoint/evidence-manifest.json)
preserves the reviewed backend successor without promoting a complete form.
Two later private files now check mandatory membership for the exact six
two-page LimitToList fields without changing SIVI flags. Focused race2.958s,
coupled87.660s and independent bounded review pass, including SQL-backed atomic
rollback/retry in both variants. This inner helper is not the missing full
reference approval/provider integration or a UI grant; the800-source seal
predates it.
Concrete mandatory readers now attach the configured VLists family read-only
on the same owned writer connection and reuse borrowed catalogue readers.
Immutable request prepare/verify hooks avoid nested context leases and retain
Master authority. Coupled race102.667s, production compilation and independent
bounded review pass, including local/external source writes/restoration, closed
catalogue denial versus NULL clearing, late verification rollback and retry.
This later four-file/reference lifecycle successor is not yet full-integrated
or sealed; optional-code acknowledgement and full service/UI wiring remain open.
Private optional acknowledgement policies now bind explicit warnings to exact
original/draft/reference identities for all56/55 source reference descriptors.
An independent review's count-only scope finding is fixed by exact binding/type
sets; CHARS cannot inherit absent BEC_Use. Final focused4.157s/coupled97.630s and
focused review pass, with real-catalogue atomic literal Save/restoration.
Private concrete catalogue providers now cover the shared/static fields,
Ecosection, plot quality, soil classification and current BEC selections.
Source NULL/literal All requests all SubZones; SiteSeries uses the current
Zone/SubZone pair rather than retaining stale choices. Topography's cc/cv/st
exclusions and numeric LocationAccuracy selectability are checked independently
without removing raw definitions. Focused race2.816s/coupled78.163s and isolated
production compilation and independent bounded review pass.
ProjectID and Master/Working Unit context-selection providers still fail
explicitly as unfinished in that static provider. Their separate private owned
read kernels now select physical project/master metadata and actual configured
Master/Env/SU units, validate transaction alias file ownership and retain literal
metadata/row identities. A known unselected SU has an explicit diagnostic.
Review found malformed SQLite TEXT could become nonselectable metadata and later
be repaired by JSON. Both Working Unit branches now reject it before projection;
valid correction/retry preserves literals. Post-fix focused6.513s/coupled145.329s
and isolated production compilation pass; focused review follow-up confirms the
finding resolved with no significant remaining issues.
Private fixed read composition now returns all56/55 reference fields with the
actual parent original, source modes and current draft BEC pair from one owned
snapshot. All2x3 preference modes pass for both forms; read failures/cancellation
return no partial snapshot and retry cleanly. Focused9.980s/coupled150.984s and
isolated production compilation and independent bounded composition review pass.
Private two-page ProjectID planning now reuses the accepted source-selection
planner with exact normal/CHARS control identities and reconstructed-original
equality. Physical duplicate/title provenance, signed64 IDs, literal/30 UTF16
bounds, historical noops and drift/cancellation pass focused race2.824s and
coupled116.292s, isolated production compilation and independent bounded review.
This is planning only, not mixed-entry history or a new assignment runtime grant.
Fixed private reference approval now combines mandatory membership and optional
acknowledgements with concrete owned readers and independently planned BEC filters.
Atomic writer rejection/retry and non-destructive restoration pass focused
race10.146s/coupled123.879s, production compilation and independent bounded review.
Direct ProjectID mutation without strict physical selection proof is denied;
Private owned writer composition now holds both source-preference selections
through commit and uses owned read-only VLists/internal/external SU aliases.
Changed family/master/Working references require rollback-journal read locks;
untouched historical noops do not inherit those mutation restrictions.
All2x3 modes/both forms/aliases, held preference/database locks, cancellation,
WAL rejection/correction, restoration and one-winner audit/history collisions
pass expanded coupled race328.001s, production compilation and independent review.
Private mixed ProjectID provenance/history is now composed with the owned writer.
Full typed EntryPlan is distinct from the audited Changes subset, so all0..3
audit strengths preserve unaudited values during restoration. Both source modes/
forms, metadata deletion/preference changes, user/file/selection drift and
order-independent mixed saves pass post-review focused47.028s/coupled360.878s
and production compilation. Independent review's edit-order finding is resolved;
the focused follow-up reports no significant issues.
The complete-entry backend facade is now registered behind independent,
default-off `VPRO_TWO_PAGE_PARENT_REVIEW` and
`VPRO_TWO_PAGE_PARENT_ENTRY_EDITING`. It exposes one owned original/reference
snapshot, explicit draft-reference reads, one atomic Save and typed restoration.
Raw Unicode and duplicate/unknown/missing/NULL properties are rejected before
publication; exact write-property names prevent case aliases from overwriting
validated inputs. Optional acknowledgements retain nullable metadata; ProjectID
requires its typed physical selection and Master authority uses the owned user.
Historical invalid BEC filters remain literal in the original, with explicit
unavailable diagnostics for dependent choices, not normalized fallback values.
Post-review coupled race427.895s and actual current-main production compilation
pass; the case-alias review finding is fixed and the focused follow-up is clean.
The separately default-off complete-entry panel now uses the actual Wails
bindings and one cached atomic draft for117 normal/119 CHARS editable source
fields. Frontend review requires `VITE_TWO_PAGE_PARENT_REVIEW` plus
`VITE_TWO_PAGE_PARENT_ENTRY_REVIEW`; editing additionally requires
`VITE_TWO_PAGE_PARENT_ENTRY_EDITING`. Existing per-scope permissions do not
authorize this writer. Source-derived policies reuse the accepted XL/Common/
Extra domains and measured UTF-16 bounds. Invalid drafts and unknown outcomes
retain Save/Lock/navigation/close barriers until explicit correction or Undo.

Current frontend validation passes32 focused tests,701 existing regression
tests and check0 errors/warnings; isolated enabled/default assets and actual-main
Wails production builds pass. Five normally closed native owners complete18
cases: both full cohorts at1400/600/320px with unique live labelled inputs and
hidden ordinary controls; one five-field mixed-scope atomic Save including
physical ProjectID selection and an acknowledged optional literal; typed audit
prune; history-preserving remount; independent review/edit/default denials; and
completed-response cancellation/retry. All16 owned/22 protected identities
restore. Native testing corrected Zone DISTINCT tuple identities/option keys and
the required ProjectID scalar-edit-plus-physical-proof transport. Earlier failed
bootstrap builds/evidence remain hash-archived, without replay or data changes.
Independent desktop/native review reports no significant issues. Fresh full Go
race integration passes root2091.501s/all packages. The measured successor has
835 application-source identities, immutably preserved in the independently
verified [1529-file successor](archives/two-page-entry-desktop-checkpoint/evidence-manifest.json).
Linked vegetation children stay read-only here. Their independently reviewed
private physical originals now preserve complete tagged rows, nonunique/NULL/
text IDs and overlapping source membership. Private cover/Collected drafts
cover25 source bindings with exact physical/context/expected witnesses,
unchanged historical omission and strict Unicode/duplicate-key transport.
Coupled races14.428s/15.955s and production/vet pass; source Species references/
NotInList, typed client lifecycle and source/native calibration
remain next. Existing SINGLE-range drafts are not native conversion proof.
Their private atomic stable-UUID writer and readonly immutable receipt recovery
are now independently reviewed as well. Complete source plans/audits/history
commit together; nonunique peers, cancellation/rollback/retry and postcommit
outcomes remain explicit. External SU observations are reserved through commit.
Exact REAL bits and canonical UUID payloads refuse silent signed-zero/affinity
normalization; new negative-zero values stay unavailable pending calibration.
Coupled race87.805s and production/vet pass. Whole integration and the
retained client lifecycle and native/source acceptance are still required.
Subsequent canonical control inspection found `<100 Or Is Null` on all21
numeric bindings. The private correction enforces the strict bound after
unchanged historical omission; no zero lower bound is invented. Expanded coupled
race103.270s, production compilation/vet and independent exact-three-file
follow-up pass with no significant issues or meaningful coverage gaps. Reviewer
execution was CGO-blocked, not another passing run. The705-input integration was stopped
before changing frozen inputs and is not accepted. Binary64 values just below100
are not proof of Access SINGLE conversion/validation timing.
Fresh corrected full integration passes exit0 with707 unchanged frozen inputs
and unchanged ordinary/production discovery. The subsequent private owned
[Species-reference reader](twopagevegetationspeciesreferences.go) reuses strict
physical definitions and literal parent scope, retaining duplicate/NULL/empty/
historical metadata. Coupled race47.367s covers both source families, schema
refusals, actual context rotation and in-flight cancellation/correction/retry;
final production/vet and independent exact-two-file review pass with no significant
issues or meaningful coverage gaps. It does not establish union
option ordering, source comparisons/UCase, NotInList decisions or Species writes.
The independently default-off [vegetation facade](twopagevegetationservice.go)
is now registered in actual main with interface-style Wails bindings.
`VPRO_TWO_PAGE_VEGETATION_REVIEW` permits owned physical originals, physical
Species references and read-only recovery; Save additionally requires
`VPRO_TWO_PAGE_VEGETATION_WRITING`. Its separate server-owned attempt ledger
seals unseen late requests, retains immutable exact UUID payloads/REAL bits,
distinguishes running/committed/original-context no-commit outcomes and leaves
rotated-context absence unknown. Cancellation cannot unseal a request or erase
an irreversible result; correction after certification requires a new UUID.
Coupled race154.269s, production/vet and independent exact-nine-file review pass
with no significant issues or coverage gaps. Exactly3 generated interface
binding files change;43 coupled client regressions, check0/0 and isolated build
13.09s pass. The private mounted vegetation editor and bounded native
acceptance are described below. These flags do not grant Species callbacks,
SINGLE parity or whole-form editing.
Strict [client originals](frontend/src/twoPageVegetationWire.ts) and
[typed drafts](frontend/src/twoPageVegetationEditor.ts) now cover all25 source
bindings, detaching actual Svelte proxies without losing NULL/empty, historical
values, nonunique identities or REAL signed-zero witnesses. Seven exhaustive
client cases, shared-Go/planner/soil-wire/compiler-fold race6.375s, check0/0 and
isolated default build5.04s pass. Identifier equivalence uses compiler-pinned
Unicode15.0.0 classes, exhaustively checked against target Go1.25.0 rather than
browser casing. A focused independent review reconciles the three initially
reported Greek/ligature pairs as nonaliases in this compiler, reruns checks and
reports no significant issues. This is outside the707-input integration freeze.
Strict [client receipts/outcomes](frontend/src/twoPageVegetationReceipt.ts) are
also independently reviewed: complete physical plans/peers, source membership
changes, readonly recovery and exact attempted bytes remain explicit. Shared
actual-Go writer receipts cover strengths0-3 and last-cover removal; production-Go
outcome models, historical BLOB, typed INTEGER/REAL transitions and signed-zero
witnesses pass14 client cases and focused race7.063s, check0/0 and private build
6.10s. A typed negative-to-positive zero change has no source audit at any
strength; it remains in complete history rather than being silently repaired.
Independent review identified that audit mismatch and repeated fixture output;
both are corrected and closed by focused follow-up.
The [retained session](frontend/src/twoPageVegetationSession.ts) and
[exact-owner registry](frontend/src/twoPageVegetationSessions.ts) are now
independently reviewed, with stable lossless requests, sealed-absence recovery,
generation guards and irreversible receipts. Their original batches pass59/39
coupled cases, check0/0 and private builds5.07s/5.32s.
The default-off [linked panel](frontend/src/TwoPageVegetationPanel.svelte) now
mounts all25 A/BC, CHARS A, C/D fields in responsive source groups. Explicit NULL,
readonly Species and complete physical peers remain distinct; the selected
physical row survives membership changes/remounts without choosing another.
The [existing A/CHARS literal Species list-selection service](twopagevegetationspecieslistservice.go)
and [retained desktop policy](frontend/src/twoPageVegetationSpeciesList.ts)
are registered behind independent default-off backend/client gates. They reuse
exact reference/row ownership and atomic source/audit/history plus recovery;
legacy Save remains unable to grant Species authority.
Coupled races85.006s, production compilation and both master/personal WAL/
reservation/retry checks5.709s pass; the review coverage finding is closed.
Independent desktop/binding review finds no significant issues; actual strict-Go
receipts,68 focused and1515 registered frontend cases, check0/0 and isolated
Wails builds pass. Disposable normal and CHARS owners prove exact once-only
Species+cover Save, two audits/one history, unchanged support/config files,
chunk-aware successful-response Cancel/recovery without replay and retained
receipt remount. Native zero-write checks prove full physical duplicate tuples,
NULL/empty metadata, three-width layout, retained coupled errors/correction,
common-cover-empty/NULL-ID guards and independent client/backend/C/D restrictions.
Complete1551-input race integration passes in5120.640562s with unchanged inputs/
discovery. Final association rehashes541 artifacts,20 canonical sources and34
reviewed inputs; freeze released. This does not establish whole Species callback
or whole-form replacement parity.
An additional isolated joint soil/Species profile changes no application source.
Native normal/CHARS zero-write acceptance proves all30 soil fields,
bidirectional retained draft barriers, owner-local correction/Undo, invalid
parent/native-close barriers and180 labelled three-width soil measurements.
All16 fixture files per owner remain unchanged; pictures/SubOther and unqualified
source callbacks remain separate.
Manual/alias/NotInList/personal creation, C/D selection and common-seven-cover-
empty warning/advance remain unavailable; no default promotion follows.
An observed Collected button only stages. Physical order and this explicit
button are adaptations, not source Species order/Current/Enter parity.
Current-owner correction stays live while other-owner drafts block new staging;
parent/root close/navigation/background reads honor retained activity. In the
enabled complete-entry profile, ordinary numeric readers/actions/totals are
explicitly unavailable instead of inventing IDs or zero totals. All8 compiled
flag combinations preserve the default reader path.
Final1102 registered frontend tests pass309+448+345, check0/0 and private
default/enabled builds5.19s/5.55s pass. Independent exact-baseline live review
reports no significant issues. A fresh1069-input Go/frontend/manifest/embed
freeze now passes full race exit0 in60m24s, with unchanged inputs and discovery.
Four normally closed actual-main owners verify default legacy refusal, all25
controls, six normal/CHARS layouts at1400/600/320, NULL-ID readonly peers and
native invalid/correction/Undo/close barriers. One physical-row Cover1 REAL5.25/
Collected TEXT C Save commits exactly two strength3 audits plus complete typed
history, preserving every peer/unmapped cell. Held-response cancellation,
readonly recovery, acknowledgement and immutable receipt remount are once-only.
The first enabled build used incorrect parent prerequisite flags; its zero-write
failed candidate/profile is retained. Correct flags build15.27s. Two harness
remount waits are corrected by resuming exact final checkpoints, never replaying
a completed Save. All16 fixture identities and protected/predecessor originals
restore;535 files and the committed database are sealed. This is bounded
existing-row cover/Collected proof, not whole-form parity.
A fifth normally closed owner proves exact C predispatch refusal and readonly
certified absence with zero writes, followed by one distinct new UUID that clears
only Cover6 on C-only physical row910404. Actual source membership excludes
that retained row; one source audit/full typed history, readonly recovery and
pinned receipt remount remain once-only. Failed UUID absence is independently
checked. All16 identities restore and twelve new C evidence files are sealed;
earlier closed proof is byte-archived outside unchanged Go discovery. Eight
disposable source-equivalent SINGLE textbox/storage cases now calibrate
narrowing before validation:99.99999/99.999996 store99.99999237060547,
99.999999/100 reject, and signed underflow becomes source +/-0. The candidate
client narrows new numeric drafts and the Go writer requires canonical typed
SINGLE assignments; historical cells/legacy receipts retain their exact binary64
values. New negative zero remains refused because SQLite cannot preserve it.
All21 bindings,47 focused frontend cases, coupled vegetation Go race70.792s,
check0/0 and a private enabled build5.67s pass. All1105 registered frontend
cases pass; independent exact-baseline review finds no significant issues.
One normally closed actual-main owner proves the eight draft outcomes/close
barriers and one exact canonical Cover1/Cover2/positive-underflow Cover3 Save,
three strength3 source audits/full typed history, once-only readonly recovery
and immutable receipt remount. Complete peers/other tables remain unchanged;
all16 identities restore and219 files/the committed DB are sealed. No native
owner remains. Fresh1071-input full race integration passes in3360.692s at
2026-10-09T04:15:47Z, exit0 with unchanged sources/discovery; this calibrated
numeric boundary is accepted within the independently enabled profile.
Source prerequisites now also execute original Current and six NotInList
branches on all four forms: old-code Yes/No, same-code/layer Yes/No, existing
personal and missing-code No. Explicit confirmed out-of-union assignments are
not new membership constraints; missing-No retains BASE/raw input and source3021
without physical mutation. Missing-Yes/USysAddSpp remains unaccepted after two
bounded attempts. All source owners close, clones byte-restore and protected
canonical/registry state remains unchanged. No Species writing grant follows.
The owned physical-reference reader now uses original master LONG/signed32 and
personal INTEGER/signed16 policies. Valid large master definitions are retained
without numeric coercion, code deduplication or NULL/empty repair; existing
SIVI/report projection policy stays unchanged. Prechange facade reproduction,
focused/coupled race, production build/vet and independent review pass.
Actual-main Wails readonly calls verify all four forms, range errors/correction,
foreign scopes and writing denial;219 files/all16 restored fixture identities
are sealed. Fresh1073-input full race passes in3335.517s at2026-10-09T05:36:10Z,
exit0 with unchanged sources/discovery; the source-width read correction is accepted.
The independently enabled vegetation panels now expose read-only physical
Species metadata, with master32/personal16 validation, original declared types,
physical identities, duplicate codes and NULL/empty retained. This paged review
is a desktop adaptation, not source UNION choices or Species editing. Cancellable
reads preserve numeric drafts/errors and immutable receipts across remounts.
Independent review's npm lifecycle-registration finding is resolved;59 focused
cases, all1117 correctly registered frontend cases, check0/0 and private
default/enabled builds pass. One normally closed actual-main Wails owner proves
all four form readers/six presentations at1400/600/320, paging, error/correction,
actual validation-yield cancellation/fresh read, stale-result refusal, remount,
native-close refusal and invalid numeric-draft retention, with zero Save intents.
All16 baseline/protected/predecessor identities restore;335 files are sealed.
The harness initially expected the wrong error wording and omitted the required
explicit clean-review close; both recover from the same owned checkpoint without
replaying any mutation. Fresh1078-input full race integration passes in3216.339s
with exit0/unchanged inputs and discovery. Default flags and accepted embedded assets remain unchanged.
Species callbacks, original-form SINGLE timing, creation/deletion/restoration and default
editing remain unavailable.
Bounded original-cover evidence now establishes normal/CHARS A informational
warnings after physical Save, followed by clean new-record/Species navigation
without insertion, and18/21 focus paths. C warns in BeforeUpdate, but immediate
ACE publication versus final closed storage remains unresolved; D Save was not
reached.14+23 sealed files retain normally closed/restored fixtures and both
completed A Saves were never replayed. This does not authorize a new callback
or whole-form workflow.
Pictures, unsupported callbacks and full combined SIVI execution are not enabled
or claimed complete in this parent review.

Combined SIVI child editing is now a verified, independently default-off
bounded desktop slice, not complete original-form execution. `VPRO_SIVI_COMBINED_EDITING` and
`VITE_SIVI_COMBINED_EDITING` authorize a separate mixed cover/height owner.
It reuses the existing domain planners and transactional audit/restoration
pipeline, with isolated `__VPRO_SIVICombinedHistory`. One responsive source
panel pairs14 normal/17 extended editable columns without duplicate live
controls. Draft/error/unknown-outcome guards extend through Save, Undo, Lock,
close, peer exclusion and read cancellation. Species/Collected/ID changes,
creation/deletion, automatic totals and remaining original warning/focus events
are not granted. Direct canonical source tracing resolves
`UpdateMetadataSppList` as an unconditional early-exit no-op, not a missing
metadata writer; the accepted child slice preserves every project metadata
row/schema. Dormant per-plot metadata code must not be enabled.
Focused Go race58.925s,51 final coupled frontend tests and check0/0
pass. Full frontend integration passes268 pretests +448 main tests plus32
entry regressions (748 total); isolated enabled/default frontend and actual-main
Wails builds pass. Four final normally closed disposable owners prove40 normal/
49 extended unique labelled controls at1400/600/320px, draft/remount/peer/Lock/
native-close guards, one three-field atomic cover/height/literal Save with
three exact audits and typed prune, history remount, independent denials and
completed-response cancellation/retry. All16 owned/22 protected identities
restore. This earlier native slice uses the FS882 host. A later current-source
standalone owner also verifies40 normal/49 extended labelled controls at three
widths, retained invalid/hidden drafts and live parent/peer/Lock/native-close
barriers, one three-field atomic Save/three exact audits and once-only typed
prune with history retained across remount. Focused frontend41/41 and Go
race20.266s pass;324 sealed files retain normal close and all16 restored/22
protected identities. Host-only harness corrections preserve all captures and
resume the sealed committed response without replay. Default gates/assets and
original-form callback exclusions remain unchanged. Fresh full Go race passes
root2064.794s/all packages. Independent review's owner-replacement
finding is fixed and its follow-up is clean: unresolved combined drafts or
operations cannot be discarded by a plot-triggered reload. The revised root
has fresh native acceptance; pre-review builds/evidence remain hash-archived
without replaying their completed write. The independently verified
[1320-file combined successor](archives/sivi-combined-desktop-checkpoint/evidence-manifest.json)
preserves845 application-source identities. The accepted835-source entry
predecessor remains unchanged. Remaining child/source callbacks and other
forms/reports work are not granted by this seal.

The follow-on child cover-warning adaptation is independently reviewed,
verified on a fresh disposable native fixture and separately sealed. Its literal/default-off
`VITE_SIVI_CHILD_SOURCE_NOTICES` gate grants no write authority and is forwarded
to the existing height, cover and combined owners. A uses only its seven normal
cover fields, including in extended presentation; C warns on NULL Cover6; D has
no active corresponding warning. Feedback is informational, never cover
inference, deletion or Save rejection. Planned warnings are shown below the
fields; acknowledged warnings retain physical identity and read-only Species
after query removal and tab remount. Explicit Reload/Undo or verified
restoration clears them. Unknown Save outcomes never receive a committed label.
Access popup/event timing and automatic Species focus are intentionally adapted
to persistent feedback and an explicit read-only focus action. This draft is
outside the immutable combined successor and does not claim full source events.
Current validation passes64 coupled tests,761 complete frontend/entry tests,
check0/0 and isolated enabled/default actual-main Wails builds. Two normally
closed owners restore all16 owned/22 protected identities. Native acceptance
proves two nonblocking A/C warnings, one exact four-cell Save/four typed audits,
query removal without physical deletion or metadata drift, retained original
Species/focus through remount and typed prune clearing the feedback. Default-off
keeps existing draft validation without notice leakage or writes. Independent
review resolved duplicate/hidden parent feedback and automatic peer-refresh
clearing; its bounded follow-up reports no significant issues.
Full Go race passes root2130.077s/all packages. The independently rehashed
[1152-file source-notice successor](archives/sivi-source-notices-desktop-checkpoint/evidence-manifest.json)
preserves847 application identities and both exact embedded candidates.
The1320-file combined predecessor is unchanged. Native warning acceptance
uses the normal combined FS882 host; extended combinations and cross-owner
refresh remain fast-test evidence. Remaining Species/Collected/identity/CRUD,
two-page children and forms/reports remain ungranted work.

Source-bound SIVI Collected is now a **separately sealed bounded native slice**
draft behind independent default-off `VPRO_SIVI_COLLECTED_EDITING` and
`VITE_SIVI_COLLECTED_EDITING`. The source action cycles NULL/C/V through a
reviewed click count, not arbitrary assigned text or an application-ID writer.
Unchanged historical text is omitted; non-text historical storage is explicitly
read-only. One live labelled cycle control owns each physical row even across
overlapping source groups. The existing transaction and persistent child
lifecycle handle typed audits, separate `__VPRO_SIVICollectedHistory`, CAS,
Undo/restoration, retained errors, peer refresh and native-close ownership.
Combined cover/height permissions remain unchanged; Species, metadata callbacks,
identity and CRUD are not granted. Coupled race71.433s,76 focused frontend tests,
check0/0, clean production bindings and actual production compilation pass.
773 complete frontend/entry tests and isolated enabled/default frontend/Wails
builds pass. Independent code and native-evidence reviews report no significant
issues/gaps. Four normally closed disposable modes restore16 owned/22 protected
hashes: seven live labelled controls at1400/600px, historical text/noop/full cycle,
remount/lock/peer guards, one held three-cell Save/three exact typed audits,
isolated history, native-close refusal, combined-peer refresh, retained history
and typed prune restoring all original business/audit tables. Independent
frontend/backend denial and completed-response read cancellation/retry pass.
Full integration race passes root2321.186s/all packages. The independently
rehashed [1272-file Collected successor](archives/sivi-collected-desktop-checkpoint/evidence-manifest.json)
preserves857 application identities and exact embedded candidates/current-main
overlays; the1152-file predecessor remains unchanged. Native acceptance covers
the normal FS882 host; extended/cross-group combinations remain fast tests.
Species/identity/CRUD and remaining forms/reports are not granted by this seal.

Source-bound SIVI Species is now a **native-verified, default-off bounded slice**,
with an immutable successor seal. Independent `VPRO_SIVI_SPECIES_EDITING` and
`VITE_SIVI_SPECIES_EDITING` are required. One live physical Species field retains
an explicit eligible A/C/D context; the group-specific lists do not collapse to
the first shared-row group. Exact listed codes and explicit old-code/personal
decisions preserve NULL/empty metadata, duplicate definitions, literal values,
8 UTF-16-unit bounds and unchanged historical invalid text. Non-text historical
Species is read-only; event uppercase conversion remains ASCII-only.
The owned master/personal reference snapshot is rechecked inside the mutation
transaction, with separate `__VPRO_SIVISpeciesHistory`. Existing child lifecycle
handles retained errors, Save/Undo/restoration, no-replay acknowledgement guards,
root Lock/close, disposal/read cancellation and all five peers. Effective Species
is used in source notices without granting cover editing or automatic focus.
The shared frontend/Go reference fixture verifies the actual `declaredType` wire
field, rather than self-consistent frontend mocks. Species race32.602s, coupled
writer race100.460s,791 complete frontend/entry/wire tests, check0/0, clean actual Wails
bindings and isolated enabled/default frontend/production Wails builds pass.
Independent code/native-evidence reviews found no significant issues.
Six normally closed disposable owners preserve16 owned/22 protected hashes:
seven live physical fields and explicit contexts at1400/600px; retained raw
errors/remount/Lock/parent/all-peer barriers; natural old-code/personal decisions;
two distinct three-cell Saves/exact typed audits/isolated history, history remount
and complete typed prune; loaded-peer refresh, independent frontend/backend denial
and completed-reference-response cancellation/retry without partial/late state.
The first Save's small-body capture missed the real chunked request; physical
history/data were validated and restored without replay, not claimed as held
evidence. A fresh owner with distinct values held the final19-chunk response and
proved native-close refusal. Native evidence uses the normal FS882 host;
extended/decision combinations remain fast tests.868 source identities/exact
embedded candidates/current-main overlays are pinned. Full integration race
passes root2252.464s/all packages. The distinct
[1306-file successor](archives/sivi-species-desktop-checkpoint/evidence-manifest.json)
is sealed and independently rehashed twice against868 application identities;
manifest SHA256 `f775f2d746f4531fe40f511d195dd532c42d7b9ce3ffa1e6bc89b1ad10655f36`.
Personal species creation, ID changes,
CRUD, metadata callbacks and the remaining forms/reports are not enabled.

The next **private source-bound identity slice** implements nullable signed32
ID planning and owned technical restoration. The newly integrated independent
default-off facade/persistent desktop editor now has bounded native ID-only
acceptance and a distinct desktop seal, not complete-form parity.
Physical rows remain stable; source AuditTrail's ID/PlotNumber exclusion is
preserved at every audit strength. New-ID collision/deleted-reservation protection
is an explicit desktop safety adaptation, not a source uniqueness claim.
Historical duplicates can be restored only with unchanged return occupancy;
neither retain nor prune changes source audits or deletes vegetation rows.
Identity/coupled writer race79.395s passes. Independent review's replacement-peer
finding is fixed with complete peer-row snapshots and real rowid-reuse tests;
focused follow-up reports no significant remaining issues. Full integration
passes root2434.345s/all packages against872 frozen identities under isolated TEMP.
The first run's unrelated shared-TEMP publication-parent disappearance is retained
honestly; three focused repetitions pass and its cause remains unknown.
The [40-file private successor](archives/sivi-identity-private-checkpoint/evidence-manifest.json)
is sealed and independently rehashed twice; manifest SHA256
`eb185c40d663d1502038bd26dfbd3a8c9757d83535c281c772500a714c5c0fd6`.
Creation/deletion and the unavailable GenUniqueID algorithm remain
separate boundaries. See the [source contract](docs/FS1333-parent-access-contract.md#next-source-identitycrud-boundary).

The successor's independent `VPRO_SIVI_IDENTITY_EDITING` /
`VITE_SIVI_IDENTITY_EDITING` gates do not grant creation/deletion or default
promotion. Exact nullable signed32 controls use immutable physical row identity,
technical history and zero-source-audit restoration. Explicit ordinary-read
availability keeps NULL/duplicate/inexact application IDs from becoming synthetic
numeric rows or preventing the physical-source panel from loading; unavailable
ordinary rows/actions/totals are visibly disabled. Standard772 frontend tests,
entry/wire34/final panel-root15/check0/0/isolated builds and six normally closed
native owners pass, with all16 fixture/22 protected identities restored.
Independent code/native-evidence reviews are clean. Full integration passes
root2258.241s/all packages against884 frozen application identities under
isolated TEMP. The [396-file desktop successor](archives/sivi-identity-desktop-checkpoint/evidence-manifest.json)
is sealed and independently rehashed twice; manifest SHA256
`787999baf14488cf1e392431cbcea8de44402af116715e39b6b511b1c3fc8c6f`.
No default promotion; private/Species predecessors remain immutable.

Linked-picture continuation is a verified default-off read-only desktop boundary;
not an installed picture family.
`PictureService` exposes only metadata
and reviewed image reads. `VPRO_PICTURE_READING=true` requires an explicit existing
`VPRO_PICTURE_LIBRARY_PATH` SQLite file with physical `tblVPics`; detached staging
is not accepted as that library. Optional `VPRO_PICTURE_CHILD_DIRECTORY` and
`VPRO_PICTURE_MANAGER_DIRECTORY` authorize separate Default directories, without
fallback or preference merging. `VITE_PICTURE_READING=true` supplies the exact
FS882 `frmVPics` source slot; the default build retains its pending placeholder.
Raw NULL/empty values, duplicate application IDs and exact physical row IDs are
preserved. Literal text scope ignores SQLite numeric affinity. External directory
literals and metadata writes remain unavailable. The preview uses each selected
row's directory rather than the source's ambiguous first record, supports decoded
JPEG/PNG up to32 MiB/16 megapixels and never copies or repairs files.
Private actual-source tests compare all10 staged rows/NULL comments, six unchanged
448x300 JPEGs and two owned previews. Focused/shared Go race49.530s and frontend
transport/pending/cancellation checks pass; the independent core review's affinity
finding is fixed and independently rechecked. Desktop review's actual Svelte-proxy
clone failure is fixed with a state snapshot and a client-runtime regression.
All782 frontend tests, check0 errors/warnings and isolated enabled/default
production Wails builds pass. Six normally closed native owners prove unchanged
Pic01/Pic02 bytes,448x300 dimensions,1400/600px visible labels, modal viewer,
remount/reload, NULL/external-directory denial, independent gates and held actual
metadata/image response cancellation with close refusal, late discard and retry.
An independent review found the first600px screenshot missed the offscreen panel.
A distinct zero-write layout owner now captures controls/image segments at both
widths with CSS visibility and viewport-intersection checks; original evidence is
preserved. All16 original,13 picture and30 protected identities remain unchanged.
Bounded native-evidence follow-up is clean. Full all-package race passes
root2184.900s against894 frozen application identities under isolated TEMP.
The distinct [485-file picture desktop seal](archives/picture-read-desktop-checkpoint/evidence-manifest.json)
is independently rehashed twice; manifest SHA256
`9a829781e7f0763d90808b1fc0f0414b80f6fc25050cf638266f66be277e6395`.
Held completed responses do not prove SQL-in-flight or image-decode cancellation.
Symlink-test creation
is unavailable on this Windows account; no native containment proof is inferred.
See the [picture contract](docs/FS882-two-page-access-contract.md#picture-source-preparation).

The separate read-only `Plot Pictures` manager is implemented under independent
`VITE_PICTURE_MANAGER=true`; it does not require the FS882 picture presentation
gate. Forms navigation selects an exact existing parent, then explicitly loads
linked metadata through the same owned service. Source `Picture Directory` and
`Picture File Name` labels, parent linkage and separate manager directory policy
are retained. Current SU/profile filters are not applied; this is an explicit
lookup adaptation of the source `USysEnv` record navigator, not an identity edit.
Independent code review,788 frontend tests/check0/0, focused unchanged-backend
race2.817s and isolated enabled/default production builds pass. Five normally
closed Wails owners prove manager-only image authority/child refusal, unchanged
Pic01/Pic02 bytes, source labels, scrolled1400/600px visuals/viewer, literal
nonmember refusal/retry, independent gates and held metadata/image response
cancellation with parent/navigation/native-close barriers and late discard.
All16 original/13 picture/59 protected identities remain unchanged. Backend
sources match the accepted all-package race2184.900s; this frontend-only successor
does not claim another all-package run. Independent native-evidence review is
clean. The distinct [322-file manager seal](archives/picture-manager-desktop-checkpoint/evidence-manifest.json)
is independently rehashed twice; manifest SHA256
`360237b3d9ffdc54a3c5d9412f8d11f0e783a322dafc01370e5f9720441f8f5b`.
Add a Picture and all metadata writes remain unavailable.
The previously missing tblVPics schema now has
[source policy evidence and tested private draft preparation](docs/FS882-two-page-access-contract.md#picture-metadata-source-policy).
Read-only schema inspection on an exact copy preserves typed NULL/empty/default/
index facts without reading user rows or launching Access views;45 packaged field
properties and both indexes independently match the receipt. PicDir/PicName
drafts reuse existing tagged-cell parsing and preserve originals/errors/literal
UTF-16 bounds. Seven new tests/807 complete frontend tests/check0/0/isolated build
pass. No metadata mutation API, audit transaction, write lifecycle or native
write acceptance is granted by this preparation. GenUniqueID is an engine Long
default expression, not a missing VBA body; allocation remains a separate boundary.
The successor implements a private existing-picture CAS writer and an independent
default-off `PictureMetadataService`, plus shared owner-scoped manager/child
draft sessions. Mutation, typed audit and durable request history share one
picture-library transaction. Fresh RW reservations hold existing project/SU
membership through commit without writing those files; this is not a cross-file
atomic commit. Lost/cancelled receipts retain drafts and block navigation/close
until read-only durable-history resolution or explicit discard/reload, never a
matching-value success inference. Both `VPRO_PICTURE_METADATA_WRITING` and
`VITE_PICTURE_METADATA_WRITING` are independent of read/preview permissions.
Independent backend/frontend reviews identified and resolved receipt identity,
parent/recovery and reload-original defects. Actual Wails checks also corrected
draft-only tab blocking and an inaccessible editor after remount: same-owner tabs
retain errors while parent inputs/Save/Undo/Lock/close remain blocked; recovery does
not require reloading the transient panel review. Successful owned reload updates
the selected original and clears its old image without fabricating a Save receipt.
Focused backend ownership/WAL/cancel/CAS/retry/facade tests and830 frontend tests/
check0/0/isolated enabled/default actual-main builds pass. Six normally closed native
owners verify four scoped commits, an independent collision with byte-identical
rejection/no history, actual cancelled acknowledgement/late-response retention,
explicit fresh reload followed by NULL CAS, read-only durable resolution with
unchanged library bytes, independent gates and actual-visible1400/600px labels.
All16 data/config/profile and13 picture identities restore;88 protected identities
remain unchanged. Typed audit provenance is contained in the durable history event,
not a claim of a separate native picture-audit table.
Pre-fix full race2195.458s is retained honestly; corrected all-package race
2245.559s/all packages passes after the receipt correction.
The [386-file metadata desktop checkpoint](archives/_picture-metadata-desktop-checkpoint/evidence-manifest.json)
is independently rehashed twice; manifest SHA256
`0e198efeb8ac0d8b7330f6a24448984e674563b60fac1df53b87a1610f419891`.
No default promotion, record creation/deletion, image-file mutations or external
directory authorization follows from that metadata checkpoint.

Source-bound picture Add is a **gated, bounded native-verified implementation**
under independent `VPRO_PICTURE_CREATION_WRITING` /
`VITE_PICTURE_CREATION_WRITING` flags, both absent/off by default. It requires
owned reading and the manager surface, not existing metadata-writing permission.
Actual main registration and nullable interface bindings connect the manager's
single-file native chooser, guarded atomic append, persistent per-parent
selection/request/receipt lifecycle and selected-file-only preview. Native Cancel
assigns nothing; unknown delivery is recovered read-only, never replayed.
Acknowledgement preserves committed receipts. Stored directories do not grant
directory/sibling access or copy files. Loaded picture peers re-read and compare
the complete committed physical row; failed preview/refresh retains the commit.
Independent private facade/client review closed in-flight recovery and synchronous
subscriber-cancellation findings. Integration review also found and closed a real
client subscription loop and reactive mutation during derived session construction;
notifications are now untracked and new idle sessions do not publish an unchanged
aggregate. Owner tracking, subsequent notifications and cleanup remain intact.
Actual client-runtime tests exercise automatic two-peer
refresh and late-response rejection; root handlers retain close/context/navigation
barriers after reader disposal. All979 frontend tests,13 integration cases,
check0/0 and isolated default/enabled builds pass. Corrected coupled Go
race66.967s, vet and production compilation pass.
Actual Wails default/backend-off owners prove absence/refusal and read-only
selection-absence recovery without granting writes. The Windows prompt now uses
a narrow documented COM boundary through the existing go-ole dependency:
separate owned HWND state, complete native arguments, signed HRESULT checks,
Show-only typed cancellation, validated literal UTF-16 paths and balanced cleanup.
The attempted public CFD dependency was rejected after an actual native ABI
crash and independent review; it is not a product dependency.
Three actual attached Cancels now leave Add immediately ready without recovery,
errors or assignments. A single planned literal selection verifies one append,
all five physical cells, history/permanent reservation, selected-only preview,
peer refresh, read-only receipt recovery and close/navigation barriers.
Narrow/wide actual visibility and replaced-file refusal pass; acknowledgement
retains the receipt, the owner closes normally and the private fixture restores
after archiving its committed database. All117 protected identities are unchanged.
Independent review closes the native boundary findings. Final full integration
race passes all packages, root3220.593s, with658 ordinary/production source/
manifest/embed inputs rehashed unchanged; interrupted earlier runners remain
unaccepted evidence.
Disposable source calibration stopped within its two-attempt budget at a private
instrumentation compiler error, before Add dispatch. Stopped snapshots were
archived, copies fully restored and canonical data/registry remained unchanged;
source physical save/default timing remains unresolved. The Windows picker uses
the system-maintained initial location, not a calibrated source directory default.
These are implementation checks, not source Add timing or whole-workflow parity.
No default promotion or whole-picture parity follows. Private deletion and
typed historical restoration kernels preserve complete physical rows,
permanent identity reservations and transactional technical history without
source audit pruning or image-file mutation. Independent review corrected
first-reservation provenance across repeated cycles; follow-up reports no
significant issues. A later historical-context correction separates the live
operation from immutable deletion identity and records the current restoring
actor/policy; actual context-switch/recovery tests pass. Current49-test race
selection passes in51.147s with vet/production compilation; focused independent
review reports no significant issues. Bounded unchanged-code Access observation confirms
live deletion permission and exactly one automation deletion, but no dialog was
shown despite the confirmation option. The source GUI confirmation/Current
gap remains sealed after two attempts. The independently default-off desktop
integration now has bounded native/full-integration acceptance, not source GUI
deletion acceptance or whole-picture parity. Details are in the
[picture source contract](docs/FS882-two-page-access-contract.md#picture-metadata-source-policy).

The private [lifecycle facade](picturedeletionservice.go) is implemented,
independently reviewed and registered in main with interface-style bindings.
Its [desktop lifecycle workbench](frontend/src/PictureLifecyclePanel.svelte)
has bounded reviewed/native/full-integration acceptance. Independent
`VPRO_PICTURE_DELETION_WRITING` / `VPRO_PICTURE_DELETION_RESTORING` flags require
explicit owned picture reading. Typed history discovery/restoration review
preserves old context identities; matching in-flight or completed-during-read
mutations invalidate receipt observations. Combined54-test race coverage
passes root54.403s, with vet/production compilation. Distinct independent
facade review reports no significant issues. Desktop runtime tests cover
stable subscriptions, independently enabled construction, retained unknown
requests/read-only recovery, irreversible acknowledgements, complete restored
peer refresh and root close/navigation barriers. Focused integration Go race
passes root52.766s; vet/production compilation and isolated default/enabled
frontend builds and all1002 registered frontend tests/check0/0 pass.
Fresh independent desktop review reports no significant issues. Actual-main
disposable Wails verifies default refusal, one Delete with held-response
cancellation/read-only recovery, actual receipt-preserving remount, occupied-ID
refusal, one exact typed Restore, permanent reservations/consumed history,
restored preview, owner/Home/native-close barriers and1400/600px layout.
Both owners normally close; committed SQLite is archived before restoration and
146 protected identities remain unchanged. Final frozen full integration passes
all packages, root3347.641s, with all669 ordinary/production source/manifest/embed
inputs and package/embed discovery unchanged. No default availability, source GUI
parity or whole-picture replacement follows from these checks.

The [source-bound SIVI creation candidate](docs/FS1333-parent-access-contract.md#next-source-identitycrud-boundary)
now has measured defaults, a transactional allocator/audit/history writer,
complete typed receipts, read-only durable lookup and persistent source-form
drafts. Independent default-off `VPRO_SIVI_CREATION_WRITING` /
`VITE_SIVI_CREATION_WRITING` gates wire actual main/bindings and the creation panel;
this is bounded verified native behavior, not complete-form acceptance.
Thirty-two focused/856 complete frontend tests, check0/0, isolated enabled/default
builds, coupled Go race/vet45.242s and full Go race1993.776s/all packages pass.
Live client regression covers synchronous subscription self-invalidation.
Receipt history identity is the exact request UUID, not a numeric row ID; audit
time is calendar-valid literal local `YYYY-MM-DD HH:MM:SS`, not UTC. Native
inspection verified four source drafts; the first write candidate exposed those
two receipt mismatches and remains explicitly failed/preserved. Both corrections
are independently reviewed and validated against the actual Go/Wails receipt.
Corrected native runs now verify nine scoped creates across four forms and
explicit keep/replace/existing-personal decisions, all44 typed cells,18 source
audits, ID reservations and independent default/backend denials. Lost/cancelled
completed-response acknowledgements resolve read-only without repeating Create.
Seven accepted owners normally closed; committed history is retained before
exact16-file disposable restoration,22 protected identities unchanged.
Actual1400/600px client widths preserve visible labels/control containment.
The bounded [3119-file creation checkpoint](archives/_sivi-creation-desktop-checkpoint/evidence-manifest.json)
is independently verified; manifest SHA256
`b4d359383f6879bdb03b204ed911e44310c710503a3635a4a4b387979396ab1d`.
Do not replay accepted writes or reseal this checkpoint. Invalid/unknown state blocks parent
Save/Lock/close and owner replacement; resolution never retries Create.
Missing history is unresolved, not permission to discard an unknown request.
Positive signed32 allocation is a documented desktop adaptation, not Access RNG
equivalence. This creation seal does not certify the separately gated deletion/
restoration successor below, full combined form acceptance or default promotion.
The picture metadata seal does not certify those workflows either.

The deletion/restoration desktop successor is integrated behind independent
default-off `VPRO_SIVI_DELETION_WRITING` / `VITE_SIVI_DELETION_WRITING` and
`VPRO_SIVI_DELETION_RESTORING` / `VITE_SIVI_DELETION_RESTORING` flags.
Eight actual SDK methods provide owned44-column target discovery, fresh exact
originals, explicit deletion, durable history discovery, reviewed restoration
and read-only receipts. Hidden extended covers and zero establish source
membership; unsupported logical IDs/BLOB audit values remain explicitly
unavailable. History absence and an existing empty owned list are distinct.
The shared UUID helper preserves accepted creation behavior.

Deletion atomically commits exact44-cell CAS, permanent ID reservation, source
audits and immutable history. Typed undelete is a desktop safety adaptation,
not original Access audit restoration. It requires vacant physical and
project-global logical identities, preserves all44 raw cells/reservations and
rechecks the reviewed SHA256 evidence inside its transaction. Separate immutable
history consumes a deletion once. Retain/prune affects only exact source audits,
including explicitly permitted table aliases; destructive cleanup is never run.
Eight shared numeric fixtures verify actual Go audit formatting, including
scientific thresholds and signed zero.

Owner-keyed sessions survive tab remount. Unknown/lost/cancelled acknowledgements
retain exact originals/reviews/requests; missing history never authorizes replay
or discard. Ordinary parent Save cannot perform Delete/Restore. Pending reviews
block parent editing, Lock, owner replacement and competing workflows; close
cannot offer implicit destructive Save. Read-only recovery remains available
despite a locked/invalid parent. Verified commits refresh peers under a host
barrier; refresh failure requires explicit reload without repeating mutation.

Independent kernel/facade/frontend reviews found no significant issues.
Integrated coupled race passes371.847s;74 focused/898 complete frontend tests,
check0/0 and isolated default/enabled frontend/actual-main Wails builds pass.
Six normally closed disposable native owners verify nine deletes/nine restores,
exact44 cells/physical IDs, source retain/prune/reservations, explicit audit-action
reconfirmation, lost/cancelled no-replay recovery, native close guards and
independent frontend/backend denials. Four source forms pass actual1400/600px
visibility/labels/containment. All16 prepared identities restore and22 protected
identities remain unchanged; closed committed histories/profiles are retained.
Independent native-evidence review found no significant issues and reproduced
closed-history integrity, identity, asset and hash checks. Fresh full integration
race passes root2206.226s/all packages. The independently verified
[2179-file desktop checkpoint](archives/_sivi-deletion-desktop-checkpoint/evidence-manifest.json)
preserves970 application source identities/592 Go files; manifest SHA256
`578085021c4a4a9eddf18c5058e8d882c208e4543b4f5e1d13601e560b20139c`.
Never replay its native writes or reseal it.

A tightly coupled close-priority correction preserves pre-existing unknown parent
source authority above a known deletion/restoration review; it cannot be masked
by the new panel.75 focused/899 complete frontend tests/check0/0 and isolated
frontend/actual-main builds pass. One additional normally closed read-only Wails
owner verifies four reviewed rows, native close Save refusal and1400/600px labels/
containment, with all16/22 identities unchanged. The composite unknown-parent
case is an executed actual-root fast regression, not an injected native source
authority claim. All592 Go files remain identical to the full-race predecessor.
[56-file priority checkpoint](archives/_sivi-deletion-close-priority-checkpoint/evidence-manifest.json)
has SHA256 `f743145db338b172f95bcec39d9d8880e5a356d3904eccff0b25706bed7aa742`.
Both immutable predecessors and accepted builds remain preserved. No default
promotion or complete combined-form claim.

Historical creation Undo is an implemented backend draft, not a verified desktop
workflow. Independently default-off `VPRO_SIVI_CREATION_UNDO` registers four
methods: owned history discovery, fresh review, explicit Undo and read-only
receipt lookup. A seven-field request with a separate UUID and SHA256 token
removes only the exact current44-cell created row, retains permanent logical
reservation, and retains/prunes only proven creation audits. Immutable one-winner
Undo history also records zero-audit events. Historical consumption is not
current eligibility; review must independently prove unchanged creation authority.
Coherent empty-cover historical evidence is supported privately, but the
accepted creation writer does not produce it. Clearing covers after ordinary
creation is not authority to Undo that changed row.
Independent private-kernel review is clean;14 new tests/coupled race-vet453.869s
and six facade/history race-vet tests9.114s pass. Actual production bindings and
backend compilation pass. Strict frontend13/seven/26-property transports and
11-property historical events reuse existing validators; coherent legacy
non-UUID histories stay visible unavailable, without blocking valid UUID choices.
Persistent owner sessions/panel and independently default-off
`VITE_SIVI_CREATION_UNDO` root integration now pass119 focused/949 complete
frontend tests/check0/0/isolated enabled/default actual-main builds. Native recovery
then exposed unnecessary initialization of an independently unavailable peer.
Shared owner/loaded-peer-only lifecycle refresh corrects it, with30 targeted/
952 complete frontend tests/check0/0 and corrected isolated builds. Loaded-peer
refresh failures still disable editing without replay. Closed pre-fix native
evidence is retained and disposable identities restored. Parent Save
and close cannot implicitly submit historical Undo; unknown receipt recovery
is read-only, and postcommit refresh failure disables editing without replay.
Independent facade/transport/session review is clean. Corrected actual Wails
evidence covers eight normal/two delivery cycles and three independent denial
modes, five normally closed/restored owners, exact raw-row/audit/reservation
checks and recovery without initializing unavailable peers. Independent
integration/evidence review finds no significant issues, including actual
raw/audit/history/candidate/closure hash checks; full Go race passes
root2423.962s/all packages against597 unchanged Go files. Corrected frontend
was separately validated. The distinct
[2650-file desktop successor](archives/_sivi-creation-undo-desktop-checkpoint/evidence-manifest.json)
is sealed/independently rehashed with987 source identities; earlier seals stay immutable.
No default promotion, overall migration completion or Access cleanup
claim follows.

[Saved single-plot label preview](docs/PLOT_LABEL_PREVIEW_CONTRACT.md) is an
verified default-off read-only boundary under `VITE_PLOT_LABEL_PREVIEW=true`; the
original Print a Plot Label entry stays disabled. It reuses the owned GetPlot
read, exact source assignments, four-unit Zone padding, UTF-16 label bounds,
NULL/empty distinctions and source ASCII-space display Trim without changing
BecLabels or printing. Source `NullToNothing` returns NULL, not empty; NULL Zone
or unsupported source bounds reject explicitly instead of repairing data.
Independent code review,798 frontend/check0/0/isolated production builds and
four normally closed native owners pass: two independently checked labels,
NULL SiteSeries,1400/600/320px actual visibility/intersection, literal nonmember/
no stale card/retry, default denial and held-response cancellation/root barriers.
All16 original/13 retained picture/88 protected identities remain unchanged.
Native review found omitted Preview/disclosure measurements; a distinct layout
owner now supplies twelve complete input/card/disclosure/assignment segments
without replacing original evidence. Focused native-evidence follow-up is clean;
the distinct [327-file label preview seal](archives/plot-label-preview-checkpoint/evidence-manifest.json)
is independently rehashed twice; manifest SHA256
`caa21fc7e5558bdd018eefdf128c08058c92531ff1a5b4c1aaeeed76c54b930c`.
Source report designs are now [mapped static evidence](docs/PLOT_LABEL_PREVIEW_CONTRACT.md#recovered-report-designs-static-evidence-not-printing):
six single controls and all72 batch controls retain exact bindings, twip positions
and explicit/inherited/omitted properties through the shared guarded extractor.
The recovered nonuniform batch spacing is not replaced with a guessed grid.
Six-position printing and Print All scope/ProjectName remain unavailable.
Original single/twelve-slot reports now also have a bounded
[native PDF projection contract](docs/PLOT_LABEL_PREVIEW_CONTRACT.md#native-report-pdf-projection-not-source-print-callbacks):
four independently reopened outputs,26 label slots/129 exact lines, NULL/empty
presentation, original Trim/Long Date and unclipped page spans on Microsoft Print
to PDF. Two disposable source owners close normally with staging/canonical bytes
unchanged; no print job, application export, callback or stock-alignment grant
follows from that selected-device observation.
A private [portable single-label PDF preparation kernel](docs/PLOT_LABEL_PREVIEW_CONTRACT.md#portable-single-label-pdf-preparation-draft)
now passes focused race, ordinary/SQLite-only production builds, independent
actual-PDF text/font/rectangle checks and distinct review. It uses runtime real
Arial rather than substituting Helvetica; fonts are not bundled. Printable ASCII,
the calibrated en-US date and unclipped single-line fields are supported, with
explicit zero-output refusal otherwise. Fixed portable spacing is not Access
driver/CanShrink/stock parity. Its private runtime font loader now passes focused
fault/cancellation checks and distinct review. The preparation kernel alone
does not grant printer output.

The separate experimental portable single-label PDF desktop now requires
`VITE_PLOT_LABEL_PDF_EXPORT=true` at frontend build time and
`VPRO_PLOT_LABEL_PDF_EXPORT=true` at runtime. Neither flag is enabled by default;
the client flag independently makes the saved-label route available without
enabling the original physical Print action. Review an exact saved plot, then
provide a new `.pdf` destination. Publication binds complete physical source,
owned paths, native date/runtime fonts, exact internal bytes and service-owner/
UUID provenance, and never overwrites a file or changes database/configuration.
Publication is not cancellable/replayed; lost responses use readonly exact-attempt
recovery. Acknowledged immutable receipts survive panel remount even without a
new preview. The ledger is service-owner scoped memory, not durable history:
retired-owner outcomes remain explicitly unknown.

Coupled race12.108s,1545 frontend case executions/check0/0, distinct desktop review
and four normally closed disposable Wails owners pass. One output independently
reopens with six exact strings, actual Arial and one Letter page; successful
response hold/loss/recovery, read cancellation, receipt remount, independent gates
and51 three-width control/receipt measurements preserve all16 database/config
files per owner. Complete1578-input full integration passes exit0 in5401.705149s
with unchanged inputs/discovery; final native/source/review rehash releases the freeze.
Fixed spacing, printable-ASCII/unclipped single-line and calibrated en-US bounds
are adaptations, not historical-data constraints. Physical printing, Print All,
positions, stock alignment and whole forms/reports parity remain unfinished.
An independently default-off regional successor requires both
`VITE_PLOT_LABEL_REGIONAL_DATE=true` and `VPRO_PLOT_LABEL_REGIONAL_DATE=true`.
It reads byte-exact physical saved Env/Admin parents and formats the captured
Windows user Long Date pattern without browser fallback. Twelve original Access
cases qualify the current en-US/Gregorian pattern, not every regional setting.
Independent review/follow-up, focused race and three normally closed disposable
Wails owners verify native dates, NULL/empty assignments, literal/source refusal,
three-width layout, cancellation/remount/root/native-close holds, backend-off
refusal and legacy compatibility; all16 fixture files per owner are unchanged.
The current combined binary also passes bounded parent text/numeric Save,
exact audits, tab draft/Undo and P1/P2/P1 saved-value reopening on disposable data.
Complete1538-input race integration passes with unchanged inputs/discovery;
final association rehashes1002focused/native and204supplement/source files.
Both that predecessor and the new PDF successor have released their verified
complete-source integration freezes; no default promotion or printing follows.

Current extended-shrub acceptance: full Go race (root858.736s),280 frontend
tests, check0/0, opt-in/default production builds, eight native cases and a
zero-write default proof. Report-preview predecessor is `e763e23`.
Native acceptance used disposable data/configuration and restored original bytes.
Feature implementation/native verification does not mean a default-off workflow
is enabled or its Access execution parity is complete.

Long Vegetation's first preparation slice is source-mapped and Go-tested, not a
native-verified report. It preserves all physical SU memberships (including
duplicates/orphans and NULL/empty unit identities), independently MAX-reduces
the original15 cover/total fields, and emits only non-NULL covers selected by
the original `LayerCode.LayerText` metadata. Zero/negative covers, extended
shrubs and layers8/9/10 remain observations; totals are not invented as layers. Duplicate layer
definitions and NULL/empty species remain distinguishable. Literal identity
matching, deterministic ordering and deterministic numeric-tie storage are
explicit desktop adaptations, not Access collation/Single-coercion parity.
No persistent tables, configuration, production data or UI gates change.
Five focused preparation tests and shared environment regressions passed with
race1.261s; full Go race integration passed (root672.153s). The accepted native
executable remains unchanged; there is no new frontend/native acceptance claim.

Continuation retains Access's constant-species-list threshold bypass and
existing YAML default (-1), rather than silently adapting it. Client option
scope remains provisional. A selective `DAO.DBEngine.120` ACE query on a new
disposable database verified that `Count([MyCover])` counts non-NULL pivot plot
columns, including zero and negative sums. Repeated SU rows multiply covers,
but not that numerator; physical SU rows remain the independent denominator.
`all-plots` divides the joined sum by that denominator; `observations` averages
non-NULL joined observations, not plot sums. Missing constant-list species have
NULL statistics/cells, not zero. Constant-list SQL equality does not match
NULL group/species keys; empty text still matches. Both thresholds are strict and apply only
outside constant-list mode. The probe omitted the identity `vpRoundUp` wrapper;
it did not run VPRO report code, Excel, application startup or production data.

The kernel accepts already joined/grouped observations, preserves literal
NULL/empty identities and computes exact rational intermediate sums with finite
double outputs. This avoids order-dependent cancellation but does not claim
Access Single/Val coercion parity. Four oracle cases are retained in
[the presence fixture](testdata/long-vegetation-presence.json). No quality,
species/reference/layer-label join, taxon lumping, summary or report UI is
enabled by that calculation evidence. A second three-case physical ACE probe
verified constant-list NULL/empty equality. Its initial constant-expression
saved-query variant unexpectedly dropped the NULL left row; that optimizer
behavior remains unknown and is not inherited as a desktop requirement.
Four crosstab tests plus five preparation regressions passed with race1.241s;
final full Go race integration passed (root664.970s). No frontend/binding change
or new native desktop build was needed; the accepted executable is unchanged.

The layer preview reads original physical project Veg, selected
SU, VLists.USysAllSpecs and VPro64.LayerCode through the existing coordinator,
immutable context lease and read-only transaction. It does not substitute the
editor's master/personal species UNION. Missing master codes retain raw Species;
non-NULL ScientificName (including empty text) supplies the source display
name, and synonym CodeType S/s is excluded. Duplicate reference definitions
multiply joins as in the source and produce explicit diagnostics.

EnglishName participates in source grouping. The constant-list join matches
only Layer and displayed species, not EnglishName: different names can fan out
into several rows. Both the list name and matched report name remain visible
in the typed result rather than silently picking one definition. An
eight-row selective ACE fixture verifies this behavior. The shared crosstab
has an explicit unfiltered intermediate mode; its ordinary/constant-list
threshold behavior remains unchanged.

Named-unit denominators count physical SiteUnit rows, including duplicate
memberships and NULL PlotNumber rows; unassigned denominators count only
non-NULL PlotNumber rows. Orphan-only named units do not appear as invented
reports. Empty identities remain literal, and per-unit pivot columns come
only from joined cover observations. The desktop explicitly permits an
unassigned-only preview instead of inheriting Access's early no-named-unit
exit; no Excel250-plot cap or generated sheet-name rewrite is imposed.
Unit names are read separately from the original `VLists.MasterSiteUnitList`
inside the same owned snapshot; they never change these calculations.

The decoder preserves the default title, average10, constant list=-1, English
names1 and ordering20. Alternative unit selection fails explicitly; it does not
silently fall back. Summary and publication preferences remain stored and
untouched, not falsely presented as computed output. The report UI is
independently default-off behind `VITE_LONG_VEGETATION_REPORT=true`.
Explicit YAML quality enforcement is supported by the bounded read-only
qualification described below; its retained default is false.
Combined acceptance:22 focused Go tests passed with race5.224s; full Go race
integration passed (root659.145s). Independent read-only review found no
significant defects; it correctly distinguishes selective ACE fixtures from
full native report/Wails acceptance, which remains unavailable.

The subsequent typed boundary exposes
[GetLongVegetationOptions and PreviewLongVegetation](vegetationreportservice.go)
with generated cancellable Wails bindings, explicit context/project/SU paths
and lower-camel-case JSON models. Preview includes the actual settings used,
not an assumed match to an earlier options read. Both calls retain strict YAML
decoding and selected-SU ownership; they have no option/title mutation or export
parameters. Whole selected-SU scope is preserved rather than filtering to a
current plot or profile. NULL/empty identities, nullable statistics, physical
membership IDs, diagnostics and list/matched English names remain distinct.
[Frontend transport validation](frontend/src/longVegetationReport.ts) rejects
stale/incomplete identities, unsupported settings, malformed typed cells,
inconsistent pivots and invalid multiplicity counts without inventing threshold
bounds or deduplicating source fanout. Focused backend race tests and frontend
report/context tests, check and build pass. Independent boundary review found
no significant issues; integrated Go race passed (root810.601s).
No native binding/lifecycle
acceptance is claimed.
The responsive preview displays read-only YAML settings, literal project/SU
database paths, separate list/matched English names and physical membership
denominators/IDs. Constant-list threshold bypass is explained next to the
options, and missing combinations remain NULL rather than zero.
Existing draft Save/Discard/Cancel, native-close busy guards and cancellable
read/remount behavior are reused.285 frontend tests and check0/0 pass. A blocked native read exposed transaction-context
cancellation discarding the pinned coordinator connection. A shared caller-owned
read snapshot now keeps each query cancellable without losing attachments/TEMP
views; both reports and scoped Find use it, with cancellation checked before
commit and explicit rollback-error propagation. The regression reproduced the
failure before the fix; focused coupled Go race passes17.084s and full corrected
race passes665.226s. Independent UI, cancellation-repair and visible-safety-error
reviews found no significant issues. Final native opt-in acceptance passes eight
cases: draft Cancel/Discard, typed scope/rows/names/diagnostics, repeat reads,
visible busy-close rejection, blocked read cancellation/retry, remount and600px
actual visibility with bounded table scrolling. Default proof keeps the report
disabled. All17 disposable database/configuration files and the accepted seed
remain byte-for-byte unchanged. Failed and seven-case predecessor builds are
archived honestly, not presented as final acceptance.
The accepted executable and canonical Sample database are unchanged.

## Architecture and safety

- Retain VPro64/VLists/VUser/VMetaData/VMessageBoard and per-project physical
  table naming. Bundled [Sample](resources/Sample.db) is byte-identical to the
  R SQLite seed, including tables/indexes/views/`_table_metadata`; SHA256
  `e63f0c2a051761701bdad3c81bcfde4067ab322883c7e4ac84a3541ae8578ad8`.
  Descriptions are native Access table-object metadata, not only version flags.
  Derived editor catalogues are read models, not replacement databases.
- Missing support files install under `<dataDir>/database-family` without
  replacing existing files. `Desktop.DatabasePaths` retains explicit absolute
  family overrides. Project/SU/hierarchy selections keep independent paths;
  selected existing files are not silently copied or converted.
- `config.init.yml` initializes persistent `config.yml`. Shared preference
  ownership serializes selection/coordinate/Working Unit/audit/user changes.
  Unknown/inactive settings retain values/types. Three old JSON files are
  validated/imported together, retained and hash-recorded; conflicts, changed
  inputs and malformed data fail explicitly. Failed replacement preserves
  prior bytes/effective settings. Obsolete Access machine paths are not imported.
- SQLite owns readonly attachments/TEMP views and separate transactional writers.
  Immutable context IDs, file observations, leases and draft-safe switching
  prevent stale calls. DuckDB remains optional; offline startup needs no
  unprovisioned extension.
- Errors/proposals retain editor/original identity through tab remounts and block
  Save/Lock/native close/context/unrelated writes. Data and audits/provenance
  share transactions; cancellation, drift, collisions, rollback and retry are
  tested. Known commits retire proposals before refresh; do not replay failures
  explicitly reported as already committed.
- Preserve raw text, NULL/empty, duplicate reference metadata, BOOLEAN true=-1,
  UTF-16 bounds and unchanged historical invalid values. Do not trim, recase,
  infer defaults, silently repair raw JSON Unicode or enforce new field-manual
  recommendations on historical projects. No phantom audits/destructive restore.
- Configuration/path failures open visible recovery with editors disabled.
  Correct the reported YAML/path and restart; automatic repair and production
  write approval are absent. Set `VPRO_DATA_DIR`/`VPRO_CONFIG_DIR` to disposable
  directories for validation.

## Workflow boundaries worth retaining

The mapped normal/CHARS single-plot Google Earth successor is an independently
default-off readonly saved-plot adaptation with bounded native acceptance. It previews one owned saved
Env plot, preserves complete original data and reuses offline KML formatting.
Backend independent review and focused coverage follow-up found no significant
issues. Owned saved-option reading, mapped controls and immutable desktop panel
pass corrected race11.197s, production compile,118 focused/1628 registered frontend
executions, check0/0 and default build. Independent review found multiline HTML
normalization; lossless escaped-original comparison and five actual-Go description
fixtures fix it; exact20-file/14-base independent closure accepts the correction.
Twelve normally closed disposable Wails owners preserve16 files each, accepting
normal/CHARS original descriptions, complete rows, actual parent/peer draft holds,
held-response and validation remount rejection, close/Return holds, labelled
three-width controls and both independent off-gates. Complete1613-input race
and exact final review/native/source association pass without drift; the
integration freeze is released. It does not implicitly Save,
overwrite configured output, write preferences or launch Google Earth.
Publication remains unavailable;
existing all-project/selected-SU Google Earth workflows are unchanged.

The active normal/CHARS View Veg Table successor is an independently default-off
mapped readonly desktop candidate. Its source query
boundary distinguishes17/14 projected columns and13/10 ordering keys, all-project
or duplicate-expanding selected-SU scope, and all-NULL-cover inclusion.
Controlled English-ignore-case membership/order is calibrated on966 native
relations, explicitly not general ACE locale parity. Same-snapshot100000-row
physical input bounds and1..200-row pages refuse overflow without truncation;
virtual duplicate counting avoids materializing the joined table and retains
complete page-original witnesses. Exact thresholds, a fifty-million virtual
join, variants/scope/historical storage/Unicode/cancellation and the saved-plot
predecessor pass focused race22.353s and production compile. Exact3-file independent
review found no significant issues and accepts only the default-off backend draft;
candidate/base/source/validation hashes match, without independent test rerun.
The registered immutable paginated panel and strict actual-Go wires (including
50/20 pages) pass focused race58.226s,153 focused/1671 registered frontend
executions, check0/0/default build/production compile and exact15-file/9-base
independent desktop review. Nine normally closed disposable Wails owners retain
16 files each, qualifying normal/CHARS selected/None scope, complete independent
originals, actual paging, parent/child draft barriers, held-response cancellation/
variant/remount rejection, native-close/Return holds, three-width labelled
columns/buttons/disclosed originals and independent off-gates. The1760-artifact
seal is verified. Complete1621-input integration passes
`go test -race -timeout 120m ./...`, exit0 without input/discovery drift.
Final association rechecks all1760 native artifacts, reviewed candidate/base/
backend/source lineage and nine absent original owners; the freeze is released.
Runtime `VPRO_TWO_PAGE_VEGETATION_TABLE_REVIEW` and build
`VITE_TWO_PAGE_VEGETATION_TABLE_REVIEW` both default off. Query DDL, editable
datasheets, implicit Save, export and general ACE locale parity remain unavailable.
The family's original NOT NULL constraints are preserved; nullable legacy
PlotNumber is backend-only evidence. See the [source-table boundary](docs/FS882-two-page-access-contract.md#view-veg-table-source-boundary).

View Env Table's next default-off, unregistered readonly backend draft preserves
owned Current.DataFormName dispatch and four declared target orders, rather than
assuming the invoking variant. It reviews whole-project physical Env/Admin
pairs, without an SU collection filter or activated saved form filter. Complete
typed page witnesses and virtual duplicate paging have100000-row physical/
1..200-row page bounds. Exact thresholds,9,999,800,002 virtual pairs, ownership,
configuration/data refusal and actual cancellation/retry pass combined focused
race76.036s and production compile. Exact two-file independent review finds no
significant issues;38 hashes match, without independent test rerun. Source-native datasheet presentation/lifecycle and general ACE locale parity
are not accepted. The registered readonly successor preserves446 source-bound
labels/page-group instances with separate complete physical originals. Actual
Go wires, focused race122.934s,194 focused/1712 registered frontend executions,
check0/0/default build/production compile and exact16-file/nine-base independent
review pass. Ten normally closed disposable Wails owners qualify four saved
targets, normal/CHARS selected/None, independent physical/source disclosures,
draft/lifecycle/paging/close, three-width selector containment and both off-gates.
The2387-artifact seal passes. Full1630-input integration hit Windows race-detector
allocation error1455 with unchanged inputs/discovery; exact-candidate retry
passed root5607.056s with bounded Go heap/concurrency, retaining the failed
receipt. Final association reverified all2387 artifacts and released the freeze;
both gates remain default-off.
The source-mapped metadata review backend is independently reviewed and its
registered desktop draft remains independently default-off. Controlled DAO matching and its
95.434s combined race/production compile pass; complete owned physical rows,
historical IDs and source comparison keys remain distinct. No Add/copy, source
NULL-to-empty conversion, master dependency or mutation is enabled. Its mapped
desktop now passes202 focused/1763 registered frontend executions, check0/0,
default build, focused race91.337s, strict actual-Go wire and production compile.
Warning-free -ts -i bindings register77 services/356 methods/385 models.
Exact15-file/nine-base independent review finds no significant issues; the
reviewer verifies hashes/physical fixtures but does not rerun primary suites.
Normal/CHARS source captions, immutable50-row paging, all74 source-labelled
fields and coupled draft/lifecycle holds are implemented. Eight normally closed
Wails owners prove normal/CHARS selected/None, both off-gates, complete source/
physical disclosures, draft/paging/lifecycle and three-width containment.
The2056-artifact seal includes ten absent originals and sixteen unchanged fixture
files each; two independent-display expectation failures are retained.
Complete1638-input race integration passes exit0 in85m08s without input/discovery
drift. Final association reverified2056 artifacts and exact review/source/backend
lineage; the freeze is released. Gates remain default-off; original editable
lifecycle is not accepted.
See the [dynamic datasheet boundary](docs/FS882-two-page-access-contract.md#view-env-table-dynamic-datasheet-source-boundary).

Mapped Plot Profiling has a separately default-off readonly
[backend](twopageprofilingservice.go) and registered desktop candidate, not the editable source lifecycle.
It requires explicit owned profile name/path, retains the paired opening
parents and all nine fields in physical row order, and preserves historical
duplicate Order and Description absence/NULL/empty/duplicate definitions.
Bounded reads and actual snapshot/cancellation retry pass combined race93.489s
and production compilation; independent exact review finds no significant
issues (hashes/helpers verified, primary checks not independently rerun).
The mapped desktop passes213 focused/1815 registered frontend executions,
check0/0/default build, production compile and focused Go race124.657s.
Exact16-file/nine-base desktop review finds no significant issues; the reviewer
independently reran213 focused tests and check0/0. Isolated native candidates
preserve the reviewed inputs and live default assets. Eight disposable-native
cases pass normal/CHARS selected/None and both independent off-gates, complete
physical witnesses, paging/drafts, response and validation-yield retirement,
native-close/Return holds and three-width visibility/containment. All ten original
owners normally closed with sixteen unchanged fixture files each;2095 artifacts
are sealed. Full race integration passes exit0 without input/discovery drift;
final association reverified2095 native artifacts and all1647 protected inputs
(1646 runner inputs plus the separately native-pinned source read model).
The freeze is released; both gates remain default-off.
Explicit50-rule pages retain immutable originals; Description validation yields
for cancellation/remount, and lazy original disclosures show50 definitions from
the same snapshot without another source read. Opening does
not clear scratch, write counts, select profiles, run rules or filter forms.
Separate static edit/suggestion evidence retains focus-time state and value
mutations, literal Sample_Env versus selected-project fields and unresolved
Criteria_AfterUpdate; standalone choices/writers are not mapped callback parity.

The separate [species-only profiling kernel](projectprofilingpresence.go) is a
private readonly implementation, not a registered action. Its source SQL uses
the nine non-NULL USysTVeg columns Cover1..7/TotalA/TotalB: zero and negative
covers establish presence, while Cover5a/5b/5c/8/9/10 do not. It intersects
Veg/project-local enabled Lump matches without interpreting Order, Criteria,
Layer, Operator or Operation. Env and NULL Table rows are excluded; an unmatched
Table seeds the scope without reading Species. The
[portable native corpus](testdata/profiling-presence-native.json) retains32
matching cases,23 scope plots and six independently reopened physical tables.
Source ASCII text equality ignores terminal spaces only for comparison; original
rules/data remain exact. Unqualified active Table spellings, raw SQL/Unicode
species, missing explicit Lump ownership and nonliteral/case-ambiguous parent or
child identities refuse explicitly. Focused coupled race69.765s, vet and
production compilation pass; exact independent review accepts this private
kernel with no significant issues. Its unexported owned-context successor now
passes coupled race74.139s, vet and production compilation, including actual
in-flight matching-query cancellation followed by a successful owned retry,
complete original-rule/Lump/file preservation and stale rules/count/schema/
Lump/current-context refusals. Distinct exact context-level review accepts this
unexported boundary with no significant issues. Complete-source integration
passes exit0 with1680 protected inputs/all51 resources/38 sealed artifacts and
unchanged source/package/embed discovery. The final association releases the
freeze; the five prerequisites remain unregistered.
No registered service, binding,
client, scratch/count mutation, subvariety preparation or original callback
acceptance is claimed.

The separate [strict species-only wire facade](projectprofilingpresenceservice.go)
is implemented. Its independent
`VPRO_PROFILE_SPECIES_PRESENCE_PREVIEW` feature is default-off. Complete original
rules and optional nullable Lump travel through an explicit raw JSON request;
the existing generic Unicode/duplicate guard (historically named for complete
entry) is reused before exact metadata keys and typed unknown-field checks.
The seven-field readonly response is deliberately distinct from ordered steps,
navigation and Save-as-SU. Coupled race93.080s, vet and production pass with1680
predecessor inputs unchanged. Initial exact review finds no significant issues;
optional/active Lump and known-field malformed-Unicode follow-up passes
race75.358s and receives final focused acceptance with no significant issues.
Its separately reviewed13-path desktop candidate adds service registration,
interface bindings and an independent `VITE_PROFILE_SPECIES_PRESENCE_PREVIEW`
action in the saved profile review. Separate readonly results never feed ordered
navigation or Save-as-SU. Reactive inputs are snapshotted, exact literal scope
is validated and large validation yields every128 items with a subsequent request
generation check.32 targeted/94 connected frontend tests, actual client-runtime
proxy/held-validation handler cases, check0 errors/warnings, isolated default/
opt-in builds and registered Go race4.985s/vet/production pass. Distinct exact
review accepts the candidate with no significant findings and verifies all
unowned predecessor inputs unchanged. Five disposable Wails modes verify both
default-off gates, readonly matching, draft/close guards, active Lump and actual
held-validation Cancel/remount/retry, preserving all16 data/config files.
A focused reviewed presentation refinement keeps literal plot identities
unbroken; its exact rebuilt Wails candidate verifies all128 identity ranges,
viewport containment and unchanged files, then normally closes. All six native
owners are freshly absent. The first complete1687-input race attempt was
interrupted without a completion receipt. Fresh recovery passes exit0 with
1687 inputs/482 sealed artifacts, canonical inputs and package/embed discovery
unchanged; its final association releases the application freeze.
Existing accepted frontend assets
and51 resources are preserved; no default or mapped callback availability is
claimed.

The standalone profiling Env numeric compiler now has a separate correction
candidate, not new mapped execution. Native source evidence distinguishes SQL
LONG/DECIMAL/DOUBLE literals; DECIMAL comparisons require their own rounding,
not just a rounded bound DOUBLE. Fifty-five literal observations and480
independent comparison expectations are sealed with all six DAO hosts absent.
The five-file candidate passes profile/formatter race78.536s and production
compilation, preserving Veg/Lump criteria, stored files/counts and cancellation.
Independent exact five-path/three-base review accepts the bounded change without
significant issues. One normally closed disposable Wails owner verifies69
expected predicates across138 physical rules through the actual standalone wire,
with complete data/history and all16 files unchanged. The2367-artifact seal is
pinned; full1671-input integration, including all51 resources, passes exit0
without input/discovery drift. Final association reverified2367 artifacts and
released the application freeze.
Original profiling callbacks, mapped Run and general locale remain unavailable.

The integrated successor cover-comparison correction preserves raw Veg/Lump criteria, but
does not substitute a bound DOUBLE for the source SQL literal. Native SINGLE
cover/helper/sum comparisons differ from DOUBLE aggregates; the DAO result type
alone does not identify runtime precision. Veg sums also require per-addition
SINGLE rounding and persistent DOUBLE promotion after overflow. Independent
[native expected results](testdata/profiling-cover-native.json) cover405 predicates,
144 midpoint comparisons and20 exact sums. The corrected production queries,
profile/formatter predecessors, readonly service/cancellation and refusal checks
pass race70.047s; production compilation passes. Exact six-path/four-base
independent review finds no significant issues. Actual disposable Wails wire
passes431 independent predicates across1293 rules/59 parents, preserving complete
typed review/history/data and all16 files. The normally closed owner is absent.
The2652-artifact predecessor/source/review/native seal is pinned; complete-source/
resource race passes exit0 with1673 protected inputs/all51 resources and no
input/package/embed discovery drift. Final association reverified2652 artifacts
and the exact reviewed/source/native lineage, releasing the freeze. This does not
enable original callbacks, change frontend/default gates or write stored counts.

The integrated raw-literal domain successor validates the independently observed
exponent bound before accepting numeric zero, retaining supported underflow and
leading zeros. Positive-exponent zero above28 is explicitly unavailable rather
than inheriting anomalous source values; this deliberately also refuses
source-valid0e29/0e100 and is not general grammar parity. Env conversion is
unchanged. The exact three-path/one-preimage candidate passes106 native fixtures,
caller/service refusal/correction/retry, race68.876s and production compilation.
Independent review finds no significant issues. Six normally closed disposable
Wails owners qualify both Veg/Lump refusals and supported underflow; all16 files
per owner and complete typed rules/counts/data/history remain unchanged.
The3686-artifact seal retains accepted predecessor lineage; complete1675-input/
51-resource race passes exit0 without input/package/embed discovery drift.
Final association reverified3686 artifacts and exact reviewed/source/native
lineage, releasing the freeze. Default gates remain unchanged.

The nullable ordinary Audit presentation candidate uses separate
ListAuditReadEntries endpoints; legacy strict ListAuditEntries and AuditEntry
history/restoration contracts are unchanged. The ordinary table distinguishes
NULL/empty actor/date values and explicitly disables restoration of NULL
actor/date rows. Focused Go race7.062s, production compile,73 focused/1597
registered frontend executions, check0/0 and default build pass. Independent
review's missing captured-context forwarding finding is fixed and independently
closed; exact18-file/15-base review finds no significant issues. Six accepted/
two rejected normally closed Wails owners preserve all16 fixture files each,
proving initial-NULL normal/CHARS source Audit launch, ordinary NULL/empty labels,
strict restoration refusal and source lifecycle/draft/layout/gate guards.
The1710-artifact native seal passes. Complete1605-input race passes exit0
without input/discovery drift. Exact association rechecks reviewed/source/
predecessor lineage and all eight original owners absent; freeze released.

The active `btnAudit` callback has an independently default-off readonly
candidate (`VPRO_TWO_PAGE_SOURCE_AUDIT_REVIEW` and
`VITE_TWO_PAGE_SOURCE_AUDIT_REVIEW`). It explicitly loads owned USysAuditXL /
UsysAuditTrail history into labelled readonly cards, preserving physical row
identity, literal Project/Plot ownership and typed NULL/empty/BOOLEAN values.
SQLite EditWhen storage order and rowid ties are displayed as an adaptation.
Load/Cancel/Return do not save the parent, change registry preferences, edit
Restore/Flag, restore records or prune audits. Production wire/client lifecycle
and parent/peer guards pass54 focused/1595 registered frontend executions,
check0/0 and default build; focused Go race5.421s and production compile pass.
Initial backend and exact22-file desktop reviews found no significant issues.
Six accepted/eight normally closed disposable native owners preserve16 files
each and prove mapped normal/CHARS reads, actual parent/child draft barriers,
Cancel/Return/remount/late-result guards, native close, three-width labelled cards
and independent backend/client-off refusal. The1802-artifact seal passes;
complete1602-input race passes exit0 without input/discovery drift. Final
association rehashes1802 artifacts and all eight closed owners, verifies exact
review/base/source lineage and releases the freeze.
The predecessor's legacy ordinary Audit loader refused NULL User/EditWhen before
source mounting; its qualified fixtures used valid values for those fields.
The separate nullable presentation successor above resolves initial-NULL native
launch without changing strict restoration/provenance; integration is accepted.
This is not whole-form or report completion.

Species changes preserve alias precedence and explicit replace/keep/personal-code
decisions; non-ASCII exact selection is allowed without claiming general Access
collation parity. C/C-height NULL Cover6 removes view membership, not the record;
A/A-height missing-cover notices do not infer covers or delete height-only rows.
Source creation does not save personal definitions implicitly.

The shared FS882 XL CHARS option uses exported `SubVegAXL` controls and source
B3/B4/B5 labels bound to existing Cover5a/b/c. Normal A membership already includes
extended-only rows; hiding fields never clears values, drafts or errors. Extended
presentation explicitly uses cover mode, disabling the height toggle until the
option is turned off; height drafts are retained. Totals remain independent,
and the existing source missing-cover notice still checks its original seven
standard A/B cover/total fields, not the extended covers. Shared Species,
Collected, numeric/audit, creation/deletion and parent-A Enter paths retain their
original ownership/lifecycle guards. The checkbox is an in-memory presentation
choice, not a YAML preference write, duplicate menu or complete CHARS-form
equivalence claim. FS1333/SIVI/other-size entrypoints remain unavailable.

Metadata's supplemental typed history is a desktop adaptation: old plaintext
audits cannot establish historical SQLite storage classes. Dedicated restoration
retains provenance and checks the final row after all pruning. Source bulk
environment/SU SQL bypasses form audits; new atomic technical transfer history
does not claim Access bulk-audit parity.

Forward Env Into SU copies Admin.UserSiteUnit through original Env/Admin links
into SiteUnit TEXT255, over the whole selected SU rather than only current plot/
profile navigation. Matching duplicate physical links reject. The SU must be the
owned project file. Reviewed NULL and empty text remain distinct; unrelated
tables/schema/history are independently checked before commit.

Reverse SU Into Env copies selected SU.SiteUnit into Admin.UserSiteUnit TEXT100.
Uniquely matching master then personal definitions supply short/long names
TEXT50/100; personal definitions override master, missing definitions preserve
names, and NULL/empty codes stay distinct. Overlength assignments, ambiguous
definitions and changes to locked/unverifiably unlocked plots reject. These
guards, technical history and selected-SU names instead of source hard-coded
`Sample_SU` are explicit desktop adaptations, not blanket Access execution parity.

A reverse-transfer successor now supports a separately stored
selected SU. It reuses that planner, binds complete typed source rows, schema,
descriptions, paths and opened-handle file identities, and reserves the source
files through the project's atomic Admin/history commit. Its private kernel and
strict UUID/service/context-bound facade each pass independent review and
ordinary compilation, including WAL/rollback writer exclusion and conservative
uncertain-commit recovery. A retained desktop candidate is registered behind
independent default-off `VPRO_EXTERNAL_SU_ENVIRONMENT_EDITING` /
`VITE_EXTERNAL_SU_ENVIRONMENT_EDITING` gates. Coupled focused race70.038s,
20 focused/1565 registered frontend case executions, check0/0 and default build
pass. Exact23-file desktop review finds no significant issues. Four normally
closed disposable Wails owners accept the ordinary outer FS882 boundary: enabled
once-only four-cell/two-row/one-history transfers, unchanged16 support/config/
source files in17-file fixtures, exact readonly lost-response recovery, retained
receipt/UUID remount, actual root/parent barriers and three-width path/hash/control
measurements; backend-off/client-off owners make zero writes. Complete1594-input
race integration and exact1424-artifact/backend/source association pass with
unchanged inputs/discovery and all seven owners closed. That accepted boundary is on the
ordinary outer FS882 toolbar, not inside the nested CHARS complete-entry layout.
That pane intentionally disables conflicting parent workflows; its ordinary-only
predecessor does not qualify the mapped CHARS source action accepted separately
below. After-child-close coverage is not that action parity. Existing same-file APIs and forward
external write refusals are unchanged.

The actual mapped normal/CHARS complete-entry source-action successor is
integrated: it reuses that transfer inline, preserves source-form
attribution with the retained attempt and explicitly refreshes source variants
on Return without replay. Source/peer/child drafts and reference errors remain
guards.43 targeted/1567 registered frontend cases, check0/0, default build and
two isolated Wails builds pass. Independent exact source review finds no
significant issues. Six normally closed disposable owners now accept actual
normal/CHARS mapped action invocation, once-only exact physical changes and
readonly recovery, retained receipts, independent gate refusals, source draft/
reference/native-close barriers and exact current/loaded-peer refresh. The
1869-artifact native seal verifies all nine accepted/rejected owners closed;
failed harness assumptions and same-owner cleanup continuations remain explicit.
Automatic blocked-root reopening has retained-session/production-root test
coverage, not forced native remount coverage. Complete1594-input successor race
passes exit0 in5354.906536s with unchanged inputs/discovery. Final association
rehashes1869 artifacts, source/backend lineage and all nine closed owners; the
freeze is released. The actual mapped source action is integrated without
default promotion.

The normal/CHARS mapped picture child is integrated from a separately reviewed nine-file candidate:
existing owned selected-row JPEG/PNG preview/viewer in child policy behind
`VITE_PICTURE_READING`, one active picture surface and picture activity held by
source/peer lifecycle barriers. No metadataSession, manager view or new mutation
authority is attached.75 targeted/1571 registered frontend executions, check0/0,
default build, owned-picture focused race16.022s and production/isolated Wails
builds pass. Exact follow-up review closes retained-receipt automatic-read and
upstream unavailable-layout findings. Receipt observation is disabled only for
this source child; a default-false layout capability routes only its exact form,
preserving other callers. Six normally closed disposable Wails owners accept
normal/CHARS exact PNG/JPEG previews, owned viewer, NULL/empty physical metadata,
explicit unapproved-directory refusal, held-response Cancel/remount and late-result
rejection, parent/source/native-close barriers and three-width containment.
Joint soil/vegetation owners verify actual peer-draft and pending-read holds.
All19 fixture files remain unchanged per owner. Four rejected zero-write owners
are retained and normally closed. Complete1594-input race integration passes
with no input/discovery drift; final association rehashes2113 artifacts and all10
closed owners and releases the freeze. Source Add,
metadata mutation/deletion and external-directory preview are not enabled.
Both exported two-page Picture Manager buttons lack an OnClick binding despite
private handlers; they remain unavailable, rather than inventing an event path.

The Long Environment preview preserves67 original Env/Admin fields and five heading
rows from `V7mdlReportsEnv.EnvReport`, including labels/punctuation. It groups by
selected SU, not Admin.UserSiteUnit or profile navigation, and adds no quality
filter or summary calculations. Numeric zero, empty text, NULL and historical
storage remain typed; missing Env/Admin retain membership with blank projections.
Deterministic raw ordering, duplicate-membership counts and explicit conflicting/
unsupported name diagnostics replace arbitrary `First()` selection. Reference
candidate IDs, NULL/empty names and duplicates remain distinguishable. Owned
read-only transactions use the original physical project/SU/reference tables,
including external selected-SU files, with cancellation and file/context identity
checks. Navigation reuses Save/Discard/Cancel; native close is guarded during
reads. The configured YAML `ReportOptions.LEReportTitle` is preserved and read
without fallback on malformed configuration. Ordinary title edits affect this
preview only; saved-title preferences are a separate explicit adaptation.
The separately default-off [workbook publication](docs/LONG_ENVIRONMENT_WORKBOOK_CONTRACT.md)
retains the transposed source labels, unit worksheets and original print metadata,
with reversible typed report provenance. Review covers original physical tables
and deterministic XLSX bytes; explicit publication never replaces an existing
file. Read cancellation and noncancellable publication/unknown-receipt barriers
share the existing title, navigation and native-close lifecycle. Twelve Wails
cases/four normally closed acceptance owners,569 frontend tests/check0/0 and
isolated production builds pass. Independent review found no significant issues;
full integrated race passes fresh root1206.963s/all packages against668 unchanged
identities. The [workbook successor](archives/environment-workbook-desktop-checkpoint/evidence-manifest.json)
seals output, restoration and source evidence without production promotion.
The accepted separately default-off vertical maps `USysEnvReportOptions.Form_Load`
and `rptSvTitle_AfterUpdate` to explicit saved-title Load/reviewed Save in the
existing YAML key. The literal source default `Long Evironment Report` is not
corrected. Shared changed-only config-string CAS/atomic commit is extracted from
the accepted Google Earth adapter; report title validation remains Unicode/
no-NUL, not KML XML validation. Unchanged historical literals remain readable for
explicit correction. The vertical passes422 frontend tests, check0/0,
independent review and22 disposable native cases with six closed/restored owners
(21 lifecycle/write cases plus one later read-only representative visual proof).
Uninitialized drafts never infer a title or Undo original; unknown receipts
remain explicitly unknown through acknowledgement, reload and remount.
The enabled two-row title editor preserves deliberate multiline input; an
unchanged raw historical CRLF/NUL title is not normalized or rewritten.
Six explicit application configuration writes changed only planned preference
keys and restored all16 fixture hashes; database/audit writes are absent.
Full402-file Go race integration passes (fresh root885.429s; unchanged packages
may be cached). The [saved-title successor](archives/long-environment-preferences-checkpoint/evidence-manifest.json)
preserves reviewed/native/integration evidence, both responsive visuals and
the unchanged44-file staging/359-file saved Google Earth predecessors.
This does not enable preferences or reports export/summaries in the default
application or promote a production executable.
The preview writes no data, audits or configuration and does not launch
Access/Excel, create export files or implement summary calculations.

The separate [Summary Environment preview](docs/SITE_UNIT_DETAIL_REPORT_CONTRACT.md)
now implements39 source SITE/VEGETATION/SOILS fields for explicitly selected
normal SU, including external-SU owned reads. Set both runtime
`VPRO_SITE_UNIT_SUMMARY=true` and build `VITE_SITE_UNIT_SUMMARY=true` to enable
this disposable-use candidate; both default off. The original form's saved
`SEOptValueMethod` and `SESuType` initialize from retained YAML through owned
read-only configuration/snapshot leases. Saved Interquartile does not silently
become Mean; malformed/missing settings fail without defaults. Saved hierarchy/
field-derived modes remain visible but unavailable, not reset to normal SU.
Changes to Mean/Interquartile are preview-only; Reload restores saved options.
Physical SU/Env/Admin
duplicate weights and excluded memberships remain inspectable. The source's
two disturbance labels both summarize SiteDisturbance2.

Selective disposable ACE measurements establish English numeric Format ties,
comma/NULL Val behavior; installed legacy Excel Quartile confirms inclusive
rank-two calculations. Literal text equality/order, explicit conflicting names
and Go shortest numeric rendering are documented adaptations. Historical text
in numeric fields is refused, never repaired. Species summaries, lifeform
covers and hierarchy/field-derived units remain unavailable. No report read writes databases,
audits, preferences or registry. This is meaningful summary UI progress, not
completion of the wider forms/reports migration.
The newer explicit species-preview candidate requires runtime
`VPRO_SITE_UNIT_SUMMARY_SPECIES=true` and build
`VITE_SITE_UNIT_SUMMARY_SPECIES=true`; both default off. Lifeform grouping also
requires the independent `VPRO_SITE_UNIT_SUMMARY_LIFEFORMS` /
`VITE_SITE_UNIT_SUMMARY_LIFEFORMS` gates. It retains physical duplicate weights,
nullable names, source-formatted strict signed16 thresholds and pre-threshold
group headers. Criteria start at preview-only all plots, AND and zero thresholds,
not saved species preferences, and retain raw drafts/errors across remounts.
Invalid criteria block report/Save/navigation/close until corrected or reset.
Focused service race15.289s and642 frontend tests/check0/0 pass. Isolated
production builds, independent review and57 native cases/six normally closed
owners pass with all16 fixture/22 protected identities unchanged and no
database/audit/config writes. Fresh full race passes root1445.861s/all packages
against732 separately verified unchanged application identities.
Species inclusion never presents layer-only workbook publication.
The implemented extended Summary workbook candidate has independent default-off
`VPRO_SITE_UNIT_SUMMARY_EXTENDED_WORKBOOK` / `VITE_SITE_UNIT_SUMMARY_EXTENDED_WORKBOOK`
gates. It supports lifeform/no-species and layer/lifeform species output through
the existing workbook panel, with explicit criteria, owned review and persistent
irreversible receipts. Existing species/lifeform gates remain additional
requirements. Focused facade publication/collision/cancellation race20.314s, actual222-method/212-model bindings,
649 frontend tests and check0/0 pass. Private integration previously passed
root2620.443s/all packages against736 verified identities, before the public
service/frontend changes. Isolated production builds and independent review pass.
Seven normally closed Wails owners complete22 cases, retaining12 independently
inspected artifacts across all three modes/both methods/species calculations,
visible1400/600 labels, collision/irreversible receipt/remount/read-cancellation
guards and independent gates. All16 fixture/22 protected identities remain
unchanged.741 current source identities/17 exact embedded assets per binary are
separately verified unchanged; fresh all-package race passes root2361.272s/all
packages. The [180-file desktop successor](archives/summary-extended-workbook-desktop-checkpoint/evidence-manifest.json)
is immutably sealed and separately verified.
This is not whole original-report parity. Strata and hierarchy remain unavailable.
The separate Summary Environment XLSX candidate requires runtime
`VPRO_SITE_UNIT_SUMMARY_WORKBOOK=true` and build
`VITE_SITE_UNIT_SUMMARY_WORKBOOK=true`; both default off and remain independent
of the preview and preferences gates. Registered review/publication methods
pass coupled race18.072s with complete raw four-table and retained-scope
approval. Requested Mean/Interquartile is an explicit read-only override;
publication never saves options. Source A/B labels and literal helper strings,
39 field rows, section headers, physical counts and typed provenance are retained.
Integrated frontend passes632 complete tests/check0/0, isolated production builds
and independent review with its root-authority finding resolved.
Four normally closed Wails owners complete14 cases, including exact39 source
cells/provenance, both methods, visible labels, irreversible receipts, collision,
native close/preferences/method overlap guards and independent gates.
All16 fixture/22 protected identities remain unchanged. Fresh714 source identities
and17 exact embedded assets per binary are verified; independent desktop/native
review and full Go race (root1502.269s/all packages) pass. The
[181-file successor](archives/summary-environment-workbook-desktop-checkpoint/evidence-manifest.json)
is immutably sealed and separately reverified.
This bounded desktop candidate is not whole original-report parity.
The initialization successor passes focused/shared race3.125s,22 targeted
frontend tests,444 complete frontend tests/check0/0, isolated builds and12
real Wails cases/three normally closed owners. Actual initialization, saved
weighted quartiles, preview-only edits/Reload, unsupported and malformed saved
settings, held-response cancel/remount/retry and independent denial are verified.
All16 fixture hashes restore; only the driver changes/restores fixture settings.
Full coherent race797.541s/all packages passes against613 unchanged identities;
the [initialization successor](archives/summary-options-desktop-checkpoint/evidence-manifest.json)
preserves the accepted None/summary predecessors without production promotion.

Separately set runtime `VPRO_SITE_UNIT_SUMMARY_PREFERENCES=true` and build
`VITE_SITE_UNIT_SUMMARY_PREFERENCES=true` for reviewed saved-option editing;
both default off. This replaces the original immediate optValueMethod/
optSiteUnitType AfterUpdate writes with explicit Load, reviewed Save and Undo.
Only `SEOptValueMethod`/`SESuType` change, using shared typed changed-only
configuration CAS and atomic replacement. Only normal SU proposals are allowed;
a retained unsupported scope changes only after explicit normal-SU selection.
No plots, audits or registry are written. Drafts/errors/receipts survive remounts.
Save is noncancellable; committed warnings retain their receipt, and lost or
contradictory acknowledgements remain unknown through acknowledgement and Reload.
No automatic replay is allowed. Thirteen Wails cases/three normally closed
owners verify four actual preference writes, no-op, independent collision,
held committed/lost receipt recovery, restoration and responsive labels. All16
fixture hashes restore. Combined Go race integration passes791.377s/all packages
against626 unchanged source identities;461 frontend tests/check0/0 also pass.
The immutable combined successor is
`archives\forms-reports-shared-checkpoint\evidence-manifest.json`.

The separate [standalone Lifeform Summary](docs/LIFEFORM_SUMMARY_CONTRACT.md)
uses runtime `VPRO_LIFEFORM_SUMMARY=true` and build `VITE_LIFEFORM_SUMMARY=true`;
both independently default off. Reports / Vegetation / Lifeform Summary provides
an owned, cancellable normal-SU preview, without changing generic Summary
Vegetation availability. It preserves both SINGLE assignments, physical duplicate/
cross-unit joins, NULL versus empty unit identities and zero presence versus
NULL mean. Definition and short-name columns supplement the original numeric
lifeform and raw catalogue label. Zero-denominator NULL ratios, literal binary
identities and physical presentation order are disclosed adaptations. No data,
audit, preference, source temporary table or Excel writes occur.
Focused race12.661s,554 frontend tests/check0/0, actual interface bindings and
exclusively embedded production builds pass. Eleven Wails cases/four normally
closed owners verify independent physical arithmetic,1400/600px actual visibility/
scoped headers, held-read navigation/native-close cancellation/remount/retry,
independent gates and FS882 valid/invalid/discard draft transitions. All16 owned
and protected identities remain unchanged. Full coherent race passes fresh
root1122.770s/all packages against644 unchanged source identities. The immutable
successor is `archives\lifeform-summary-desktop-checkpoint\evidence-manifest.json`;
this does not complete the wider forms/reports migration.

The [combined Lifeform Summary workbook](docs/LIFEFORM_WORKBOOK_CONTRACT.md)
adds independently default-off `VPRO_LIFEFORM_WORKBOOK` /
`VITE_LIFEFORM_WORKBOOK`. One owned five-table snapshot feeds the two different
accepted planners. Six count summaries are always included; six explicit
boolean choices control only optional details. Review binds complete raw source,
typed unit/sheet mappings and deterministic XLSX bytes. Explicit new-destination
publication never overwrites and retains unknown/committed-error receipts until
acknowledgement; it writes no database, audit or configuration.
Focused race222.040s,617 frontend tests/check0/0, actual bindings and isolated
enabled/frontend-off production builds pass. Independent code review found no
significant issues. Four normally closed Wails owners verify physical source
arithmetic, exact published cells/styles/provenance,1400/600px visible labels,
collision refusal, held irreversible known/lost/warning receipts, native-close/
navigation/read barriers, read cancellation/remount/retry and independent gates.
All16 fixture/22 protected identities remain unchanged. Native cancellation holds
response delivery after backend completion, not in-flight SQL. Two guarded
harness recoveries corrected source-label and post-acknowledgement option-reset
assumptions without replaying either committed output. Fresh all-package race
passes root1692.501s against703 unchanged native-tested source identities; each
binary embeds its exact17 isolated assets. The bounded successor is
`archives\lifeform-workbook-desktop-checkpoint\evidence-manifest.json`;
hierarchy and wider reports are not enabled by this workflow.

The [Long Vegetation Lifeform grouping](docs/LONG_VEGETATION_LIFEFORM_CONTRACT.md)
successor separately enables retained LVGroupBy3 using runtime
`VPRO_LONG_VEGETATION_LIFEFORM=true` and build
`VITE_LONG_VEGETATION_LIFEFORM=true`; both default off in addition to the existing
report presentation gate. This is the source13/23 conversion, not standalone
Lifeform Summary: master/personal five-cell UNION, extended covers/omitted Cover10,
actual99.9 cap/three SINGLE boundaries and later separate master/LayerCode joins.
Typed nullable INTEGER groups, source physical/quality weights, averages,
constant-list NULL/name matches and numeric lifeform ordering are preserved.
No unavailable grouping silently becomes Layer or resets the stored option.
Focused coupled race46.207s,556 frontend tests/check0/0 and exclusively embedded
builds pass. Eleven native cases/three closed owners verify exact independent
physical report arithmetic, orders/averages/constant lists, responsive labels,
held-read cancel/remount/retry and independent gates; all16 owned/protected hashes
remain unchanged. Full coherent race passes fresh root1038.681s/all packages
against650 unchanged source identities. Immutable successor:
`archives\long-lifeform-desktop-checkpoint\evidence-manifest.json`; wider reports/
forms, additional summaries and publication remain outstanding.

The [Long Vegetation typed Strata successor](docs/LONG_VEGETATION_STRATA_CONTRACT.md)
adds source12/22 with independent runtime `VPRO_LONG_VEGETATION_STRATA` and build
`VITE_LONG_VEGETATION_STRATA`, default off. Original TotalA/B override fallback
cap99; B includes extended covers; C/D retain non-NULL zero/negative cover.
The earlier private projection/provenance tests remain unchanged; a runtime
wrapper represents held/inserted SINGLE assignments and physical Strata labels.
Strict numeric/NULL storage and ordered addition are explicit adaptations,
**not recovered Access Nz/Val text/locale/numeric-to-string calibration**.
Focused coupled race68.208s,558 frontend tests/check0/0 and isolated builds pass.
Eleven Wails cases/three closed owners prove independent physical arithmetic,
orders/averages/constant lists,1400/600px labels, held-read cancel/remount/retry and
independent gates; all16 owned/protected hashes and651 source pins are unchanged.
Full coherent race passes fresh root988.758s/all packages against651 unchanged
source identities. Immutable successor:
`archives\long-strata-desktop-checkpoint\evidence-manifest.json`.

The [supplementary Code successor](docs/LONG_VEGETATION_CODE_CONTRACT.md) implements
retained LVShowEnglishName2 behind independently default-off runtime
`VPRO_LONG_VEGETATION_CODE` and build `VITE_LONG_VEGETATION_CODE`.
Original master Code cells, their separate source grouping and constant-list
match semantics are preserved without eight-character truncation, normalization
or missing-master fallback. The Lifeform conversion UNION remains unmodified;
only its later display join changes. Public settings/bindings add explicit
showSpeciesCode, mutually exclusive with showEnglishName; existing supplementary
row slots are labelled List Code/Matched report Code. Source4 selected attributes
remain unavailable rather than gaining an inferred join.
Coupled race83.025s,560 frontend/check0/0, proper interface bindings and isolated
builds pass. Twelve Wails cases/three closed owners verify physical arithmetic/
constant fanout, unchanged Lifeform UNION, actual labels, held-read lifecycle and
independent Code gates; all16 fixture/protected hashes and653 pins are unchanged.
Full coherent race passes fresh root1154.074s/all packages. Immutable evidence:
`archives/long-code-desktop-checkpoint/evidence-manifest.json`.

The separate [species attribute summaries](docs/SPECIES_ATTRIBUTE_SUMMARY_CONTRACT.md)
are implemented in the Lifeform Summary host with independent default-off
`VPRO_SPECIES_ATTRIBUTE_SUMMARY` / `VITE_SPECIES_ATTRIBUTE_SUMMARY` gates.
SRank, Wetland Indicator, Weed Status, Red Blue List, Est. ASMR and Climate use
the original raw physical Veg/SU/USysSppAttributes joins, not MAX-reduced cover
observations or the unresolved Long source4 join. Repeated rows/definitions retain
their weights; totals include non-pivot values, distinct plot counts remain
separate and absent grouped/pivot sums remain NULL. All six details are displayed
without applying or writing saved detail flags. Coupled race18.691s,564 frontend/
check0/0, proper interface bindings and isolated builds pass. Nine Wails cases/
three normally closed owners verify all72 raw joins/five units/six query families,
actual1400/600px labels, shared held-read/close/cancel/remount/retry and independent
gates; all16 hashes restore. Full coherent race passes fresh root1166.610s/all
packages against660 unchanged source identities. Immutable successor:
`archives/species-attribute-summary-desktop-checkpoint/evidence-manifest.json`.
Hierarchy breaks,
generic Summary Vegetation, selected Long attributes and publication stay disabled.

The existing SIVI parent panel also has thirty-two independently gated live controls:
AirPhotoNum, XCoord, YCoord, Tree/Shrub/Herb/Moss covers, VegNotes and four SOIL
measurements (HumusThickness, SeepageDepth, RootingDepth, RootRestrictingDepth),
plus SiteSurveyor, FieldNumber, Location, NtsMapSheet, UTMZone, PlotRepresenting,
SiteSeries, MapUnit and SiteNotes. Source Notes binds literal `siteNotes`, explicitly
mapped to physical SiteNotes. Its multiline editor preserves unchanged historical
CRLF; deliberate LF drafts retain their literal content. Source UTF-16 text bounds
are enforced without trimming, completion, case changes or empty-to-NULL conversion.
UTMZone remains text; SiteSeries does not inherit XL/BGC callbacks.
The added source numeric fields are UTMEasting/Northing, Latitude/Longitude,
LocationAccuracy, Elevation, SlopeGradient, Aspect, StandAge and Admin.StartDate
("Yr."). Singles retain precision within their finite range; DAO Integer fields
use signed16/canonical INTEGER, corroborated by exporter and retained Type3/Size2
evidence. No positive-only, slope/aspect/year limits or metadata/estimation effects
are invented. Geographic DD ±90/±180 validation is an explicit desktop adaptation;
stored signs/full precision remain direct, not west-inverted or Fixed6-rounded.
Env.Date uses complete exact wallclock `YYYY-MM-DD HH:MM:SS[.fraction]` text,
0100..9999 with up to9 fractional digits, or explicit NULL. This replaces source
Medium Date presentation intentionally to retain time/precision; no timezone,
localized coercion, trimming or date-only completion.
Enable runtime `VPRO_SIVI_PARENT_SHARED_EDITING=true` and build
`VITE_SIVI_PARENT_SHARED_EDITING=true` only with parent review enabled.
They reuse scoped SIVI transactions and proven ordinary-field validation, not
whole-header Save or XL callbacks. Corresponding read-only copies disappear only
when owned controls load. Shared drafts serialize sibling/child editors and join
Save/Undo/Lock/close/navigation and physical owner replacement. Twelve native
cases/four normally closed owners verify invalid remount/error gates, one
eleven-field mixed numeric/date Admin/Env Save and typed audit prune, held read cancellation/
retry, actual wide/narrow labels and independent gates. HumusThickness retains
its certified Admin row; the three depths store canonical signed INTEGER cells,
not guessed nonnegative depths or REAL values. All original data/audit tables restore;
the closed disposable file is additionally restored byte-for-byte from its
preserved baseline, retaining the closed event/history evidence. A real raw-SDK
lone-surrogate request is rejected before decoder repair, without data/audit/history
writes. Nine invalid drafts survive remount; malformed memo blocks Lock/native close.
This is32 bounded controls, not all62 shared bindings or complete FS1333 parity.
Focused race38.211s,503 frontend tests/check0/0, real-wire tests and isolated
production Wails builds pass;32-field integration passes941.447s/all packages
against630 unchanged identities. Immutable successor:
`archives\sivi-numeric-date-desktop-checkpoint\evidence-manifest.json`.
The21-field predecessor full integration passes800.909s/all packages
against626 unchanged identities. Immutable successor:
`archives\sivi-text-desktop-checkpoint\evidence-manifest.json`.
The twelve-field predecessor passes795.293s/all packages; immutable successor:
`archives\sivi-soil-desktop-checkpoint\evidence-manifest.json`.

The native-accepted reference continuation implements28 additional shared bindings behind
independent default-off `VPRO_SIVI_PARENT_REFERENCE_EDITING` /
`VITE_SIVI_PARENT_REFERENCE_EDITING` gates, in addition to parent review/shared
editing. Four changed non-NULL fields require exact listed values
(Exposure1/2, SoilDrainage, SuccessionalStatus); the other24 retain bounded literal
free entry with nonblocking unmatched advisories. Suggestions never complete,
recase or create catalogue rows. NULL Zone produces no SubZone suggestions;
Zone changes never clear SubZone or change SiteSeries. Six catalogues retain the
owned imported VLists family's ten raw metadata cells and physical rowids,
including NULL definitions, rather than flattened legacy Lists or invented DAO
provenance. The other22 reuse verified catalogue readers. The UI has32 controls
with the new build gate off and60 when enabled; accepted32-field Save remains
available when reference reads are denied. Focused race54.400s,546 frontend tests,
check0/0, actual emitted wire tests and isolated production candidates pass.
Four normally closed native owners complete14 cases, including one29-field
reference/Admin Save and typed prune restoring every original data/audit table,
raw catalogue metadata, invalid remount, direct SDK membership refusals,
independent gates, held reference cancellation/retry and1400/600px visible labels.
All16 fixture hashes restore; closed history is preserved before disposable byte
restoration. An audit-suffix harness assumption was corrected under the same
owner without replaying Save. Full integration passes996.700s/all packages against
635 unchanged identities; immutable successor:
`archives\sivi-reference-desktop-checkpoint\evidence-manifest.json`.
See [the source contract](docs/FS1333-parent-access-contract.md) for exact bounds,
provenance and remaining form boundaries. This is not complete FS1333/forms parity.

Long Vegetation also supports the source's `LVGroupBy: 4` ("None") with the
separate runtime gate `VPRO_LONG_VEGETATION_NONE=true`, default off. The existing
Long Vegetation UI/backend gates are still required. This is **ordering, not
pooling**: `V7mdlReportsLongVeg.GroupAndOrder14/24` still converts and groups by
layer, but orders nonconstant rows by species, or presence descending then
species, without putting layer first. Statistics, physical memberships and
layer names remain unchanged. The source constant-species-list query retains
layer/species ordering even in None mode. Equal source keys receive the existing
deterministic name/layer tie break; this is not an Access collation claim.

The transport identifies `grouping` explicitly as `layer` or `none`. With the
None gate off, both options loading and report reads refuse retained mode4
without resetting configuration. Modes2/3 remain unavailable. Focused/shared Go
race9.784s,442 frontend tests, check0 errors/warnings, isolated enabled/default
frontend and production Wails builds and independent review pass. Nine real
Wails cases cover unchanged complete row multisets/statistics, both nonconstant
orders, the constant-list exception, visible responsive labels and independent
default/backend refusal. Three normally closed disposable owners restore all16
fixture hashes; only the test driver changes/restores options, not the application.
Full integration and the immutable successor are recorded in the migration plan.

## Feature gates

These are build-time Vite flags, not user permissions or hidden backend grants.
Keep independent gates; do not turn on unavailable workflows as part of cleanup.
Set default-on flags to `false` for read-only opt-out; set default-off flags to
`true` only for their verified experimental surfaces.

| Default | Flags |
| --- | --- |
| On: parent groups | `VITE_COORDINATE_EDITING`, `VITE_BEC_EDITING`, `VITE_WORKING_UNIT_EDITING`, `VITE_MASTER_BEC_EDITING`, `VITE_QUALITY_EDITING`, `VITE_SUBSTRATE_EDITING`, `VITE_SITE_CODES_EDITING`, `VITE_REGION_CODES_EDITING`, `VITE_SOIL_CODES_EDITING`, `VITE_GEOLOGY_CODES_EDITING`, `VITE_PARENT_CODES_EDITING`, `VITE_ORDINARY_PARENT_EDITING`, `VITE_PARENT_FLAGS_EDITING`, `VITE_SOIL_DRAINAGE_EDITING`, `VITE_ADDITIONAL_PARENT_EDITING` |
| On: child groups | `VITE_OTHER_EDITING`, `VITE_SOIL_CHILD_EDITING`, `VITE_VEGETATION_ATTRIBUTE_EDITING`, `VITE_VEGETATION_COLLECTED_EDITING`, `VITE_VEGETATION_SPECIES_EDITING`, `VITE_HEIGHT_EDITING`, `VITE_VEGETATION_NUMBER_EDITING` |
| On: bounded infrastructure | `VITE_EXTERNAL_PROJECTS`, `VITE_AUDIT_RESTORE` |
| Off: lifecycle/checking | `VITE_VEGETATION_CREATE_EDITING`, `VITE_VEGETATION_DELETE_EDITING`, `VITE_PERSONAL_SPECIES_EDITING`, `VITE_SPECIES_CODE_CHECK_EDITING` |
| Off: metadata | `VITE_PROJECT_METADATA_EDITING`, `VITE_PROJECT_METADATA_CREATION`, `VITE_PROJECT_METADATA_TEMPLATE_CREATION`, `VITE_PROJECT_METADATA_RESTORE` |
| Off: navigation/transfer | `VITE_SOURCE_ENTER_NAVIGATION`, `VITE_SOURCE_PLOT_FIND`, `VITE_SOURCE_ENV_SU_TRANSFER`, `VITE_SOURCE_SU_ENV_TRANSFER` |
| Off: profile rules/results | `VITE_PROJECT_PLOT_PROFILE_REVIEW`, `VITE_PROJECT_PLOT_PROFILE_RUN`, `VITE_PROJECT_PLOT_PROFILE_EDITING`, `VITE_PROJECT_PLOT_PROFILE_CREATION`, `VITE_PROJECT_PLOT_PROFILE_DELETION`, `VITE_PROJECT_PLOT_PROFILE_FILTERING`, `VITE_PROJECT_PLOT_PROFILE_SAVE_SU`, `VITE_PROJECT_PLOT_PROFILE_SAVE_SU_PROJECT` |
| Off: stored profile ownership | `VITE_PLOT_PROFILE_SELECTION`, `VITE_PLOT_PROFILE_WRITE_OWNERSHIP`, `VITE_PLOT_PROFILE_FILE_CREATION`, `VITE_PLOT_PROFILE_TABLE_CREATION` |
| Off: read-only catalogue | `VITE_SOIL_CODES_REFERENCE` |
| Off: read-only reports | `VITE_LONG_ENVIRONMENT_REPORT`, `VITE_LONG_VEGETATION_REPORT`, `VITE_SITE_UNIT_SUMMARY` |
| Off: reviewed report workbooks | `VITE_LONG_ENVIRONMENT_WORKBOOK` (also requires runtime `VPRO_LONG_ENVIRONMENT_WORKBOOK=true`) |
| Off: reviewed summary preferences | `VITE_SITE_UNIT_SUMMARY_PREFERENCES` |
| Off: reviewed Long Vegetation title preferences | `VITE_LONG_VEGETATION_PREFERENCES` (also requires runtime `VPRO_LONG_VEGETATION_PREFERENCES=true`; no other report option is enabled) |
| Off: reviewed extended Summary workbooks | `VITE_SITE_UNIT_SUMMARY_EXTENDED_WORKBOOK` (also requires runtime `VPRO_SITE_UNIT_SUMMARY_EXTENDED_WORKBOOK=true` and corresponding species/lifeform gates) |
| Off: shared SIVI fields | `VITE_SIVI_PARENT_SHARED_EDITING` |
| Off: standalone SIVI entry presentation | `VITE_SIVI_STANDALONE` (also requires `VITE_SIVI_PARENT_REVIEW`; existing runtime parent/child gates still apply independently). [Bounded native/integration acceptance](docs/FS1333-parent-access-contract.md#standalone-entry-presentation); combined cover/height requires its separate gates and full-event execution remains unavailable |
| Off: extended shrub presentation | `VITE_EXTENDED_SHRUBS` |

`VITE_HEIGHT_EDITING=false` also disables numeric vegetation editing.
Master edits additionally require source authorization; reference membership,
availability and physical validity remain separate checks. Profile sub-actions
require their parent workflow/selected ownership, not just a flag.

## Run and validate

Install Go, Node/npm and platform Wails prerequisites:

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.26
wails3 dev
```

The native development window uses Vite at `http://127.0.0.1:9245/`.
A browser preview cannot prove Go bindings/lifecycle. Release: `wails3 build`.
Windows executable/tool paths are in [WINDOWS_HANDOFF.md](WINDOWS_HANDOFF.md).

```sh
go test -race -timeout 75m ./...
cd frontend
npm test
npm run check
npm run build
```

On Windows use `npm.cmd`. Focused tests during implementation; full validation
at integration. New frontend tests must be added to explicit package selectors.
Use native Wails with disposable data/config for binding/lifecycle acceptance.

## Repository organization and distribution

The root Go files currently share the application package and its internal
ownership/transaction contracts. Keep white-box `_test.go` files beside their
package; moving them wholesale into a separate test directory would change
which internals they can verify, not simplify the application. Frontend client
tests remain beside their controllers/components. Shared preference lifecycle
behavior already lives in the retained session/registry helpers: another generic
registry would duplicate those boundaries. Extract a real package only when it
provides a demonstrated dependency boundary for a workflow, not to reach a file
or helper count. Bundle size and native responsiveness are separate measurements
from test volume; no features or availability gates were removed in this pass.

The manual [release workflow](.github/workflows/release.yml) accepts a validated
SemVer and independent draft/prerelease flags. It requires all fourteen packages:
Windows amd64/arm64 EXE, ZIP and NSIS installer; macOS universal DMG and app ZIP;
Linux amd64/arm64 AppImage, deb and tarball. Native Linux runners avoid an
unprepared cross-build image. Windows uses cgo with a pinned, checksum-verified
target compiler; macOS packages the universal app without rebuilding a host-only
binary. [Release validation](scripts/release.py) checks exact assets, packaged
CPU witnesses, checkout/version provenance and SHA256 checksums, then verifies
the remotely attached names and downloaded bytes. Existing remote lightweight
or annotated tags must resolve to the built checkout before release creation;
a tag belonging to different source refuses publication. Version metadata preparation
is restricted to a fresh CI checkout, not local migration data/settings.

SQLite remains canonical on every target. The optional
[DuckDB driver boundary](duckdbdriver.go) preserves existing supported-platform
linkage; [Windows ARM64](duckdbdriver_windows_arm64.go) explicitly refuses that
optional coordinator because its upstream driver has no library for this target.
This does not make DuckDB extensions an offline-startup dependency.

For a smaller Windows demo executable, use the explicit SQLite-only profile:
`wails3 task windows:build:sqliteonly`. It retains the normal production stripping
flags and all canonical SQLite workflows, but does not link the optional DuckDB
driver. Additional feature tags can still be passed through `EXTRA_TAGS`; this
profile does not enable experimental workflows. The equivalent Go build tag is
`production,sqliteonly`. Normal builds and the release workflow retain their
existing optional-driver platform policy.

With identical frontend assets on Windows amd64, the stripped profile measured
38,410,752 bytes versus 87,768,576 bytes for the normal stripped executable
(56.2% smaller). A disposable native Wails first installation reached readiness
in about 1.5 seconds; sample project state/paging reads returned unchanged data.
These measurements include harness overhead and are not general performance
guarantees. The omitted optional coordinator explicitly reports the build-profile
restriction instead of claiming Windows ARM64 on an amd64 machine.

The [download portal](docs/index.html) uses
[exact asset and architecture routing](docs/downloads.mjs), including ARM64
client hints. Unknown architecture is never guessed as amd64; missing packages
stay disabled. It shows latest **stable** releases, not drafts/prereleases,
and provides the complete release list and checksum/installation guidance.
Preview Windows packages may be unsigned; macOS packages are ad-hoc signed,
not Apple-notarized. Linux packages still require GTK4/WebKitGTK 6.0.

Local checks include `python -m unittest discover -s scripts/tests -p 'test_release*.py'`
and `node --test scripts/tests/downloadPortal.test.cjs`, plus actionlint.
Actual-browser checks cover exact ARM64 selection, all fifteen asset links,
1400/600/320-width containment and unpublished-release feedback. These checks
are supplemented by pinned Windows production compiler/link and actual Wails
interface generation checks. A disposable pinned-GCC production Wails owner also loads
all52 Sample plots across25/25/2 pages, independently matches reopened SQLite
and closes normally without application mutations. These results
are not evidence that hosted macOS/Linux/ARM packaging or public publication
has run. The audited baseline has zero release runs/releases; Pages failed at
site setup. [Pages deployment](.github/workflows/pages.yml) now explicitly attempts
enablement and surfaces permission failures; a successful authorized deployment
is still required. No push, release dispatch or repository-settings mutation
was performed during this repair.

## References and next work

- [Migration plan](MIGRATION_PLAN.md): ordered deliverables and bounded next round.
- [Client scope/domain index](docs/CLIENT_SCOPE.md): page-cited scope decisions,
  uncertainties, companion R boundary and help/manual reference locators.
- [Access contract](docs/FS882-6x4XL-access-contract.md): source/measured behavior
  and explicit adaptations; not proof of all workflows.
- [Development instructions](AGENTS.md): source-first safety and work budget.
- [Release workflow](.github/workflows/release.yml) /
  [download portal](docs/index.html): distribution surfaces, not replacement
  acceptance for every platform.

The shared [unit-name resolver](reportunitnames.go) preserves all physical
reference candidates, including duplicate, NULL and empty definitions. A unique
non-NULL literal supplies the long name; conflicting or unsupported definitions
remain explicit rather than inheriting Access's arbitrary `First()`. Unassigned
units retain the source's empty name and never join a NULL master code.
[Transport validation](frontend/src/reportUnitNames.ts) independently derives
the status from the candidates; both reports reuse it. Five native cases verify
literal Unicode/NULL presentation, actual expanded candidate visibility,
ambiguity after a reference change, unchanged statistics/denominators and exact
restoration of all17 disposable files. Application writes:0.
Focused Go race22.553s, full integration race662.139s, frontend286 tests,
final targeted9/check0/0, isolated build and independent review pass.

Next: source-defined Long Vegetation strata preparation, FS1333/two-page
implementation and priority reports/locations. Constant-species-list versus
species-threshold behavior needs an explicit scope decision before report expansion.
Plot quality uses the original `DataQuality` ranking, not the differently ordered
`PlotQualitySite` editor choices. The [quality qualifier](vegetationreportquality.go)
now feeds the same owned read snapshot and weighted layer calculation, without
inventing physical SU IDs or switching the selected SU. It retains original
ENV/Admin/reference IDs and raw quality values, three-way reference fanout,
complete minimum definitions and exclusions. NULL toggles distinguish absent
references from non-domain-only matches. Default-init legacy `"True"`/`"False"`
NULL flags are decoded without rewriting stored YAML types; unused BEC settings
are preserved, not applied as an invented predicate.
Read-only criteria/provenance and independently validated frontend denominators
distinguish qualified occurrences from retained physical rows. Conflicting,
NULL, fractional or out-of-Integer minimum definitions and ambiguous parent
links fail explicitly rather than guessing source `First()`/rounding/DISTINCT.
Broader Access locale collation remains unverified.
Full Go race718.522s, focused race40.062s, frontend290/check0/0 and isolated
production builds pass. Independent reviews resolved a threshold-completeness
finding. Eleven native cases match an independent SQLite translation of QCLV,
including reference multiplicity, NULL toggles, invalid/empty qualification,
cancellation/retry, busy-close and remount. A final two-case native wording proof
corrects the average label to the qualified denominator when enabled; the earlier
calculation proof is retained rather than replayed. Both default-disabled proofs
pass. Zero application writes; all17 fixture files restored; no default promotion.
The quality successor is `archives\long-vegetation-quality-checkpoint`; earlier
UI/name seals remain immutable. This is native Wails plus source-SQL evidence,
not a full native Access/Excel report or an option editor/publication workflow.
Static form/report
contracts alone do not prove native execution parity. The historical agreement's
cloud/public-map/BECWeb/publication tracks remain visible but separately scoped.
Private native evidence, original documentation snapshots and closed-build hash
maps are retained outside app assets; no client PDFs or temporary tools are bundled.

The original private Strata projection is now reused by the separately gated
typed-numeric desktop preview described above. Its MAX/provenance/alias/
cancellation tests are preserved; live held/inserted SINGLE assignments are
explicitly wrapped and tested. Access `Nz`/`Val` text/locale/numeric-to-string
conversion remains uncalibrated, not replaced by the passing numeric proxy.
Two bounded original-expression attempts were stopped: standalone DAO cannot
resolve Access `Nz`, and the owned Access attempt asserted closure too early,
before retaining its results/owner receipt. A subsequent read-only process check
found no live Access process; it is not a recovered numeric acceptance receipt.
No source database was opened. Typed Strata and the distinct Lifeform preview
remain independently default-off; do not claim the missing Access results.

The independent next source boundary packages original FS1333/SIVI metadata:
[normal parent/children](resources/fs1333-sivi-layout.json) and
[extended A child](resources/fs1333-extended-shrub-layout.json).
The unchanged extractor preserves hashes, labels, event paths and three
PlotNumber child links. A exposes HeightA/HeightB, C exposes Height6 and D has no
height editor; HeightB is Access TEXT(255), canonical SQLite VARCHAR, not a
numeric height inferred from its label. Extended A adds only Cover5a/b/c with
B3/B4/B5 labels, retaining aggregate heights; parent CHARS drift is not imported.
Source A/C membership remains cover-driven, so height-only rows are not silently
made visible. Source after-update cover feedback is not an invented aggregate
sum; code after `Exit Sub` is not an active copy/update requirement.
Three fixture tests and focused race1.339s pass; unchanged extractor package
race1.333s/1.361s passes. Independent review confirmed the corrected test
declaration and no significant remaining source-alignment issues. The20-file
static successor is sealed at `archives\fs1333-source-checkpoint`; subsequent
full Go integration race806.255s passes. Everything
remains read-only/unmapped: this is neither a usable FS1333 entrypoint nor native
behavior acceptance. The next implementation must preserve nullable text HeightB
and numeric HeightA/Height6 using existing guards/owned transaction patterns.
The private [typed SIVI projection](sivivegetation.go) and
[owned reader](sivivegetationread.go) now preserve those raw values, physical
identities and original cover-driven memberships, including extended-only rows
and hidden extended values in normal mode. Historical invalid HeightB storage
is retained, not converted into a numeric height. The unexported context reader
reuses the existing pinned read snapshot/lease and literal per-plot filter.
Focused/coupled race28.661s and independent reviews pass, with real blocked-read
cancellation/retry and unchanged data/support bytes. Full integration race838.967s passes;
no new public bindings, UI, source-form mutation authorization or native/editor
acceptance is implied by this private read boundary.
The private [aggregate-height planner](sivivegetationedit.go) validates current
source-form membership, physical row identities and exact expected storage.
It preserves NULL/empty/literal HeightB text (255 UTF-16 units) and nullable
HeightA/Height6 numbers within the existing float64-preserving SINGLE-range
adaptation. Unchanged historical invalid cells create no assignments.
Focused/coupled race10.995s and independent review pass; this is planning only,
not SQL mutation authorization or an enabled editor. Owned audited writes,
typed restoration and native lifecycle acceptance remain outstanding. The
original species-metadata helper bypasses its mutation body, so its invocation
does not justify new metadata writes or implicit totals.
The private [owned writer](sivivegetationwrite.go) and
[typed restoration](sivivegetationrestore.go) now implement atomic source-scoped
height changes using existing audit strengths, explicit physical/application
identities, typed history and independent complete-project verification.
Unchanged invalid storage remains untouched; historical BLOB corrections are
explicitly unavailable. Restoration preserves proven original values, validates
raw physical audit cells (not just normalized boolean DTOs), handles the explicit
Veg table aliases, stores retained Restore=true as -1, prunes only selected
audits and rejects replay. Focused/coupled race33.503s passes; independent review
and raw-audit fix closure pass. Full integration race891.433s passes. That private
checkpoint does not establish live form, editor or native acceptance.
The [SIVI presentation/draft model](frontend/src/siviHeightEditor.ts)
prepares canonical A/C/D transport checks and persistent original/raw height
drafts using existing shared helpers. It independently verifies cover-driven
membership and cross-group physical cells, preserves NULL/empty/literal text,
and changes only A source identity for extended presentation.294 frontend
tests/check0/0 and an isolated production build pass with independent review.
The27-file pure model successor is sealed; it does not enable a mounted component,
native lifecycle or full FS1333 form.
The separate [height-only ContextService facade](siviheightservice.go) adds
caller-context-aware source read, atomic Save and typed restoration methods with
the original named/tagged JSON shape. Every method is disabled by default.
The temporary acceptance-only runtime opt-in `VPRO_SIVI_HEIGHT_EDITING` must be
the literal `true`; unset or literal `false` disables it, and any other present
value stops startup explicitly before project initialization. The gate is set
once before service registration, not persisted or imported into user YAML.
Focused/coupled race32.765s and actual Wails binding generation/type-check pass
(15 services/171 methods/130 models/two enums). All294 frontend tests and an
isolated production build pass; full Go race integration passes798.314s.
A production-tagged Wails core compiles, and an actual malformed runtime gate
exits1 before data/config initialization. Direct facade review found no new
gate/transport/delegation issue; it is not independent native acceptance.
The facade's earlier standalone checkpoint did not establish native acceptance.
The [SIVI session owner](frontend/src/siviHeightSession.ts) adapts
the actual nullable generated models and keeps original/raw drafts outside
component lifetime. Explicit original reload cannot discard drafts; failed
uncommitted operations retain them. Missing/malformed acknowledgements and
committed cleanup/refresh failures block retry/close until explicit Undo/reload,
without replaying mutations. Typed retain/prune restoration keeps only this
session's verified history and checks the exact existing result fields.
Disposed owners cannot schedule new reads or refresh a remounted parent.
The responsive [height-only panel](frontend/src/SIVIHeightPanel.svelte) is now
mounted in the FS882 vegetation workspace behind the independent default-off
`VITE_SIVI_HEIGHT_EDITING=true` build gate. Backend authorization still requires
`VPRO_SIVI_HEIGHT_EDITING=true`; the existing `VITE_EXTENDED_SHRUBS=true` gate
controls the shared normal/extended presentation option. These are acceptance
flags, not imported or persisted user preferences.
The parent owns drafts outside tab lifetime and replaces the ordinary vegetation
presentation while this panel is shown, retaining one live control per height.
Save, Undo, Lock, other editors and context/window transitions share that owner.
Invalid raw drafts and unresolved acknowledgements also block discard, while
explicit Undo/reload remains available. HeightB uses a textarea for literal
multiline text and a separate NULL control; unchanged original values remain
omitted from assignments.
All311 frontend tests, check0 errors/warnings and isolated default-off/opt-in
production builds pass. Disposable native evidence covers14 cases: generated
RPC/default denial, source groups, remount errors, normal/extended identity,
actual600px visibility/labels, atomic audited Save, typed prune restoration,
UTF-16 bounds, native WM_CLOSE/context barriers, consumed-history retry,
NULL versus empty storage and an injected incomplete committed acknowledgement.
Explicit recovery reloads that committed value without replay. All original
source tables and historical audits are restored; consumed
`__VPRO_SIVIHeightHistory` receipts remain in sealed evidence, while the closed
disposable fixture is byte-restored,
and support/configuration bytes stay unchanged. Protected builds, embedded
frontend assets and canonical Sample are not replaced.
This accepts the bounded opt-in height adaptation, not the complete FS1333
entrypoint, cover/species/parent workflow parity or default promotion.
The [distinct parent contract](docs/FS1333-parent-access-contract.md) now traces
the original inner Env/Admin join,77 direct bindings and nine active events.
It distinguishes implicit BOOLEAN/plot-type options and ProjectID source/
metadata paths from XL. Existing YAML retains ProjectIdSource default2 but does
not implement that preference. Source defects and CHARS parent drift are
recorded rather than imported. Focused source race1.356s and full Go integration
race797.009s/all packages pass; all parent metadata remains read-only/unmapped
and full parent editing is unavailable. A subsequent representative visual
review corrected misleading XL cover-mode guidance while SIVI is shown:
accurate SIVI guidance is below fields, safety feedback stays above, and ordinary
XL behavior is unchanged.311 frontend tests/check0/0/build and a separate
read-only native600px verification pass without replaying accepted writes.
The private [parent model](siviparent.go) and [owned reader](siviparentread.go)
now preserve77 metadata-derived direct bindings, a separate implicit
SpeciesListComplete target, raw Env/Admin storage/types and physical duplicate
pairs through the pinned coordinator. Membership is explicitly the SQLite
`literal-binary-inner-pairs` adaptation; Access Unicode collation/DISTINCTROW
equivalence is not claimed. [Fixture tests](siviparent_test.go) cover clone
isolation, orphans, invalid history, external projects, cancellation/retry and
zero database/config writes. Focused race9.051s and full Go integration
race778.680s/all packages pass; independent read-only review found no significant
issues. No public API, bindings or parent workflow was enabled.
Selective disposable DAO evidence now confirms source joins match case,
trailing spaces and composed/decomposed accents where literal BINARY does not,
and all-table DISTINCTROW preserves four identical physical2x2 pairs.
The [parent contract](docs/FS1333-parent-access-contract.md) records these
observations and the unverified empty-TEXT case (the prepared parameter was
physically NULL), without claiming complete collation or parent-form parity.
The private [ProjectID choice reader](siviprojectchoices.go) preserves the existing
YAML/database family and source Env/Master options, raw physical/NULL/empty
definitions and duplicate provenance without source fallback or configuration
writes. Focused parent/choice/source race16.771s and full Go integration
race740.732s/all packages pass; independent review found no significant issues.
62 direct SIVI columns share
proven FS882 header storage;15 are distinct. This is reuse evidence, not parent
event equivalence or permission to enable the form; TEXT option coercion and
full source membership remain explicit gaps.
The next scalar contract separates six distinct SINGLE fields and
SV_FloodPlain from apparent TEXT fields that are actually categorical controls.
Source SnowCoverRegime/SoilTexture lists and Ah/Ae/blank choices need their own
membership and NULL/empty-write evidence. The private
[seven-field scalar planner](siviparentscalar.go) now checks reviewed physical
ownership/values and omits unchanged invalid history before new-value guards.
Focused combined race17.051s passes; independent review found no significant
issues after verifying the test-placement fix; full integration race731.402s/
all packages passes.
No scalar writer, public API or parent workflow is enabled.
Controlled native Access TEXT(2) option binding now verifies `"1"`/`"2"`/NULL
storage for stand age/height estimation, not caption strings. A private option
planner shares existing changed-only/physical-owner guards without widening the
numeric/BOOLEAN domain. Focused combined race17.100s and full integration
race736.941s/all packages pass; independent follow-up review found no significant
issues. This is not original parent lifecycle acceptance.
The private [owned proposal boundary](siviparentread.go) combines the seven
scalar and two option domains with a complete cloned original under one owned
read snapshot. Errors/cancellation return no partial proposal; neither a writer
nor a persistent draft session is implied. Focused parent/choice race25.027s
passes, including external paths, invalid history, stale values, duplicate pairs
and rollback retry. Independent review's counts-only acceptance gap was fixed
with literal full nine-field payload checks; follow-up found no significant
issues. Full integration race716.771s/all packages passes; the later test-only
assertion improvement is covered by the focused rerun.
Three categorical source definitions are now bounded separately: selected
VLists physical SnowCoverRegime/SoilTexture rows retain NULL Item definitions
and fractional ordering, and the A-horizon value list has an explicit blank.
A read-only DAO check on the existing closed fixture confirms single-table
DISTINCTROW preserves physical duplicates, unlike DISTINCT. The private
[categorical reader](sivicategoricalchoices.go) now preserves selected physical
VLists definitions and source-instance IDs, separately from control names.
Complete table_xinfo preflight rejects generated/hidden columns and rowid
shadows before reading provenance. Focused race33.281s and reviewer follow-up
pass after fixing the generated-schema finding; final-code integration
race757.356s/all packages passes. No public catalogue API, membership enforcement or categorical
bound-write parity is claimed.
Read-only DAO observation of original unlinked Sample_Env and USysEnvTable now
resolves omitted TEXT table properties: PolygonNumber25/CanopyComposition50 are
nullable but prohibit zero-length storage; the three categorical and two
estimation fields are nullable and permit it. This is table-object evidence,
not bound-control empty coercion or arbitrary external-project parity. No new
empty string is silently converted or admitted through ordinary-text planning.
Private [ordinary TEXT planning](siviparenttext.go) now supports exactly those
two fields with shared ownership/history/clone guards and25/50 UTF-16 bounds.
NULL is explicit, new empty/nontext values fail, unchanged invalid history is
omitted, and literal whitespace/case remain unchanged. Focused race33.095s
passes; independent review found no significant issues. Full integration
race753.107s/all packages passes;
owned proposal signatures/public surfaces/writes remain unchanged.
The private [owned storage successor](siviparentread.go) combines all eleven
accepted fields in three isolated assignment domains under the same read
snapshot. The old nine-field method/DTO behavior remains unchanged. Invalid
third-domain tails return no partial proposals; raw originals/history, clones
and data/YAML bytes are preserved. Focused race33.937s passes and independent
review found no significant issues; full integration race742.091s/all packages
passes. No persistent
draft session, public API, source join/write authorization or form enabling.
Original-export Design-view observation now confirms the three categorical
controls have LimitToList=false and BoundColumn=1. A controlled equivalent
binding probe distinguishes cleared focused Text (NULL) from explicit Value
empty (empty TEXT), preserves valid unlisted text and leaves sibling fields
unchanged. This is not keyboard/dropdown-click or original parent lifecycle proof.
Private [categorical planning](siviparentcategorical.go) now accepts explicit
NULL/empty/nonempty TEXT within1/100/5 UTF-16 units, preserving literal/history
and shared ownership/domain guards without catalogue membership enforcement or
implicit editor coercion. Focused combined race33.874s passes; independent review
found no significant issues; full integration race710.778s/all packages passes.
The eleven-field owned successor is unchanged,
and parent writes/public APIs/navigation remain disabled.
The private [owned categorical successor](siviparentread.go) now groups all14
accepted physical fields in four independent assignment domains under one
owned read snapshot, preserving the old nine/eleven-field signatures and DTOs.
Tests cover all15 nonempty domain subsets, local/external context ownership,
literal payloads/original clones, stale/raw history, fourth-domain rejection
and late cancellation/retry without data/YAML changes. Focused race34.831s
passes after correcting categorical dispatch independence; independent review
found no significant issues; full integration race733.778s/all packages passes.
No persistent draft, implicit source-action,
editor-empty translation, writer/public API or parent promotion is enabled.
Private [source-action planning](siviparentactions.go) now derives Admin
PlotType's fixed five literals and implicit Env SpeciesListComplete-1/0/NULL
from explicit original normal option-group identities/selections. Shared layout
parsing is cached, source event metadata checked, and implicit-target allowance
is isolated from all four ordinary planning domains. Unknown PlotType clear/
options and unsupported species options fail explicitly, without normalizing
display initialization or historical storage. Focused combined race35.411s
passes; independent review found no significant issues; full integration
race772.844s/all packages passes.
This adds no owned action
proposal, source callback/Refresh/keyboard acceptance, writer/API or promotion.
The private [owned action successor](siviparentread.go) now combines all16
accepted distinct physical targets in five independent assignment domains under
one current-original read snapshot, preserving old nine/eleven/fourteen-field
signatures and DTOs. Tests assert full literal payloads/all31 nonempty subsets,
local/external ownership, raw history/clones, explicit species NULL, fifth-domain
failures and cancellation/retry without data/YAML changes. Focused race36.330s
passes; independent review found no significant issues; full integration
race719.316s/all packages passes. No persistent frontend
draft, callback/Refresh proof, Access-equivalent join authorization, writer/API
or parent promotion is enabled.
Owned parent originals now preflight complete table_xinfo, reusing the
categorical schema guard before reading Env/Admin rows. Eight reproduced
generated rowid/_rowid_/oid/ExtraGenerated cases now error with no parent/
proposal or data writes, then retry after disposable schema restoration.
Focused combined race38.196s passes; independent review found no significant
issues; full integration race741.108s/all packages passes. This is
bounded original-row provenance hardening, not a global raw-reader refactor.
The pure [parent draft model](frontend/src/siviParentEditor.ts) and
[owned session](frontend/src/siviParentSession.ts) now implement the sixteen-target,
five-domain contract, reusing shared typed-cell/Unicode/SINGLE guards without
height-specific field policies. They retain complete raw originals and exact
context/project/plot/table/row/control identities; initialization is not an edit.
Categorical editor-clear is NULL, selected empty storage is TEXT, and source
actions remain explicit option intent. Unchanged invalid history is omitted.
Remount-safe raw errors block close/Lock and Save; only valid correction or
same-editor Undo clears them. Save remains unavailable even for valid proposals.
All72 integrated SIVI frontend tests, check0 errors/warnings, isolated production
build and independent review pass. This pure draft boundary has no mounted
parent editor, writer or source callback/join authorization; the separately
gated read-only reader below does not enable these drafts.
The unwired [parent transport boundary](frontend/src/siviParentTransport.ts)
also checks complete unknown/nullable responses against the independently active
context/project/plot before creating a session. Source-defined action display
never stages edits; exact literal TEXT and BOOLEAN mappings are distinguished
from unsupported historical storage/aliases with explicit diagnostics.
All74 integrated SIVI tests/check0/0/isolated build pass. Original ProjectID
initialization/shared state and callback Refresh remain separate lifecycle gaps.
Actual owned disposable Go originals now pass lossless JSON roundtrip and
[frontend interoperability tests](frontend/src/siviParentWire.test.cjs), including
all16 targets with actual expected cells and unchanged database/YAML bytes.
Run `npm run test:sivi-wire` in the frontend with
`VPRO_SIVI_PARENT_WIRE_RECEIPT` pointing to a successful verbose
`TestSIVIParentOriginalWireRoundTrip` Go test receipt. Shared test assembly is
reused; sparse binding/cell slots and malformed editor identities are rejected.
Focused Go race1.371s,74 integrated plus2 actual-wire frontend tests/check0/0/
isolated build and full Go integration race692.436s/all packages pass.
This proves JSON/model interoperability only, not public Wails dispatch,
mounted parent lifecycle or write authorization.

### Native read-only parent review

The independent default-off `VPRO_SIVI_PARENT_REVIEW=true` backend flag and
`VITE_SIVI_PARENT_REVIEW=true` frontend build flag expose
[owned originals](siviparentreviewservice.go) through generated Wails bindings
and [a read-only panel](frontend/src/SIVIParentReadPanel.svelte) under FS882.
Both flags are required; invalid backend flag values stop startup. All77 direct
bindings retain source labels/groups, tagged storage, NULL versus empty TEXT,
and literal option-display diagnostics. With the separate editing gate off,
there are no inputs or parent Save actions. Persisted originals are explicitly
distinguished from FS882 drafts.

The [read session](frontend/src/siviParentReadSession.ts) compares independent
context/project/plot ownership, cancels actual calls, ignores cancelled/disposed
late responses, and survives tab remount. Conflicting drafts prevent opening;
busy reads prevent editor close and other operations, with explicit Cancel and
Reload. Seven enabled native cases and a backend-disabled case pass against one
disposable fixture: all77 labels/values, invalid draft/remount/Undo, tab remount,
actual exclusive-lock cancellation/retry, two-plot navigation, stale context
denial and actual600px visibility. All16 database/config hashes stay unchanged;
exact owned PIDs exit normally. A proof initially skipped Undo while remount
reference loading was busy; the draft was restored, cleanup verified, and the
corrected readiness guard passed without saving or replaying a write.

Initial focused Go race25.527s, final focused regression race1.751s,
full integration race772.916s/all packages,327 frontend
tests, check0 errors/warnings, opt-in/default isolated production builds and
independent review pass. Protected executables/embedded assets and Access source
are unchanged. This accepts native read transport/presentation only; complete
Access join/collation, ProjectID metadata, callback Refresh, editing/restoration
and full FS1333 lifecycle remain unavailable.

### Parent join and ProjectID source

[Bounded join review](siviparentjoin.go) now detects physical case/trailing-space
aliases across the original Env/Admin tables instead of assuming BINARY
filtering establishes Access membership. A1,302-case disposable DAO16.0/
General1033 probe supports nonempty ASCII alphanumeric TEXT7 identities only;
unknown Unicode/punctuation/empty/overlength/non-TEXT identities are explicitly
unverified. Duplicate pairs are not collapsed. A positive one-pair review does
not itself authorize a writer.

The [owned ProjectID source workflow](siviparentreviewservice.go) implements
explicit Env/Master preference changes through the shared YAML coordinator,
with expected-source collision detection, cancellation-before-commit and atomic
replacement. Metadata choice reads preserve duplicate/NULL/empty definitions
and reject hidden/generated physical schemas. The
[source session](frontend/src/siviParentSourceSession.ts) survives tab remount,
validates independent ownership and raw wire cells, cancels failed parallel
reads, and blocks close/disposal/cancellation during a preference commit.
Buttons immediately change only `Current.ProjectIdSource`; bound ProjectID,
plot data and audits are untouched. Other and unknown YAML keys are retained.
Metadata creation and source NotInList/Refresh defects are not enabled.

Focused Go race66.630s, full integration race715.661s/all packages,333 frontend
tests, check0/0 and isolated builds pass. Native source checks exercise the
immediate preference with exact restoration, duplicate physical definitions,
alias rejection, actual blocked commit and lifecycle guards, stale/invalid/CAS
failure, remount and600px labels. A read-only successor proof covers parallel
failure cancellation/retry without replaying completed preference writes.
Protected delivery, canonical data and original Access remain unchanged.

### Transactional directly bound SIVI parent writes

The [private writer](siviparentwrite.go) now stages fourteen directly bound fields
through four existing planners: six SINGLE fields and nullable FloodPlain
BOOLEAN0/-1, two Est./Meas. TEXT1/2 options, two nullable/nonempty TEXT25/50
fields and three nullable/empty/unlisted categorical TEXT1/100/5 fields.
PlotType/SpeciesListComplete callbacks, ProjectID and identity changes are
excluded. The initial ten-gate private checkpoint remains the immutable backend
predecessor; the separately gated mounted successor below does not widen it.

The write transaction independently rechecks the complete bounded ASCII source
join, exact physical Env/Admin pair, selected physical SU membership and all
reviewed original cells. Local/external project and SU paths use existing owned
file leases. Audits and typed provenance share that transaction; complete
project-table comparisons reject unexpected trigger effects. Unchanged invalid
history is omitted, UTF-16 limits remain literal, new true stores -1 and no-op
or rejected writes create no history. BLOB replacement remains unavailable
because source audit text cannot represent it losslessly.

[Typed restoration](siviparentrestore.go) restores only audited cells, leaving
unaudited committed changes intact. It requires unchanged complete committed
parents and exact raw audits, accepts existing qualified/unqualified Env/Admin
aliases, retains Restore=-1 or prunes only the verified audits, and prevents
replay. It never deletes children or restores an entire row from an old snapshot.
Reopening the same physical owner may rotate context identity without losing
restoration ownership.

Ten bounded backend gates cover source scope, transactional membership, each
of the four domains, audit/provenance, failure/retry, restoration and integration.
Focused SIVI race132.748s and full integration race894.016s/all packages pass,
including all15 domain subsets and audit strengths0-3. Independent read-only
review found no significant issues; source and validation are sealed in the
[writer checkpoint](archives/sivi-parent-direct-writer-checkpoint/evidence-manifest.json).
That checkpoint records private Go acceptance, not the later native successor.

### Mounted SIVI direct-parent editor

The [public boundary](siviparenteditservice.go) independently requires
`VPRO_SIVI_PARENT_REVIEW=true` and `VPRO_SIVI_PARENT_EDITING=true`.
The build requires both `VITE_SIVI_PARENT_REVIEW=true` and
`VITE_SIVI_PARENT_EDITING=true`; all four remain default off. A separately guarded
editor-original API prevents an opt-in frontend from initializing live controls
against an editing-disabled backend. Explicit original/four-domain request
properties, known edit fields and raw Unicode are checked before decoder repair.
The read-only original API retains its existing review-only behavior.

The [writer session](frontend/src/siviParentWriteSession.ts) owns drafts outside
tab lifetime. Exactly fourteen [source-labelled controls](frontend/src/SIVIParentDirectField.svelte)
replace read-only values after explicit editor loading; the other63 fields stay
read-only. NULL, empty categorical TEXT, raw text and retaining original storage
are explicit intents. Invalid drafts/errors survive remount and block Save,
Lock, close and source/context replacement. Valid correction or Undo clears the
appropriate draft. Read cancellation uses the actual binding; Save/Restore and
explicit recovery cannot be cancelled or disposed. Resolved/malformed or known
committed acknowledgements retire replayable drafts/history before validation
and refresh, with explicit blocked recovery rather than repeating a write.

Focused SIVI race117.910s, full integration race714.605s/all packages,
344 frontend tests, check0 errors/warnings, isolated opt-in/default production
builds and independent read-only review pass. Actual Wails proves1400px/600px
visibility and associated labels, raw invalid draft/remount/Undo/Lock/OS-close
guards, stale/alias denial, guarded read cancellation/retry and busy commit
guards. One fourteen-cell four-domain Save has exactly fourteen parent audits
and one typed history, with BOOLEAN=-1 and unchanged unrelated storage.
Its verifier initially expected qualified audit table names and NULL Restore,
instead of the accepted `_Env`/`_Admin` and0. The completed Save was preserved;
only its unfinished prune restoration continued after normal closure/context
rotation. A distinct WRITE02 NULL-deletion event verifies the mounted retain
button, committed history/remount, Restore=-1 and replay rejection; it does not
repeat the fourteen-cell write. All16 disposable DB/config hashes restore,
with fresh PID absence and separate retained history evidence.

Actual default production navigation/controls are absent and all three editor
APIs reject valid transports. The opt-in frontend with backend editing off also
cannot initialize controls. Evidence is sealed in the
[mounted writer checkpoint](archives/sivi-parent-mounted-writer-checkpoint/evidence-manifest.json).
Protected executables/assets, canonical data and original Access exports are
unchanged; no default promotion or Access execution. This is bounded native
direct editing, not complete FS1333 replacement or callback/metadata acceptance.

### Private source-action storage successor

The [private action writer](siviparentactionwrite.go) uses the existing normal
source planner for Admin.PlotType options1..5 and implicit Env.SpeciesListComplete
options1/2/explicit NULL. It shares the direct writer's complete original/join/
SU/ownership transaction, changed-only assignments, audit strengths and raw
whole-project verification. It does not widen any public direct-field request.
Action history lives in a separate technical table, with source-specific
validation before the shared audited-cell restorer. A direct history endpoint
cannot restore an action event; unaudited committed values are not restored.

`SourceRefreshRequired` records the normal PlotType callback's explicit Refresh
path, including an explicitly invoked no-op. It neither executes Access
Refresh nor commits other drafts; species-list assignment has no source Refresh.
NULL PlotType remains rejected because the normal String/Integer callback path
is unverified, unlike the CHARS Variant handler. Source display initialization
still creates no edits/audits.

Focused coupled SIVI/FS1333 race160.585s and independent read-only review pass.
Tests cover all15 PlotType/species choices on owned local/external fixtures,
three action subsets/strengths0-3, historical restoration, direct-history
isolation, no-op/cancellation, stale whole originals, second-audit/BLOB rollback
and corrected retry, malformed/repeated history and unaudited deletion retention.
Full Go integration race768.342s/all packages passes; source and receipts are
sealed in the [action storage checkpoint](archives/sivi-parent-action-storage-checkpoint/evidence-manifest.json).
That checkpoint covers private storage only; the separately enabled mounted
successor below owns public/native acceptance. Its predecessor evidence is unchanged.

### Separately gated mounted source actions

The [public action boundary](siviparentactionservice.go) exposes owned original,
Save and retain/prune Restore APIs with strict explicit nullable-option transport.
It requires `VPRO_SIVI_PARENT_REVIEW=true`, `VPRO_SIVI_PARENT_EDITING=true` and
the independent `VPRO_SIVI_PARENT_ACTION_EDITING=true`; all default off.
Production controls likewise require all three corresponding `VITE_` flags.
These flags enable only the normal PlotType1..5 and SpeciesListComplete1/2/NULL
actions, not full FS1333, ProjectID assignment or metadata creation.

The [scoped writer lifecycle](frontend/src/siviParentWriteSession.ts) is reused
without widening the fourteen-field adapter. The [action adapter](frontend/src/siviParentActionWriteSession.ts)
verifies `SourceRefreshRequired` against the actual request after retiring
replayable drafts. Initialization/retaining original storage never stages a
write. Literal Grnd/Visual/Note/Full/Other and Comp./Part. labels are preserved
with explanatory stored-value text. There is one live control per action;
normal PlotType has no NULL option.

Drafts/errors survive tab remounts and participate in shared Save/Undo/Lock/
close/context guards. Conflicting editor drafts block staging and recovery in
the other scope. A committed Save/Restore reloads clean peer originals without
clearing that peer's restoration handle. Direct/action history remains isolated.
This is an explicit audited desktop Save followed by owned reload: the PlotType
source Refresh directive is checked, but literal Access AfterUpdate timing and
implicit saving of other drafts are not inherited.

Focused coupled race169.690s, full Go race770.171s/all packages,353 frontend
tests/check0 errors/warnings, isolated opt-in/default production builds and
independent read-only review pass. Actual Wails verifies two labelled controls
at1400px/600px, zero-write initialization, draft/remount/Lock/close and peer
guards, a direct scalar event followed by a two-action event with both history
handles retained, exact prune restoration in each scope, and a distinct
species-only NULL event followed by remounted retain restoration. Actual action
read cancellation/retry and default-mounted/independent-backend denial pass.
All16 disposable DB/config hashes restore and every owned process closes normally.
The initial harness stopped at a hidden plot-list navigation button only after
both coupled events were captured/restored; only its unfinished WRITE02 tail
continued with a fresh context. No successful mutation was replayed.

Evidence is sealed in the [mounted action checkpoint](archives/sivi-parent-action-mounted-checkpoint/evidence-manifest.json).
Protected defaults, embedded assets, canonical data and source exports remain
unchanged. No commit, push, default promotion or Access execution.

The next [private ProjectID planner](siviprojectassignment.go) and
[owned read-only proposal](siviprojectassignmentproposal.go) select an explicit
physical existing metadata definition without collapsing duplicate IDs or
NULL/empty titles. Only changed nonempty TEXT assignments within30 UTF-16 units
are planned; unchanged historical invalid values are omitted. This is not a
writer, NotInList completion or metadata creation. Pure/coupled focused race
1.360s/65.741s and full race734.979s/all packages pass with local/external source,
ownership and zero-write checks. Evidence is sealed in the
[assignment preview checkpoint](archives/sivi-project-assignment-preview-checkpoint/evidence-manifest.json).
This read-only checkpoint does not authorize assignment writes.

The next private [assignment writer/restorer](siviprojectassignmentwrite.go)
adds transaction-held source-preference, physical metadata/schema, complete
parent/SU and file-ownership guards. It requires the observed projected column
names/types as well as selected raw ID/title. Master metadata is attached
read-only and requires rollback-journal locking; WAL/memory modes are explicitly
unavailable for assignment rather than treated as stable commit guards.
Assignments/audits share the project transaction, with a third isolated
technical history domain and audited-only typed restoration. Neither Save nor
Restore invokes metadata creation/NotInList or changes metadata/preferences.
Independent review finds no significant issues; coupled race184.245s and full
Go race814.398s pass. This accepted private storage successor is not a
public/native workflow or default promotion. Its immutable checkpoint is
`archives\sivi-project-assignment-storage-checkpoint`, preserving the36-file
preview and191-file mounted action predecessors.

The [assignment facade](siviprojectassignmentservice.go) now exposes owned
original/choices, explicit physical selection Save and isolated typed Restore.
It requires literal `VPRO_SIVI_PARENT_REVIEW=true`,
`VPRO_SIVI_PARENT_EDITING=true` and independent
`VPRO_SIVI_PROJECT_ASSIGNMENT=true`; action editing is not a prerequisite or a
substitute. All flags default off. Transport rejects missing/NULL provenance,
unknown/coerced fields and malformed raw JSON Unicode before decoder repair;
physical IDs stay strings and NULL/empty titles and duplicate choices stay distinct.
Original and metadata are read in one pinned transaction; the preference is
observed for this read, then independently held/rechecked by Save through commit.

Assignment focused race49.946s, full Go race811.644s, frontend353/353,
Svelte check0/0, generated Wails bindings and isolated production/Wails builds
pass. Actual Wails denies all three assignment APIs with complete valid transport
when only the assignment flag is off, even with the other backend flags on.
The actual default FS882 editor exposes no opt-in assignment controls; project,
support and config hashes restore exactly and both owned processes close.
The first harness read packed variadic arguments incorrectly; its owner closed
before a corrected fresh process completed the zero-write proof.
That checkpoint validates the facade/default denial, not enabled mounted
assignment. Default builds/source/data are not promoted or modified.

The independently gated [physical selection owner](frontend/src/siviProjectAssignmentSession.ts)
and [ProjectID field](frontend/src/SIVIProjectAssignmentField.svelte) now have
mounted Wails acceptance. Matching `VITE_SIVI_PARENT_REVIEW=true`,
`VITE_SIVI_PARENT_EDITING=true` and `VITE_SIVI_PROJECT_ASSIGNMENT=true` are also
required for presentation; the action flag is separate. One labelled responsive
selector replaces only the persisted ProjectID value. Physical duplicates and
NULL/empty titles remain separate; unsupported new NULL/empty/non-TEXT,
overlength or historical-BLOB replacement choices remain visible but disabled.
Source-specific availability is observed in the same snapshot, displayed
explicitly and independently rechecked by Save; it is not membership or write
authorization. Master WAL remains read-only/unavailable for assignment.

Draft/error/history ownership survives remount, with mutual direct/action/
assignment guards for Save/Undo/Lock/close/context/source. Clean peer/source
refresh retains each restoration handle; typed restoration never looks up or
creates metadata and remains valid after changing the current choice source.
The shared writer lifecycle now injects typed draft behavior without widening
the existing sixteen-field/direct14/action2 domains.

Focused Go50.977s/full race825.824s, frontend368/368/check0/0, isolated opt-in/
current-default builds and independent review pass. Nine native cases verify
Env/Master selection, actual1400px/600px visibility, cancellation/retry,
three coupled history owners, remounted restoration after source switching,
WAL-disabled UI/independent denial and current-default mounted denial.
Four successful events affect exactly five audited cells, with whole-project/
audit/history differences checked; every event is restored, all16 original
DB/config hashes return exactly and every owned process closes. No successful
mutation is replayed. Two harness-only interruptions occurred before writes
(dirty-peer readiness and CLI variable shadowing); their cleanup and fresh
zero-write continuations are retained honestly.
The mounted checkpoint preserves the307-file facade predecessor.
Metadata creation/NotInList completion, full Unicode collation and complete
FS1333 remain unavailable; no Access execution or default promotion is claimed.

The SIVI source **Edit Metadata** action now reuses the existing current-project
metadata editor under `VITE_PROJECT_METADATA_EDITING=true`, not a second editor
or new backend. Env/Master determines choices, not the metadata edit destination.
Duplicate matching definitions and NULL/non-NULL dates require explicit
physical selection. Parent/peer drafts/errors/Lock/busy guards remain shared.
Acknowledged metadata commits refresh attempted parent/peer/source reads without
losing independent history; failed/null reads are explicitly retried.
Raw errors and unresolved post-commit refresh now block manual/native/context
close; Reload retries refresh only, never Save/creation/restoration.
Creation/template/restore flags remain independent. SIVI new-ID/NotInList
completion and typed creation restoration remain unavailable.
Coupled backend race71.848s (unchanged Go825.824s full baseline), frontend371/
check0/0 and isolated enabled/independent-off Wails builds pass. Six native cases
include one metadata Title/stamp event/four audited cells, exact typed prune,
choice refresh and actual Master-read-lock post-restoration recovery without
replay. All16 fixture hashes restore after owned closure. Interrupted probes,
including the independently captured saved event and its unfinished same-owner
tail, remain evidence rather than being silently rerun.

Current-ID SIVI blank and master-template creation now have separate disposable
native acceptance through the existing editor/core, with no new backend or
production enablement. Blank-on/template-off and blank-off/template-on builds
verify independent presentation. Seven blank and eight template cases cover
explicit proposals, tab persistence/Undo without allocation, complete audit-trigger
rollback/retry and stale successful-proposal rejection. Template timestamp and
historically invalid collection values require explicit year/NULL decisions;
master drift rejects the complete source snapshot. Source unique ProjectID is
preserved, and a single physical template is selected, never bulk copied.
Two total creations produce two full75-cell creation audits. Exact whole-project
comparisons verify unchanged parents/unrelated definitions and all73 blank
ordinary/stamp NULLs or all32 template assignments/unmapped NULLs. All16 hashes
per fixture restore after owned closure; accepted post-creation databases remain
offline evidence. This is fixture restoration, not a typed application undo of
creation. Focused creation Go race4.037s, metadata frontend14/14 and check0/0 pass;
both isolated production Wails candidates pass. Default delivery stays unchanged.
The [current-ID creation checkpoint](archives/sivi-current-id-creation-sealed/evidence-manifest.json)
preserves113 files and verifies the immutable936-file metadata-entry predecessor.

### Table interchange preparation

A [private version1 typed CSV codec](docs/TABLE_CSV_CONTRACT.md) now roundtrips
canonical physical table snapshots without NULL/empty/BLOB ambiguity, integer
precision loss or text line-ending repair. It retains physical IDs, column
types and duplicate nullable Description candidates, rejects corrupt/incomplete
input and returns no partial result on cancellation. Focused Go race1.185s,
independent review/fix follow-up and kernel integration883.751s/all packages pass.
The successor adds an owned read-only Wails review of one whole physical core
table, separately default-off under backend `VPRO_TABLE_CSV_REVIEW` and frontend
`VITE_TABLE_CSV_REVIEW`. Typed transport/checksum validation, cancellation/remount,
existing draft navigation and busy-close guards are native-tested. All376 frontend
tests, check0/0, isolated production builds and16 native cases pass with zero
writes and all16 fixture hashes restored. Fresh full Go integration826.279s/all
packages passes; the kernel receipt is not validation of this successor.
The [owned-review checkpoint](archives/table-csv-owned-review-checkpoint/evidence-manifest.json)
preserves123 exact source, build, native and validation files and verifies the
immutable18-file kernel predecessor.
Import, RDS/TurboVeg support and companion R package delivery remain unavailable.
The separately default-off migration-archive export successor is described below;
it is not final client approval of a plain-value analysis CSV/RDS format.
The [private owned archive successor](docs/TABLE_CSV_CONTRACT.md#private-owned-archive-successor)
adds an explicitly versioned envelope preserving physical Description metadata
absence versus present-empty inside the single ZIP artifact. The original table
document and generic bundle remain unchanged and distinct. Strict validation,
owned source reobservation and the shared atomic no-replace writer are reused;
focused/shared race12.453s, vet and fresh full Go race962.800s/all packages pass.
The [owned archive checkpoint](archives/table-csv-owned-archive-checkpoint/evidence-manifest.json)
retains exact private evidence; no independent-review/native-export claim follows.
ZIP delivery remains provisional, not a final client analysis-format decision.
No new desktop export/import or database reconstruction is enabled.
The [private source-bound approval facade](docs/TABLE_CSV_CONTRACT.md#private-source-bound-approval-facade)
now prepares complete-owned-source approval and typed irreversible publication
receipts for a later desktop surface. Exact archive hash/size/format accompany
the review; metadata/schema/ownership drift invalidates approval even when CSV
bytes are unchanged. Strict request and outcome helpers are shared with KML;
coupled race16.047s/vet and fresh full Go race982.703s/all packages pass.
The [private approval checkpoint](archives/table-csv-archive-approval-checkpoint/evidence-manifest.json)
retains exact evidence. At that private checkpoint the facade was unregistered,
with no new bindings or UI/native claim.
Its [desktop successor](docs/TABLE_CSV_CONTRACT.md#independently-gated-desktop-migration-archive)
now registers the independently default-off archive service, actual bindings and
explicit reviewed Export with a literal destination/native chooser. UI export
requires `VITE_TABLE_CSV_ARCHIVE_EXPORT=true` plus existing
`VITE_TABLE_CSV_REVIEW=true` navigation; backend
`VPRO_TABLE_CSV_ARCHIVE_EXPORT=true` is independent of the read-only review flag.
Known/unknown historical receipts survive acknowledgement/remount/new attempts;
publication cannot be cancelled and holds close/navigation until acknowledged.
Primary438 frontend tests/check0/0, isolated builds and20 native cases across
four closed owners pass. Four exact archive outputs are retained; all16
source/config hashes restore and no data/audit/preferences writes occur.
Fresh full desktop Go race passes root931.623s/all packages. The
[desktop archive checkpoint](archives/table-csv-archive-desktop-checkpoint/evidence-manifest.json)
retains exact builds, native proof and four planned outputs. Production defaults
remain unchanged; this is not full migration or final analysis-format acceptance.
The original go-mdbtools checkout remains unchanged; a disposable Windows
candidate now compiles with bundled MDBTools/fakeglib and local GNU libiconv.
Public MDB/ACCDB and copied VPRO fixtures, Unicode-path/corruption conformance,
fresh race19.274s and vet pass. Independent review closed three conversion/string
boundary defects. Embedded source-text NUL is explicitly unsupported rather than
silently truncated. The [private scalar boundary](docs/ACCESS_IMPORT_CONTRACT.md)
preserves native values and Access true=-1; fresh isolated consumer race1.452s
round-trips932 cells through SQLite, including eight true values and five NULLs.
No reader dependency or absolute replacement was added to the application.
Schema/metadata, ownership, transactional publication and redistribution remain pending.
Access import remains unavailable.
The private column-affinity planner now retains native field order/names and
uses TEXT for decimal/currency/GUID values rather than allowing NUMERIC coercion.
Actual SQLite tests and native complete-row consumer race1.479s verify932 cells;
constraints/defaults/identities and source-version conversion remain separate.
The complete native row adapter now rejects missing/extra cells and native-type
mismatches before any storage, returning no partial result on errors/cancellation.
Focused race1.405s and complete-row native consumer race1.506s pass; fresh full
integration852.401s/all packages and package vet pass.
The separate memo/OLE reader correction rejects corrupt chained values that
previously returned partial bytes without an error. Reader race22.664s/vet,
bounded independent review and fresh complete-row consumer1.467s pass.
Jet4 synthetic native/bound chains now pass94 checks, race24.214s/vet and
independent review without a production change. Actual ACCDB chained callers,
legacy flags and all-size memo support are not established.
The disposable reader's timestamp correction now preserves representable
source-double fractions and neutral UTC negative/date-range semantics.
Corrected clone race28.134s/vet, independent40-case native buffer review and
exact native-to-SQLite fractional consumer1.450s pass. Values that cannot be
represented without changing source-double evidence fail explicitly; no fixed
millisecond or original decimal-nanosecond precision is inferred. Native Access
runtime, DATETIME Extended, Linux and redistribution remain unverified, and
no production reader dependency or importer is enabled.
Additive reader-side description observations now retain catalog-local ordered
raw entries, type/flag bytes and duplicate/position evidence without loading
linked targets or changing compatibility Description. NULL/DAO type/definition
origin remain explicitly unknown. Clone race28.930s/vet, independent
inspection-only review and exact metadata/native-SQLite consumer1.544s pass;
semantic NULL/empty, actual linked/unavailable fixtures and Linux remain gaps.
This does not enable Access import or add a production dependency.
The disposable reader's additive native-column candidate retains ordered
source numbers/names, native type codes/backend labels and raw size/flags.
Size units, flag/identity semantics, required/nullability/defaults and decimal
precision remain unavailable. Fresh clone full16.623s/race34.040s/vet and primary
exact source-byte/shape/value/metadata/fraction consumer1.503s pass. Independent
review verified46 baseline/38 unchanged files and10 candidate hashes, found no
significant issues and ran native focused tests1.636s. The
[81-file exact successor](archives/access-reader-columns-checkpoint/evidence-manifest.json)
verifies18 application predecessor/78 description evidence/46 reader baseline
files;372 application Go hashes are unchanged.
Legacy affinity labels are not native backend labels.
No production dependency, linked-target loading or import enablement is added.
Private single-artifact CSV preparation now passes focused race5.278s/vet and
independent correction review and fresh full integration833.343s/all packages.
It avoids later half-published
CSV/manifest pairs. No ZIP delivery, file publication or client-format approval
is claimed by that draft.
The private no-replace filesystem kernel now passes final coupled race5.672s,
independent frozen-parent Windows correction review and full race828.253s/
all packages. It stages and verifies one artifact, preserves collisions/unowned
replacements, and reports committed state after cleanup errors. Two symlink
cases remain privilege-limited; caller/project leases and source re-observation
are not provided by this kernel. No public writer is enabled.
Private source-owned orchestration reuses existing review/context boundaries.
Focused race12.798s/237 cases, vet, independent inspection-only review and fresh
full integration849.089s/all packages pass. Fresh snapshots compare exact
documents and separate metadata presence; postcommit errors preserve the
published result. Already-borrowed and external writers can still race, and
metadata-table presence is not encoded in the generic artifact. This is not a
publicly reachable desktop export or a client output-format decision.

The [source-defined plot location review](docs/PLOT_LOCATION_CONTRACT.md) now has
an owned read-only facade and frontend, independently default-off under
`VPRO_PLOT_LOCATION_REVIEW` and `VITE_PLOT_LOCATION_REVIEW`. It preserves the
original eight-field presentation, Env/Admin/selected-SU scope, physical
provenance and exact numeric longitude negation without coordinate inference.
Focused backend race2.266s,381 frontend tests/check0/0, isolated enabled/default
production Wails builds and independent review pass. Twelve native cases verify
whole-project500-row SQL parity, presentation/pagination and read/draft/close
barriers; all16 hashes restore after four owners close. Selected external SU
remains fast-test evidence. Fresh full Go race775.988s/all packages passes;
the [122-file exact successor](archives/plot-location-owned-review-checkpoint/evidence-manifest.json)
preserves native/build/review evidence and verifies the immutable predecessors.
File publication and Google Earth launching remain disabled.
Private direct-Env Google Earth multi-plot preparation is now a candidate:
it preserves source SU membership fanout and raw tagged descriptions rather
than reusing ReportLocation's Admin/DISTINCTROW scope. Focused race1.176s/vet,
independent reviewer's zero-significant-issue report and fresh full
integration937.495s/all packages pass; no independent reviewer runtime claim.
No XML/file/preference/launch action
or single-plot parity is implied.
The unexported owned Google Earth reader uses the same
context/snapshot ownership as existing read-only workflows and a shared physical
table guard. Focused owned/pure location race2.297s/vet, independent review
(17 focused tests1.545s/owned race2.290s) and fresh full integration925.947s/
all packages pass with372 Go source hashes unchanged during integration.
External-SU fanout and no-Admin scope are tested;
this private boundary alone does not imply a public workflow.
The separate [Google Earth preparation facade](googleearthreviewservice.go) and
responsive frontend now expose that owned multi-plot snapshot only as an
independently default-off read-only preview (`VPRO_GOOGLE_EARTH_REVIEW` and
`VITE_GOOGLE_EARTH_REVIEW`). Dynamic description choices are observed literal
current Env fields, not Sample bindings or saved preferences. Exact raw NULL/
empty/non-text descriptions, signed64 identities, per-pair SU fanout, numeric
longitude negation and bounded backend pages are validated without invented
coordinate ranges. Coupled Go race4.622s/vet, independent review,387 frontend
tests/check0/0 and opt-in/default production builds pass. Four fresh Wails
owners prove16 native cases: independent SQL/RPC/no-Admin/raw descriptions,
pagination, visible labels/cells at1400/600 widths, remount, independent denial/
default gating and valid/invalid FS882 draft transitions without Save. All16
DB/config hashes restore after each normal close. Initial proof-only null-row
wait was corrected; preserved observations were compared, not overwritten.
Fresh full integration899.304s/all packages passes with374 Go hashes unchanged
throughout validation. The
[147-file exact successor](archives/google-earth-owned-review-checkpoint/evidence-manifest.json)
preserves81 reader-column predecessor files and closed native builds/evidence.
KML/XML/file creation, output
path/Place Name preferences, remote icons, viewer launch and single-plot
behavior remain unavailable; original launch/file menu items stay disabled.

The private [KML byte preparer](googleearthkml.go) accepts only explicitly
finalized text and east-positive map coordinates, not raw preview cells.
Focused coupled race1.763s/package vet pass: exact KML2.1 namespace/element
shape, Unicode/XML1.0 validation, literal HTML-plus-XML description escaping,
duplicate/order retention, signed zero, finite coordinate bounds and
nonpartial cancellation/retry. Source zero altitude and trailing description
period are retained; remote styles/icons are deliberately omitted. Empty
strings are preserved without defaults, trimming or repair. Primary inspection
found no significant issues; no independent review, formal KML schema or viewer
acceptance is claimed. Fresh full Go race943.664s/all packages passes against376
Go hashes unchanged during validation. The
[private KML successor](archives/google-earth-private-kml-checkpoint/evidence-manifest.json)
verifies147 predecessor evidence files and protected source/data/defaults.
This is not a database
NULL/non-text coercion policy, public KML API, file writer or privacy guarantee:
viewers may still automatically turn URLs into links. See the
[private KML boundary](docs/PLOT_LOCATION_CONTRACT.md#private-finalized-text-kml-byte-preparation).

The private [TEXT/NULL point adapter](googleearthkmlpoints.go) now connects
direct-Env projected rows to finalized KML inputs. Live source VBA ampersand
expressions and the Microsoft language reference map NULL to empty output
text, while the original tagged metadata remains distinct. Exact TEXT is
preserved; numeric/date/boolean/BLOB text formatting is explicitly unavailable,
not guessed. Canonical cells, physical-row error context, source order/fanout,
already-negated longitude, signed zero and no-partial cancellation/retry are
covered by coupled race1.913s/vet. Source projection through XML verifies
Admin-orphan inclusion, NULL output and six physical Env/SU placemarks.
Coordinate guards are shared with the serializer. Primary inspection only;
fresh full integration995.902s/all packages passes against378 Go hashes
unchanged during validation. The
[TEXT/NULL KML successor](archives/google-earth-text-kml-checkpoint/evidence-manifest.json)
verifies14 private KML predecessor files/protected hashes. No public API,
binding/UI, file, preference or launch action is added.

The private [owned KML reader](googleearthkmlread.go) now keeps the existing
operation lease and one read-only owned snapshot through projection, TEXT/NULL
adaptation and byte generation. Its scope, explicit title/field and placemark
count accompany detached bytes. The shared direct-Env snapshot extraction
preserves existing raw-preview behavior. Coupled location/Earth race4.975s/vet
pass: independent SQL/no-Admin scope/NULL periods, external-SU fanout,
ownership/physical-view rejection, queued cancellation/retry and unchanged
database/preferences. Unsupported descriptions or KML-only bounds errors
return no output while raw preview stays available. Primary inspection only;
fresh full integration987.040s/all packages passes against380 Go hashes unchanged
during validation. The
[owned KML successor](archives/google-earth-owned-kml-checkpoint/evidence-manifest.json)
verifies22 TEXT/NULL predecessor evidence files/protected hashes.
That private checkpoint did not include a public service or UI.

The separate [KML text preview](googleearthkmlservice.go) now has real generated
Wails bindings and an independently default-off frontend capability. Opt in only
with both `VPRO_GOOGLE_EARTH_KML_PREVIEW=true` and
`VITE_GOOGLE_EARTH_KML_PREVIEW=true` within the opted-in raw-review panel;
the existing raw-review flags are separate.
Strict string JSON requires exact `descriptionField` and explicit `title`,
rejecting malformed raw Unicode and duplicate/case-folded/escaped property
collisions. The typed result carries exact owned scope, title/field, complete
placemark count, UTF-8 byte count and detached XML. Only original TEXT/NULL
names/descriptions are supported; other storage types fail explicitly.

Place Name starts visibly at the source default `"VPro Plot Locations"` but is
ephemeral, never a preference write. Labelled responsive controls show XML in a
read-only textarea, not live HTML. Asynchronous real-browser XML shape checks
yield during validation, with generation checks preventing cancelled/remounted
results from populating a new panel. Existing busy navigation/native-close and
valid/invalid parent-draft barriers remain active.

Coupled focused Go race5.169s/vet,391 frontend tests/check0/0, isolated enabled
and KML-default-off production builds and independent scoped review pass.
Five disposable Wails owners close normally:19 zero-write cases, each restoring
16 hashes against its applicable baseline. These include all500 SQL/XML marks,
actual browser parsing, literal markup/visible labels, independent gates and
seven final-async busy/cancel/remount/retry/draft cases. Initial native observations
preceded asynchronous scheduling; their36 build files were archived before
rebuilding. The first owner correctly rejected historical Latitude190. Only
after it closed/restored was that one disposable cell changed to90; every other
row/schema/audit stayed unchanged, and the original database is retained.
Fresh full race926.536s/all packages passes against382 Go hashes unchanged
throughout validation. The [exact preview successor](archives/google-earth-kml-preview-checkpoint/evidence-manifest.json)
verifies18 owned-byte predecessor files/protected hashes and exact review identity.
This is text preparation, not file export, preference persistence, remote-icon
loading or viewer launch; no formal KML schema, viewer or native Access parity
claim follows. See the [public preview contract](docs/PLOT_LOCATION_CONTRACT.md#independently-gated-kml-text-preview).

The next [private KML file preparer](googleearthkmlpublication.go) now reuses a
[shared no-replace artifact kernel](artifactpublication.go), extracted from the
existing CSV publisher rather than duplicating its path/ownership/cleanup logic.
CSV compatibility aliases retain its bundle encoder, staged decoder, error
labels, temporary prefix and source-authority ordering; the unchanged CSV
collision/cancellation/ownership/restoration suites pass race11.785s/vet.
The KML caller accepts only explicit finalized title/map points, detaches before
callbacks and stages exact UTF-8 serialization. It preserves the committed
path/hash/Published result after cleanup errors, refusing replacement or replay.
Coupled CSV/shared/KML race14.101s/vet passes; independent review finds no
significant issue and runs uncached coupled race12.821s. Fresh full
race881.906s/all packages passes against386 unchanged Go hashes; the
[private publication successor](archives/google-earth-private-publication-checkpoint/evidence-manifest.json)
verifies139 predecessor files,36 closed initial build files and protected hashes.
This is not a desktop file-export action or
an owned source-revalidation boundary. Source fixed-name `Open For Output`
overwrite is deliberately not inherited. Preferences, icons and launch remain
disabled; no native file/legacy-VBA-byte/schema/viewer parity is claimed.

The next [private owned KML publisher](googleearthkmlownedpublication.go) now
retains original tagged projected cells and physical Env/membership IDs alongside
the reviewed KML scope/title/field/count/bytes. Approval cannot rely only on
rendered XML: NULL-to-empty descriptions and changed duplicate-membership row IDs
can leave the XML identical. One context lease/owner mutex spans independently
completed initial, prelink and postlink source observations; read cleanup is
shared with the unchanged CSV boundary and completes before file commit.
Committed path/hash/Published survives subsequent raw drift, cancellation or
read-cleanup errors with an explicit do-not-replay message.

Coupled CSV/shared/KML race15.428s/vet and independent scoped review (uncached
race15.344s/vet, no significant issue) pass. Fresh full race962.046s/all packages
passes against389 unchanged Go hashes; the
[owned publication successor](archives/google-earth-owned-publication-checkpoint/evidence-manifest.json)
verifies26 predecessor files/protected hashes and exact review identity.
Tests include independent complete SQL/XML/no database or preference
writes, deep caller detachment, raw-only NULL/empty drift at all three phases,
same-XML external-SU physical drift, scope/provenance/byte tampering, queued
operation-lease cancellation and pinned-reader retry after cleanup faults.
This protects observation-time equality, not exclusion of external/already-
borrowed writers. No desktop file-export workflow or new native/file/schema/
viewer parity is enabled.

The separate [desktop KML export facade](googleearthkmlexportservice.go) and
[publication session](frontend/src/googleEarthKMLExport.ts) are independently
default-off. `VPRO_GOOGLE_EARTH_KML_EXPORT=true` authorizes its backend independently;
`VITE_GOOGLE_EARTH_KML_EXPORT=true` exposes its controls inside the opted-in
Google Earth review panel. Export does not require enabling the separate text-only
preview flag. One shared Place Name/Description pair produces a complete review,
a raw-source approval fingerprint and an independently checked displayed-byte
SHA256. An explicit absolute new output path is required; no overwrite, implicit
extension, preference save or viewer launch occurs.

Publication is noncancellable. The typed result retains committed path/hash even
after an error; lost transport means unknown outcome, never proof that no file was
written. Every outcome needs acknowledgement and a new review before another file.
Receipts and the acknowledgement barrier survive tab remounts within the running
application, not a full reload or restart. Source fingerprints are content guards,
not signatures or authorization tokens.

Focused backend race12.627s/vet,398 frontend tests/check0 errors/warnings and
isolated enabled/export-default-off builds pass. Real Wails bindings use interfaces
(`-ts -i`),18 services/191 methods/156 models. Independent eleven-file review finds
no significant issue and runs seven uncached Go tests (not race) plus22 frontend
tests. Fifteen native cases across four normally closed owners verify actual
secure-context crypto/DOM parsing, all500 SQL/XML placemarks and exact file hashes,
1400/600px visible labels, no replacement, busy/acknowledgement/draft/close barriers,
retained receipts and independent disabled gates. Warning/lost transport are
deliberately injected only after a real backend commit; actual postcommit read
cleanup failure is tested through the Go facade. Raw NULL-to-empty drift leaves XML
identical but changes approval and refuses publication. All16 disposable data/
config hashes are restored; three KML files and a collision sentinel are retained.
Fresh full race920.120s/all packages passes against391 unchanged Go hashes.
The [desktop export successor](archives/google-earth-desktop-export-checkpoint/evidence-manifest.json)
preserves exact source/bindings/builds/native outputs and review/integration
evidence; the protected default and owned-publication predecessor remain unchanged.

The separately gated [saved-preference service](googleearthpreferencesservice.go)
reuses existing runtime YAML `ReportOptions.GoogleEarthPlaceName` and
`GoogleEarthDescField`, not Access's registry or another store. Only explicit
Load applies saved literals, and explicit reviewed Save persists a compare-and-set
change against the originals. This is an intentional adaptation of source
`AfterUpdate` setters, not an automatic input/export write.
`VPRO_GOOGLE_EARTH_PREFERENCES=true` and `VITE_GOOGLE_EARTH_PREFERENCES=true`
are independent of export/text flags. Existing fields remain single live controls.

Missing/NULL/wrong-type settings produce errors, not inferred defaults. A saved
unavailable/invalid field stays literal with visible availability feedback.
Unchanged historical invalid strings are not repaired. New validation errors,
drafts and irreversible receipts are separate persistent state: invalid drafts
block Save/native close but leave correction controls live; valid correction,
explicit Load or Undo clears the appropriate draft error. Noncancellable Save
and acknowledgement retain committed/error/unknown outcomes across tab remounts.
Unknown transport requires explicit reload before another Save.

Coupled configuration/preferences/KML/CSV race15.744s/vet,412 frontend tests/
check0/0, isolated builds and independent thirteen-file review pass. The reviewer
also runs uncached preference/config and coordinate/working-unit regression races.
Twenty-one native cases across five normally closed owners verify exact whole-YAML
differences for five explicit application saves, no database/audit changes,
1400/600px visible labels, cancelled-read suppression, stale CAS, real committed
held-response close/navigation barriers, receipts/remount/reload, correction/Undo
and parent draft/close barriers. Historical XML-invalid title/NUL unavailable field
survive actual Load and title-only correction without silent replacement.
Warning/unknown transport is deliberately injected after actual commits; Go
tests separately prove real postcommit cancellation transport.
All16 fixture data/config hashes are restored. Fresh full integration916.340s/all
packages passes against395 unchanged Go sources. The
[saved-preference successor](archives/google-earth-preferences-checkpoint/evidence-manifest.json)
retains exact reviewed source, generated bindings, isolated builds, native
observations/restoration and integration evidence while verifying the357-file
export predecessor and protected defaults. No production promotion follows.

New-project administration now also has a private
[schema-only template mapping](docs/PROJECT_CREATION_CONTRACT.md). It preserves
retained SQLite DDL bodies and original index properties while renaming only
literal table/index headers; focused mapping/observer race2.873s passes.
It executes no application DDL and enables no creator/publication. Independent
review found no significant defects and fresh full race801.140s/all packages
passes; the [16-file exact private successor](archives/new-project-ddl-preparation-checkpoint/evidence-manifest.json)
is sealed. Native version/default evidence remains
outstanding.

Accepted private [Access table staging](docs/ACCESS_IMPORT_CONTRACT.md) applies the
fixture-tested reader value/affinity/complete-row boundary in exclusively owned
in-memory SQLite, with explicit image/row budgets and independent reopened
schema/type/value/order/count verification. Corrected focused race1.411s/vet and an
actual isolated go-mdbtools consumer race1.559s pass across1782 cells/155 rows from three
unchanged fixtures. Native REAL negative zero that SQLite would normalize is
explicitly rejected. Final-hash cancellation found by review is corrected and
tested; independent closure passes. Fresh all-package race passes
root954.209s/staging1.248s against397 unchanged Go files. The
[exact staging successor](archives/access-import-staging-checkpoint/evidence-manifest.json)
retains reviewed source/consumer/five actual images/independent readonly SQLite
checks and verifies the359-file preference predecessor/protected defaults. This enables
no canonical conversion, reader redistribution, file publication or production
import and does not infer original constraints, identities or Description semantics.

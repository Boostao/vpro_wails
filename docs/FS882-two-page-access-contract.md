# Two-page FS882 source and gated parent field editors

## Availability

Normal `FS882-8x6XL` and `FS882-8x6XL-CHARS` have source-exact metadata,
owned reads, typed additional-field planning and transactional storage.
The service facade is registered with Wails and a source-labelled additional-field
review is mounted in the existing FS882 host. The separately scoped common-field
facade/editor implements **13 normal/14 CHARS fields beyond XL**. The additional
panel remains **nine normal/eight CHARS fields**. Neither is a complete two-page
form or an enabled default workflow.

The facade requires explicit, independently default-off
`VPRO_TWO_PAGE_PARENT_REVIEW` for reads and
`VPRO_TWO_PAGE_PARENT_EXTRA_EDITING` for Save/restoration. These do not inherit
SIVI authorization or authorize edits to the existing FS882/SIVI field sets.
Missing flags disable their respective capability; malformed flags are errors.
The corresponding frontend flags are `VITE_TWO_PAGE_PARENT_REVIEW` and
`VITE_TWO_PAGE_PARENT_EXTRA_EDITING`, both absent/off by default. Review-only
assets can load originals without enabling edits.
Common-field writes additionally require `VPRO_TWO_PAGE_PARENT_COMMON_EDITING`;
the common service never inherits Extra/SIVI editing authorization. Its UI requires
`VITE_TWO_PAGE_PARENT_COMMON_REVIEW` alongside source review, with independent
`VITE_TWO_PAGE_PARENT_COMMON_EDITING` for mutation. All are absent/off by default.
Malformed two-page flags are rejected before startup data/config work.

## Canonical source

Canonical exports are read-only under `VPro64_forAI`. Metadata is generated with
the existing `cmd/fs882layout` extractor, not a second Access parser.

| Variant | Source SHA256 | Distinct bindings | Instances |
| --- | --- | ---: | ---: |
| Normal | `d780476c2a69470190ba86a7b4bd8e97df20357e185b0578fd1b454122267aec` | 118 | 121 |
| CHARS | `df5af8d95a5f8999ca192c7b80441ecf0e7033f93f58a3535b6924f821222a99` | 120 | 122 |

Both use `USysEnv`, the physical Env/Admin inner join, and pages `Site/Veg`
and `Soil/Terrain`. PlotNumber has three source instances. Normal has two
`SV_StandAgeEstMeas` bindings and no `SV_StandHeightEstMeas` binding; CHARS
has one of each. This discrepancy is retained, not silently repaired.

Six children are PlotNumber-linked, in source order: normal `SubVegAXL_BC`
or CHARS `SubVegAXL`, then `SubVegCXL`, `SubVegDXL`, `frmVPicsXL`,
`SoilHumusXL` and `SoilMineralXL`. Metadata resolution does not authorize
pictures or complete child execution.

### Picture source preparation

The local linked picture library is a separate owned data family, not a project
vegetation table or the unrelated `USysPictureBlob` definition. Canonical
`V7mdlSplash.AttachSupportTables` links `tblVPics` from
`<InstalledDir>\PlotPictures\VPics.accdb`. Both `frmVPicsXL` and
`frmPlotPictures` bind that table; the parent manager links its child by
PlotNumber. Canonical `VPics.accdb` has been inspected only on an exact disposable
copy through the already verified Windows go-mdbtools reader, without Access.
Its SHA256 remains
`b84312d7f19f5a84b54b98be80f6b46fd86223a24eb168d11a0a90b570debcea`.
The copy has one local table, five ordered columns (`ID`, `PicDir`, `PicName`,
`PlotNumber`, `PicComment`) and ten complete streamed rows. IDs are native
integers, the first three text fields are non-NULL in those rows and all ten
PicComment values are NULL. This is observed fixture data, not required/default
column policy. Native column numbers retain the missing ordinal1. Raw native-reader sizes and flags alone did not establish nullable/default/identity
or text length rules. The table's full exported DDL remains unavailable. A later
selective read-only schema inspection now supplies explicit TableDef/Field/Index
policy, not another data reader; see the source policy below.

Source child `Form_Current` opens all pictures for the plot, takes the first
record's PicDir and combines it with the current PicName. Its `Default` directory
uses `Location & "PlotPictures\"`; the manager child's equivalent uses the
separate PictureDir preference. Do not silently merge these preferences or
inherit first-record directory ambiguity as an intended desktop requirement.
Child double-click opens `frmPictures` and populates its preview. The manager
add action uses a single-file picker, explicitly moves to a new record, assigns
PlotNumber/PicName/PicDir and requeries only already loaded ordinary source forms.
Cancelled file selection assigns nothing. No source file-copy/import operation
is inferred from storing a path.

The same exact copy now crosses the existing fixture-tested
`internal/accessimport.PrepareTable` boundary under a private consumer module.
Race validation passes1.479s: all ten original native rows are independently
compared cell-for-cell with reopened read-only SQLite storage, including all ten
NULL comments; the native reader closes exactly once and the source hash remains
unchanged. The detached staged SQLite image SHA256 is
`3b4cc61e82b643c2dd67a590d55b7a31dbd1d999f5b4d59d8c6a619a2caed02b`.
It is a private staging image, not an installed/published canonical picture library.

Pictures remain default-off: there is no installed canonical SQLite picture
family. The optional owned metadata/image desktop candidate now has disposable
native proof, full integration and independent evidence review, with a
[distinct485-file seal](MIGRATION_EVIDENCE.md#snapshot-picture-read-desktop-checkpoint).
Reuse the
existing fixture-tested Access import boundary rather than a second reader.
Metadata/image read-only preparation can proceed independently; metadata writes
must not infer unsupported ID allocation, defaults or length constraints.

#### Picture metadata source policy

An exact disposable VPics.accdb copy was opened read-only through installed DAO;
no Access.Application, forms/reports, Recordsets or data rows were opened/read.
Canonical/copy SHA256 and VPRO registry remain unchanged after closing the
database. Original native row reading/import remains owned by go-mdbtools and the
fixture-tested import boundary. The private `picture-schema-policy/policy.json`
receipt retains typed source properties; packaged
[picture metadata policy](../resources/picture-metadata-policy.json) contains no
user rows or machine paths.

| Field | DAO type / Size | Required | AllowZeroLength | Default |
| --- | --- | --- | --- | --- |
| ID | Long /4 | false | false | GenUniqueID() |
| PlotNumber | Text /7 | false | false | empty default metadata |
| PicDir | Text /255 | false | true | empty default metadata |
| PicName | Text /255 | false | false | empty default metadata |
| PicComment | Text /255 | false | false | empty default metadata |

Every table/field ValidationRule and ValidationText is empty. Default metadata
is not a stored empty-string assignment. The ID field has Attributes17
(fixed field plus AutoIncr) and the primary ID index is unique/required, despite
the field's Required=false. The separate PlotNumber index is nonunique and not
required. AutoIncr flags/default expression alone do not establish generator
behavior, restoration or deleted-ID reservation; no creation grant follows.
`GenUniqueID()` here is not a missing VBA function: the
[DAO DefaultValue contract](https://learn.microsoft.com/en-us/office/client-developer/access/desktop-database-reference/field-defaultvalue-property-dao)
defines this special database-engine expression as generating a random Long for
a new record, and excludes user-defined functions from field defaults. Actual
AutoIncr/default interaction, allocator implementation and recovery must still be
verified separately; do not invent a particular random sequence or GUID format.
The DAO logical ordinal order (ID/PlotNumber/PicDir/PicName/PicComment) differs
from native streamed storage order (ID/PicDir/PicName/PlotNumber/PicComment).
Do not reorder native records by DAO display ordinals or change the accepted
five-cell transport.

Existing PicDir/PicName source controls have no exported new-value validation
event; Form_Current updates the preview and the manager's Add a Picture action
creates a new linked row. Metadata edits must preserve literal text/NULL and
unchanged historical invalid values, reject new overlength/malformed values and
keep PicDir empty distinct from NULL. PicName permits NULL but not new empty
text. Source bounds are UTF-16 units, not UTF-8 bytes. PicComment has no mapped
control in these picture forms. ID/PlotNumber reassignment is not an implicit
metadata edit. The Add action stores a filename/directory; it does not copy the
selected image. Directory changes never create image-read authority.

The live manager path is `btnAddAPicture.OnClick` -> `btnAddAPicture_Click` ->
`AddPic Me.PlotNumber` in canonical `frmPlotPictureMaster`. The single-file
picker exits without assignments on Cancel. An accepted selection switches the
linked `frmPlotPictures` to a new record, assigns the literal parent PlotNumber,
`Dir(selectedPath)` to PicName and the remaining path prefix to PicDir, then
calls `DoCmd.Save`, updates the parent preview and requeries loaded
`FS882-8x6` / `FS882-6x4` picture children. ID is left to the engine default;
PicComment is not assigned. Empty DefaultValue metadata is not evidence for
creating empty text in either unassigned field.

The similarly named private AddPic helpers in `frmVPics` / `frmVPicsXL` are not
evidence of a live child Add event. Their exported BeforeInsert handlers only
display PlotNumber. Future creation must keep stored selected-directory literals
distinct from the existing owned reader's `Default` authorization: saving a path
cannot authorize that directory, rewrite it to `Default`, copy its file or imply
that an unsupported preview succeeded. Creation/allocation and selected-file
authorization remain unimplemented, separate boundaries.

A private [existing-picture draft](../frontend/src/pictureMetadataEditor.ts)
reuses tagged metadata parsing for only PicDir/PicName. It requires independently
approved owner and exact physical row selection, preserves detached original
cells even when initialized from Svelte proxies, rejects ambiguous signed Long
IDs and omits unchanged historical invalid assignments. New values enforce
source255-unit/NULL/empty rules and malformed Unicode/NUL guards; errors survive
unrelated correction and request construction revalidates before returning any
changes. Seven tests cover these invariants;35 coupled tests,807 complete frontend
tests/check0/0 and an isolated Vite build pass. This is tested draft preparation,
not an enabled metadata editor: no service/writer/audit/root write lifecycle or
native mutation acceptance is wired yet. Existing picture reads remain unchanged.
Source read receipts are retained privately; no canonical database/image was
modified or installed support path assumed.

The actual local `PlotPictures` folder also contains the six referenced default
JPEGs (`Pic01.jpg` through `Pic06.jpg`). A private standard-library race probe
passes1.385s: all six decode as448x300 JPEGs, retain their individually recorded
SHA256/byte sizes and original file identities after read/close. The six Default
metadata rows therefore have concrete source files; four other rows retain
historical external directory literals, not automatically authorized or repaired
paths. This is read-only source evidence, not an installed SQLite library,
published image asset, directory preference merge or desktop preview grant.

The owned draft reads an explicit existing SQLite `tblVPics`, not the detached
staging image. It validates the original five ordered columns and retains typed
raw cells, NULL/empty values, duplicate IDs and exact signed64 physical row IDs.
Parent/context and library-file ownership share cancellable read snapshots.
Literal plot membership requires text storage and byte-exact BLOB comparison;
SQLite BINARY collation alone cannot prevent numeric-affinity coercion. The
independent core review found that issue, and its focused follow-up accepts the
fix and preserved legacy-reader behavior.

Read-only `PictureService` has two independently gated methods, with strict raw
JSON Unicode/duplicate/complete-row guards before decoding reviewed image input.
Child and manager Default directories require separate explicit grants; an absent
grant is an error, never inherited from its peer. External/NULL/empty PicDir values
remain visible metadata without file authority. JPEG/PNG previews use the selected
row's own directory rather than the source's first-record ambiguity, reject
Windows filename repair/traversal/streams/devices, open within an owned OS root,
verify decoded dimensions and return unchanged embedded bytes plus SHA256.
32 MiB and16-megapixel limits are explicit desktop preview adaptations, not Access
storage constraints. Metadata editing/creation/deletion, file copying, external
directory authorization, automatic installation and all unsupported source
generator/default/length rules remain unavailable.

The exact FS882 `frmVPics`/`Form.frmVPicsXL` slot now has a separately default-off
responsive read-only panel and full-size modal viewer; other subforms cannot
inherit this renderer. Pending reads feed the existing root Save/Lock/close/context
barrier, own actual Wails cancellation and discard late/superseded responses.
Private actual-source consumer/race proof compares ten complete original rows,
ten NULL comments, all six unchanged448x300 JPEGs and two owned previews. Shared
regression race49.530s passes. Independent facade/desktop review found and resolved
an actual Svelte state-proxy clone failure using a snapshot inside the guarded
operation; the regression uses the real client proxy runtime, not plain objects.
All782 frontend tests/check0 errors/warnings and isolated enabled/default
actual-main production Wails builds pass. Six normally closed disposable owners
prove unchanged Pic01/Pic02 images,448x300 decoded/display dimensions,1400/600px
actual visibility/labels, modal viewer, tab remount/reload, distinguishable NULL
and duplicate-ID metadata, NULL/external-directory denial and independent gates.
Held actual completed metadata/image responses prove native-close refusal,
explicit cancellation, late-result discard and retry; they do not prove SQL or
decode cancellation in flight. All16 original,13 picture and30 protected identities
remain unchanged. The native reviewer found the first600px screenshot missed the
offscreen panel despite CSS visibility. A separate zero-write layout owner
preserves that evidence and supplies scrolled control/image captures at both
widths, asserting viewport intersection as well as CSS visibility/containment.
Independent native-evidence follow-up is clean; full all-package race passes
root2184.900s/all packages against894 frozen identities. The distinct485-file seal
is independently rehashed twice; manifest SHA256
`9a829781e7f0763d90808b1fc0f0414b80f6fc25050cf638266f66be277e6395`.
The account cannot create the disposable external-symlink test, so that runtime
containment case remains explicitly unverified. Deterministic cancellation or
replacement during active image reading/decoding is not yet an observed claim.

### Picture manager source boundary

The read-only manager successor is implemented and natively exercised; independent
native-evidence review is clean. Its distinct
[322-file seal](MIGRATION_EVIDENCE.md#snapshot-picture-manager-desktop-checkpoint)
is independently rehashed twice; manifest SHA256
`360237b3d9ffdc54a3c5d9412f8d11f0e783a322dafc01370e5f9720441f8f5b`.
`frmPlotPictureMaster` has caption `Plot Pictures`, binds `USysEnv`, and links
`frmPlotPictures` through both master and child `PlotNumber`. Its `PicPreview`
belongs to the parent; the child binds `tblVPics` and displays PlotNumber,
PicDir (`Picture Directory`) and PicName (`Picture File Name`). Its Form_Current
uses the selected row's PicDir and the separate PictureDir preference for Default.
This is not permission to inherit the FS882 child directory grant.

It reuses the owned metadata/image service and panel with an explicit manager policy,
not another picture reader. The separately default-off standalone manager selects
an exact existing parent through the current owned context; explicit lookup and
Load replace Access record-navigation/requery without changing data or identifiers.
Pending reads must retain root context/navigation/native-close barriers, explicit
cancellation and late-result rejection. Source labels and parent/child linkage
remain visible; manager previews require their own configured directory grant.
`VITE_PICTURE_MANAGER` is independent of the FS882 `VITE_PICTURE_READING` gate.
Forms navigation and a literal Plot number selector replace source record
navigation; Load verifies existing parent membership without applying SU/profile
filters. Code review,788 frontend tests/check0/0, focused shared backend race2.817s
and isolated enabled/default actual-main builds pass. The Go backend is byte-exact
to the accepted full-race2184.900s predecessor, not a new all-package-run claim.
Five normally closed disposable owners verify unchanged Pic01/Pic02 previews with
only the manager grant, independent child refusal, source labels, six scrolled
1400/600px parent/controls/image visuals, viewer, literal nonmember/replacement/
retry, gates and held completed metadata/image response cancellation. Parent
selection/navigation/native close are blocked while pending; cancellation discards
late results and explicit retry succeeds. All16 original/13 picture/59 protected
identities remain unchanged.
At this read-only manager milestone, Add a Picture and editable metadata were
unavailable. The existing-picture successor below implements metadata editing;
Add a Picture remains unavailable because creation/identity and selected-file
authorization are separate, unimplemented boundaries.

### Existing-picture metadata successor

The private `picturemetadatawrite.go` boundary now updates only PicDir/PicName
on the independently approved existing physical row. Its complete five-column
original, signed32 application ID, exact parent/context and pinned library
ownership are checked; only literal permitted text/NULL assignments are planned.
Library mutation and typed audit provenance in complete original/committed durable history
share a fresh no-create immediate transaction. Parent/project and external SU
membership are freshly reserved with BEGIN IMMEDIATE, including under WAL, and
held through library commit. Those reservations require RW access but make no
parent changes. This is explicitly not an atomic commit across independent files.
Protected support/context/profile aliases and identity/comment/parent edits
remain forbidden.

Stable request identities provide verified retry without a second write.
Read-only lookup requires matching durable request history and the exact
committed row; matching values alone are not a receipt. Rollbacks produce no
successful result; post-commit cleanup errors retain the private receipt and
are surfaced rather than described as unchanged data.
The independent default-off `PictureMetadataService` requires the separately
enabled owned reader. Actual production-interface bindings are generated with
the repository's production/TypeScript/interface flags, preserving nullable
arrays rather than substituting fabricated DTOs.

The frontend manager and FS882 child share one persistent context/project/plot
session. Raw errors and unknown receipts survive tab remounts; busy, dirty and
invalid states block parent Save/Lock/close and other editor operations. Metadata
Save is explicit and scoped, not a mixed parent/library transaction. Cancellation
holds unknown authority until durable lookup or explicit discard/reload.
Discard observes the current row and clears the retained draft only after a
successful owned read; it neither proves a lost Save committed nor restores a
completed Save. Directory/File Name labels, NULL choices and exact physical
identity remain visible with responsive grouping and safety feedback above fields.
Ordinary parent inputs wait for picture drafts and operations; retained picture
recovery has a separate permission and cannot be trapped behind a dirty parent.
Same-owner presentation tabs wait for actual operations, not draft-only errors.
The editor renders independently of the panel's transient read review after a
remount. Accepted Save and successful discard/reload publish owner-scoped original
revisions: the selectable physical row updates and its preview clears, without
replaying a consumed revision into a later fresh read.

Core focused race14.909s/vet, coupled facade/reader/image race14.210s and actual
facade Save/receipt/stable replay race1.606s pass. Backend review found missing
result.requestId verification: empty/different corruptions now reject both
replay/lookup with unchanged library; focused corrected race15.257s/vet passes
and the retained reviewer confirms resolution. Both frontend review findings and
two subsequent native remount/presentation gaps are corrected and reviewed.
Complete830 frontend/check0/0 and isolated enabled/default actual-main builds pass.
Six normally closed owners in `evidence/private/native-picture-metadata-acceptance`
prove four explicit scoped commits, complete twelve-row plans, typed actor/time/
original/committed provenance, invalid tab remount and parent/native-close guards,
completed Save acknowledgement cancellation with late-response retention, fresh
discard/reload then a NULL CAS, byte-identical read-only durable resolution,
independent collision rejection without audit/history commit, independent gates
and four actual-visible1400/600px field/label screenshots. All16 original and13
picture identities restore;88 protected identities remain unchanged.
Three closed preliminary candidates preserve the independently found defects;
the last required an explicit disposable page reload of unsubmitted memory for
cleanup, not accepted recovery. No completed mutation was replayed.
The pre-fix full race2195.458s is not corrected-runtime certification; corrected
full race2245.559s/all packages passes. No default promotion follows. Add/delete, historical restoration, file
copy/rename/repair and external image-directory authority remain unavailable.
The distinct386-file `archives/_picture-metadata-desktop-checkpoint` preserves
exact source deltas, actual-main enabled/default candidates/embedded assets,
typed native histories/committed libraries, four representative layouts, closure/
restoration receipts, tests and bounded review dispositions. Its manifest SHA256
`0e198efeb8ac0d8b7330f6a24448984e674563b60fac1df53b87a1610f419891`
is independently rehashed twice. Frozen archive documentation retains its honest
pre-seal state; live documents carry the completed seal.

Exported coordinates and the extractor's historical layout policy are source
evidence only. A desktop implementation must preserve labels, relationships and
meaningful grouping with responsive layout, not fixed Access pixel geometry.

## Additional fields

These fields are outside both accepted parent registries. Their exported controls
have no validation-rule or event-handler overrides. Physical limits come from
the original table definitions; the existing nullable text/SINGLE/Integer
policies are reused without adding membership rules, trimming or case repair.

| Physical owner | Field | Domain |
| --- | --- | --- |
| Env | SV_WaterTableCM | SINGLE |
| Env | ActiveLayerDepth | SINGLE |
| Env | SV_FullCruiseCard | TEXT(50) |
| Admin | PlotSize | SINGLE |
| Admin | ProvinceStateTerritory | TEXT(255) |
| Admin | SiteUnitLongName | TEXT(100) |
| Admin | GIS_BGC | TEXT(255) |
| Admin | GIS_BGC_VER | signed 16-bit Access Integer |
| Admin | BEC_Use | TEXT(255), normal only |

Module2 maps `dbInteger` to `INTEGER` and `dbLong` to `LONG`; SQLite affinity
is not evidence for widening GIS_BGC_VER. Text limits count UTF-16 units.
New empty text is rejected by the existing nullable/nonempty desktop policy;
historical NULL and empty storage remain distinguishable and unchanged
historical invalid values are omitted rather than reassigned.

## Ownership, writes and restoration

The existing owned context lease, physical schema checks and SQL snapshots are
shared. Reads preserve tagged NULL/text/integer/real/BLOB values, physical
row identities, duplicate join pairs and detached copies. Literal BINARY
membership remains an explicit adaptation, not full Access collation parity.

Save requires one unambiguous physical pair and the exact reviewed original
projection. Context, table, row, column and expected tagged values must match.
Normal-only BEC_Use cannot be planned through CHARS. Malformed, repeated or
foreign edits fail without publishing a partial plan.

Data, audits and provenance share the existing verified transaction. The normal
and CHARS variants use separate additional-field history tables. No-op edits
produce no assignments or history. Audit strength retains the existing policy:
typed restoration restores recorded audited changes, not unaudited changes.
Historical BLOB replacement remains unavailable because the source audit has
no representation; omitting unchanged BLOB values is supported.

Cancel performs no writes. Retain preserves the source audit with Access
`Restore=-1`; prune removes only the typed event's verified audit rows.
Both retain a consumed history marker and refuse replay, tampered histories
and changed physical originals. Existing table aliases are reused. Restoration
does not clean vegetation or overwrite independent changes.

Strict typed JSON ingestion rejects malformed UTF-8 and unpaired raw UTF-16
surrogates before decoder repair, unknown/duplicate members and missing required
members. Failed decoding does not partially replace the request.

## Verification boundary and next work

Focused race coverage includes both variants, local/external ownership, source
instances, typed thresholds, literal storage, historical omission, cancellation,
no-op/retry, cross-table collision, late audit rollback, exact audit values,
restoration aliases and consumed-history replay denial. The coupled two-page/all-SIVI-parent
race passes207.428s; the private-backend predecessor's all-package integration
passes root2316.664s. Registered-service focused race passes15.121s. Fresh
registered/mounted all-package race integration passes root2091.731s, against
unchanged Go source and the explicitly verified frontend-only host repair.

The private frontend session reuses the existing scoped write lifecycle and its
conservative unknown-outcome recovery, ordinary parsers and detached view state.
Shared physical-original validation leaves the SIVI-specific source wrapper
strict; two-page decoding checks all source instances and additional-field
owners. Fifteen session/controller/component tests cover exact variants, source/schema rejection,
typed limits, historical omission, persistent errors/Undo, stale proposals and
lost-response no-replay recovery, actual controller factories and host guards.
Full frontend670 tests and check0 errors/warnings pass. Actual Wails bindings and
isolated enabled/default frontend and production binary builds pass without
replacing protected default assets.

The root owns durable owner/variant session caches: review remounts preserve
drafts, errors and restoration history. An open review exclusively owns the plot;
ordinary root Save/Lock are unavailable until it is explicitly closed. Invalid
drafts and unknown outcomes block source switching, review disposal and native
close; Undo/reload is explicit. Save/restoration refresh clean parent peers.
The root's child-owner expression also disables ordinary common-field inputs
while the review is open, so a later peer refresh cannot replace a competing
ordinary draft. Closing the review explicitly restores ordinary editing.

Five normally closed production Wails owners on a fresh sixteen-identity fixture
prove normal/CHARS scope, visible associated source labels at1400/600px,
signed16 rejection, persistent draft/close guards, exactly one literal text Save,
history retention across actual remount, exactly one typed prune, default-off
assets and independent backend review/edit denial. Independent table/audit
comparison proves one business-cell change and one audit; prune restores all
original business/audit tables while retaining consumed history. Committed and
consumed-history bytes remain preserved before disposable byte restoration.
All16 fixture/22 protected identities restore, with current owner absence checked.
Actual read cancellation holds completed-response delivery, not in-flight SQL;
late delivery never populates controls and explicit retry restores nine fields.
Completed native modes and mutations must not be replayed.

Primary host review found the ordinary-input exclusion gap after those five
modes. Their exact binaries/assets/source were hash-archived before repair.
Two new normally closed guard-only owners on rebuilt enabled/default assets
prove actual common-input exclusion, clean release, refusal to open the review
over an ordinary draft, explicit Undo and default-off behavior with zero writes.
The combined native evidence is20 cases/seven closed owners, not a replay of
the original Save/prune.783 source identities and exact isolated embedded assets
are frozen; Go sources are unchanged across this frontend-only repair.

The accepted [1012-file seal](MIGRATION_EVIDENCE.md#snapshot-two-page-extra-desktop-checkpoint)
has manifest SHA256
`df8efcd0362ffc965ff5ec047510561dd5a402e0f38427fcf19d697594fac4cf`.
It preserves all783 source identities and the closed build/native evidence.
Do not reseal it or replay completed mutations.

The common-field successor implements13 normal/14 CHARS fields beyond XL:
six SINGLE fields, FloodPlain BOOLEAN0/-1, one/two source Est/Meas options,
PolygonNumber and CanopyComposition text, RootZoneTexture and AhorizonType text,
and the source-bound PlotType TEXT10 combo. It reuses existing SIVI typed
validators but has separate Common histories and exact variant authorization.
Normal has no Height Est/Meas binding. Two-page PlotType is not the SIVI
unbound five-option action. Source AhorizonType/RootZoneTexture TextBoxes accept
literal bounded text, not SIVI categorical list membership. Explicit empty TEXT
and NULL remain distinct. Shared source option/scalar/text/categorical validators
preserve unchanged historical invalid cells by omitting assignments.

The strict facade is registered and its default-off review is mounted with
durable owner/variant caches. Common/Extra reviews share the same exclusive root
owner; no scope switch or ordinary draft can compete with refresh. Commits refresh
clean opposite-scope/variant originals without discarding history or replaying
writes. Source labels and Est/Meas options are derived from exact variant metadata.
One live normal Age editor retains both exported binding identities.

Coupled two-page/all-SIVI-parent race204.982s and registered two-page race24.665s
pass.685 frontend tests/check0/0, actual Wails bindings and isolated enabled/default
production builds pass. Five normally closed owners prove17 native cases:
source scope/labels, natural PlotType list suggestions, independent denial,
ordinary-input/native-close guards, one literal PlotType Save with one exact
Admin audit, one typed prune, actual remount history and read cancellation/retry.
All16 fixture/22 protected identities restore. Cancellation holds completed-response
delivery, not in-flight SQL.

Primary visual review identified host `.grid` CSS overriding local responsive
columns. The113-file closed prelayout archive retains original builds before
repair (manifest SHA256
`c3e55294077f2641fc8bc7fd7fbb96f1f62904eea75d027b2fc2b7e1514be420`).
The common editor now uses a distinct scoped grid class, retaining source-related
Stand row grouping. Two new zero-write owners measure actual3/2/1 columns at
1400/800/600px and1 at320px, both variants/all associated visible labels, plus
default-off behavior. Static class names or nonzero rectangles alone do not
establish native layout. Combined27 cases/seven closed owners,792 frozen source
identities and exact embedded assets. Fresh all-package race1853.589s passes;
Go is unchanged across the frontend-only layout repair.

The independently verified [1052-file common-field seal](MIGRATION_EVIDENCE.md#snapshot-two-page-common-desktop-checkpoint)
has manifest SHA256
`3e883ea12b32c875118e97a1ab8aa50b5365177f6f200978743dafbf07ea6556`.
Never reseal it or replay completed native mutations. Remaining work is complete source
presentation of the existing96/98 XL fields,
common-field/event/child integration and the picture workflow. Existing accepted
standalone SIVI evidence remains immutable. Neither the additional-field panel
nor its native proof establishes complete two-page forms/reports parity.

## Current live scoped source presentation

[twoPageEntryProjection](../frontend/src/twoPageEntryProjection.ts) is a private
source assembly, not a complete native workflow. It partitions118 normal/120 CHARS
logical bindings into exactly96/98 XL,13/14 Common and9/8 Extra owners. PlotNumber
has one unpaged header owner with all three source-instance identities preserved.
Pages retain their exact Site/Veg and Soil/Terrain placement; duplicated normal
Age binds one live slot, not an invented Height Est/Meas field.
All six child identities and their literal PlotNumber master/child links are
checked. Pictures remain unavailable. Returned evidence is detached from packaged
source metadata, and the default XL page helper retains its original behavior.

[SourcePage](../frontend/src/SourcePage.svelte) accepts an explicit source
projection instead of unconditionally selecting XL. Bound OptionGroups now retain
typed editor ownership. The projection removes their descendant source option
buttons, including nested exported Group wrappers and normal's duplicate group,
so a later editor composition cannot create phantom duplicate controls.
Unbound coordinate/project/working-unit selectors remain present and unavailable
unless an accepted owner is supplied; no action is enabled by this preparation.
Source captions take precedence only for an explicitly supplied page. If absent,
binding labels precede control-name aliases: the two-page soil combo control
names and their actual SoilClassGroup/SubGroup bindings are swapped.
Decorative source lines remain geometry evidence, not empty desktop fields.

Both variants and dd/dm/dms have exhaustive unique semantic-group coverage.
[TwoPageEntryLayout](../frontend/src/TwoPageEntryLayout.svelte) composes the header
and exact pages with disjoint XL/Common/Extra editor slots. Wrong-scope, duplicate
and header-as-page owners fail explicitly; the five supported child links retain
their source identities and links, while pictures never reach the caller's child
renderer. Composition tests currently use SSR editor/child snippets, not Wails
controllers. The successor now also reuses real Common/Extra input snippets
through their actual Wails controllers; it does not copy staging/write logic.
Force the SSR rendered `.body` when checking guard exceptions:
creating a lazy render result without evaluating its body does not test a denial.
Explicit-source readonly BOOLEAN controls normalize Access true=-1 (including
the exact integer-text transport representation). Historical1 or other invalid
storage is disclosed separately, not presented as valid false or rewritten.
The default XL rendering path remains unchanged.
`VITE_TWO_PAGE_PARENT_SOURCE_LAYOUT` independently gates presentation in the real
Common/Extra reviews. Their original strict values, input callbacks, transport,
independent write authorization and durable history/draft/close ownership remain.
Labelled slots do not acquire a second outer label or duplicate live controls.
Other parent fields and the five linked child tables are read-only during the
exclusive review. The ordinary host is hidden without unmounting, then restored.
No aggregate Save or new history is introduced; pictures/actions stay disabled.

697 frontend tests (250 pretests +447 main), check0 errors/warnings, coupled
two-page Go race20.167s and isolated enabled/default production Wails builds pass.
Fifteen zero-write native cases/two normally closed owners prove both variants/
scopes at1400/600/320px, actual visible labels/unique IDs/header, five readonly
literal-linked tables, unavailable pictures/actions, ordinary-host restoration
and default denial. All16 fixture/22 protected identities remain unchanged.
The792-source common-field seal remains an immutable predecessor. Full XL editing
inside this source layout and remaining child/event/picture behavior are still
unfinished. Do not replay sealed Common/Extra writes or these completed guard modes.
The [890-file source-layout seal](MIGRATION_EVIDENCE.md#snapshot-two-page-entry-layout-desktop-checkpoint)
preserves794 application-source identities, both exact embedded builds and
closed native receipts. Manifest SHA256 is
`4a75e6c74c50da12d5d9c7783ecd30140f8215b5e2b232ee2b06685dded7e8f6`.
Every archived file and current owner absence was verified before sealing;
backend/bindings match the accepted Common pins. The20.167s focused race is fresh;
the predecessor's all-package race is retained evidence, not a successor rerun.

Integration must retain transaction scope: legacy PlotService header Save owns
supported XL values only, while Common/Extra writers retain their distinct strict
originals and histories. Sequencing these writes is not an atomic whole-form Save.
Fresh canonical static reading also distinguishes event paths: both two-page
`btnCopyToUserSU_Click` procedures assign UserSiteUnit from BECSiteUnit and Requery;
XL's Requery is commented. Normal has an additional right-click MouseDown path;
CHARS does not bind it. Normal `PlotNumber_AfterUpdate` exits immediately before
its historical rename body, whereas CHARS retains active rename code. Preserve
the existing desktop identity guard and document deliberate adaptations; do not
infer event equivalence from matching storage or blindly add control aliases.

### XL controller composition preparation

HeaderEditor now exposes its existing input renderer and seven typed controller
families as labelled slots, while preserving its default XL host. An explicit
source page supplies captions/grouping rather than aliasing XL geometry.
Unbound coordinate parts/options are owned by exact control IDs, separately from
physical columns. Both variants/DD/DM/DMS compose with one label/input per
visible coordinate part and one independent PlotNumber header. Wrong scope,
foreign identity, bound/action-as-display claims and duplicate unbound ownership
fail explicitly. Inputs resolve source flags independently of caller copies.

701 frontend tests/check0/0, isolated frontend assets and production Wails build
pass. Five new zero-write native cases verify the unchanged default editor at
1400/600/320px, visible labels/IDs/no input overflow, rejected Save, remounted
invalid XCoord draft and correction/Undo. All16 fixture/22 protected identities
remain unchanged; the owner closed normally. This is private full-source XL
composition preparation, not a mounted whole-form editor. Fresh all-package Go
race passes root2041.922s/all packages against794 pinned sources/exact assets.
The [860-file header-composition checkpoint](MIGRATION_EVIDENCE.md#snapshot-two-page-header-composition-checkpoint)
preserves the exact candidate and closed evidence. Manifest SHA256:
`c1aee275d83d668e45bbf9cd24207a6de95003f8096a95c188eea1e437b14cba`.

Further canonical reading identifies normal's right-click handler precisely:
it reads the current SU row's SiteUnit into BECSiteUnit. CHARS has no corresponding
bound handler. This implicit Master assignment must not bypass desktop Master
authorization or become an ordinary-copy gesture. Both left-click handlers
Requery after assigning UserSiteUnit; a reviewed desktop draft must not inherit
that implicit whole-form commit. Working-unit aliases/actions remain disabled in
this preparation.

### Private complete-entry physical planning

The exact source/header intersection supplies95 normal/97 CHARS XL page-field
policies. It excludes PlotNumber and cannot grant normal the CHARS-only
EnteredBy/UpdatedFromCards controls. Existing shared and typed field-family
policies supply text/numeric bounds; the remaining source text bounds come
from the canonical Env/Admin definitions. Date uses the existing explicit
wall-clock transport adaptation, not timezone conversion or implicit completion.
New BOOLEAN storage requires INTEGER0/-1; unchanged historical invalid values
remain byte-preserving omissions.

The combined planner produces one117/119 XL/Common/Extra assignment plan.
Foreign identities/scopes, stale expected values, repeated targets, invalid
tail fields, cancellation and unauthorized Master changes yield no partial plan.
Focused race passes3.057s; prior coupled XL/shared-policy race passes78.570s.
These six new backend files/shared hooks are after the794-source/2041.922s/native
header seal. The private complete-entry writer applies the combined plan in one
shared audited transaction with separate normal/CHARS Entry histories and typed
restoration. Strict original CAS precedes planning and required transaction-held
approval. No sequential scope writes or phantom no-op history are introduced.

Master editing intent is not authority. Both writing and typed history replay
intersect it with the source-authorized current user from the immutable owned
context snapshot. The request does not reacquire a nested context lease or read
mutable live user state during planning. Tests prove denial for an ordinary user
even with a permissive callback, authorized-user denial when editing is off,
unchanged files after denial, retained authorization despite a live-user change
during approval, current-user restoration denial, read-only cancellation and
unrelated ordinary writes/restoration for both variants. Final atomic/authority
race passes11.020s, shared writer/restorer regression passes160.862s and isolated
production compilation passes. Fresh all-package race passes root2172.776s/all
packages. Independent bounded read-only review found no substantive issues;
800 application-source identities/17 exact embedded assets are pinned and all
16 closed fixture/22 protected identities remain unchanged. No new native actions
were performed; compile-only verification is not live whole-form acceptance.
The [838-file private atomic-entry checkpoint](MIGRATION_EVIDENCE.md#snapshot-two-page-entry-atomic-checkpoint)
preserves this800-source successor; neither predecessor may be repinned.

There is no registered facade, bindings or UI write grant. Physical validity
does not approve reference membership or acknowledgements: an independent
implementation remains required before publication. Sequencing existing scope
writers is still not a valid whole-form Save.

### Complete-entry reference decisions still required

Fresh extraction with the existing canonical inventory/layout tools identifies
56 normal/55 CHARS bound ComboBox instances. Both variants explicitly require
`LimitToList` for six fields: SuccessionalStatus, MesoSlopePosition, SurfaceShape,
Exposure1, Exposure2 and SoilDrainage. The two-page approval boundary must not
inherit the SIVI form's different required-field flags. NULL clearing and
unchanged historical omissions remain separate from new non-NULL membership.

ProjectID has different initial exported RowSources, but both bound GotFocus
handlers select the global metadata registry when optProjectID is2, otherwise
the current project's physical Metadata table. Both NotInList handlers call
metadata loading and SaveRecord; those implicit mutations remain unavailable.
Their existence is not permission for a generic parent Save to create metadata.

Both SubZone GotFocus handlers call SubZoneList with the current Zone, or the
literal `"All"` when Zone is NULL. The canonical V7mdlFormTools helper also
treats a literal Zone `"All"` as the all-subzones branch. Reuse verified BEC
readers while preserving that source-specific filtering, duplicate definitions
and literal code values; do not inherit SIVI's empty NULL-zone choice behavior.
These are static source facts, not implemented approval or runtime parity.

A new private mandatory-membership inner helper derives those six policies from
the exact source controls, reuses verified readers' policy descriptors and
overrides required flags only in copies. It accepts NULL clearing without a
catalogue lookup, requires independent transaction-held lookup for changed
non-NULL codes, and rejects foreign identities/policies, unavailable or invalid
choices, case/prefix mismatches and cancellation. Duplicate definitions remain
selectable without changing their literal code. Both variants prove all-or-none
mixed-scope rollback and retry against independent SQL-backed fixture choices.
Focused race2.958s and coupled two-page/shared-reference race87.660s pass;
independent bounded read-only review found no substantive issues. These two new
files are not covered by the preceding800-source full integration/seal.
The helper alone is not complete-entry approval: optional-code acknowledgements
and registered service/UI wiring remain unfinished. Concrete mandatory-reader
ownership is described below; no runtime grant or native verification is introduced.

Concrete mandatory-reader preparation now captures the immutable owned request,
attaches configured VLists read-only before the existing transaction, and checks
owned family identities before preparation and before commit. Borrowed site and
parent catalogues reuse the accepted SIVI reference reader and remain owned by
their caller. No nested context request or mutable selected-owner lookup is used.
Separate full approval is still required after mandatory checks; this path is
private and unregistered.

Both variants/local-external contexts prove9-field single audited Save and typed
restoration with unchanged VLists bytes. Tests cover missing/closed catalogues,
explicit NULL clearing, unlisted choices, rejected/cancelled separate approval,
immutable prepare/verify snapshot identity, late verification rollback and
cleanup exactly once followed by retry. Coupled race102.667s, isolated production
compilation and independent bounded review pass. The four newer files/private
lifecycle refactor are not covered by the preceding800-source integration/seal.
Optional-code acknowledgement, fixed complete approval and facade/UI/native
acceptance remain unfinished.

The optional-code preparation now maps all56 normal/55 CHARS reference controls
to reused physical/list policies and their actual storage kinds. Normal binds
BEC_Use as a ComboBox; CHARS has no BEC_Use binding. An exact per-form
binding/type cohort rejects count-preserving substitutions, duplicate/missing
controls and inherited CHARS fields. This fixes an independent review finding;
the focused follow-up reports no substantive remaining issues.

Optional non-NULL codes use existing ASCII-only insensitive matching without
changing literal stored values. Unmatched or explicitly unavailable references
require acknowledgement tied to current context/project/plot/form/table/row,
original value, exact draft and complete fresh reference snapshot. NULL/empty
metadata and duplicate definitions remain distinguishable. Foreign, duplicate,
unused, cleared or stale acknowledgements fail; lookup errors and cancellation
remain errors. INTEGER LocationAccuracy retains canonical transport and physical
bounds rather than silently converting numeric choice strings.

Focused post-review race4.157s and final coupled97.630s pass, including real
borrowed-catalogue four-field XL/Common/Extra/RealmClass transactions and typed
restoration in both variants, literal preservation and byte-identical rejection.
These are private inner policies only. Concrete private providers now reuse
the accepted shared/static readers and borrowed Ecosection, quality, soil and
BEC catalogue owners; they neither create fallback catalogues nor close borrowers.
Their policy must exactly match the chosen normal/CHARS source descriptor.
NULL and literal All use the source SubZoneList all-subzones branch. SiteSeries
is filtered by the current Zone/SubZone pair; clearing a filter does not inherit
the source control's potentially stale previous RowSource. This is an explicit
desktop adaptation, not fresh native event acceptance.

Source SurfaceTopographyType excludes cc/cv/st from selectable choices, including
ASCII case variants, while preserving raw definitions and exclusion diagnostics.
LocationAccuracy offers only canonical short INTEGER choices; malformed lexical
and out-of-bound definitions remain visible but nonselectable. Source list aliases
accuracy/ecosection use the current imported Accuracy/borrowed Ecosection lists
without rewriting their raw metadata. Quality's NA code and literal description
"null" remain text, not SQL NULL. All26 imported SiteSeries metadata fields,
duplicate rows, NULL/empty values and Access BOOLEAN true=-1 remain distinguishable.
Focused race2.816s/coupled78.163s and isolated production compilation pass;
independent bounded review passes with no significant issues.

ProjectID/BECSiteUnit/UserSiteUnit still require their owned source-option/context
providers and fail explicitly; they cannot become acknowledged unavailable
success in the static provider. Separate owned read kernels now preserve actual
ProjectIdSource project/master physical metadata, configured MasterSiteUnitList
Level11 definitions and AssignedSuSource Env/Master/SU units. Env units use the
actual Admin/Env join boundary; SU units use the owned selected physical table.
Generated distinct unit rows are not presented as physical source rowids.
PRAGMA database_list aliases must resolve to the owner's exact attached file
identities, including writer main/two_page_entry_refs/main-or-sivi_su aliases.
Master/metadata definitions preserve duplicate/NULL/empty values and signed
physical rowids. Unselected SU is a known unavailable reference with an explicit
diagnostic, not a swallowed query/ownership/schema error.

Independent review found malformed SQLite TEXT could enter Working Unit typed
definitions with only a nonselectable diagnostic. Both source branches now reject
invalid UTF-8 before projection; disposable CAST(X'FF' AS TEXT) cases prove
byte-identical read errors and literal valid correction/retry. Post-fix focused
6.513s/coupled145.329s and isolated production compilation pass; focused review
follow-up confirms the finding resolved, with no significant remaining issues.
Actual writer-alias tests save
one unrelated field and perform typed audit-prune restoration: the history event
and physical history table remain, marked Restored, while every other original
table/schema row and support-family bytes are preserved.

Fixed private read composition now combines all56/55 fields and the actual
physical parent original from one owned snapshot. The result records selected
Project/Working sources and clones the explicit current draft BEC pair.
Provider/form policies must match exactly; lookup/closed/ownership/cancellation
errors return no partial snapshot, while known None-SU diagnostics remain scoped
to UserSiteUnit. All2x3 modes in both forms, source counts/required6, literal
metadata, late cancellation and retry pass focused9.980s/coupled150.984s.
Isolated production compilation and independent bounded composition review pass.

Private ProjectID planning delegates to the accepted strict source planner using
the exact form control identity. The original is reconstructed and compared
before planning. Env/Master source metadata, physical signed64 row/title identity,
duplicate NULL/empty titles and literal text remain independently bound.
Thirty UTF16 units are accepted; new NULL/empty, malformed or overlength
assignments are rejected, while unchanged historical invalid values are omitted.
Focused race2.824s/coupled116.292s, isolated production compilation and independent
bounded review pass. This does not implement mixed complete-entry history,
metadata loading or source implicit SaveRecord.

Fixed private reference approval now composes mandatory6 membership and optional
acknowledgements through the concrete owned providers. BEC filtering uses the
original plus independently reverified assignments, never a separately supplied
client filter. Exact form/provider cohorts, transaction alias files, source modes
and assignment integrity are checked; stale references, missing acknowledgements
and unused acknowledgements fail explicitly. Direct ProjectID mutations without
strict physical source-selection proof remain denied.
Actual writer tests prove rejection without partial commit, corrected retry,
literal mixed writes and typed restoration preserving all original physical
tables/schema plus the retained marked history event/table. Internal SU/project
aliases share a physical file and are not falsely compared as independent
immutable reference bytes. Focused race10.146s/coupled123.879s, isolated production
compilation and independent bounded review pass. Role authorization precedes this
kernel; it does not itself acquire preference leases.

Private owned writer composition now captures the owned request's SQLite owner
and preferences, holds the preference mutex through transaction commit, compares
both observed source modes before approval and rechecks them before commit.
VLists is attached read-only; selected internal/external SU aliases reuse the
existing writer lifecycle. Mutation-only family/master/Working2 catalogue reads
and external Working3 reads require rollback-journal locking. A WAL snapshot is
not accepted as a through-commit read lock; untouched/noop references do not
inherit this new mutation restriction. Journal modes are inspected, never changed
by the application.
All2x3 source modes/both forms/internal and external SU aliases prove atomic
rejection/retry, literal writes and restoration. A bounded probe proves concurrent
preference CAS and actual VLists/SU writes cannot escape held leases/read locks;
cancellation releases leases. Disposable WAL fixtures prove explicit rejection,
no writes, journal correction and retry. Two competing writers commit exactly
one typed history and the expected added audits, preserving fixture historical
audits. Expanded coupled race328.001s, isolated production compilation and
independent bounded review pass.

Private mixed ProjectID source/history provenance now composes with the owned
writer. A typed physical selection must independently plan the exact ProjectID
assignment; Source2 attaches owned metadata read-only and retains the accepted
rollback-journal read-lock requirement. Schema/metadata evidence is rechecked
before commit. An explicit audit user is required, as for standalone assignment.
Full typed EntryPlan stores every committed assignment independently of audited
Changes. This distinction preserves all0..3 audit strengths: strength1 can commit
NULL additions unaudited alongside an audited update. Mixed restoration validates
the complete committed projection but applies only the audited subset, never
undoing unaudited values. Source proof remains bound even for an unaudited
ProjectID addition. Legacy non-Project entry history without EntryPlan retains its
audited-subset behavior. Standalone SIVI/non-entry/other-form histories cannot
inherit complete-entry plans or ProjectID provenance.
Both forms/sources/all audit strengths/null and nonnull original IDs, current
metadata deletion/preference changes, control/title/source/row/literal/user/file
drift and retry pass. Independent review found an unused-proof check inside the
assignment loop made saves depend on ProjectID being first. It now runs after the
complete loop; the full matrix puts FieldNumber before ProjectID. Post-review
focused47.028s/coupled360.878s and fresh isolated production compilation pass.
Focused independent follow-up reports no significant issues.

No implicit metadata load/SaveRecord or ProjectID assignment acceptance is
inferred from a choice read.

### Complete-entry backend facade

The separately default-off `VPRO_TWO_PAGE_PARENT_ENTRY_EDITING` service also
requires `VPRO_TWO_PAGE_PARENT_REVIEW`. It does not inherit XL/Common/Extra or
SIVI edit permission. Borrowed catalogue services retain their existing owners;
nil concrete services are not converted into nonnil reader interfaces.
GetOriginal derives BEC filters from the owned physical source pair. Historical
invalid filters stay unchanged, with explicit unavailable dependent SubZone/
SiteSeries choices. GetReferences validates explicit current draft filters.
Project choices and assignment availability use the same owned transaction;
Source2 WAL remains readable but is explicitly unavailable for assignment.

Save delegates to one strict owned complete writer, not sequential scope saves.
Optional acknowledgements preserve explicit nullable codes/descriptions and
full definition identities. Raw invalid Unicode, duplicate/unknown/missing/NULL
request properties and case-alias write/choice properties are rejected without
replacing the receiver. ProjectID uses its exact typed physical selection;
Master authorization comes from the owned current user, not a client grant.
Restore preserves the exact form/history domain and audited-subset behavior.

Independent review's case-alias finding is resolved; focused follow-up is clean.
Post-review coupled race427.895s and actual current-main production compilation
pass. The separately default-off complete-entry desktop now uses its actual
generated bindings; fresh full successor integration passes root2091.501s/all
packages. The measured835-source successor includes twenty-eight newer Go files,
seven frontend/binding files and shared changes; the frozen800-source predecessor
remains unchanged. The [1529-file desktop successor](MIGRATION_EVIDENCE.md#snapshot-two-page-entry-desktop-checkpoint)
is immutably sealed and independently hash-verified.

### Complete-entry desktop lifecycle

Frontend review requires both `VITE_TWO_PAGE_PARENT_REVIEW` and
`VITE_TWO_PAGE_PARENT_ENTRY_REVIEW`; editing independently requires
`VITE_TWO_PAGE_PARENT_ENTRY_EDITING`. Policies advertise117 normal/119 CHARS
editable logical fields, excluding PlotNumber identity, and reuse the accepted
XL/Common/Extra domains rather than a second validator. One session owns all
parent scopes; no sequential scope Save can partially publish the draft.
Physical ProjectID changes carry both the exact scalar assignment and the
independent physical selection. Same-code selections carry neither a no-op
assignment nor an unused proof, and a changed ProjectID counts once.

Reference identity is source-specific. Zone uses DISTINCT `(code, description)`
value identities with empty rowId and no physical definitions, only under its
exact verified BEC provenance. NULL and empty descriptions remain different
tuples. Other choices retain strict physical definition-row membership. Rendered
option keys use `(rowId, code, description)` so Zone tuples and duplicate-code
physical rows remain separate. Required codes enforce exact selectable
membership; optional unlisted literals need a fresh full-reference receipt.
Obsolete acknowledgements are omitted when refreshed choices list the literal.
Invalid drafts/references and unknown outcomes retain their owner/error and
block disposal or replay. Explicit Undo/reload recovers uncertain outcomes.

32 focused tests plus701 existing frontend regressions and check0/0 pass.
Isolated enabled/default assets and actual-main Wails production builds pass.
Five final normally closed owners prove18 cases on one unseeded disposable
fixture:117/119 unique live labelled fields at1400/600/320px; hidden ordinary
controls; invalid text/Save/Lock/variant/review/native-close barriers; one planned
five-field Save across all three parent scopes, including an optional RealmClass
literal and natural physical Master Project choice; five exact audits; typed
prune recovering all original business/audit tables with consumed history;
committed-history remount; independent frontend/default/backend denials; and
completed-response read cancellation/retry, not in-flight SQL cancellation.
All16 owned/22 protected hashes restore. Earlier bootstrap failures are retained
and hash-archived; the rejected Project proof Save committed no data, audit or
history and recovered explicitly without replay. Independent desktop/native
acceptance review reports no significant issues; full Go race passes
root2091.501s/all packages.
Linked children remain read-only in this parent panel. Pictures, unsupported
callbacks, identity changes and full combined SIVI execution remain unavailable.

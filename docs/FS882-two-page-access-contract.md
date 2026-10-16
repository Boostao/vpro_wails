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

The mapped picture child has a separate readonly candidate. Actual parent
controls named `frmVPics` link `Form.frmVPicsXL` by literal PlotNumber on both
sides. The child binds PicName/PicDir over tblVPics and imgCtl double-click opens
the source viewer. The candidate reuses existing owned child-policy JPEG/PNG
preview and modal viewer behind `VITE_PICTURE_READING`, with explicit Load/
Preview rather than automatic Current file reads. Each selected physical row
uses its own directory; no arbitrary first-row/current-name directory mixing,
unbound AddPic, manager mutation or new source-form grant is inherited.
No metadataSession is attached. The ordinary picture snippet is absent while
complete-entry owns the source; picture pending activity holds parent fields/
actions/variant/review close and peer editors while Cancel remains live.
75 targeted/1571 counted registered frontend executions, check0/0, default build,
owned-picture race16.022s and production/isolated Wails builds pass. Exact
independent nine-file follow-up closes receipt auto-read and upstream layout
dispatch findings. The source explicitly opts out of retained mutation receipts;
default-false reader availability bypasses the layout fallback only for its exact
frmVPicsXL control. Common/extra/default-off callers remain unavailable.
Six closed disposable Wails owners accept normal/CHARS selected PNG/JPEG bytes,
viewer, typed NULL/empty/duplicate-ID metadata, unapproved external-directory and
invalid-path refusal, cancellation/remount and late-result rejection, source/
parent/native-close barriers and three-width containment. Two joint owners prove
actual humus/vegetation draft and pending-picture peer holds. All19 files remain
unchanged per owner; four rejected owners are closed and retained. Complete1594-input
successor race passes without input/discovery drift; final association rehashes2113
artifacts and all10 closed owners, accepting this bounded reader and releasing
its freeze. Whole-picture/form execution is not accepted.

The exported `btnManagePictures` controls in both two-page forms have no OnClick
binding (normal control lines5729-5752; CHARS5935-5959). Their private
`btnManagePictures_Click` bodies would open the manager filtered by PlotNumber,
but a named dormant procedure is not an active source event. Those mapped buttons
remain unavailable; ordinary manager capabilities are separate adaptations.

### Linked vegetation source differences (execution unavailable)

Normal `SubVegAXL_BC` and CHARS `SubVegAXL` share `USysVegA`, Species,
Cover1/2/3, TotalA, Cover4/5, TotalB and Collected bindings. Only CHARS has
live Cover5a/5b/5c controls; similarly named normal focus procedures do not
create missing controls. `cover3` binding recasing is a VBA identifier
difference, not a different field. C binds Species/Cover6/Collected through
`USysVegC`; D binds Species/Cover7/8/9/Collected through `USysVegD`.
The A query admits any non-NULL Cover1/2/3/4/5/5a/5b/5c/TotalA/TotalB,
including zero and CHARS-only values in normal mode. C admits non-NULL Cover6;
D admits any non-NULL Cover7/8/9. These overlapping queries are not exclusive
strata assignments. Physical-row evidence must not disappear merely because
clearing a cover moves a row out of one source query.

Every live bound cover/total control explicitly has `ValidationRule =
"<100 Or Is Null"`:7 normal A/BC,10 CHARS A,1 C and3 D. The control rule is
additional to the table's SINGLE type/range, rejects100 itself, and has no lower
bound or sum constraint. Species and Collected have no explicit control rule.
Unchanged historical100/>100 values must be omitted rather than repaired.
Binary64-neighbor checks do not prove the source's SINGLE narrowing/validation
timing; the later source-equivalent bound-control calibration below establishes
numeric engine behavior, not original callback timing.

All four source Species combos are LimitToList and have active NotInList
handlers; this is different from the soil free-entry choices. Collected Click
cycles NULL to `C`, `C` to `V`, then `V` to NULL. Current focuses Species.

The Species row source projects Code, ScientificName, Codetype, Lifeform and
EnglishName from `USysAllSpecies`, ordered by Code. That query unions master
`USysAllSpecs` and personal `USysUserSpp` five-column projections, excluding
Codetype `s`; it is not an identity-preserving physical definition catalogue.
Source layer criteria are:

| Form | Codetype | Lifeforms | NotInList focus |
| --- | --- | --- | --- |
| Normal A/BC | `u` or `x` | 1,2,3,4 | Cover1 |
| CHARS A | `U` or `X` | 1,2,3,4 | Cover1 |
| C | `u` or `x` | 5,6,7,8,12 | Cover6 |
| D | `u` or `x` | 1,2,9,10,11 | Cover7 |

Literal recasing in these SQL criteria does not prove case-sensitive source
comparison. Keep physical duplicates and NULL/empty metadata separately from
the union's displayed tuples; do not silently choose a DLookup duplicate.
The shared SIVI physical definition projection is a reuse precedent, not XL
callback or collation acceptance.

All four NotInList bodies first assign `DATA_ERRCONTINUE`, then look up master
Code by OldCode. A match prompts either for replacement or same-code/layer
confirmation; Yes assigns `UCase(Dusty)`, No assigns `UCase(NewData)`, then
focuses the first cover and exits. Without an alias, personal Code lookup
precedes the missing-code prompt. Missing personal codes prompt with default No;
Yes assigns the uppercase entered code, sets `sysNewSpp` and opens `USysAddSpp`.
An existing personal match temporarily disables LimitToList, focuses the first
cover, restores LimitToList and assigns its uppercase matched code. Do not
replace this modal event path with an automatic personal-list write or claim
locale/Unicode UCase equivalence. Species edits remain unavailable until the
owned references, decisions and event/lifecycle acceptance are implemented.

A/BC BeforeUpdate calls AuditTrail and the reviewed no-op
UpdateMetadataSppList, then unconditionally exits before its dormant peer merge.
D likewise exits before its merge. No peer overwrite follows from those bodies.
A/BC AfterUpdate contains the source concatenated-cover `IsNull` warning and
next-record navigation; it is not a BeforeUpdate cancellation rule. C warns
when Cover6 is NULL without setting Cancel, then audits; its Species AfterUpdate
body is commented-only. Exact runtime criteria/event timing still need acceptance.

The canonical `Sample_Veg` DDL has a nonunique ID index, unlike the unique
Humus/Mineral ID indexes. Do not transfer soil uniqueness/default assumptions,
invent an ID for historical NULL or grant editing through legacy numeric readers.
These source observations prepare the remaining child contract; no linked
vegetation write or whole-form grant follows.

#### Existing A/CHARS literal list-selection draft

The canonical A/BC and CHARS A exports have Species `LimitToList =
NotDefault` and no `Species_AfterUpdate` procedure. Their active BeforeUpdate
audits and calls `UpdateMetadataSppList`, then exits before peer merging.
The metadata helper itself jumps unconditionally to `MyExit` before mutation.

The private [list-selection policy](../twopagevegetationspecieslist.go) reuses
the physical reference projection and existing vegetation planner invariants.
It qualifies byte-exact, nonempty TEXT8 codes with ASCII `u/U/x/X` Codetype and
Lifeform1-4, preserving all matching master/personal physical definitions,
duplicate rows and NULL/empty metadata. It does not normalize entered codes,
resolve aliases or extrapolate source locale/width equivalence. Master LONG
and personal INTEGER bounds retain their distinct existing policies.

Private planning checks exact reference/original ownership, physical source
membership, expected cells and signed32 application ID. It omits unchanged
historical invalid Species and refuses a changed Species when either original
or planned state has all seven common covers NULL; CHARS-only covers do not
authorize the unavailable warning/advance route. Numeric/Collected planning
remains shared, not synthesized as a no-op ownership check.

This bounded existing-list workflow is implemented and surfaced only behind
independent default-off backend and client capabilities.
The optional `speciesList` request carries the complete strict physical-reference
snapshot. The omitted legacy envelope is unchanged, and legacy public Save
still denies Species selection authority. The registered
selection service requires review, writing and its independently default-off
`VPRO_TWO_PAGE_VEGETATION_SPECIES_LIST` gate. It reuses the existing attempt
ledger, atomic writer and typed durable history rather than a second engine.
Both readonly reference attachments require rollback journaling and remain
reserved through commit; their complete observations must equal the reviewed
snapshot. Recovery reads original durable provenance after later reference drift,
without writing or requiring the selection gate.

The backend batch passes coupled vegetation/selection race85.006s and production
compilation. Independent review found no implementation defect but requested
reference-reservation acceptance. Both master and personal tests now prove actual
WAL refusal without writes, DELETE correction/retry, a competing precommit write
blocked, and release after commit (focused race5.709s); focused review closes
that coverage finding. Retained client drafts and bindings/service registration
pass independent desktop review,68 focused/1515 registered frontend cases,
check0/0 and isolated Wails builds. Actual strict-Go receipts and Svelte proxy
tests cover the production wire rather than plain-object-only transports.
Disposable normal/CHARS Wails owners each prove exact Species+cover Save,
complete physical peer preservation, two source audits/one durable history and
unchanged support/config files. Actual successful final responses, including
UTF-8 chunked requests, are held through native-close refusal and Cancel;
late responses cannot resolve retained unknown state, readonly recovery does
not replay Save, and immutable receipts survive subsequent tab/panel remount.
Additional zero-write owners prove full duplicate tuples and NULL/empty
metadata, three-width visibility/label/viewport containment, original
common-cover-empty/NULL-ID refusal, coupled off-row errors with live correction,
client-off/backend-off refusal and actual C/D read-only Species observations.
Complete-entry review retains its existing regular-tab/close barrier while
active; this is not a new source implicit-navigation grant. Failed navigation/
cleanup and Python numeric audit-text expectations remain rejected receipts.
The completed mutation was sealed and recovered on its same owner, not repeated.
Complete1551-input race integration now passes exit0 in5120.640562s with unchanged
inputs and ordinary/production package/embed discovery. Final association
rehashes541 artifacts,20 canonical sources and34 reviewed inputs, verifies all13
source/native owners normally closed, and releases the freeze without default
promotion.
Separate zero-write joint-profile acceptance preserves the existing normal/
CHARS literal PlotNumber links to both soil forms and A vegetation. Both soil
forms/all30 fields are actually loaded; a Species draft blocks soil staging, a
soil draft blocks Species staging, and each active owner retains correction/
Undo. Nine-unit Horizon input independently exercises the source eight-unit
bound, blocking parent Save/Lock/close and native close without disabling valid
correction. Two normal/CHARS owners preserve all16 files and supply180 labelled
three-width soil measurements. No application source, implicit callback or
picture/SubOther workflow is added by that private flag combination.
NotInList/manual/alias/personal creation, C/D selection, creation/deletion/
restoration, implicit focus Save, peer merging and default promotion remain
excluded.

#### Source-equivalent Species query/lookup engine calibration

One fresh disposable Access database executes the original exported UNION and
all four complete combo RowSource SQL strings, without rewriting their literals.
Master/personal projected fields use the source TEXT255/Code TEXT8 and LONG/
INTEGER distinctions and the personal unique Code index. Explicit fixture-only
AllowZeroLength supports historical empty metadata; this is not a claim about
the original tables' AllowZeroLength settings. All107 master and4 personal
physical definitions remain distinct; the source UNION yields46 display tuples.
Normal/CHARS A each return16 identical ordered tuples; C returns11 and D19.

The bounded LCID1033 observations establish case-insensitive layer criteria and
alias/personal lookup. Source database text comparison treats case variants
and trailing spaces as equal, but E/accented E as different. UNION collapses
case/trailing-space-equivalent complete tuples, including identical master/
personal duplicates, while NULL and empty metadata remain distinguishable.
The retained literal representative is not a stable physical-definition choice:
the controlled case-paired lifeform rows return mixed retained code casing.
Do not replace physical references with the displayed union or silently pick
one ambiguous definition.

Source UCase preserves sharp-s, dotless-i and long-s in these cases, uppercases
accented lowercase E, preserves trailing spaces, and leaves a combining dot
after capitalizing i. JavaScript `toUpperCase` or Go Unicode uppercasing is not
source-equivalent for this boundary. These representative results do not
establish whole-Unicode collation/casing or original NotInList dialogs/focus.
The source owner is absent, canonical/export hashes match, and six evidence
files are sealed. Species writing remains unavailable; no application source changed during the
separate1071-input SINGLE integration run.

#### Original Current and old-code Yes callback proof

A disposable full original database copy disables startup and every original
linked table before opening any Form view. Local source-shaped Veg/master/
personal tables contain one existing controlled BASE record; default expressions
are removed only for this explicit existing-row fixture. Original form code,
bindings and query SQL remain unchanged. All four actual source forms open in
CurrentView1 with Species focus, correct BASE/CAL001 values, no dirty/new-record
state and unchanged physical data. Before/after form exports are byte-identical.
The owner normally closes and the entire clone is byte-restored;24 files retain
the adapted proof and original baseline.

A successor executes the original old-code NotInList Yes branch on all four
forms. Typed `old` matches OldCode OLD, the exact source replacement prompt
offers NEW, and only that planned Yes is invoked. Each actual form retains
Species NEW, dirty state, restored LimitToList and first-cover focus
(Cover1/Cover1/Cover6/Cover7) without a physical Save. Explicit Undo/normal close
leave every complete physical Veg cell unchanged. The source registry and
protected canonical/export files remain unchanged.

The first attempt stalls because the COM Text setter itself invokes the modal
NotInList event before the planned focus call. Its exact known prompt is observed
and resolved once, without replaying Text assignment; NEW/Cover1 state is captured,
undone and normally closed. The second guarded attempt dispatches Text assignment
on the owned COM worker, records intent before that fallible call, and completes
all four original branches. Both clones are byte-restored;7 first-attempt files
and18 completed-attempt files are sealed, and no native source owner remains.
Successors now verify old-code No across all four forms: uppercase OLD is retained
with first-cover focus even though OLD is not a displayed union member. The
same-code/wrong-layer Yes and No branches both retain OTHER and first-cover focus;
an existing personal PERS excluded by Codetype also becomes an uppercase dirty
draft without a prompt. All12 latter cases execute the original callbacks with
unchanged physical data. Do not invent a layer/union-membership requirement for
these explicitly confirmed source paths.

Missing-code No is observed on all four forms: physical/typed Species remains
BASE, raw `missing` remains in the focused Species control, and the form is dirty.
The native COM setter reports explicit Access3021 (`No current record`) after
the known No decision; it is retained as a source error, not a successful edit
or an empty-field fallback. Undo/close restores the complete original rows.
The12-case successor retains its final missing-No stop; the second targeted
missing-No observer completes all four known-error cases. Closed clones are
byte-restored, with18 alias-No,41 partial-successor and22 completed missing-No
files sealed. No source owner remains.

Missing-code Yes/USysAddSpp is still unaccepted after two bounded attempts.
The first known Yes returns explicit source3270 (`Property not found`); the
second reaches the expected opened child before its Report Recordset read raises
Access3021 on the new-record buffer. The original handler's OnError Resume Next
must not become an application success fallback. Both attempts Undo/close with
unchanged complete physical Veg/personal rows, protected sources/registry and
byte-restored clones;9+8 files are sealed. No third debugging attempt or source
creation grant follows. Remaining creation/save/validation, warnings/navigation
and callback audit timing stay pending. No Species/default/whole-form grant follows.

The private [vegetation original reader](../twopagevegetationread.go) now
returns complete tagged physical rows independently of source-query membership,
with separate physical `sourceRowIds`. NULL/duplicate/text logical IDs, unmapped
cells, HeightB text and overlapping zero/nontext cover membership are preserved.
The [shared child read scope](../twopagechildreadscope.go) extracts only the
demonstrated-equivalent selected/literal physical parent guard; soil errors and
callers remain unchanged. Physical views, missing parent/filter columns,
generated/shadowed identities and malformed text refuse explicitly.

Coupled vegetation/soil/reference race14.428s, production compilation and vet
pass. Tests include1024 A and8 D cover masks, both SU attachments, six affinities,
physical-parent failures, correction/retry, queued cancellation and actual
context rotation across all four forms. Two fixture-only failures are retained:
Env rename invalidated an existing report view, and a partial/lowercase Veg
fixture could not satisfy ordinary context initialization. No application
fallback was added. Independent exact-four-file review reports no significant
issues and accepts the bounded readonly prerequisite; its test rerun was blocked
by local DuckDB/CGO constraints, not counted as an independent pass.
This readonly prerequisite alone grants no editing or native acceptance.

The private [vegetation draft planner](../twopagevegetationplan.go) now covers
the existing bound cover/Collected subset:8 normal A/BC,11 CHARS A,2 C and4 D
assignable fields. Species remains explicitly unavailable until its owned
reference/NotInList callback contract is complete. Plans check current source
membership independently of physical-row retention, exact original parent/
context/form/expected cells and complete distinct physical rowids. Duplicate
signed32 logical IDs are valid peers, not invented uniqueness constraints;
NULL/noninteger edited IDs remain unavailable without repair.

Unchanged historical invalid assignments are omitted. New Collected values
retain nullable/empty/manual TEXT1 semantics and UTF-16 bounds without trimming,
case changes or invented C/V membership. Cover changes reuse existing nullable
REAL/SINGLE-range draft adaptation with the strict source `<100` control bound.
The later calibrated candidate below narrows new drafts before that bound,
without rewriting prior historical assignments or receipts.
Original-form callback/timing parity is not claimed. Clearing a last cover may leave the source query and is not
rejected as a new membership rule. Complete typed assignments detach their
before/after cells, without changing any original or peer.

Coupled planner/reader/soil/reference race15.955s, production compilation/vet
and independent exact-three-file review pass with no significant issues.
Tests cover all25 editable bindings, NULL, duplicate-ID peers, raw Unicode/
UTF-16 bounds, exact/escaped duplicate JSON properties, expected/owner/member/
physical witness refusals, unchanged omission and cancellation. Those initial
reviews did not catch the additional cover-control rule. Facade/client lifecycle
and actual Wails/source calibration remain next;
these are implemented drafts, not a desktop write grant.

The private [vegetation writer](../twopagevegetationwrite.go) now commits the
reviewed cover/Collected plan, source audits and full typed stable-UUID
[history](../twopagevegetationhistory.go) together. Its strict
[request](../twopagevegetationrequest.go) retains complete originals and rejects
duplicate/escaped JSON keys and repaired Unicode. Exact replay and read-only
receipt recovery retain irreversible provenance after later source drift.
Nonunique logical-ID peers are never merged or overwritten; unchanged
historical rows and unrelated project tables remain distinct.

The equivalent selected-parent writer query is shared with soil without
changing its messages. Initial/final membership stays on the writer transaction,
and external SU rollback-journal reads remain reserved through commit; WAL is
refused with correction/retry available. Precommit cancellation/refusal rolls
back audits/history; postcommit completion errors retain the committed result.
Every audit strength retains the complete typed committed plan, including
legitimate unaudited changes.

Exact REAL bits are checked separately from Go's ordinary numeric equality.
New negative-zero values remain unavailable pending lossless source/storage
calibration; different signed-zero originals/expected values/UUID payloads
cannot masquerade as exact witnesses. INTEGER-affinity normalization of a
planned REAL is refused and rolled back; REAL physical storage passes.
This hardening is confined to the new vegetation path, not unrelated legacy
editors. Coupled new vegetation/soil writer/spill/reader/reference race87.805s,
production compilation/vet and independent exact-eight-file review pass with
no significant issues. Formatting failures from nested test insertions were
corrected before the accepted compilation and remain recorded as failures.
Whole integration of retained client lifecycle and native
acceptance remain required; Species, creation/deletion/restoration and source
SINGLE conversion are not granted.

The additional cover-control correction is a bounded three-file follow-up:
the planner checks changed non-NULL covers strictly below100, while explicit
tests enumerate all21 source bindings independently of implementation lists.
Expanded exact/adjacent100,
negative/NULL, unchanged100/>100, zero-output rejected Save, correction/replay
and historical-receipt validation pass in coupled race103.270s; all-package
production compilation and root vet pass. Independent exact-three-file
follow-up reports no significant issues or meaningful coverage gaps, limited
to this correction; its test execution was CGO-blocked, not counted as a pass.
The705-input full integration was stopped before frozen inputs changed and
retained as unaccepted partial runner evidence; a fresh freeze/run is required.
No facade/client or desktop write grant follows from this correction.
The fresh corrected full integration passes exit0 with707 unchanged frozen
inputs and unchanged ordinary/production package/embed discovery.

The next private [owned Species-reference reader](../twopagevegetationspeciesreferences.go)
returns master/personal tagged physical definitions with explicit context,
project, literal plot and source form. It reuses the existing strict SIVI
projection, preserving physical rowids, aliases, duplicates, NULL/empty and
historical overlength metadata rather than replacing them with union choices.
Both attached source tables must be physical, without shadowed/generated
identities. This reader does not grant Species edits or interpret DLookup,
Access union ordering/collation, UCase or modal events.

Coupled reader/shared-SIVI/soil-reference race47.367s, final all-package production
compilation/vet and independent exact-two-file review pass with no significant
issues or meaningful coverage gaps. Tests include all four form
owners and both SU arrangements, both reference families, raw malformed UTF8
correction/retry, schema refusal, actual context rotation, complete no-write
bytes and in-flight cancellation followed by successful pinned-coordinator
retry. Initial fixture failures are retained: canonical personal Code is
unique/NOT NULL; duplicate/NULL personal definitions require an explicitly
weakened disposable legacy fixture. Context rotation uses the complete canonical
Veg shape, not the lowercased replacement test schema. Duplicate Admin ownership
uses an explicitly copied disposable parent without its canonical unique Plot
constraint. No application fallback or canonical rewrite was added.

The independently default-off [vegetation facade](../twopagevegetationservice.go)
and [server-owned attempt ledger](../twopagevegetationattempt.go) are now
implemented and independently reviewed. Review uses
`VPRO_TWO_PAGE_VEGETATION_REVIEW`; cover/Collected Save additionally requires
`VPRO_TWO_PAGE_VEGETATION_WRITING`. Original and physical Species reference
reads plus receipt/outcome recovery need review, not writing. No Species
decision/write, creation/deletion/restoration or native conversion grant follows.

Strict existing request decoding precedes registration. The context-owned
ledger retains canonical immutable payload strings, including distinct REAL
signed-zero witnesses, rather than caller-owned pointers or ordinary numeric
equality. Active facade calls prevent absence certification. Sealing an
inactive request permanently excludes unseen late matching dispatch in this
server session; changed payloads collide explicitly. Exact committed replay
remains read-only. Original-context missing history can become `notCommitted`
only after sealing and a successful owned physical read; missing history in a
rotated context remains `unknown`. Read errors/cancellation never unseal the
request or certify absence. Once irreversible history commits, a later
completion error retains its truthful result and permits readonly recovery.

Coupled vegetation/soil/SIVI-Species/context race154.269s, all-package production
compilation and vet pass. Independent exact-nine-file baseline review reports
no significant correctness/source-alignment/logic or coverage issues. Actual
main registration and isolated interface-mode generation change exactly3
planned binding files with no generator warnings.43 client lifecycle/reference
regressions, check0 errors/0 warnings and isolated default bundle13.09s pass;
accepted embedded assets are unchanged. The partial rotation fixture's missing
HeightA failure is retained and corrected with the complete canonical Veg shape.
The initial npm.ps1 execution-policy refusal is retained; npm.cmd succeeds
without changing machine policy.

Strict [client originals](../frontend/src/twoPageVegetationWire.ts) and
[drafts](../frontend/src/twoPageVegetationEditor.ts) now implement all25 source
bindings. Validated scalar copying detaches actual Svelte proxies; membership
remains separate from complete physical rows. Nonunique signed32 IDs remain
editable without peer writes; NULL/noninteger IDs remain unavailable, not zero.
Unchanged historical values omit before new bounds; nullable/manual Collected
retains NULL/empty, UTF-16 limits and raw Unicode refusal. A dedicated serializer
preserves negative-zero source witnesses, without authorizing new negative zero.
Shared strict-Go fixtures, seven exhaustive client cases, coupled
fixture/planner/soil-wire/compiler-fold race6.375s, check0/0 and isolated default
bundle5.04s pass. Initial VM-realm assertion failures were corrected by
structuredClone, preserving actual proxy inputs and REAL bits.

Identifier keys use1424 generated Unicode15.0.0 SimpleFold classes, not browser
case conversion. An exhaustive compiler test checks version, unique ordered
membership, EqualFold equivalence and every rune's SimpleFold closure/coverage.
Client tests exercise every class and complete-original collision/noncollision
refusals. The initial review's Greek/ligature pairs U+0390/U+1FD3,
U+03B0/U+1FE3 and U+FB05/U+FB06 are nonaliases in target Go1.25.0; the
equal:true shared witnesses failed Go validation and are retained as failed
evidence. Corrected independent nonalias expectations pass. A distinct focused
reviewer reconciles this compiler-version distinction, independently reruns
Go/client tests and reports no significant issues. This table affects identifier
ownership only, not Species values, source collation or UCase.

The strict [client receipt/outcome validator](../frontend/src/twoPageVegetationReceipt.ts)
now confirms exact retained requests, complete committed physical tables and
ordered source audits, including unchanged nonunique/NULL peers and unrelated
cells. Five shared actual-Go atomic-writer receipts cover strengths0-3 and
last Cover6 removal. The latter recomputes source membership without dropping
its physical row or its Collected write. Four production-Go outcome shapes
prove the wire contract, not a new native protocol acceptance: exact request
bytes/status/owner, readonly committed receipts and original-context sealed
absence remain required; rotated absence remains unknown.

Actual Svelte proxy receipts detach through scalar copying. Typed INTEGER/REAL
changes do not reconstruct unavailable raw input; historical BLOB audits use
Go byte-slice text. REAL signed-zero witnesses remain exact in requests and
committed peers. Source-audit equality is deliberately different: Go
`reflect.DeepEqual` treats negative/positive zero equally, so a permitted
negative-to-positive zero assignment stays in complete typed history but has
no source audit at any strength. Independent Go and actual-editor client cases
check this distinction and readonly recovery without implicit sign repair.
Review found the initial audit mismatch and an exclusive fixture generator
nested in a validation loop. Both are corrected; the separate top-level output
probe completes once and matches the existing fixture hash. Focused race7.063s,
14 client cases, check0/0 and private build6.10s pass. Distinct focused review
closes both findings with no significant issues; failures remain evidence.

The [retained session](../frontend/src/twoPageVegetationSession.ts) and
[exact-owner registry](../frontend/src/twoPageVegetationSessions.ts) are now
independently reviewed. Original coupled batches pass59/39 cases, check0/0
and private default builds5.07s/5.32s. Stable lossless requests, validation-yield
generation checks, original-context absence certification and permanent receipt
retention survive remounts. Unknown delivery refuses new writes and Undo;
readonly physical selection cannot replace its retained request.

The default-off [linked panel](../frontend/src/TwoPageVegetationPanel.svelte)
now presents all25 fields once in source-bound A/BC, CHARS A, C and D groups.
Labels and relationships are retained using responsive groups rather than fixed
Access geometry. NULL, historical storage, readonly Species and physical rows
outside source membership remain visible. Session-owned selection survives
last-cover Save/reload/remount; a missing selected row does not select a peer.
Current-owner correction remains live, with only the exact owner excluded from
aggregate blockers. Other-owner drafts, busy operations, unknown requests and
unacknowledged receipts block parent/root lifecycle and background reads.

The enabled complete-entry review profile explicitly bypasses ordinary numeric
vegetation readers/actions/totals; default calls are unchanged. All8 compiled
flag combinations and nullable totals are tested. An independently observed
row/control/display guard drives an explicit Collected staging button, with no
implicit Save. Physical reader order and the explicit button are adaptations:
source Species collation, Current/Enter, callback warnings and NotInList remain
unverified. Creation/deletion/restoration remains unavailable.

Final1102 registered frontend tests pass309+448+345, check0/0 and isolated
default/enabled builds5.19s/5.55s pass. Exact-baseline independent live review
reports no significant issues. Earlier fixture/SSR failures remain evidence;
lazy server rendering must consume its body to execute production flag probes.
Fresh immutable integration now passes full race exit0 in60m24s with1069
unchanged Go/frontend/manifest/embed inputs and unchanged ordinary/production
discovery. Four normally closed actual-main owners prove default legacy
NULL-ID refusal, all25 live bindings, six normal/CHARS layouts at1400/600/320,
visible associated labels/interactive buttons, scrolled viewport containment
and historical NULL-ID readonly reachability.

One distinct native lifecycle proves invalid100 correction/Undo/native-close
and parent-variant barriers, pinned physical remount selection and a once-only
Cover1 REAL5.25/Collected TEXT C Save on row910401. Independently reopened
complete physical peers/unmapped cells remain unchanged. Exactly two source
audits at independently frozen strength3 and one complete typed UUID history
commit together. Actual held-response cancellation, unknown-close refusal,
readonly recovery, acknowledgement and immutable receipt remount do not replay.
The committed database is retained; all16 baseline identities plus protected/
predecessor sources restore, with535 sealed files and no active native owner.

The first enabled candidate used incorrect parent prerequisite flag names and
never exposed source panels or dispatched Save; it remains zero-write failed
evidence. Correct exact flags build15.27s. Two harness remount waits read the
panel before parent readiness; continuation is restricted to the exact final
checkpoint, never a repeated staging/Save/recovery. These are not application
fallbacks. A fifth normally closed owner proves exact C predispatch refusal
and readonly certified absence with zero writes. A distinct new UUID then clears
only Cover6 on C-only physical row910404, creating one strength3 source audit
and full typed history. The failed UUID is independently absent from history;
the actual C source predicate excludes the retained physical row. Readonly
recovery and remount retain its selection/receipt without replay or peer changes.
All16 identities/protected originals restore and twelve new C files are sealed.
The preceding535-file proof plus seal is byte-archived outside Go discovery;
ordinary/production discovery remains17 packages with unchanged embeds.
### Original master LONG and personal INTEGER reference boundaries

The original master `USysAllSpecs.Lifeform` is LONG (signed32), while personal
`USysUserSpp.LifeForm` is INTEGER (signed16). These are separate physical-read
policies even when the imported SQLite columns both declare INTEGER. The
two-page reader had reused the legacy SIVI signed16 master projection; an actual
facade consumer first reproduces refusal of a valid source master LONG.

The corrected shared projection/read loop now takes explicit source policies:
two-page master nullable signed32, personal nullable signed16. Existing
SIVI/report policy remains unchanged. Complete physical rows/duplicate codes,
NULL/empty metadata, declared types and detached tagged cells are retained.
Master32768 and signed32 endpoints succeed; master overflow and personal32768
fail without partial output, coercion or writes. Malformed-source correction,
fresh reads, owned contexts/parents, cancellation and caller coverage remain
in the coupled tests.

Focused race10.340s, coupled race102.656s and production build/vet pass.
Independent exact-baseline review verifies canonical widths and reports no
significant issues/coverage gaps; its targeted race passes in10.538s. A normally
closed actual-main Wails owner independently reads all four form references,
verifies source range refusals/corrected retry and foreign scopes, and proves
writing remains disabled. All16 fixture/protected/predecessor identities
restore;219 files retain the readonly acceptance. Fresh1073-input full race
passes in3335.517s at2026-10-09T05:36:10Z, exit0 with unchanged inputs/discovery;
the explicit source-width read boundary is accepted. No new Species writing, callback, creation or
whole-form permission follows.

### Read-only physical Species review in the linked desktop panels

The accepted physical reader is now reachable from each independently enabled
vegetation panel through an explicit read-only action. Master LONG/signed32 and
personal INTEGER/signed16 remain separate policies, irrespective of SQLite
declared types. The strict client wire retains original column names/types,
signed physical identities, duplicate codes, NULL/empty and historical long
text. Tagged scalars are copied rather than cloning Svelte proxies.

The exact-owner retained session shares operation/cancel/native-close/context
barriers without clearing numeric drafts, validation or committed receipts.
Validation yields every128 rows and before publication; an awaited old generation
cannot populate a fresh read. Responsive metadata pages contain20 physical rows
per source. Paging is an adaptation, not Access sorting/UNION membership or a
Species choice/editing grant.

Independent exact-baseline review closes one npm test-registration finding.
Actual-client owner resolve/reject and independent/clamped paging regressions
close its coverage findings; focused59/59 and all1117 registered tests
(309+448+360), check0/0, private default/enabled builds13.42s/5.86s and actual-Go
strict facade/shared-wire race3.359s pass. Independent follow-up2/2 reports no
significant remaining issues. The initial1106 lifecycle receipt omitted the
new reference tests and is superseded, not acceptance of that coverage.

One actual-main Wails owner proves all four readers and six normal/CHARS
presentations at1400/600/320, retained typed seed metadata against reopened
SQLite, native paging, source error/correction, real held validation-yield
cancellation/fresh read/stale-result refusal, remount, native-close refusal and
invalid numeric-draft retention/Undo. No Save is dispatched and backend writing
remains disabled. The harness initially expected signed16 wording instead of
the actual source INTEGER error; continuation resumes the same observed error
checkpoint. Explicitly closing the clean complete-entry review is also required
before normal native shutdown, as the existing lifecycle intends. Both recovery
receipts are retained without replaying successful writes or relaunching Access.
All16 baseline/protected/predecessor identities restore and335 files are sealed.
Fresh1078-input full race passes in3216.339s at2026-10-09T06:54:07Z:
exit0/unchanged inputs and ordinary/production discovery.

### Original cover warning and focus observations (bounded)

Two normally closed, byte-restored original-form fixtures retain14+23 sealed
files. Original normal A and CHARS A save all live cover/total controls as NULL,
then display the exact informational message
`You must enter some cover value for this species`. Complete independent physical
reads while that message is still open prove the Save already committed. After
the planned OK, the original callback moves to a clean new-record buffer with
Species focus; no new physical row is created. Normal A still has its unmapped
Cover5a/b/c values, while the cleared CHARS row leaves the A query. The source
concatenated-NULL predicate must not be replaced by an assumed ordinary-string
result, and this postcommit message is not a Save cancellation rule.

The normal A observation initially stopped because the warning was incorrectly
predicted absent. Only the now-observed original informational OK was later
acknowledged for normal cleanup, without replaying the completed Save. CHARS
capture stopped on missing Pillow; the dependency was installed and the same
owner continued from its exact committed checkpoint, again without replay.
Measured original GotFocus/LostFocus label colors cover18/21 live bindings:
seven normal A, ten CHARS A and one C. The remaining D controls were not reached.

Original C displays the same message in BeforeUpdate. Its immediate independent
ACE read still showed the old Cover6, so acceptance stopped; final normally
closed storage contains the planned NULL. This timing discrepancy remains
explicitly unresolved, not a cancelled Save or successful complete C acceptance.
The D Save was never dispatched. Complete peers, unmapped cells and personal
rows remain unchanged in retained storage. Effective source audit strength1
produces zero deletion audits; this does not calibrate strength3.

No third debugging launch follows these two attempts. C publication/after-state,
D Save/focus and complete callback lifecycle remain unavailable. Canonical
exports/data and registry are unchanged; no native owner remains and no
Species/default/whole-form permission follows.

### Original Collected click observation remains unavailable

Two bounded attempts stop before any actual native mouse click. Source COM
ActiveControl reports Collected, but this alone does not identify the Windows
focused element. The second observed global UIA focus belongs to the foreign
VSCode Agents document, not the owned Access PID/control/value; guards reject
it before input. The first Value-pattern read failed before sufficient native
provider evidence. Neither establishes a source accessibility-provider defect
or original Click acceptance. No guard is relaxed and no third debug launch
follows.

Both owners normally close after Undo, complete physical Veg/personal rows
remain unchanged, original bindings/code and canonical/registry hashes are
preserved, and9+11 files retain sealed byte-restored clones. Fixture-only
AllowZeroLength was enabled to prepare historical-empty observations but no
empty case or Click was reached; it is not an original table-property claim.
Existing reviewed/native desktop Collected/shared-helper behavior remains
separate evidence. No original-source Click, creation or whole-form grant follows.

### Calibrated vegetation SINGLE assignment candidate

Two fresh disposable source-equivalent Access bound-control/storage attempts
retain the first expected-rejection stop and the successful eight-case run.
The successful owner is absent; canonical/export hashes remain unchanged.
The [shared scalar fixture](../frontend/src/twoPageVegetationSingle.json) records
independently observed SINGLE bytes:5.25,99.99999,99.999996,-1 and signed underflow;
99.999999/100 reject before saving and preserve the physical prior1.
This is the matching DAO SINGLE/textbox `<100 Or Is Null` engine contract,
not copied original VPRO callbacks, focus ordering or whole-form execution.

The candidate client preserves exact unchanged historical values first,
then narrows new assignments with `Math.fround` before the source bound.
99.99999/99.999996 prepare REAL99.99999237060547;99.999999 rejects because its
SINGLE is100. Positive underflow prepares+0; negative underflow remains explicitly
refused because SQLite cannot losslessly preserve new negative zero.
Raw text and correction/Undo/lifecycle barriers remain intact.
The new-write planner requires canonical SINGLE-as-binary64 typed REAL values
rather than silently normalizing transport intent. Request decoding/detachment,
durable history validation and readonly recovery retain the historical validation
policy; old noncanonical binary64 commitments/replays remain exact.

All21 numeric controls use the same independent eight-case fixture; midpoint
neighbors and0.1 have explicit independent expected client values. New-write
refusals leave data/audits/history unchanged; corrected retry, exact canonical
storage, complete peers and historical recovery have focused race coverage.
47 focused client tests, all coupled vegetation Go race tests70.792s,
check0/0 and private enabled build5.67s pass. All1105 registered frontend tests
(309+448+348) pass. Independent exact-baseline read-only review finds no significant
issues; original Access callbacks/timing and fresh full integration are excluded
from that disposition. Subsequent fresh1071-input full race passes in3360.692s
at2026-10-09T04:15:47Z: exit0, no changed frozen inputs or discovery.

One normally closed disposable actual-main owner proves all eight draft cases,
zero writes before Save, native-close refusal with correction/Undo live, and clean
remount retaining physical row910401. One exact Save prepares raw99.999996,0.1
and1e-46 as Cover1 REAL99.99999237060547,Cover2 REAL0.10000000149011612
and Cover3 REAL+0. Independent reopened storage tags/scalars, all three retained
typed edits, exactly three strength3 source audits and full UUID history agree.
Complete peers/unmapped cells and all other tables are unchanged. The held actual
response is cancelled once; readonly recovery/acknowledgement/remount preserve
its immutable receipt and physical identity without replay. The committed DB is
retained, all16 baseline identities/protected originals restore, and219 files
are sealed. No native owner remains. Calibrated SINGLE integration is accepted.
No Species/callback/default/whole-form grant follows.

### Linked soil physical-read prerequisite

The private [soil original reader](../twopagesoilread.go) preserves complete
ordinary physical Humus/Mineral columns and tagged rows for one exact literal
parent. It reuses the owned SQLite context, caller-owned pinned snapshot,
literal-text storage filter and ordinary/unshadowed schema guard. Selected
context membership, actual physical parent-column existence and unique physical
Env/Admin parents are checked within the snapshot; TEMP shadows and project views cannot substitute for source
tables. NULL, empty text, BLOBs, unmapped cells and historical invalid/nullable
logical IDs are retained without inventing editable identities.

Both normal and CHARS link the same `SoilHumusXL` / `SoilMineralXL` source
forms. Their `UsysHumus` / `UsysMineral` queries select the complete original
tables; BeforeUpdate calls `AuditTrail`, Current focuses Horizon and the
focus helpers select Enter's Next Record/Next Field behavior. Humus source
presentation orders UpperDepth descending and Mineral ascending. The new
reader deliberately returns physical-rowid order for review evidence, not
source presentation-order parity.

Ten top-level race tests and matrix subcases pass, including six affinities,
embedded/external families, raw invalid/nullable identities, physical-schema
and duplicate-parent refusals, malformed UTF-8 correction, actual context
switching and actual in-flight SQLite cancellation followed by a successful
read on the retained coordinator. Coupled reader/parent-owned race passes,
root14.334s; vet and production compilation pass. Independent review found
SQLite's double-quoted missing-column fallback could produce empty success or
expose unrelated rows for a parent literally named `PlotNumber`. Physical
Env/Admin/soil parent columns are now required before predicates. Both soil
families, embedded/external storage, the literal-name trap, mutation-free refusal,
schema correction/retry and missing Env/Admin columns are covered. Focused
independent follow-up closes the finding with no significant issues and reruns
the regression matrix under race, root3.374s. This is a private read boundary only: no
service/main/bindings, UI permission, mutation, audit or metadata repair is
introduced. The two-page linked-child host remains explicitly read-only.
Its ordinary soil handlers reject the open parent workflow, so enabling the
host or bypassing those guards is not an implementation. A reviewed shared
parent/child draft boundary and stable unknown-mutation receipt handling
remain necessary before existing-row editing; creation/deletion and whole-form
acceptance are separate unresolved boundaries.

### Existing soil assignment planning

The next private prerequisite is [existing-row assignment planning](../twopagesoilplan.go),
not a writer. Its owned files are that planner and its
[tests](../twopagesoilplan_test.go). Canonical Forms, Queries and Tables_Def were
read before implementation. Exactly12 Humus and18 Mineral visible data bindings
reuse the existing `childFields` and physical soil validators; ID, PlotNumber,
Flag and other unbound physical cells are not edit permissions. Both variants
share these exact soil forms. No active per-field soil callbacks or exported
LimitToList override adds a new membership restriction.

The contract checks original source identity, complete canonical tagged rows,
literal parent storage, exact physical rowids, edited signed32 logical IDs and
same-parent peer collisions. Every draft carries its original context/form and
expected tagged cell. Unchanged historical invalid values are omitted before
domain validation. Changed cells preserve NULL/empty distinctions and enforce
UTF-16 text bounds, finite SINGLE-range REAL and signed16 INTEGER storage using
the existing adaptation that retains binary64 precision within SINGLE range.
Strict nested transport rejects unknown/duplicate decoded keys and malformed
raw Unicode before decoder repair. Assignments detach caller-owned cell pointers.

Acceptance requires independent expected results for all30 bindings, physical
domain/transport/ownership failures, actual original-reader/no-write integration,
coupled existing soil regressions, vet/production compilation and independent
read-only review. Validation passes under race, root40.039s, with vet and
production compilation passing. Independent review reports no significant
issues; its reviewer could not rerun tests because Go was absent from its
session PATH. This does not
prove current live context authority, uniqueness outside the scoped original,
SQL mutation/trigger behavior or native child editing. A separately contracted
transactional writer must reread the complete physical sources, prove global ID
ownership, atomically commit mutation/audit/stable-request provenance and retain
committed receipts across cancellation/cleanup errors. No facade, bindings,
frontend ownership, creation/deletion/restoration or depth-order/navigation
behavior is introduced by planning.

### Private transactional soil writer

The next owned batch is [the private stable-request writer](../twopagesoilwrite.go)
and its [tests](../twopagesoilwrite_test.go). No existing editor, facade, main,
bindings or frontend file is owned by this batch. A lowercase UUIDv4 request
contains the complete tagged original and explicit expected-value edits. An
owned context operation lease and the existing immediate selected-file writer
freeze context ownership and serialize project transactions. The writer proves
literal selected membership, physical ordinary Env/Admin/soil tables, unique
physical parents and global numeric/text logical-ID peer ownership. It does not
call the legacy affine-parent/ID mutator or the SIVI wrapper that discards
postcommit results.

New requests compare the complete scoped schema/rows, then update exactly the
physical rowid and literal parent. Mutation, operation-specific child audits and
`__VPRO_TwoPageSoilHistory` commit together. Full-project before/after snapshots
verify planned soil/audit changes and exclude trigger/affinity drift; the exact
technical history append is reread in the same transaction and
validated before commit. Technical provenance records the full changed plan even
when source audit strength0/1 leaves legitimate changes unaudited.

Exact UUID replay and a separate read-only receipt lookup never replay SQL
updates. Absence remains unresolved without creating tables; corrupt,
substituted, shadowed or differently scoped history explicitly refuses. Receipts
retain immutable original context, actor, audit policy, complete committed rows
and actual audits after later legitimate source changes or context rotation.
Postcommit cancellation/completion errors retain the committed result and direct
the caller to recover its exact request rather than retry the mutation.

Acceptance covers both physical families/forms, all30 data bindings, all four
audit strengths and explicit NULL/empty audit outputs; complete source/peer
replacement, global aliases, generated/shadowed schema, trigger/history errors,
rollback/retry and INTEGER affinity normalization; strict request/raw-Go
Unicode, UUID collision/concurrency, historical receipt corruption/correction
and actual context rotation. An independent database connection proves staging
is uncommitted before in-flight cancellation and proves visibility before
postcommit cancellation/error. Coupled new/read/plan/parent-owned/legacy soil
race tests pass, root77.674s; vet/production compilation pass. Independent
writer review reports no significant issues and reruns all11 writer tests under
race, root30.561s, and the coupled matrix, root95.587s. Its scope excludes
unrelated dirty changes, repeated vet/production checks and native actions.
This is not native acceptance or a UI/write grant.
Creation/deletion/restoration, source depth ordering/focus/navigation and shared
parent/child draft ownership remain outside this private writer batch.

### Independently gated soil facade

The next bounded facade owns only [the service](../twopagesoilservice.go) and its
[tests](../twopagesoilservice_test.go), not main registration, bindings or frontend.
`VPRO_TWO_PAGE_SOIL_REVIEW` and `VPRO_TWO_PAGE_SOIL_WRITING` use the existing
literal `true`/`false` feature parser and are absent/off by default. Writing alone
cannot grant review; read-only review can recover a committed exact request even
when writing is disabled. GetOriginal retains the complete physical DTO and its
`declaredType`/tagged-cell wire. Save and RecoverReceipt accept the retained
request JSON through the reviewed strict decoder rather than decoding a repaired
or permissive property map.

Three facade tests cover default/explicit-false/read-only/writing-only/enabled
states without source writes, malformed feature values, nil/cancelled/stale scope,
exact original/result round trips through the actual kernel, mixed audit-strength1
provenance, duplicate/escaped keys and raw-Unicode refusal. Coupled private soil
race passes, root37.863s; vet and production compilation pass. Independent review
reports no significant issues and independently runs all three facade tests under
race. No source child editing, references, identity changes, lifecycle or
native authority is enabled. Actual Wails registration/interface bindings and
persistent parent/child draft/error/irreversible-receipt ownership require the
separate following integration batch.

The facade is now constructed and registered in actual main with repository-standard
startup error handling. Interface-style Wails generation (`-ts -i`) adds exactly
the soil service module, its index export and nullable interface models; all other
binding bytes are unchanged against the privately preserved pre-integration
baseline. No generation warnings occur. Actual-main production compilation,
all1002 registered frontend tests, Svelte check0 errors/0 warnings and an isolated
production build pass without replacing accepted assets. Independent registration/
binding review compared the exact preserved baseline and reports no significant
issues. Registration is not a child editor: the embedded host
is still disabled, and actual Wails lifecycle/mutation acceptance remains required.

### Typed soil client drafts

The bounded [client draft boundary](../frontend/src/twoPageSoilEditor.ts) owns
only original decoding, edit eligibility, field staging and complete request
construction, its [client tests](../frontend/src/twoPageSoilEditor.test.cjs),
the shared [wire fixture](../frontend/src/twoPageSoilWire.json) and its
[strict Go consumer](../twopagesoilwire_test.go). Session/registry/panel/root
wiring, source navigation and native acceptance are excluded.

Originals require exact context/project/parent/form/source, complete columns
with `declaredType`, canonical tagged cells and literal TEXT parent ownership.
Physical rowids remain signed64 strings; selected IDs must be actual signed32
INTEGER storage, never inferred from NULL. Validated scalar copying detaches
actual Svelte client-runtime proxies without `structuredClone` on a proxy.
All12 Humus/18 Mineral fields reuse existing source policies, preserve unchanged
historical storage by omission and distinguish explicit NULL from empty text.
New values reject malformed Unicode, UTF-16 overlength text, invalid signed16
integers and nonfinite/out-of-SINGLE-range numbers without whitespace repair.
Requests recompute raw drafts and refuse changed expected values, inconsistent
owners, duplicate assignments and invalid UUIDs.

The shared strict-Go round trip passes under race, root1.204s; independent
expected assignments retain negative signed32 IDs and max signed64 peer rowids.
All1009 registered frontend tests pass (309+448+252), Svelte check reports
0 errors/0 warnings and the final isolated production build passes5.87s.
Vet and production compilation pass. The initial check failed because generated
private model names are not index exports; types now derive from the public
result alias without modifying generated files. Failed runner evidence is retained.
Independent draft review found a peer collision mismatch: exact tagged equality
missed REAL/TEXT aliases accepted by the Go identity guard. The corrected client
guard follows Go decimal/hexadecimal float grammar, rounding and Unicode spaces
without JavaScript-only coercion.33 shared independent witnesses exercise both
the client request and actual Go planner, including normal halfway/adjacent values
and signed subnormal boundaries. Coupled strict-wire/planner race passes5.005s.
Focused independent follow-up closes the finding with no significant concerns.
This prerequisite grants no child editing or native lifecycle authority.

### Retained soil sessions and receipt boundary

The private [soil session](../frontend/src/twoPageSoilSession.ts) and
[receipt validator](../frontend/src/twoPageSoilReceipt.ts) add persistent
in-memory originals, drafts/errors, exact attempted request JSON and immutable
receipt logs. Registry/root/panel/native wiring remains excluded.
Unknown delivery blocks editing, Undo, reload, new Save and disposal; read-only
recovery uses the original bytes, never repeated mutation or matching source
values. A transport settlement barrier prevents premature recovery.
Missing history remains unresolved: promise settlement alone is not proof of
backend quiescence and cannot authorize replay or discard. A server-owned
no-commit certification was excluded from this boundary; the separately reviewed
server-owned protocol below now provides it.

Committed results require the exact retained request, complete physical source
plan including unchanged peers/unmapped cells, actor/time and policy-specific
source audits. The full typed plan remains separate from that audited subset.
INTEGER-to-REAL changes with identical audit text remain real typed assignments;
receipt validation never reconstructs raw input and accidentally omits them.
Existing calibrated Go audit text handles numeric fields; historical BLOB cells
retain Go byte-slice audit notation. Validated scalar copying detaches actual
Svelte proxy receipts, and acknowledgement/reload/disposal retain receipt logs.

The [shared receipt fixture](../frontend/src/twoPageSoilReceiptWire.json) was
generated by actual Go serialization and history validation for all four audit
strengths; its temporary generator is removed. The
[Go consumer](../twopagesoilreceiptwire_test.go) verifies independent committed
cells/counts/provenance, strict round trips, typed storage transitions and BLOB
audits. Coupled private soil race passes54.832s, vet/production compilation pass;
1021 frontend tests and check0/0 pass, isolated build5.62s.
Independent read-only review reports no significant issues and independently
runs24 client cases and uncached strict-Go race1.371s. Two additional tests close
the noted load/recovery validation-yield coverage gap; all26 focused cases pass.
None of these checks establishes native linked-child lifecycle acceptance.

### Persistent soil registry

The private [registry](../frontend/src/twoPageSoilSessions.ts) retains sessions
by exact literal context/project/parent/source-form keys. Normal and CHARS share
their actual soil form owners. Frontend `VITE_TWO_PAGE_SOIL_REVIEW` and
`VITE_TWO_PAGE_SOIL_WRITING` are independently default-off and intersect the
backend gates; writing alone cannot grant review. Review-only sessions refuse
staging/Save before transport without inventing an unknown mutation.
Bindings use actual GetOriginal/Save/RecoverReceipt signatures and preserve
the exact retained request JSON.

Aggregated activity includes busy operations, raw draft errors, unknown outcomes
and unacknowledged receipts across owners/remounts. Clean zero-receipt entries may
be explicitly released; acknowledgement and release never erase receipt-owning
entries.32 focused cases and all1029 registered frontend tests
(309+448+272) pass, check0/0 and isolated build4.92s pass. Independent read-only
review reports no significant issues and reruns31 focused cases; an added
multi-owner/unsubscribe/resubscribe case closes its coverage suggestion.
Root parent/child interlocks, live panel, source ordering/focus and native
acceptance remain excluded; the old child host is still disabled.

### Server-owned soil attempt sealing and outcomes

The [attempt ledger](../twopagesoilattempt.go) belongs to the context service,
not individual facade instances. Exact UUID/payload registrations count all
active public soil Save calls. Read-only RecoverOutcome reports running while
an attempt is active. Otherwise it seals the request before reading history,
including unseen requests: a late RPC can thereafter only recover an existing
receipt or explicitly refuse, never dispatch another mutation.
Successful original-context owned history absence can therefore certify
`notCommitted`; absence after context rotation remains unknown. Positive
history preserves the original committed provenance and existing explicit replay.
Sealing is in-memory coordination, not a database write or new audit row.
The legacy RecoverReceipt remains available without treating its NULL as proof.

The [client outcome decoder](../frontend/src/twoPageSoilReceipt.ts) requires
the exact context/UUID/retained request bytes, status/reason and nullable receipt.
Only explicit server certification releases an unknown attempt, retaining drafts
for correction/Undo and retiring its UUID. New Save is explicit with a new UUID,
never automatic replay. Both committed and no-commit validation recheck their
generation after yielding. The registry uses actual RecoverOutcome bindings.
The [shared outcome fixture](../frontend/src/twoPageSoilOutcomeWire.json) and
[Go consumer](../twopagesoiloutcomewire_test.go) prove the actual wire; its
temporary generator is removed.

No-commit ownership exposed a tightly coupled Windows file-ID issue:
[SQLite attachment capture](../sqlitecontext.go) now uses an opened handle's
Stat and surfaces stat/close failures, then verifies identity after ATTACH.
Path-based Stat could lazily adopt a renamed replacement during SameFile.
Actual same-path replacement after closing disposable handles proves frozen
identity/profile refusal; this is not a timed replacement-during-ATTACH probe.
The existing family/configuration/TEMP-view infrastructure is unchanged.

Tests cover shared-facade concurrency, payload collisions, unseen/late dispatch,
rollback/correction/new UUID, context rotation, physical retarget/correction,
lease cancellation and exact pre/postcommit facade observations. Independent
SQLite reads prove staged invisibility and actual committed visibility.
Cancelled sealed recovery keeps late dispatch refused; postcommit completion
errors retain the committed result and release registration before read-only
recovery. Coupled soil/SQLite context race65.043s, vet/production compilation,
1033 frontend tests (309+448+276),36 focused cases, check0/0 and isolated
build5.17s pass. Interface generation has no warnings and changes only index,
models and the soil service against the exact preserved baseline.
Independent backend, focused cancellation follow-up and client/binding reviews
report no significant issues and rerun their bounded race/client tests.
The frozen687-input full integration race passes with unchanged inputs and
ordinary/production package/embed discovery. It does not accept the separately
built candidate UI's native lifecycle.

### Experimental live soil host and ownership barriers

The default-off review flag now mounts
[TwoPageSoilPanel](../frontend/src/TwoPageSoilPanel.svelte) in complete-entry
review only, for `SoilHumusXL`/`SoilMineralXL`. Other children keep their disabled
ordinary `SourceChild` host. Writing still
requires the independent client and server flags.

When soil review and complete-entry review are both enabled, the parent does
not dispatch legacy numeric-ID Humus/Mineral readers or ordinary soil suggestion
reads. Those readers reject historical NULL IDs and cannot represent all tagged
physical storage. Ordinary soil rows, counts and actions (including partial
parent review scopes) instead display an explicit unavailable-profile notice,
never empty-record or zero-count claims. Complete-entry physical `GetOriginal`
remains independent. Other required reads/capabilities still run and their
errors propagate; header Soil/Terrain editing remains independent. With either
review flag off, legacy soil reads and presentation remain unchanged. This is
an intentional default-off read-profile adaptation, not a legacy DTO conversion
or a new write grant. All1051 frontend tests, check0/0, isolated builds and
independent exact-baseline review pass. A compiled production flag matrix covers
all8 combinations; independent focused rerun18/18 closes its coverage gap.
Disposable Wails default/enabled read acceptance proves preserved legacy
refusal, parent readiness, all30 live bindings and NULL-ID read-only physical
reachability. The fixture explicitly removes only the copied INTEGER ID's
primary-key rowid alias to represent nullable historical IDs, retaining field
types, FK and indexes; Mineral includes a BLOB-depth diagnostic witness.
Both owners close normally, with all16 owned identities/protected originals
unchanged. This read acceptance does not establish responsive layout, mutation,
keyboard/lifecycle or complete source parity; those remain pending.

Subsequent disposable acceptance passes normal/CHARS at1400/600/320 with all30
associated labels/fields and interactive buttons, actual visibility/containment
and scrolled viewport checks. Partial scopes show unavailable notices instead
of empty/count claims, while ordinary Soil/Terrain header inputs remain live.
Actual Enter changes existing-record Horizon focus without saving. Invalid
draft correction/Undo and parent/native-close guards pass. One Humus Save
commits three fields, three audits under the original configured strength3 and
complete typed history; held actual-response cancellation/read-only recovery
does not replay it. Separate actual chunked final-upload refusal is certified
no-commit; correction/Undo and immutable receipt remount/acknowledgement pass.

A distinct chunked Mineral Memo attempt and one explicitly new UUID after
no-commit certification both fail at the final parent-scope check. The writer
queries the coordinator connection after its own large write has spilled the
SQLite cache and acquired an exclusive project lock. A focused Go regression
reproduces this on both forms and embedded/external SU contexts. The candidate
fix performs initial/final source-equivalent Env/Admin/SU membership checks on
the write transaction, not a second reader; external SU membership requires a
rollback-journal read reservation through commit (WAL/memory/off unavailable).
The reservation invariant is shared with complete-entry reference writing,
without changing its operation-specific errors. Focused race tests pass,
including large committed recovery, final uncommitted scope refusal, competing
SU writer blocking, WAL correction and cancellation/rollback/retry. Independent
exact-five-file baseline review reports no significant issues; shared journal-mode/
scope race28.878s and production/vet checks pass. Fresh690-input full integration
passes with unchanged inputs/discovery. A distinct corrected actual-main candidate
passes the previously failing actual chunked Mineral Memo case: one Comments
commit, one source audit at frozen strength3, complete typed history and no
unrelated/NULL-ID/BLOB peer changes. The final actual chunk response is held;
cancellation, unknown-outcome native-close refusal, read-only committed recovery
without replay/extra audits and retained acknowledged receipt all pass.
All five native owners are closed and16 identities/protected originals restored;
the new committed database and accepted/rejected cases remain separate.
The corrected seal verifies all453 predecessor files before launch and records
418 unchanged after close plus35 disposable WebView profile changes, with the
predecessor owner identity archived byte-exactly. This accepts only the bounded
existing-row soil host; source lists/mixed-storage calibration and creation/
deletion/restoration remain unavailable. No full/default-on grant follows.

The next private reference reader follows the six Humus and four Mineral combo
bindings from the exported forms. In particular, `FecalAbundance` and
`MycelAbundance` both query `MycelAbundance`; `ASP` uses `MinSoilAspect`, `Texture`
uses `SoilTexture` and `vonPost` uses `vonPost`. These source Items remain tagged
TEXT definitions, including numeric-looking Items, not invented bound-value
coercions. The reader uses the actual configured physical
`VLists.USysTableOfLists`, never the ordinary TEMP view or a frozen catalogue.
All ten physical columns, rowids, NULL/empty metadata, duplicate definitions and
Access BOOLEAN integer storage remain distinguishable. Source `ItemOrder` is
retained; rowid is the explicit stable-tie adaptation.

The same selected/literal/physical parent guard now protects original and
reference reads with unchanged original-reader errors. Missing groups and
malformed text fail the complete result; corrected physical metadata can be
retried. Coupled race10.179s, production compilation/vet and independent
exact-three-file review/focused coverage follow-up pass. Actual cancellation
during parent SQL followed by successful reference reading retains pinned
attachments/TEMP views. Reference-file replacement, concurrent catalogue
changes and cancellation specifically during VLists retrieval remain
uncalibrated shared-helper cases.

The next candidate exposes readonly `GetReferences` behind the existing
independent review flag, with additive interface-style bindings. A shared actual
Go facade fixture proves the client wire's `declaredType` schema and complete
tagged cells. Exact owner/source/field mappings and physical definition identity
are validated before UI state; raw snapshots detach actual Svelte proxies without
`structuredClone` on reactive input. A per-panel cancellable lookup refuses late
cancelled, superseded or disposed/remounted delivery. Failures remain explicit
and retryable, never success-shaped empty choices.

Each source definition has a separate physical-ID disclosure and selection
button, retaining duplicate codes, NULL versus empty descriptions, all ten
metadata columns and true=-1 storage. This is an explicit responsive desktop
adaptation of the source's two-column combos, not fixed Access geometry.
Selecting observes the current target physical row/form and current definition
identity/value, then uses existing guarded TEXT/NULL draft staging. Numeric
Items pass through the existing Integer parser; no extra Access coercion,
membership restriction, trimming/completion/case change or automatic Save is
introduced. Unavailable/historical nontext Items cannot be chosen; manual entry
and existing correction/Undo remain independent.

Independent exact-thirteen-file baseline review reports no significant issues
and reruns24/24 focused tests. Additional direct actual-Svelte tests cover both
ASP/vonPost TEXT-to-parser staging, NULL versus empty selection and wrong-form
refusal;25/25 pass. All1058 frontend tests/check0/0, coupled Go race11.204s,
production/vet and private final default/enabled builds11.04s/11.13s pass.
The695-input full race passes in58m05s: exit0, unchanged inputs and ordinary/
production discovery. A sixth actual-main Wails owner accepts all6/4
disclosures in six normal/CHARS layouts at1400/600/320. Actual visibility,
opened metadata, scrolled viewport intersection and containment are checked;
duplicate physical definitions retain NULL/empty descriptions and -1/0 flags.
Mycel/Fecal share the actual source group; ASP/vonPost TEXT choices use the
existing Integer parser, and NULL versus empty remains explicit. An invalid
`+1` fixture choice blocks Save, parent disposal/variant changes and native
close, survives source refresh, and remains correctable/undoable.
An actual held reference response is cancelled before parent close/remount;
late delivery cannot populate the new panel, while a fresh read succeeds.
Malformed fixture Note UTF8 refuses all usable definitions before transport;
restoring that fixture cell and retrying succeeds without a project write.
One planned MycelAbundance/vonPost choice Save commits exact TEXT `A`/INTEGER
`1`, two source audits at independently frozen strength3 and complete durable
typed history. Unresolved-response cancellation/native-close refusal, read-only
recovery and acknowledgement/actual parent remount preserve the receipt with
no replay or unrelated physical changes. Normal close restores all16 baseline
identities/protected sources; the committed DB and fresh profile are retained.
The seal records220 new files and16 unchanged preceding sealed files.
This accepts bounded existing-row source choices, not mixed-storage source
calibration, creation/deletion/restoration, whole-form or default editing.

All12/18 visible source fields retain labels and logical responsive groups.
IDs and parent links are read-only physical provenance; nullable/noninteger/
colliding application IDs never become ordinary ID0. The host uses explicit
NULL controls, distinct from empty text, and preserves raw drafts/errors and
immutable receipts across row selection and tab remount. A missing source
column explicitly makes the row read-only instead of inventing it.

The parent's actual aggregate draft/reference/operation state is separate from
the intentionally always-unsaved review-open close wrapper. Actual parent
ownership and Lock block new child staging/Save. Child ownership blocks new
parent staging, Save/restoration, variant changes and review close. Existing
parent correction/Undo and child Undo, cancellation, read-only recovery and
receipt acknowledgement remain available without replay or implicit disposal.
The registry also owns root close/context/navigation/Lock barriers.

The canonical source specifies Humus descending and
Mineral ascending UpperDepth, Horizon focus on Current, and utility application
options Next Record while focused/Next Field on loss. The host now sorts exact
INTEGER/finite REAL/NULL depth storage in those directions without converting
numeric-looking TEXT. NULL sorts first ascending/last descending; ties keep
physical reader order. Missing or historical TEXT/BLOB depth explicitly reports
physical-order presentation. These are stated desktop ordering adaptations,
not mixed-storage Access calibration, and never reorder the immutable original
or complete technical receipt/request plan.

Explicit load pins the selected physical record so a committed depth resort
does not implicitly change its identity. Selection/load focus the selected
record's visible editable Horizon; unmodified Enter moves to the next existing
record's Horizon after checking actual
panel/record/focus/value observations. Enter never Save/creates or wraps; source
validation/unknown/busy/acknowledgement and parent ownership stop it with a
visible diagnostic. Modifiers preserve normal control behavior. No global
application preference is written on focus/loss; this is a scoped desktop
explicit-Save adaptation. Browser newline display normalization is used only
for independent navigation observation, never to rewrite original source bytes
or stage unchanged assignments. NULL inputs stay editable with an explicit NULL
marker; typing creates a deliberate non-NULL assignment.

Source combo suggestions use the accepted bounded choice workflow above.
Creation/deletion/restoration are not enabled. Real DOM/keyboard/native
existing-row order/focus and source-choice acceptance are verified above;
this is not complete source creation or automatic-Save behavior.

SSR and actual Svelte client-script acceptance cover all30 bindings, NULL-ID/
missing-column refusal, error remount/correction/Undo, once-only mocked Save/
immutable acknowledgement, unknown remount/certified no-commit recovery and
parent/child guards. This is not real DOM/Wails or complete source parity.
The live-host/root batch passes1041 frontend tests, check0/0, isolated default/
enabled builds and independent review with no significant issues. The subsequent
depth/Enter batch passes1047 frontend cases, check0/0 and corrected isolated
default/enabled builds10.36s/11.21s. Independent review reproduced a successful
depth Save switching current record after a retained-draft remount; synchronous
attachment selection fixes it. Primary reproduction/fix and both focused
regressions plus independent follow-up close that finding. The source3-file
baseline and failed reproduction are retained. Disposable native lifecycle/
layout/mutation acceptance remains required before this host can be accepted.

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
[distinct485-file seal](../archives/picture-read-desktop-checkpoint/evidence-manifest.json).
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
that an unsupported preview succeeded. Desktop creation and selected-file
authorization remain unavailable, separate boundaries.

The private, unregistered [creation kernel](../picturecreation.go) prepares an
exact owned parent and literal Windows filename/directory prefix, appends the
five native-order tagged cells with NULL PicComment, and commits dedicated
technical history and permanent library-wide identity reservations atomically.
Positive smallest-safe signed32 allocation is a documented desktop adaptation,
not Access's random `GenUniqueID()` sequence. Numeric aliases and deleted
reservations cannot silently reuse an identity. This kernel neither copies nor
reads the selected file, grants its directory, infers source audits or accepts
that a native chooser ran.

Twenty focused race tests pass, root 22.030 seconds, and vet passes. Independent
SQL expectations cover exact append/NULL/storage, full read-only UUID recovery,
postcommit cancellation/enclosing read-finalization receipt preservation, and
same/different-UUID completion both through owned callers and independent
library transactions. Initial independent review found no demonstrated defect;
focused reliability follow-up closed both coverage gaps without further
findings. No service, binding, picker, preview authority, UI, default promotion
or native acceptance follows.

The private [selected-file authority](../pictureselection.go) freezes identity
with an opened handle, preserves the chosen literal separately from its resolved
target and rechecks both paths around a bounded image read. Actual disposable
Windows junction cases verify alias retargeting, physical replacement and
after-read rejection without elevated privileges or skips; focused race passes
in 2.350 seconds and vet passes. Identity/size/time checks do not establish
immutable bytes against same-size changes with restored timestamps. The reused
[image decoder](../pictureimage.go) retains JPEG/PNG, 32-MiB and 16-megapixel
limits; exact-limit and limit-plus-one cases preserve ordinary owned-directory
read behavior. No directory or sibling read authority follows from selection.

The private [native chooser adapter](../picturenativechooser.go) requires Wails'
actual request window, its live native handle and exact application registration.
It attaches a single-file dialog to that owner, retains the literal result and
rechecks owner/context after the modal returns. Invalid/nonlive/cancelled request
tests pass under race in 1.226 seconds and vet passes. An already-open native
dialog is dismissed through its own Cancel action, not by pretending promise
cancellation closes it. Coupled picture race tests pass in 55.220 seconds; vet
and production compilation pass. Independent selection/decoder/chooser review
reports no significant issues within this private boundary. No registered
facade, UI or actual native modal acceptance exists yet.

The private [creation facade](../picturecreationservice.go) independently gates
creation behind `VPRO_PICTURE_CREATION_WRITING` and owned reading, not existing
metadata-writing permission. Opaque tokens bind the selected object to the
actual native app/window/handle and literal context/project/parent. `Choose`
Cancel returns no selection and performs no append. `RecoverSelection` reads
the retained same-scope capability after lost delivery without opening a
chooser, changing lifecycle state or granting another file. `Create` accepts
only the strict five-key owned request matching that token and refuses
automatic re-entry after writer entry. `LookupReceipt` is read-only and needs
no surviving file token. Selected preview requires a matching durable complete
physical row and returns complete owner/row/image identity; preview failures do
not erase a metadata commit. Release and acknowledgement relinquish only file
authority, never history or reservations.

Private [client validation](../frontend/src/pictureCreationEditor.ts) and
[retained sessions](../frontend/src/pictureCreationSession.ts) preserve literal
paths, raw Unicode/source bounds, stable request identity and all five committed
cells including unassigned NULL PicComment. Open native dialogs cannot be
dismissed by cancelling a promise. Late Create delivery cannot replace cancelled
client state; recovery reads history instead of repeating the mutation.
Acknowledgement and subsequent Add attempts retain earlier immutable receipts.
The actual strict-Go selection/request/receipt/image fixture passes these client
validators, including `declaredType` columns and nullable transport fields.
Independent review identified premature null receipt recovery while Create was
running and subscriber cancellation before request registration. Public recovery
now refuses matching in-flight activity and observes request activity across
the read; internal preflight remains separate. Client generations are captured
before subscriber notification and rechecked before dispatch. Focused review
closes both defects without further significant issues. Final coupled Go race
passes in 58.112 seconds; primary focused race 9.749 seconds, vet and
production-tag compilation pass. Client 60 coupled/966 full tests/check and an
isolated fixed Vite build pass.

Actual main registration and nullable interface bindings now connect the manager
candidate behind independent `VITE_PICTURE_CREATION_WRITING`, still absent/off
by default. The source live child Add remains unavailable. A per-parent session
registry keeps modal/unknown outcomes visible to root close/context/navigation
guards even when a reader or panel is disposed. Successful Add attempts preview
only the selected object and prompt loaded peers to re-read the complete committed
physical row; a changed or missing row is a refresh error, not invented success.
Receipt acknowledgement and new Add attempts retain earlier immutable receipts.
Integration review found a real immediate-subscription effect loop and missing
behavioral coverage. Creation and coupled metadata revision notifications are
now untracked, preserving session identity tracking and cleanup. New idle session
construction suppresses only its unchanged initial aggregate notification,
avoiding reactive mutation inside the manager's derived owner selection. Thirteen
integration cases exercise installed client effects, actual receipt-triggered
two-peer refresh, complete-row drift and cancellation/disposal/owner-change late
rejection, once-only Add/Cancel0 and retained root close/context/navigation/
decision barriers. Focused independent follow-up closes both findings.
All979 frontend tests/check0/0 and rebuilt isolated default/enabled assets pass;
corrected coupled Go race66.967s, vet and production compile pass.
Read-only selection recovery remains available under independently enabled
owned reading when creation writing is off; every write/selection/image grant
keeps its existing gate and scope checks. Actual Wails verifies backend refusal
followed by absence recovery and normal close.
The Windows prompt uses documented IFileOpenDialog/IShellItem interfaces via
existing go-ole. Owner state is a separate Go object, not native COM memory.
Complete native argument lists and signed HRESULT checks preserve errors;
only Show's ERROR_CANCELLED produces typed normal Cancel. Path decoding rejects
malformed UTF-16 instead of repairing a selected identity. Acquired interface
references and allocated path memory release before balanced STA uninitialization.
The public CFD v1.2.0 trial crashed before chooser observation and was rejected
for ABI/owner-memory/error-classification defects; its dependency is removed.
Its failed owner absence, unchanged protected bytes and archived build are
retained as failure evidence, never acceptance. Focused independent review of
the replacement closes all three findings with no significant issues.
Three actual attached Windows Cancels return immediately to ready Add without
recovery, error, receipt or physical library/history/allocation changes.
One literal spaced/#100% selection commits exactly one row; independent reopening
checks all five physical cells, durable history and permanent reservation.
Native selected-only preview/hash/sibling refusal, complete-row peer refresh,
read-only receipt recovery, Home/native-close barriers,1400/600px actual
visibility/viewport containment and replacement refusal pass. Acknowledgement
retains the commit receipt; the owner normally closes. The committed private
database is hash-archived before fixture restoration;117 protected identities
and all owned originals remain unchanged. Candidate pins cover920 source inputs,
including manifests; closed predecessor/failed candidates are archived without
deletion. Final full integration race passes all packages, root3220.593s;
all658 package-discovered ordinary/production source/manifest/embed inputs
rehash unchanged. Earlier interrupted runners remain unaccepted evidence.
Selective disposable source calibration stopped within its two-attempt budget
at a private instrumentation compiler error before Add dispatch. Stopped
snapshots were archived before full fixture restoration; the exact stalled
owner required forced closure after normal-cleanup COM failures. Canonical
data/registry remained unchanged. This failed oracle establishes no save/default
parity. Native implementation results do not resolve source
`DoCmd.Save` is an observed callback, not independent proof of physical save timing.
The picker uses the system-maintained initial location; source default-directory
behavior remains uncalibrated. No default promotion, source Add acceptance or
whole-picture parity is claimed.

#### Private picture deletion/restoration boundary

Canonical `frmPlotPictures` binds `tblVPics` and declares only Form_Current;
`frmVPics`/`frmVPicsXL` declare BeforeInsert, Current, private Add and image
double-click/test procedures, not a custom deletion callback. None exports
AllowDeletions or deletion confirmation events. Export absence is not live
permission or confirmation proof. The separate `clsPicture.DeletePicture`
operates on `tblPictureBlob`/`USysPictureBlob`, not this five-cell path library;
its behavior must not be imported into this workflow.

The private implementation remains unavailable: one existing physical
row, complete typed schema/cells, literal parent/context/project and an
unambiguous signed32 identity. Deletion/history/permanent ID reservation share
one transaction; image files are never deleted or newly authorized.
Read-only receipt recovery distinguishes no commit from unknown delivery.
Historical restoration refuses occupied/reused rowids, numeric ID aliases,
changed complete physical peer snapshots and owner drift without pruning
history/reservations. Explicit rowid/five-cell insertion preserves historical
NULL, empty, BLOB and signed identities; typed restoration acknowledgements
consume a deletion once while repeated cycles retain the first ID reservation.
Independent review corrected a first-reservation provenance gap: rebinding
the ledger to a later deletion cycle now refuses recovery, restoration and
creation allocation without writes. Combined deletion/restoration/creation
race coverage initially passed 48 top-level tests with no significant
independent follow-up findings. A subsequent historical-context correction
separates current operation context from immutable deletion context, recording
the current restoration actor/policy independently. Two actual context switches
prove stale-write refusal and read-only historical recovery. Current49-test
race coverage passes root51.147s; vet/production compilation pass; focused
independent review of this correction reports no significant issues. This
accepts the private kernel only; lifecycle facade integration is a separate batch.

Two bounded fresh-source attempts preserved form code/bindings and normally
closed, archived observed copies and restored originals, with594 protected
family/image/export identities and VPRO registry unchanged. Exported subform
SourceObject includes `Form.`; the actual runtime property omits that prefix.
The final child reports AllowDeletions/AllowEdits/Confirm Record Changes true.
One owned automation delete command removed exactly the planned physical row
without showing confirmation; no Cancel/Yes was invoked and the planned Cancel
checkpoint stopped without replay. This establishes a bounded automation
observation, not source GUI confirmation or postdelete Current/preview behavior.
The two-attempt source budget is exhausted; no routine retry is authorized.
No deletion/restoration availability or parity is
claimed from private kernel tests. Facade/UI integration and actual Wails
acceptance remain separate from that source observation. The separately gated
facade has passed independent review and is registered with nullable interface
bindings. Its desktop session/workbench is an unaccepted integration candidate:
client-runtime tests exercise retained reviews/unknown requests, readonly
recovery, irreversible acknowledgement, complete-row refresh and root ownership
barriers. Fresh independent desktop review reports no significant issues.
Actual disposable Wails verifies default refusal, one Delete, held-response
cancellation/read-only recovery, irreversible receipts across acknowledgement
and manager remount, occupied-ID refusal/exact recovery, one complete typed
Restore, permanent first-deletion reservation/consumed history, restored preview,
owner/Home/native-close barriers and1400/600px visibility/viewport containment.
Both owners normally close; committed library is archived before fixture
restoration and146 protected identities remain unchanged. Final frozen full
integration passes all packages, root3347.641s, with669 unchanged ordinary/
production source/manifest/embed inputs and unchanged package/embed discovery.
This accepts the bounded default-off desktop implementation; none of these checks closes the sealed source
GUI confirmation/Current gap.
No source audit
pruning, image-file mutation, BLOB library or broken Add oracle is included.

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
[322-file seal](../archives/picture-manager-desktop-checkpoint/evidence-manifest.json)
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

The accepted [1012-file seal](../archives/two-page-extra-desktop-checkpoint/evidence-manifest.json)
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

The independently verified [1052-file common-field seal](../archives/two-page-common-desktop-checkpoint/evidence-manifest.json)
has manifest SHA256
`3e883ea12b32c875118e97a1ab8aa50b5365177f6f200978743dafbf07ea6556`.
Never reseal it or replay completed native mutations. Remaining work is complete source
presentation of the existing96/98 XL fields,
common-field/event/child integration and the picture workflow. Existing accepted
standalone SIVI evidence remains immutable. Neither the additional-field panel
nor its native proof establishes complete two-page forms/reports parity.

## Current live scoped source presentation

### Single-plot Google Earth backend draft

Both source variants have `btnGoogleEarth`, tip `Plot on Google Earth` and
resolved `btnGoogleEarth_Click`: SaveRecord, NULL-coordinate message, then
SinglePlotPlotInGE for the current PlotNumber. GetSinglePlot reads current-project
Env only, prefixes minus to stored longitude, uses GoogleEarthDescField and
appends a description period. BuildKmlFile uses GoogleEarthPlaceName/KML2.1;
WriteKML overwrites configured VProPlotLocations.kml before the external shell.

The independently gated readonly candidate explicitly previews saved data instead.
It requires one literal paired Env/Admin parent in the selected context, retains
the complete original Env row and produces one placemark without SU expansion.
Accepted offline XML/HTML escaping and numeric formatting are reused; the remote
icon is not fetched. Negative original longitude, missing/NULL/out-of-range
coordinates and malformed/unsupported text fail explicitly, without data repair
or an empty success document. NULL/empty description remains distinct in the
original DTO while both retain the source's final-period semantics.
Backend race10.444s and production compilation pass; independent exact review/
focused follow-up race10.098s finds no significant issues and closes membership/
None/retired-context and numeric-boundary/integer-storage coverage gaps.
The registered mapped desktop successor adds owned saved options, temporary
preview-only corrections, raw immutable DTOs and parent/peer/lifecycle barriers.
Independent desktop review found HTML textarea normalization incorrectly rejected
valid multiline descriptions. Direct lossless escaped-original comparison replaces
HTML reparsing; five actual-Go cases retain LF, CRLF, CR and entities/Unicode.
Corrected wire/options pass race11.197s/production compile;118 focused/1628
registered frontend executions, check0/0 and default build pass. -ts -i generates
74 services/353 methods/379 models without warnings. Exact20-file/14-base desktop
v2 independent closure resolves the finding, including nine rejected altered
associations. Twelve normally closed disposable Wails owners preserve16 fixture
files each. Actual mapped normal/CHARS action, independent physical Env rows,
LF/CRLF/CR/NULL/empty/entities, parent/humus/vegetation draft holds, held actual
response and validation Cancel/remount/late rejection, close/Return holds,
labelled controls/data at1400/600/320 and both backend/client off-gates pass.
The2418-artifact native seal is verified. Complete1613-input race passes exit0
without input/discovery drift. Final association revalidates exact20 candidates/
14 bases, canonical/source/predecessor lineage and all twelve absent original
owners; the freeze is released.
Preference writes, file publication, implicit parent Save and external launch
remain unavailable. Acceptance is the readonly saved-plot adaptation, not the
source's implicit-save/overwrite/external-launch callback.

### View Veg Table source boundary

Both exact active `btnViewVegTable` controls caption View Veg Table and bind
`btnViewVegTable_Click`. Normal projects17 columns: PlotNumber, Species, A1,
A2, A3, Total A, B1, B2, CHARS shrubs a/b/c, Total B, C, D, Dr/Dw, Ep and
Cover10. CHARS projects the first14 only. Both sort PlotNumber ascending and
Cover1/2/3, TotalA, Cover4/5, TotalB, Cover6/7 descending; normal also sorts
Cover8/9/10. Cover5a/b/c are displayed but not ordering keys.

CurrProject and CurrPlotlist read registry context. None queries the entire
current-project Veg table; otherwise an ordinary PlotNumber INNER JOIN expands
duplicate SU membership. Neither branch filters to the current parent, excludes
all-NULL covers or depends on an application ID. Source deletes/recreates/saves
TempVegView and opens an editable datasheet, then adjusts widths. Those mutations
are not implicit authority for a readonly successor.

Four exact extracted queries execute on a controlled disposable ACE1033 fixture:
15 all-project rows and28 selected rows in each variant. All-NULL-cover rows
remain present; A/a join case-insensitively and tied CHARS rows have unspecified
relative order. This is query calibration, not original callback execution or
proof of other project locales. Existing literal SQLite joins are explicit read-
model adaptations, not an equivalent source-collation implementation. Both DAO hosts are closed
and unlocked; canonical/catalogue hashes and active1613 integration inputs are
unchanged. The failed field-factory attempt remains retained.

The independently default-off backend draft now declares controlled English-
ignore-case membership/order, exact numeric/NULL cover ordering and deterministic
physical-row ties. The cached comparator matches966 controlled native relations;
`GeneralLocaleVerified` remains false. Literal opening-parent ownership stays
byte-exact and separate from the broader collection. Same-snapshot preflight
limits each Veg/SU physical input to100000 rows, refusing max+1 without truncation.
Duplicate expansion is counted/paged virtually; each1..200-row page retains the
complete typed original rows needed for its unique physical witnesses. NULL and
duplicate application IDs do not invent identity. Exact thresholds, a fifty-
million virtual join,17/14 variants, embedded/external/None/retired contexts,
historical storage, Unicode refusals and actual cancellation/retry pass combined
Google Earth regression race22.353s and production compile. Exact3-file independent
backend review found no significant issues and accepts the saved-data draft.
Candidate/base/source/validation hashes match; tests were not independently
rerun. Its registered default-off desktop successor uses the exact mapped
normal/CHARS control, immutable paginated read panel and strict production Go
wires (normal/CHARS/page/end/None and real50/20 pagination). Current-context/
project/SU paths, complete page originals and joined physical witnesses are
validated without inventing application identity. Parent/peer/child drafts and
errors refuse opening; an open source review suspends edits, peer actions,
variant switching and close, with pending reads blocking Return until Cancel.
Actual Svelte Cancel/destroy/remount rejects late successful responses.
Focused race58.226s,153 focused/1671 registered frontend executions, check0/0,
default build and production compile pass. Exact15-file/9-base independent
review finds no significant issues; retained checks were hash-verified, not
independently rerun. Nine normally closed disposable Wails owners retain all16
fixture files each, qualifying normal/CHARS selected/None, full independent
physical originals, actual first/next/previous pages, real parent/humus/vegetation
draft barriers, held actual-response Cancel/variant/remount rejection, native
close/Return holds and labelled columns/controls/disclosed fields at1400/600/320.
The1760-artifact seal passes. A pre-owner NULL PlotNumber seed hit the family's
NOT NULL constraint; retry retains original schema rather than weakening it.
Nullable legacy PlotNumber remains backend-only evidence.
Complete1621-input integration passes `go test -race -timeout 120m ./...`,
exit0 without input/discovery drift. Final association rechecks1760 native
artifacts, reviewed candidate/base/backend/source lineage and nine absent
original owners; the freeze is released. Runtime
`VPRO_TWO_PAGE_VEGETATION_TABLE_REVIEW` and build
`VITE_TWO_PAGE_VEGETATION_TABLE_REVIEW` remain independently default-off.
Query DDL, editable datasheets, implicit Save/export, original source callback
execution and general locale parity are not accepted or enabled.

### Mapped vegetation profiling opener source boundary

Both `btnVegProfile_Click` bodies are exact equivalents: open USysPlotProfiling
with an empty stLinkCriteria, without explicit parent Save or invoking-plot
criteria. The target's Load is **not read-only**. It deletes result scratch
USysPlotProfilePlots, counts USysEnv through MoveLast, displays filtered count0,
reads registry CurrLump/CurrVegProfile and binds the selected `<name>_Profile`
ordered by Order. Empty USysEnv can interrupt this initialization; loaded-target
reuse is separately unknown. Stored MyOrder ordering becomes Order on Load.

The private target read model preserves51 nodes and nine exact bound instances/
physical columns from Sample_Profile/USysProfileTable. Eighteen exported event
routes resolve; declared Criteria_AfterUpdate does not have an exported body.
The separate Frame53_AfterUpdate body is empty and unbound, not an active route.
Reset defaults to2 (no plots), subvariety treatment to0, while Load replaces the
control's declared Sample profile with registry selection. Exact option sources,
labels and callback bodies remain sealed rather than recreated from the editor.

Run Recalc/Refresh calls ProfilePlots: clear both result scratch tables, NULL
physical profile PlotCount, construct lump/variant queries, evaluate ordered
Env/Veg/Lump steps, update counts/labels and loaded entry forms. Species-only
ignores Env steps and intersects grouped initial USysEnv plots with Veg/Lump
presence. Reset button instead starts all/no plots and writes PlotCount0.
Changing reset choice asks confirmation, but No does not set Cancel; no control
value rollback is invented.

UpdateForms visits seven IsLoaded form names. WriteVegProfile2Filter disables
FilterOn first; up to200 results become literal PlotNumber OR criteria, zero
matches leaves filtering off, and larger results/error2176 offer SU creation.
SaveFilterAsSU creates a table and copies result plots, joining current SU to
retain SiteUnit when applicable. Profile Attach/New/Unattach, lump Edit/Attach/
New, reference selection and local PDF help are distinct workflows, not an
implicit current-plot filter.

This source contract accepts dispatch/bindings/entry control flow only, not
original native execution, all module predicates or unqualified external
workflows. The existing owned native profile engine/editor is reuse precedent.
A mapped successor must explicitly adapt opening to avoid scratch deletion and
reconcile source-parent ownership, drafts, late results, close and filter
application; unresolved edits/runs/creation must remain unavailable.

Read-only prior-art reconciliation seals eleven current inputs and seven
non-equivalent boundaries without changing the metadata integration freeze.
The standalone selector ignores CurrVegProfile when ProfilePath is absent;
its nil-source fallback to the current project must not be presented as the
source's selected profile. A mapped successor needs explicit owned name/path
authority. Existing preview commits isolated TEMP work without source scratch/
PlotCount writes, while the source Run resets/persists counts and filters loaded
forms. Reset/species-only are not ordinary-preview aliases; zero/over200
navigation retention is an explicit adaptation, not source FilterOn behavior.
The standalone review also auto-loads on mount; it is not the proposed explicit
readonly Load/paging/lifecycle transport. Reuse physical readers/Description
metadata/operation leases, not unqualified mutators. All nine source fields
remain in the proposed bounded parent/profile/description snapshot; historical
duplicate Order must remain visible even though execution refuses ambiguity.
Full predicate reconciliation, original lifecycle and external selector/help
workflows remain unaccepted. The initial static receipt's TEMP-rollback wording
is retained and corrected by its hashed successor.

### Mapped project metadata opener source boundary

Normal `btnLoadMetadata` is captioned `Edit Project Metadata`; CHARS is
`Edit Metadata`. Both bind the resolved `btnLoadMetadata_Click`. Their exact
bodies differ only by one completely empty line. They open `frmProjectMetaData`,
focus ProjectID and search its recordset using the invoking parent's
`Me.ProjectID.Value`. Both StartDate branches use ProjectID only; the date
criteria are comments. There is no explicit Save or filter on the existing-row
path. A preloaded target's state is not established by this static trace.

The target binds `UsysMetadata`. Its exported query uses Sample_Metadata;
SetCurrentProject rebuilds it against `<current project>_Metadata`, not a generic
metadata catalogue or the global template table. All74 bound instances uniquely
resolve, retaining original binding spelling, to75 columns shared by the
Sample_Metadata/USysMetadataTable definitions. The unbound physical ID has
LONG `GenUniqueID()` default and a unique index; ProjectID is nonunique.
These project rows are separate from SQLite `_table_metadata` Description
storage. The global ProjectMetaData/ProjectMetaDataCodes are external MetaDataDB
links; USysProjectMetadata is not evidence of their current physical schema.

An unmatched target, or error3021, enters the Add prompt. Yes searches global
ProjectMetadata using MyRS2 but then tests **Not MyRS.NoMatch**, not MyRS2.NoMatch.
The two branches perform a33-field global SELECT insert or a ProjectID-only
insert, then Refresh and a ProjectID filter. The empty-target error path and
ordinary unmatched path must not be assumed equivalent. No repaired copy rule,
deduplication, apostrophe escaping, NULL criterion, collation or native outcome
is invented from this callback.

All eleven active target event routes are sealed. BeforeUpdate stamps Now and
the reference TableDef.Description versions only when Dirty; AfterUpdate stamps
Now only if still Dirty. Current and ProjectID_BeforeUpdate are comment-only:
there is no active plot-ID cascade. ProjectID_GotFocus captures CurrentProjectID;
title/region events Recalc, with title error3058 redirecting to ProjectID.
Collection-standard Change asks for DEIF*/DTE*/LMH25 and on Yes applies its exact
source defaults, versions and date. Close uses DoCmd.Close. The two attachment
buttons invoke distinct confirmation/file-selection/link-replacement workflows,
not metadata-row updates.

This accepts static dispatch, physical associations and active control flow
only. Existing reviewed standalone metadata services are reuse precedents, not
mapped opener or original target lifecycle proof. Add/copy/attachments remain
unavailable until separately qualified; no source bug repair, application
implementation, native action or canonical mutation was performed.

The subsequent controlled General1033 DAO primitive oracle covers nine lookup
and three branch cases. Case/terminal ASCII-space aliases match and007/7 remain
distinct. Unescaped apostrophe errors3077. DAO expression concatenation with
Null produces ProjectID='' and selects the empty row, but original Me.Value/
form execution was not proved. Ordinary unmatched MyRS.NoMatchTrue chooses the
ProjectID-only branch even with a master match. Empty MoveFirst3021 leaves
MyRS.NoMatchFalse and chooses SELECT even with a master miss. No Add prompt,
insert/copy or allocator was executed. Seven artifacts are sealed with normal
fixture close/unlock, absent original DAO host and unchanged canonical hashes.

An independently default-off readonly successor backend now
passes combined metadata/environment/vegetation race95.434s, fresh production
compile and exact two-file independent review with no significant issues
(tests not independently rerun). It reuses owned pinned snapshots and literal
unique selected Env/Admin parents, returns complete parent witnesses plus
current-project physical metadata pages, and bounds100000 source rows/page
1..200/offset0..100000. Controlled ASCII alphanumeric/space comparison ignores
case/terminal U0020 only in derived keys; original values remain unchanged.
NULL parent and unsupported source membership are unavailable, not silently
converted/omitted. Complete historical IDs/storage/NULL/empty/blobs/extra
columns remain available without signed32-ID repair. Physical-row ordering is
an adaptation, not source first-match cursor proof. Existing-record review
does not touch/depend on master templates; Add/copy/edit/DDL/audits/config and
original native lifecycle remain excluded.

The registered mapped desktop has an independent default-off client gate,
exact normal/CHARS source captions and callback identities, immutable50-row
pages and all74 source-bound labelled fields using existing metadata groups.
Complete physical originals preserve historical IDs and extra columns; missing
historical fields are explicitly unavailable. Parent/variant/peer/child/draft/
reference/retained-error barriers and pending-read Return/native-close holds
are wired. Strict actual-Go fixtures cover empty identities, beyond-total empty
pages, None and50/20 pagination. It passes202 focused/1763 registered frontend
executions, check0/0/default build, focused race91.337s, wire roundtrip and
production compile; -ts -i generation is warning-free at77 services/356 methods/
385 models. Exact15-file/nine-base independent review finds no significant
issues and verifies physical fixtures/associations, without rerunning primary
suites. Eight normally closed Wails owners qualify normal/CHARS selected/None,
both off-gates, complete74-field/physical disclosures, paging/draft/lifecycle/
native-close and all planned selectors at1400/600/320. The2056-artifact seal
rehashes with ten absent originals and sixteen unchanged fixture files each.
Two independent-display expectation failures remain retained; no application
change was needed. Complete1638-input race integration passes exit0 in85m08s
with unchanged inputs and package/embed discovery. Final association reverified
2056 artifacts, exact15 candidates/nine bases and source/backend/review/DAO/
predecessor lineage; the freeze is released. Gates remain default-off;
registered readonly adaptation is not original editable callback parity.

### Owned existing-row metadata editing

The normal `Edit Project Metadata` and CHARS `Edit Metadata` actions now reuse
the existing context-bound metadata editor inside retained complete-entry
ownership. The Forms menu delegates to that same admission/opener, with a fresh
explicit in-place capability after blur/tick; ordinary navigation and global
operation/publication/native-close barriers remain unchanged.
Literal `VITE_TWO_PAGE_PARENT_REVIEW`, `VITE_TWO_PAGE_PARENT_ENTRY_REVIEW` and
`VITE_PROJECT_METADATA_EDITING` are required. Parent direct editing is separate;
optional restoration keeps `VITE_PROJECT_METADATA_RESTORE`. There is no new
backend metadata flag. A nonempty saved TEXT ProjectID, literal physical-source
authorization and explicit physical row selection are required. The alias-aware
saved-data reader remains separate; it does not authorize this writer.

Existing parent variants, extra/common cohorts and linked owners retain their
sessions/history. Postcommit refresh reads only; failed peers with cleared
originals remain eligible through their retained errors. Persistent failure
continues blocking recovery, while correction/retry cannot repeat Save.
Independent review found that eligibility omission and accepted the reproduced
production reload regression/fix for both cohorts. All2593 lifecycle/70 entry/
176 affected frontend cases, check, focused races/vet and five isolated actual
Wails builds pass.

Six closed disposable native profiles prove both source captions/menu paths,
separate reader and independent grants/backend denial. One parent Save/prune
and one selected metadata Save/four exact audits/prune preserve histories across
repeated outer-cohort failure and read-only recovery. Soil/vegetation and fresh
specific CancelClose acknowledgements hold competing actions;147 labelled
controls pass at actual1400/760/420 widths. Business/audits/old histories and
fifteen other files are exact; two consumed technical histories remain.
Rejected startup/navigation expectations are preserved, with the same retained
metadata owner continued without mutation replay. This fixture has no tblVPics
SQLite library: its picture backend remains disabled, not newly seeded or
native-qualified. Fresh full race passes root4137.598s, preserving1872 inputs/
147 artifacts, discovery and canonical/protected/closed data.
No Add/copy/template, ProjectID assignment, attachments, original focus/Current/
NotInList timing, new backend/binding/schema/flags or default/demo/whole-form
promotion is granted.

The subsequent [mapped profiling backend draft](../twopageprofilingservice.go)
is independently default-off. It explicitly refuses legacy
implicit project fallback and None profile selection. Literal selected paired
opening parents establish owner intent only; they do not filter the profile
rules. One pinned snapshot retains complete parent witnesses, all nine profile
fields/extra columns and Description originals. Physical row order is an
explicit adaptation, not source Order execution; duplicate/NULL/historical Order
is retained, not compiled. Profile and Description sources each have an exact
100000-row bound; pages require1..200 with legitimate empty beyond-end pages.
Description absence, NULL, empty and duplicate rows are distinguishable with
literal TEXT table identities. Source scratch deletion, count writes, registry,
editing/run/reset/species-only/filter/SU/attachment/help effects are excluded.
Combined predecessor race93.489s and production compilation pass, including
actual concurrent WAL witness consistency and in-flight cancellation/retry.
The sealed exact two-file/source/predecessor independent review finds no
significant issues: hashes/helpers verified, primary checks not independently
rerun. The registered desktop candidate retains the exact normal/CHARS opener,
all nine source captions and header order, explicit50-rule pages and immutable
complete original DTOs. Chunked validation yields for cancellation/remount and
rechecks generation afterward. Lazy original disclosures show50 Description
rows locally from the same snapshot; no source definitions are discarded or
reread. Parent/variant/peer/linked-child/draft/reference/error/staging/
acknowledgement/Return/native-close barriers remain held throughout.
It passes213 focused/1815 registered frontend executions, check0/0/default build,
production compile and wire/predecessor race124.657s; warning-free -ts -i
bindings are78/357/386. The public nine-field read model is checked against
sealed source captions/bindings/header order. Exact16-file/nine-base independent
desktop review finds no significant issues; reviewer independently reruns213
focused tests and check0/0. Isolated candidates retain reviewed inputs and default
assets. Eight native owners qualify the readonly normal/CHARS selected/None
and independent off-gate boundaries, complete physical witnesses, paging/drafts,
held actual response and candidate-associated validation-yield retirement,
native-close/Return and57 planned three-width combinations per enabled owner.
The2095-artifact seal retains ten normally closed absent owners with sixteen
unchanged fixture files each. Full race integration passes exit0 without runner
input/discovery drift; final association reverified2095 artifacts and1647
protected inputs, releasing the freeze. This does not qualify original editing,
Run/reset, selectors, filters or scratch writes; both gates remain default-off.

### Species-only profiling SQL boundary

The active btnSppOnlyProfile callback recalculates then calls SpeciesCodeFilter
with the SubVarSpp option. That procedure clears scratch/counts, prepares Lump
state and selects Table/Species where Table is not Env. Zero selected rows exits
with an empty result; otherwise it seeds grouped project plots. Each Veg or
available Lump branch intersects the existing result with matching USysTVeg
presence. Unknown tables have no matching branch but still cause seeding.
Env/case/trailing-space and NULL Table exclusions are source SQL observations,
not inferred NULL-to-empty conversions.

The exact exported USysTVeg UNION exposes Cover1..7 and stored TotalA/TotalB
only when each column is non-NULL. Zero/negative values establish presence;
Cover5a/5b/5c/8/9/10 alone do not. Duplicate parents, vegetation records and Lump
mappings do not multiply grouped output. Lump requires Use=True. Three exact
query exports,25 physical vegetation rows,20 initial match observations and
six independent physical/schema reopens qualify this SQL boundary. A separate
readonly comparison fixture supplies14 ASCII text cases/28 directional
Integer(-1/0) observations: terminal spaces compare equal in both directions,
while leading spaces, tabs/CR/LF/control characters and literal */? do not.
The portable corpus retains32 unique match cases and23 scope plots.

The private compiler/executor does not interpret Order, Criteria, Layer,
Operator or Operation and never writes source scratch/counts. It uses a
caller-owned snapshot and request context, comparison-only terminal space
handling, complete physical-row identities and explicit unavailable-domain
errors. Literal identity differences/case-ambiguous scope, unqualified active
Table spellings, non-ASCII/raw-SQL Species and missing explicit project-local
Lump ownership refuse rather than repair or inherit stale global state.
Refusing a missing Lump relation is a conservative desktop boundary, not the
source None_Lump branch, which skips that intersection. Initial source text
comparison failures reproduce before correction;32 native match cases,
complete readonly typed preservation, irrelevant-field isolation, Boolean/
identity refusals and explicit correction/cancellation retry pass coupled
race69.765s, vet and production compilation. Exact independent review accepts
the private kernel with no significant issues and verifies all candidate/live/
canonical/evidence hashes. Its kernel pre-cancellation limitation is addressed
by a separate unexported owned-context successor: actual matching-query
in-flight cancellation/owned retry, exact original rules/schema/count/Lump
witnesses and retired-context/complete file preservation pass coupled
race74.139s, vet and production build. Distinct exact context-level review
accepts with no significant issues and verifies all27 artifact hashes/sizes and
unchanged kernel basis. Full integration passes exit0 over1680 protected inputs/
all51 resources/38 sealed artifacts without input/source/package/embed drift.
Original source owners are freshly absent; final association releases the
freeze. This integrates five private prerequisites, not application-default,
registered desktop or original callback availability.
This is an unregistered kernel: no original VBA call, source callback timing,
subvariety preparation, service/binding/client or whole workflow acceptance
follows. Closed suggestion/public-dispatch work units are not replayed.

### Profiling edit and suggestion event boundary

A separate source-only contract resolves the exact Table AfterUpdate and four
Field/Layer/Operator/Species GotFocus routes. Table changes to Veg/Lump assign
Field=Species/LumpCode; Env/other values preserve Field. Field suggestions use
literal Sample_Env for Env, not the selected project's Env schema; an unmatched
Table preserves the previous Field suggestion state. Layer and Operator empty
their suggestion lists for unsupported tables without clearing stored values.
Only Layer and Operator have explicit LimitToList declarations in this export;
the base ComboBox declaration adds no such restriction.

Species focus for Env empties suggestions without clearing Species; Veg reads
grouped USysAllSpecs.Code with no explicit ordering. Lump uses the exact
registry-named grouped/ordered query, preserving the spaces between SQL clauses.
The unmatched-Table branch instead assigns bound Species NULL and RowSource NULL.
These focus-time mutations and retained suggestion states are not equivalent to
stateless selectors. The standalone choices reader uses selected-project Env and
explicit BINARY species ordering, and its writer requires explicit coupled
Table/Field values; those remain precedents, not mapped event acceptance.
Criteria_AfterUpdate is still declared but unresolved. Five active routes,
six raw combo declarations and prior-art/source hashes are sealed with all1646
integration inputs unchanged. No native callback execution or mapped editing
is enabled by this static contract.

Two disposable copied-body host/dispatch attempts did not qualify these routes.
CreateForm supplies a module header, while AddFromString automatically wires
named event procedures. The corrected fixture replaces that header and disables
the import-created bindings before opening its actual bound six-combo Form view.
All five private bodies are exact, but the first explicit fixture dispatch fails
with DISP_E_BADPARAMCOUNT. Zero explicit calls completed; zero partial execution
is not established. Both owners normally quit, both locks are absent and the
canonical/static inputs plus1673 frozen application/resource inputs are unchanged.
The work unit is time-boxed after two attempts; no third retry, original event
timing, whole-form execution or mapped editing is accepted.

A separate pure-sentinel dispatch prerequisite also stops after two attempts.
Ordinary Run reproduces BADPARAMCOUNT without any original source procedure.
Explicit optional-slot dispatch instead returns a mismatched result whose value
was not retained; later routes/Eval were not qualified. No validated sentinel call
completed and partial fixture execution is unknown. Installed static type-library
metadata declares a VARIANT return and thirty optional input/output
pointer-to-VARIANT arguments, not an accepted runtime wire. Twenty-nine sealed
artifacts retain both normally closed/absent owners, absent locks and unchanged
canonical/base/current1673 inputs. No third source-focus or dispatch retry is
authorized by this prerequisite.

### Profiling Env criterion conversion boundary

The active private ChangeType first attempts CDbl and, on error, surrounds the
input with single quotes without escaping embedded apostrophes. ProfileEnv calls
it before its NULL checks and concatenates the resulting Variant into SQL; that
is not automatically equivalent to binding the initially converted DOUBLE.

A disposable General1033 DAO fixture calibrates twenty conversion/concatenation
cases and independently reads eight complete binary64 witnesses. The input
`1.0000000000000002` converts to the DOUBLE immediately above1 but concatenates
as `1`. The controlled concatenated `>1` query selects the adjacent row and16;
binding the original converted DOUBLE selects only16. Spaces, leading zero,
exponent, comma, hexadecimal/octal, NULL, empty and rejected conversion cases
remain explicit in the sealed receipt. This qualifies controlled DAO expressions,
not the original VBA ChangeType/ProfileEnv callback, general locale behavior or
new mapped execution. The owned DAO host normally exited, its fixture lock is
absent, and canonical source/all1646 runner inputs remain unchanged. The existing
strict standalone compiler and its refusals are not silently broadened.

An additional owned Access fixture now executes the unchanged private ChangeType
body through two explicit fixture-only public callers. Reexport comparisons
before and after twenty actual VBA cases preserve the complete original body;
all value types, converted bits, fallback quote strings and concatenated literals
match the controlled DAO expectations. Numeric predicates reproduce the adjacent
DOUBLE threshold difference, and the copied eight physical witnesses remain
unchanged. The19-artifact seal verifies both original Access owners normally
closed with no fixture locks or canonical/frozen drift. The first attempt stopped
before execution because its comparator omitted `As Variant`; the corrected
comparator also proves that first reexport was unchanged. This qualifies the
private conversion helper on this machine, not the original ProfileEnv/form
callback, general locale behavior, scratch/count writes or mapped Run.

Additional controlled SQL evidence distinguishes source LONG32, DECIMAL96 with
scale at most28, and DOUBLE literals. `0.1` is a DECIMAL literal: all three
adjacent DOUBLE witnesses compare equal, unlike a bound DOUBLE. Integral1 is
LONG and keeps exact DOUBLE comparison. Larger integral literals can be DECIMAL;
out-of-range or unrepresentable-scale literals use DOUBLE. Fifty-five observed
literal cases and480 independent comparisons are sealed across five accepted
readonly fixtures plus one normally closed unsupported-CDec-query attempt.
The physical eight-row witnesses remain unchanged and all six hosts are absent.

DECIMAL row conversion is distinct from the existing fixed-mask formatter:
binary scaling and half-even rounding are capped at scale28. The adjacent
DOUBLE above2.5e-28 rounds to3e-28 while its predecessor and2.5e-28 round to2e-28.
Formatting these values to fifteen decimal digits first loses that distinction.
The [numeric candidate](../sourcefloatdecimal.go) therefore shares only the
demonstrated fifteen-significant-digit criterion/Format boundary, and uses a
separate DECIMAL comparison for accepted Env numeric rules. LONG/DOUBLE retain
bound numeric comparisons; Veg/Lump retain raw criteria. Numeric Like, date,
locale and malformed-storage refusals remain, and source literal overflow is
an explicit refusal. Focused race78.536s and production compilation pass,
including all480 source expectations through compiled production SQLite
queries and a full service readonly/cancellation test. Independent exact
five-path/three-base review accepts this bounded change with no significant
issues, verifying source hashes and encoded expectations without rerunning tests.
One normally closed disposable Wails owner passes69 independently expected
predicates across138 rules through the actual standalone preview wire. Complete
typed review, stored counts, physical data/history and all16 files are unchanged.
The2367-artifact seal pins source, review, native and readonly predecessor
lineage. Full1671-input race integration, including all51 resources, passes
exit0 without input/package/embed discovery drift. Final association reverified
2367 artifacts and the exact review/absent native owner, releasing the freeze.
This is not original
ProfileEnv, scratch/count, form callback, editable mapped lifecycle or general
locale acceptance.

A subsequent unchanged-private-ProfileEnv probe is retained as failed evidence,
not procedure acceptance. Its isolated fixture-only registry/Recalc host is not
the original form lifecycle. Two attempts stop before any original call: fixture
identifier recasing is corrected without changing either private body, then
unsaved-host Save naming fails. Both original Access owners normally quit with
warnings restored and no locks;17 sealed artifacts retain contracts, exports
and failures. All1671 frozen application inputs and canonical source remain
unchanged. The native debugging time-box is reached; qualify the host lifecycle
separately before any later procedure attempt. Scratch/count/form effects remain
unqualified rather than inferred from the accepted numeric wire.

The separate blank-host lifecycle is now qualified without another original
procedure attempt. `CreateForm` supplies a generated open name: save that name,
close it, rename the saved form, then open the intended name. Actual Design and
Form views, persisted unbound CurrVegProfile/default, physical database reopen,
Recalc and byte-identical before/after form exports pass. The owner normally
closed with no lock, eleven sealed artifacts and unchanged1671 application/five
canonical inputs. No source module was imported or ProfileEnv call executed.

### Cover literal and arithmetic successor

Original ProfileVeg takes Criteria directly from its TEXT255 rule cell and
concatenates it without ChangeType. Preserve that literal text; applying Env's
fifteen-digit formatting would invent another threshold. Raw SINGLE covers,
original MadMax results and ordinary Nz sums use seven-significant-digit DECIMAL
coercion. The DAO field descriptor may report DOUBLE even while comparisons
retain SINGLE runtime precision. Lump SUM aggregates instead use fifteen-digit
DOUBLE coercion; calling MadMax on those aggregates still returns SINGLE.

Veg Nz additions are left-associated SINGLE arithmetic. Source `.1+.2` becomes
`.30000001192092896`; `-.3+.1+.3` becomes `.09999999403953552`; `16777216+1+.1`
becomes16777216. Arithmetic overflow promotes to DOUBLE rather than failing,
and subsequent cancellation does not restore SINGLE precision. The independently
observed `MaxSingle+MaxSingle-MaxSingle-MaxSingle+.1` result matches the literal
`.100000001490116`, not `.1`. Unsupported individual SINGLE values and malformed
historical storage still produce explicit errors; no stored value is rewritten.

The [portable source corpus](../testdata/profiling-cover-native.json) supplies405
native predicate sets,144 adjacent SINGLE midpoint comparisons and20 independent
sum values. Original MadMax bodies are reexported exactly before/after; no private
ProfileVeg/ProfileEnv or original callback is invoked. All owned Access hosts
normally close; the failed unqualified aggregate alias is retained separately.
The six-path compiler/query/helper/test candidate reproduces failures before
the fix, then passes coupled profile/formatter race70.047s and production
compilation, including readonly full-service/cancellation/retry and typed
rule/schema/count/file preservation. Exact six-path/four-base independent review
accepts the bounded candidate without significant findings, verifying all60
source artifacts/encoded expectations and hashes without rerunning tests/native.
Actual disposable Wails wire passes431 predicates across1293 physical rules/59
parents. Complete typed review/counts/schema/data/history and16 files remain
unchanged, and the original owner normally closes. The2652-artifact seal retains
accepted Env/source/review/native lineage. Complete-source/resource full race
passes exit0, UTC2026-10-13T14:49:16.625227 to2026-10-13T16:06:43.669853,
with1673 protected inputs/all51 resources and no input/package/embed discovery
drift. Final association reverified2652 artifacts and exact reviewed/native/source
lineage, releasing the freeze. Source absence/set handling,
NULL criteria, Env conversion, grammar refusals, frontend/bindings/resources and
default gates remain unchanged. This is not full source Run or editable lifecycle
acceptance.

The broader raw-literal domain remains a separate successor. Six readonly DAO
fixtures qualify106 direct lexical observations and30 boundary-search queries:
negative exponent1073741819 is accepted,1073741820 refused, and leading zeros
retain that boundary. Finite zero underflow and a syntax refusal are distinct.
Positive zero exponents can return anomalous nonzero DOUBLE values; those are not
desktop requirements. The47-artifact association rechecks all six absent owners,
unchanged/lock-free fixtures and canonical/current1673 inputs. No application
correction or general literal parity is claimed; precise refusal/domain changes
are now implemented in the following reviewed/native-accepted successor.

The three-path/one-preimage domain correction validates the measured exponent
bound before accepting zero. Supported underflow/leading zeros remain available;
existing Env conversion is unchanged. Positive-exponent zero above28 instead
produces an explicit unavailable-domain error. This conservative adaptation also
refuses source-valid0e29/0e100 and must not be described as native syntax failure
or general literal parity. The106 independently encoded native cases and
compiler/service readonly refusal/correction/retry checks pass coupled
race68.876s and production compilation. Exact independent review finds no
significant issues and verifies all47 source artifacts/106 direct/30 search
observations and live/candidate/preimage hashes; tests/native not rerun by reviewer.
Six disposable Wails owners qualify both Veg/Lump refusal branches and supported
underflow through actual public bindings, preserving complete typed review/counts/
data/history and16 files each. All normally close and are absent. The3686-artifact
seal retains source/review/native/accepted-cover lineage; complete1675-input/
all51-resource race passes exit0, UTC2026-10-13T16:32:16.515840 to
2026-10-13T18:00:13.236306, without input/package/embed discovery drift. Final
association reverified3686 artifacts and exact candidate/preimage/source/
six-absent-owner lineage, releasing the freeze. Defaults remain unchanged.

### Remaining SU/filter/close static dispatch boundary

Both source variants resolve identical `btnCreateSuFromFilter_Click`,
`btnEnvIntoSu_Click` and `btnG2MainMenu_Click` procedures. The sealed private
nine-input source/prior-art association is static evidence only.

`GetFilteredFormRecords` requires an allowed active entry form and consumes
its `RecordsetClone`, not just the invoking plot or arbitrary profile results.
USysFilter2SuList is unbound: new-list choice2 is the default, modify-current is1,
and include-existing defaults1. Continue publishes the two controls to globals;
Cancel sets3. Window-close behavior is unqualified. Modify physically deletes
current rows absent from the clone and adds missing cloned plots. New creation
uses CreateSuTable's name/destination/template export/attachment paths, including
possible remote database creation, then inserts cloned plots. Existing
assignments use a copied/imported FilterQuerySu and a subsequent join. That
scratch copy precedes the caller's Cancel branch; cleanup deletes scratch even
on exit. This is neither a simple export nor proof of side-effect-free Cancel.

`InsertEnvSuIntoSu` warns before transferring USysEnv.UserSiteUnit to the
current SU.SiteUnit, then separately offers missing personal definitions and
optional loaded personal-list editing. The guarded local physical
[transfer](../siteunittransfer.go) is prior art, not acceptance of those source
opener/offer effects. Source Close closes frmPictures, calls Hide (sets the
current form invisible, not closed), opens the main menu and restores display.
The current Return adaptation does not prove that hidden loaded-form lifecycle.
Original clone/filter membership, source locale, empty/large records, remote
creation/attachment/selection, cancellation cleanup and personal-list/hidden
lifecycle remain unqualified. No native source action or mapped mutation was
enabled by this trace.

### View Env Table dynamic datasheet source boundary

Both active normal/CHARS `btnViewEnvTable` controls caption View Env Table.
Their callback opens `FormSet(), acFormDS` and shows VProFV. There is no explicit
parent Save, current-plot query or raw Env-only export in this handler.
FormSet reads Current.DataFormName; the source registry getter's missing-setting
default is FS882-6x4. This is not necessarily the invoking form.

The four known FS882-6x4, FS882-6x4XL, FS882-8x6XL and CHARS targets bind USysEnv.
SetCurrentProject creates that query as DISTINCTROW current-project Env.*
INNER JOIN Admin.* on PlotNumber=Plot, without an SU/opening-parent filter or
explicit order. An exact extracted query on a disposable ACE1033 fixture
produces12 physical pairs:3x3 A/a membership plus B,007 and7. Four identical
complete pair values remain, rather than being collapsed by DISTINCTROW.
Orphan Env/Admin rows are excluded; leading-zero text remains distinct.
This is controlled SQL calibration, not original callback/form execution.

Target Open sets the project/SU caption; Current prepares coordinate display;
Load initializes coordinate/project/assigned-SU controls. Assigned-SU AfterUpdate
writes registry and modifies UserSiteUnit's choice RowSource, not the collection
RecordSource. Those lifecycle mutations are not readonly desktop authority.
The15-artifact seal verifies13 source/catalogue hashes, normally closed/unlocked
DAO host and all1621 active integration inputs unchanged. Effective datasheet
column presentation and unsupported dynamic form targets remain unqualified;
no original callback/native or general locale acceptance is claimed.
Supplemental static target qualification preserves a meaningful distinction:
the USysEnv query has no ORDER BY, but the forms export OrderByOn=NotDefault.
Default FS882-6x4 orders RootingDepth and SeepageDepth descending, then PlotNumber;
XL/normal/CHARS order PlotNumber. All export FilterOnLoad=0. Stored nutrient/
date filters must not silently become active criteria. Effective ordering/
filtering when switching an already-loaded form remains a runtime unknown.

The independently default-off, unregistered readonly backend draft reads
Current.DataFormName from owned YAML, retaining the source missing-setting
default only when absent. Explicit NULL, malformed type, empty and unsupported
values fail without silent repair or persistence. Four supported targets keep
their declared order; loaded-instance state and general ACE locale equivalence
are not inferred. The saved-data collection is whole-project Env/Admin physical
pairs; selected SU and opening parent are ownership context, not row filters.
Each physical source is bounded at100000 rows in the same owned snapshot, with
virtual duplicate paging at1..200 and complete tagged originals for page pairs.
Nullable/historical application IDs, true=-1, blobs and empty text stay intact.
Exact100000/max+1 source and200/max+1 page thresholds,9,999,800,002 virtual pairs,
all target orders, selected/None/external/retired contexts, raw text refusal,
readonly config/data and actual in-flight cancellation/coordinator retry pass
combined focused race76.036s and production compile. Exact two-file independent
review finds no significant issues and verifies38 hashes; tests were not
independently rerun. The initial incomplete disposable-schema switch failure is
retained; retry preserves original fixture columns. Registration, UI projection,
datasheet presentation and native lifecycle remain pending. No implicit Save,
editing, DDL, registry/toolbar mutation or default promotion is authorized.

The registered independently gated desktop successor uses immutable50-row pair
pages and strict actual-Go ownership/target/order/witness validation. All446
root source binding instances across four targets retain attached form captions
and page groups as an explicit presentation adaptation. Complete physical
originals remain separately disclosed; missing/ambiguous legacy bindings are
explicitly unavailable rather than invented. Generated interfaces are76
services/355 methods/384 models, warning-free with -ts -i. Four-target/None/end/
50+20 wires pass race2.764s; combined predecessor race122.934s,194 focused/
1712 registered frontend executions, check0/0/default build/production compile
and exact16-file/nine-base independent review pass. Tests were not independently
rerun because reviewer Node PATH was unavailable.
Ten normally closed disposable Wails owners verify four saved targets, mapped
normal/CHARS selected/None actions, independent full physical pair originals and
source-labelled disclosures, paging, actual parent/humus/vegetation draft holds,
held successful response Cancel/variant/remount rejection, native close/Return,
every planned source/control selector at1400/600/320 and both off-gates. Sixty
new unselected historical-date/nutrient rows remain included despite saved
filters. The2387-artifact seal verifies all twelve original owners, including
two failed runner profiles, absent and16 fixture files unchanged each. Failed
owner capture and ordinary None launcher pagination were corrected without
application changes or source mutation replay; their unchanged cleanup is
retained. Full1630-input integration first hit Windows ThreadSanitizer allocation
error1455 after3555.054s; its unchanged-input/discovery failed receipt is retained.
The exact candidate retry passed root5607.056s with bounded Go heap/concurrency
(CGO1/GOMEMLIMIT1GiB/GOGC50/GOMAXPROCS2), without reducing race instrumentation.
All1630 inputs/discovery are unchanged. Exact final association rehashed2387
artifacts, sixteen candidates/nine bases, backend/source/predecessor lineage and
twelve absent owners with sixteen unchanged fixture files each. Integration is
accepted and the freeze is released.
Runtime
`VPRO_TWO_PAGE_ENVIRONMENT_TABLE_REVIEW` and build
`VITE_TWO_PAGE_ENVIRONMENT_TABLE_REVIEW` remain default-off. This does not accept
original Access callback execution, loaded-form state, native datasheet headers/
visibility/order, computed/unbound behavior or general ACE locale equivalence.

### Readonly source Audit candidate

The nullable ordinary presentation successor adds distinct ListAuditReadEntries
DTOs/endpoints without changing strict legacy AuditEntry/ListAuditEntries
restoration/provenance. Shared exact schema/query/order/flag policy feeds both
scanners. The actual ordinary form reads NULL User/EditWhen without inventing
strings/timestamps; displayed NULL/empty values remain distinct, with unsupported
NULL actor/date restoration explicitly nonselectable. Malformed UTF-8 fails
before JSON repair. Focused race7.062s, production compile, real Go round-trip
fixture,73 focused/1597 registered frontend executions, check0/0 and default
build pass. Independent review found the actual merged-service adapter omitted
captured-context forwarding. The fix and delayed-read/stale-context coverage
are independently accepted; exact18-file/15-base successor review finds no
significant issues. Six accepted/two rejected normally closed disposable Wails
owners preserve all16 files each and prove initial-NULL ordinary reading/source
launch in normal/CHARS, explicit strict restoration refusal, parent/peer drafts,
held-response cancellation/remount, native-close/Return, three-width visibility
and independent source gates. The1710-artifact native seal is verified.
Complete1605-input race passes exit0, UTC19:52:59.449269 to21:28:05.794068,
without input/discovery drift. Exact final association rehashes1710 artifacts,
18 candidates/15 bases and canonical/predecessor lineage, confirms all eight
original owners absent and releases the freeze.

Both normal/CHARS exported `btnAudit` controls have the resolved
`btnAudit_Click` OnClick event, opening USysAuditXL filtered by literal
PlotNumber. Its UsysAuditTrail query reads the current project's Audit table.
The independently default-off candidate retains the eleven bound source fields
and every physical tagged column/row. Unlike the legacy ordinary audit DTO, it
requires actual text storage and byte-exact plot/project ownership; malformed
UTF-8, foreign ownership and missing bindings fail explicitly.

The panel pins context/project/plot/form, loads only on request and preserves
NULL, empty text, nullable IDs and BOOLEAN -1/0. SQLite EditWhen/rowid storage
ordering is an explicit adaptation rather than Access coercion parity.
Parent/current/loaded-peer drafts, reference errors and pending child ownership
block opening. The open panel suspends the parent/children; pending reads block
Return while Cancel remains live. Generation/destroy guards reject late results.
Restore/Flag editing, the source registry audit-strength option and restoration
remain unavailable; Load and Return never implicitly save or prune.

Backend focused race5.421s and production compile, shared real Go wire fixture,
54 focused/1595 registered frontend executions, check0/0 and default build pass.
Initial backend and exact22-file desktop reviews found no significant issues.
Six accepted/eight normally closed native owners preserve16 files each, proving
mapped normal/CHARS typed history, actual parent/humus/vegetation draft holds,
held response cancellation/remount, native-close/Return barriers, three-width
labelled cards and independent backend/client-off refusal. Native seal rehashes
1802 artifacts. Complete1602-input race passes exit0, UTC17:26:26.719054 to
19:04:18.773109, without input/discovery drift. Final association rehashes1802
artifacts, exact review/base/source lineage and all eight closed owners; freeze
released. The bounded readonly source Audit workflow is integrated, not promoted.
The first rejected fixture exposed legacy ordinary Audit NULL User string
scanning before source mounting; EditWhen has the same strict string boundary.
Qualified native fixtures keep User/EditWhen valid. Backend raw NULL reading is
not NULL-user/date desktop launch parity. The separately scoped nullable ordinary
successor above now proves initial-NULL native launch while preserving strict
restoration/provenance; its complete integration is independently accepted.

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
The [890-file source-layout seal](../archives/two-page-entry-layout-desktop-checkpoint/evidence-manifest.json)
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
The [860-file header-composition checkpoint](../archives/two-page-header-composition-checkpoint/evidence-manifest.json)
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
The [838-file private atomic-entry checkpoint](../archives/two-page-entry-atomic-checkpoint/evidence-manifest.json)
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
remains unchanged. The [1529-file desktop successor](../archives/two-page-entry-desktop-checkpoint/evidence-manifest.json)
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

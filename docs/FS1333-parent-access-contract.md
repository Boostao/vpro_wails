# FS1333 / SIVI parent source boundary

## Status

Static source contract plus separately gated native parent review and fourteen
directly bound parent editors/restoration, not an enabled full-form entrypoint.
The private writer checkpoint and public/mounted successor are distinguished
below; callback actions, ProjectID assignment and metadata creation stay disabled.
The separately gated SIVI height panel is a bounded adaptation mounted under
FS882; it does not implement this parent form. Exported coordinates are evidence,
not a requirement to reproduce fixed Access pixels.

Sources were read directly from the local `VPro64_forAI` exports. The R form is
not parity proof. Existing [packaged metadata](../resources/fs1333-sivi-layout.json)
preserves the normal parent, source labels, control relationships, events and
three PlotNumber-linked children. Parent fields are read-only unless the separate
editing gates explicitly load the fourteen approved controls. Mapped original
storage is not source-equivalent join/callback/write authorization.

| Export | SHA256 |
|---|---|
| `Forms\frmSIVIsite.txt` | `49ec8926aea92c21c57cb99ab571cd3b55ac2f64940060e23c6b7d9c9230a85a` |
| `Forms\frmSIVIsite-CHARS.txt` | `14e5bf440431009fe4eca7c7282ed63263cc05852d9ac43ef301581fa7801df7` |
| `Queries\UsysEnv.txt` | `95b5c319dd9ce294d90989aa81f0fb0c8b26c248c148dbad353720b7434cad2d` |
| `Modules\clsFormInfo.txt` | `5a55cb77053ce87490e2ecc2e294e9409b5ee1bf2b5d52d11f9dc00eefbd5074` |
| `Tables_Def\Sample_Env_CreateSQL.txt` | `eb1b6e14206256e5a9a02e86b9fed443f3369fff8048722c5f6656df859e01cd` |

## Storage and ownership

The original `USysEnv` query selects Env and Admin through an **inner join** of
Env.PlotNumber to Admin.Plot, using Access DISTINCTROW. This is not permission to
silently change orphan membership, collapse physical duplicates, create missing
Admin rows or assume an existing header DTO fully represents the source.

The normal parent has77 direct bindings; each resolves to exactly one canonical
Sample_Env or Sample_Admin column. Distinct SIVI storage includes polygon,
floodplain, stand height/estimation, A-horizon and root-zone/soil observations.
HumusThickness and StrataCoverTotal are physical fields too; do not invent
calculations from their captions. Option-driven SpeciesListComplete is an
additional implicit write target, not a direct option-group ControlSource.

Canonical SQLite table names, data and `_table_metadata` descriptions stay
unchanged. Future raw reads should preserve Env/Admin physical provenance and
nullable historical values before any typed parent editor is enabled.

## Active parent events

There are nine bound event routes, independent of the child BeforeUpdate paths:

| Owner | Source route | Meaning / desktop boundary |
|---|---|---|
| Form | OnCurrent -> Form_Current | Initializes species-list and plot-type option displays from stored values; no option audit should be manufactured on display |
| Form | OnLoad -> Form_Load | Reads the ProjectID-source preference |
| optPlotType | AfterUpdate | Writes PlotType; normal parent also restores the selected option and refreshes |
| optSpeciesListComplete | AfterUpdate | Writes true, false or NULL to SpeciesListComplete |
| optProjectID | AfterUpdate | Changes the registry-backed source preference, not the bound ProjectID itself |
| ProjectID | GotFocus | Rebuilds choices from master or current-project metadata |
| ProjectID | NotInList | Enters metadata creation/editing and record-save logic; not a simple free-text completion |
| btnEditMetadata | Click | Opens metadata review, locates a definition and prompts before creation |
| btnClose2 | Click | Closes the form; desktop draft/unknown-commit safety must remain authoritative |

The normal parent's exported event set has no parent BeforeUpdate audit or Lock
handler. An unbound `btnClose_Click` procedure also exists; it is not an
additional active event. Reusing desktop audited transactions/Lock is a safety
adaptation, not evidence that XL parent callbacks belong to this source form.

Plot-type options map1 Ground,2 Visual,3 Note,4 FS882 and5 Other. The caption
`Full` therefore represents stored `FS882`, not stored `Full`. Species-list
options map1 true and2 false, otherwise NULL. Read-time BOOLEAN normalization
must not rewrite historical storage; new true remains -1. Unknown historical
plot types must not be completed, trimmed or silently replaced.

Stand-age/height estimation controls have Est./Meas. captions and numeric option
values bound to TEXT(2). A controlled native binding probe now observes option1
storing literal TEXT `"1"`, option2 storing TEXT `"2"` and NULL storing NULL.
Captions are not stored values. This evidence does not establish the original
parent's callbacks, subforms or complete lifecycle.

## Configuration and metadata

`clsFormInfo.ProjectIdSource` reads/writes the `Current/ProjectIdSource` registry
setting, with default2. On focus, option2 selects master ProjectMetaData;
other values select the current project's Metadata table. `Env` is the local
option caption, not another storage database to create.

The retained YAML default contains `Current.ProjectIdSource: 2`. The independently
gated parent review now exposes explicit Env/Master source switching through
the existing YAML coordinator, with expected-source collision detection and
unknown-key preservation. This is a shared immediate preference, not a plot
draft or a bound ProjectID assignment. Unsupported retained settings fail the
workflow explicitly without silent repair. Full registry migration, metadata
editing/creation and ProjectID assignment remain separate.

Metadata lookup and creation cannot be copied literally:

- The non-NULL StartDate lookup uses only ProjectID; the NULL branch additionally
  requires a NULL StartDate. Duplicate/NULL definitions remain distinguishable.
- The creation branch searches a second recordset but then tests the first
  recordset's NoMatch state. This is a source defect, not a desktop requirement.
- NotInList concatenates input into RowSource SQL and requests Save/Refresh.
  Do not inherit malformed SQL, implicit identity changes or rejected-save
  history commits.

Reuse the existing reviewed metadata boundaries with explicit selected physical
identity, source choice, creation confirmation and atomic audited persistence.
The exact missing-definition UX and return focus require their own acceptance.

### Next bounded read/presentation contract

The original control tree, not captions alone, defines option ownership:

| Group | Source values/captions | Desktop boundary |
|---|---|---|
| optProjectID |1 Env;2 Master | Unbound preference, not another ProjectID editor |
| optPlotType |1 Grnd;2 Visual;3 Note;4 Full;5 Other | Unbound display/action group targets the separate directly bound PlotType; `Grnd` stores `Ground`, `Full` stores `FS882` |
| optSpeciesListComplete |1 Comp.;2 Part. | Unbound group targets implicit nullable SpeciesListComplete; display initialization must not manufacture writes/audits |
| StandAgeEstMeas |1 Est.;2 Meas. | Direct SV_StandAgeEstMeas TEXT(2); controlled native binding stores `"1"`/`"2"`/NULL |
| StandHeightEstMeas |1 Est.;2 Meas. | Direct SV_StandHeightEstMeas TEXT(2); same controlled binding evidence, not full lifecycle |

Source option items can be CheckBox controls within a group, not standalone
BOOLEAN fields. Keep one live control per storage field; do not render an extra
editable PlotType textbox alongside its option group merely because both source
nodes exist. Preserve exported visibility as evidence rather than inventing
defaults for omitted properties.

Rechecked normal-parent source actions target physical Admin.PlotType TEXT(10)
and implicit Env.SpeciesListComplete BIT. PlotType selected options1..5 store
`Ground`, `Visual`, `Note`, `FS882`, `Other`, then restore the captured option and
Refresh. The callback first assigns the option to an Integer and declares its
mapped value as String; its Else assigns NULL to that String. Do not convert
this source defect into a verified NULL-clear action or copy the CHARS Variant
callback. Species-list-complete uses Variant:1 maps to True,2 to False, Else
to NULL. A future explicit storage proposal must preserve Access BOOLEAN=-1/0
and the implicit target identity without widening ordinary scalar allowlists.
Form_Current calls the separate display initializers: known stored values map
back to option values, otherwise the group is NULL. Display initialization is
not an edit, an assignment or an audit. These are static event paths, not new
callback/Refresh/lifecycle acceptance.

Of77 direct SIVI bindings,62 share a physical column with the proven98-field
FS882 header allowlist. This establishes storage reuse, **not shared parent event
parity**. Fifteen direct bindings are outside that allowlist:

- TEXT storage: SV_PolygonNumber (25), SnowCoverregime (1),
  SV_CanopyComposition (50), SV_AhorizonType (5), SV_RootZoneTexture (100).
- SINGLE: SV_StandHeight, SV_AhorizonDepth, SV_GleyingMottlingCM,
  SV_PercentCoarseFrags, SV_SoilDepth, StrataCoverTotal.
- BOOLEAN: SV_FloodPlain.
- TEXT-backed numeric option groups: SV_StandAgeEstMeas,
  SV_StandHeightEstMeas (2 each).
- PlotType (TEXT10), with the source-specific option mapping above.

The [private owned ProjectID choice read](../siviprojectchoices.go) is now
implemented, not another database family, public editor or form promotion:

1. Reuse the existing runtime YAML snapshot. Recognize source options1/2
   explicitly; an unsupported retained value makes only this workflow unavailable,
   without rewriting it or breaking unrelated application startup.
2. Option2 reads the already attached VMetaData ProjectMetadata physical table;
   option1 reads the selected project's `<Project>_Metadata`. Do not silently
   fall back between them or infer a database named Env.
3. Preserve physical rowid, actual table/column names, declared types, nullable
   ProjectID/ProjectTitle and duplicate definitions. The source has no ORDER BY;
   deterministic physical-row order is a desktop adaptation, not an invented
   alphabetical source order.
4. Keep the two source columns visible together when eventually rendering
   choices (source ColumnCount2/ColumnWidths1440;7200). A choice is not permission
   to edit the parent's identity or choose a metadata definition implicitly.
5. Test local/external projects, both sources, NULL/empty/duplicates, malformed
   configuration, cancellation/retry, clone safety and zero database/YAML writes.
   Reads stay on the pinned coordinator/caller-owned snapshot with explicit
   errors, not per-call file pools. No public bindings or parent enabling.

[Choice fixtures](../siviprojectchoices_test.go) preserve the canonical master
TEXT primary key rather than removing it to manufacture duplicate non-NULL
IDs. The project fixture covers repeated text IDs; the master fixture covers
two distinct physical NULL-ID definitions, alongside literal empty text.
Both sources preserve physical provenance, raw declared types and title values.
Focused parent/choice/source race16.771s and full integration race740.732s/all
packages pass; independent read-only review found no significant issues.
This remains a context-scoped backend read, not native
acceptance of the parent GotFocus/NotInList or preference-edit workflow.

New metadata definitions retain the Access Metadata ProjectID TEXT(20) boundary,
distinct from the Env ProjectID TEXT(30) binding. Historical wider identities
remain raw and must not be trimmed to make a definition valid. Reuse the existing
reviewed physical-identity/template/create/restore paths for any later mutation,
but do not carry over the defective recordset check, unescaped RowSource appends
or implicit Save/Refresh behavior.

### Scalar planning boundary

Further source inspection separates the five TEXT fields by control semantics,
not just physical type:

- SV_PolygonNumber and SV_CanopyComposition are ordinary textboxes.
- SnowCoverregime selects Item/ItemDescription from exact list name
  `SnowCoverRegime`, ordered by ItemOrder.
- SV_RootZoneTexture selects DISTINCTROW Item/ItemDescription from exact list
  name `SoilTexture`, ordered by ItemOrder.
- SV_AhorizonType has a value-list RowSource `Ah;Ae;""`. Its explicit blank
  choice does not prove whether an eventual bound write stores NULL or empty
  TEXT. Do not infer that from the label or silently repair historical values.

No ValidationRule/InputMask/Format/DecimalPlaces was exported for these15
distinct bindings in the packaged metadata. Absence alone does not establish
Required, AllowZeroLength or LimitToList. The later bounded table/design probes
below resolve the specific TEXT properties without generalizing to other controls.

The [private scalar planner](../siviparentscalar.go) now covers the six SINGLE
fields and SV_FloodPlain. It reuses `metadataCellValue`, tagged-value clone/
expected equality and `validateSingleRangeChange` from the accepted height
boundary, omitting unchanged assignments before new-value validation.
It requires one unambiguous literal Env/Admin pair and exact context, source
table, physical rowid, allowed column and reviewed value; invalid tails never
return partial assignments. It preserves raw historical invalid cells and an
explicit no-write result. New BOOLEAN
storage is0/-1, never inherited raw2; NULL and unchanged invalid storage remain
distinguishable. The existing SINGLE helper intentionally retains float64
precision within the physical SINGLE range: do not silently add float32
quantization or invented0..100 constraints from percentage captions.

Do not drive these new fields through a complete FS882Header save that assigns
unrelated parent columns. Keep62 shared storage columns behind their existing
individually proven guards and record any later shared audit/Lock behavior as a
desktop safety adaptation, not a missing SIVI BeforeUpdate event.

Ordinary TEXT empty-write semantics, the three categorical controls,
PlotType source actions, join authorization,
metadata mutations and full parent lifecycle remain separate gates. A private
numeric/BOOLEAN planner is not permission for any writer, API or form promotion.
[Planner tests](../siviparentscalar_test.go) cover all seven fields, both owners,
exact SINGLE boundaries/overflow/nonfinite values, precision preservation,
BOOLEAN0/-1/NULL, unchanged invalid history, stale/foreign/duplicate requests,
excluded targets, clone isolation and in-flight cancellation. Focused combined
parent/choice/source race17.051s passes; independent read-only review verified
the corrected mixed-owner test placement and found no significant issues.
Full integration race731.402s/all packages passes.

### Native-derived TEXT options

One fresh unlinked Access fixture reproduced source OptionGroup107, CheckBox106
children with values1/2 and both physical TEXT(2) bindings. Independent Form-view,
focus, record ID, binding/type/size and before/after observations passed six
actions (1,2,NULL per field), with no sibling changes. The owned PID/start/path/
HWND was recorded and the process exited after Quit. An immediate final hash
initially encountered an outstanding COM handle; evidence was finalized only
after owner exit, without reopening the form or replaying completed writes.
Canonical export hashes stayed unchanged. Incidental Access MRU/profile effects
were not assessed; no VPRO registry calls or original parent callbacks ran.

The private `planSIVIParentOptions` now shares proven physical-owner/expected/
changed-only/clone guards with the numeric/BOOLEAN planner. New values are exact
TEXT `"1"`/`"2"` or NULL; numeric/caption/trimmed aliases are rejected while
unchanged historical invalid values remain omitted. Source domains remain
separate, and no writer/API or form was enabled. Combined focused race17.100s
passes; independent follow-up review found no significant issues and verified
scalar preservation/domain isolation. Full integration race736.941s/all packages
passes. No source write authorization or parent enabling is implied.

### Owned nine-field proposals

The private [proposal boundary](../siviparentread.go) now reads the current
physical parent and plans both domains under one context operation lease,
coordinator lease and caller-owned read snapshot. The ordinary raw reader uses
the same typed snapshot projection helper. Proposals retain the complete cloned
original (schema, raw values, physical identities and labelled membership), with
separate scalar and option assignment lists. At least one explicit domain edit
and exactly one literal physical pair are required. Unchanged invalid values
are omitted; a failed second domain, late cancellation or cleanup error returns
no proposal. This is a read/planning boundary, not a persistent draft session or
mutation authorization; Access-equivalent nonliteral alternatives remain an
unresolved writer gate.

[Proposal tests](../siviparentproposal_test.go) cover all nine fields, local and
external paths, domain isolation, clone ownership, unchanged invalid history,
stale source/contexts/rows, duplicate targets/pairs, invalid tails, blocked and
late cancellation, attachment-preserving rollback/retry and unchanged database/
YAML bytes. Focused combined parent/choice race25.027s passes after an independent
review identified a counts-only acceptance gap. Literal expectations now compare
complete payloads for all nine assignments in combined/single domains and local/
external contexts; reviewer follow-up verified the fix with no significant
issues. Full integration race716.771s/all packages passes with the final
application code; the test-only literal assertion improvement was verified
afterward in the focused race suite. No public transport or parent workflow
changed.

### Three categorical source definitions

Read-only source inspection confirms normal-parent `SnowCoverregime`,
`RootZoneTexture` and `AhorizonType` controls have no exported LimitToList in
their control blocks. Snow and texture use two-column Item/ItemDescription
choices ordered by ItemOrder; A-horizon has the explicit `Ah;Ae;""` value list.
This does not establish new-value membership or NULL/empty binding semantics.

The selected database-family VLists source is physical `USysTableOfLists`, not
a frozen editor catalogue. Its ItemOrder is REAL (including fractional texture
orders), and both lists contain physical NULL Item definitions with non-NULL
descriptions. Preserve those definitions rather than replacing NULL with blank
or a caption. Physical rows, descriptions and duplicate definitions remain
distinct from availability, physical field validity and write membership.

A bounded read-only DAO16/collation1033 check reused the sealed closed join
fixture: two identical physical rows survive both plain SELECT and single-table
SELECT DISTINCTROW, while SELECT DISTINCT returns one. The database was opened
read-only, no forms/new fixture/writes were used, and closed bytes stayed
unchanged. Thus the texture source must not become a two-column SQL DISTINCT
deduplication merely because its RowSource says DISTINCTROW. This controlled
case does not prove arbitrary source collation/order or bound-write behavior.
The private [categorical reader](../sivicategoricalchoices.go) now reuses the
owned coordinator and tagged storage reader for exactly those three bindings.
It preserves complete physical VLists schemas/rows, NULL/empty/BLOB/BOOLEAN
history, duplicate definitions and fractional ordering. Source ControlID comes
from cached normal-parent metadata and is distinct from ControlName. Ah/Ae/blank
values have no invented physical row IDs or NULL substitution. Unsupported or
unavailable sources fail explicitly, without fallback between controls.

The independent review identified a generated-rowid provenance hole: SQLite
table_info excludes generated columns. The reader now preflights complete
table_xinfo before reading definitions, rejecting all generated/hidden columns
and rowid/_rowid_/oid shadows. Four one-row regressions verify errors, nil results,
unchanged bytes and attachment-preserving retry after schema restoration.
Focused categorical/parent/choice race33.281s passes; reviewer follow-up verified
the fix with no significant issues. Final-code integration race757.356s/all
packages passes. No writer,
public API, membership enforcement, bound-write proof or parent enabling exists.

### Original omitted TEXT table properties

A second read-only DAO16 check reused the same sealed closed disposable copy.
Both original unlinked physical `Sample_Env` and `USysEnvTable` agree:

| Field | Native TEXT size | Required | AllowZeroLength |
|---|---:|---|---|
| SV_PolygonNumber | 25 | false | false |
| SV_CanopyComposition | 50 | false | false |
| SnowCoverregime | 1 | false | true |
| SV_AhorizonType | 5 | false | true |
| SV_RootZoneTexture | 100 | false | true |
| SV_StandAgeEstMeas | 2 | false | true |
| SV_StandHeightEstMeas | 2 | false | true |

No forms/new fixture/writes were used; closed fixture bytes remain unchanged.
These observations resolve omitted properties of the original two table
objects, not every selected external project or bound-control empty coercion.
In particular, ordinary textboxes and categorical fields must not inherit each
other's empty-storage policy. Next private ordinary-text storage planning can
accept explicit NULL or valid nonempty TEXT within the two UTF-16 bounds,
preserving unchanged history and rejecting new empty TEXT without converting it.
There is still no empty-input UI/save/lifecycle proof or parent write permission.

### Categorical binding evidence and physical planning

A fresh disposable database imported the original normal-parent form export,
then opened only Design view (CurrentView=0). Access16 reported LimitToList=false
and BoundColumn=1 for all three categorical controls, with the original bindings,
column counts and RowSources intact. No original data or parent Form view opened.
This resolves the omitted design properties, not the parent's callback behavior.

The same workflow fixture then hosted a controlled equivalent bound form using
the observed control properties and the original TEXT1/100/5 bounds, nullable
fields and AllowZeroLength=true. Independently checked focus/record/binding and
physical reads showed:

| Controlled input path | Physical result |
| --- | --- |
| Focused Text with valid unlisted text | Literal unlisted TEXT |
| Focused Text cleared to empty | NULL |
| Explicit Value assigned empty | Empty TEXT |
| Explicit Value assigned NULL | NULL |
| Third A-horizon ItemData assigned through Value | Empty TEXT |

All13 actions left sibling fields unchanged. The design baseline was retained
before construction; both native owners closed, fixture locks disappeared and
the canonical export hash stayed unchanged. These are COM Text/Value and
explicit-item observations, not keyboard/dropdown-click proof, original parent
callbacks/lifecycle, arbitrary Access collation or write authorization.

The private [categorical planner](../siviparentcategorical.go) reuses the accepted
physical-pair/context/table/row/expected-value guards for exactly these three
Env fields. It accepts explicit NULL, empty TEXT and valid nonempty TEXT within
1/100/5 UTF-16 units, without catalogue membership enforcement, trimming or
recasing. It omits unchanged invalid history and clones assignment values.
Reference availability, physical validity and membership remain separate;
this planner neither loads choices nor translates an editor's empty input.
[Tests](../siviparentcategorical_test.go) independently assert complete payloads,
BMP/astral bounds, malformed tags/UTF-8, NULL/empty separation, historical
correction/clones, domain isolation, stale originals, invalid tails,
ambiguous pairs, cancellation and retry. Focused combined race33.874s passes
after explicitly preparing the cancellation fixture's A-horizon original.
Independent review found no significant issues; full integration
race710.778s/all packages passes. The eleven-field owned
successor is unchanged; no public API, persistent draft, parent writer or enabling.

### Fourteen-field owned storage successor

The private [owned categorical successor](../siviparentread.go) adds the three
categorical assignments as a separate fourth domain under the existing owned
original-read snapshot. The accepted nine/eleven-field signatures and returned
DTO behavior remain unchanged through compatibility wrappers. Categorical-only
requests do not depend on ordinary TEXT edits, and a failing fourth domain
returns no earlier-domain proposal.

[Acceptance tests](../siviparentcategoricalproposal_test.go) independently verify
complete literal14-field payloads and all15 nonempty domain subsets in local and
external project contexts, legacy equivalence, original/assignment clones,
NULL/empty/BLOB history, stale originals, invalid targets/tails, pre/late
cancellation and attachment-preserving retry. Database-family and YAML bytes
remain unchanged. Focused combined race34.831s passes after correcting an
initial nested-domain dispatch caught by categorical-only tests. Independent
review found no significant issues; full integration race733.778s/all packages
passes. This is private storage planning, not an
editor-empty translator, persistent frontend draft, catalogue-availability
decision, implicit SpeciesListComplete action or parent writer/navigation.

### Explicit source-action storage planning

The private [source-action planner](../siviparentactions.go) now derives physical
targets from the two original normal-parent source group instance IDs, rather
than accepting a caller-supplied column. Shared original-layout parsing is cached;
the source cache requires unbound read-only OptionGroups and their resolved
AfterUpdate procedures. PlotType selected1..5 maps to Admin TEXT `Ground`,
`Visual`, `Note`, `FS882`, `Other`. Explicit species-list-complete1/2/NULL maps
to implicit Env INTEGER-1/0/NULL.

This is a bounded source-defined planning subset: PlotType NULL/other choices
remain errors because the String/NULL callback path is not accepted; unsupported
species options also error instead of silently taking the source Else. An edit
record's NULL species option is explicit clear intent, not display initialization.
No source callback, option restoration, Refresh or keyboard interaction runs.

The accepted [shared cell guard](../siviparentscalar.go) now has an explicit
implicit-target allowlist for this action boundary. Its old wrapper passes no
implicit authorization, so all four ordinary domains still reject that target.
Context/table/row/expected/raw-history/changed-only/clone/cancellation guards
remain shared; unchanged canonical selections produce no assignments.
[Tests](../siviparentactions_test.go) assert literal source IDs/owners and complete
five PlotType/three species payloads, historical correction/clones, unknown
options/source aliases/CHARS/preference rejection, ordinary-domain isolation,
stale/duplicate/tail/ambiguous failures, cancellation and retry. Focused combined
race35.411s passes; independent review found no significant issues, full
integration race772.844s/all packages passes. The owned
fourteen-field DTO/methods remain unchanged; no owned action proposal,
persistent frontend draft, writer/public API or parent promotion.

### Sixteen-field owned source-action successor

The private [owned action successor](../siviparentread.go) now adds separate
source-action assignments as a fifth domain under the existing owned current-
original snapshot. The nine/eleven/fourteen-field signatures and DTO behavior
remain unchanged through compatibility wrappers. Action-only requests are
independent of the ordinary domains; a failing fifth domain returns no earlier
assignments. The implicit species target is reachable only through the accepted
explicit source-group action planner, not an ordinary field edit.

[Acceptance tests](../siviparentactionproposal_test.go) independently assert full
literal16-field payloads and all31 nonempty domain subsets for local/external
contexts, predecessor equivalence, raw historical originals/assignment clones,
explicit species NULL, stale/foreign/duplicate/source failures, pre/late
cancellation and attachment-preserving retry. Database-family and YAML bytes
remain unchanged. Focused combined race36.330s passes; independent review found
no significant issues; full integration race719.316s/all packages passes.
This is not a source callback/Refresh acceptance, persistent
frontend draft, Access-equivalent join authorization, writer/API or promotion.

### Complete physical schema before parent originals

Before draft preparation, eight disposable regressions reproduced the older
parent-read gap: table_info omitted generated columns, allowing rowid/_rowid_/oid
shadows or an ExtraGenerated column in either Env or Admin to return a parent.
The owned reader now reuses the categorical reader's complete table_xinfo
preflight before reading either original table. Generated/hidden columns and
all row-identity aliases error with parent-specific scope; failures return no
parent or proposal, without discarding coordinator attachments.

[Eight regressions](../siviparentphysicalschema_test.go) verify rejection, unchanged
database bytes during reads/planning, and exact original/retry restoration after
the disposable schema column is dropped. Categorical wrapper behavior remains
unchanged; other raw readers are not broadly refactored. Focused combined
race38.196s passes; independent review found no significant issues,
full integration race741.108s/all packages passes.

### Accepted unwired parent draft contract

Reuse the accepted height editor/session's ownership and remount-safe error
patterns, not its field-specific HeightB/BLOB rules. A parent model must preserve
explicit original context/project/table/Env-Admin row/tagged cells and source
control identity across all16 distinct targets/five domains. Keep ordinary TEXT
nonempty validation, categorical clear-to-NULL versus explicit empty choice,
selected source actions and BOOLEAN-1/0 separate. Display initialization is not
action intent. Invalid raw Unicode/new overlength and draft errors block Save,
Lock and close across remount; correction or explicit Undo clears only the
appropriate editor/original identity. Preserve unchanged history and do not
write a blanket FS882Header. The pure [model](../frontend/src/siviParentEditor.ts)
and [owned session](../frontend/src/siviParentSession.ts) now implement this
contract. Complete ordered original bindings/physical columns/raw cells and
one literal pair are checked, not promoted to Access-equivalent join authority.
All16 targets and31 nonempty domain subsets are covered by
[tests](../frontend/src/siviParentEditor.test.cjs), including literal complete
payloads, UTF-16 bounds, malformed raw Unicode, SINGLE/BOOLEAN, explicit
clear/empty/option identity, unchanged invalid history and BLOB correction
without height-specific restrictions. Proposal construction rechecks serialized
intent against the owned expected cell; clones cannot alter session state.
Context replacement/disposal refuses dirty drafts, and Undo requires the exact
editor owner. All72 integrated SIVI frontend tests, check0 errors/warnings,
isolated production build and independent review pass. There is no write port,
public binding, mounted parent editor/navigation or native lifecycle acceptance.
The [wire boundary](../frontend/src/siviParentTransport.ts) now checks unknown/
nullable responses structurally before applying the complete original validator
and comparing an independently supplied active context/project/plot. A foreign
or incomplete response cannot initialize a session. This remains an unwired
frontend boundary, not a public Go service or proof of a mounted Wails lifecycle.

Original `Form_Current` calls `SetSppListComplete` then `SetPlotType`, both
display-only assignments to the unbound option groups. `Form_Load` initializes
`optProjectID` from `clsFormInfo.ProjectIdSource`; its AfterUpdate changes that
shared source. That project metadata lifecycle is outside the sixteen-target
model. The fourteen bounded scalar/option/TEXT/categorical controls have no
exported field callbacks; the two action groups retain their resolved
AfterUpdate paths. PlotType's callback Refresh and defective String/NULL Else
remain unverified, not normalized by display initialization.

The pure display helper maps only exact `Ground/Visual/Note/FS882/Other` TEXT
and species BOOLEAN INTEGER-1/0 to selected options. NULL has no selected
option. Unsupported historical storage/case/trailing-space aliases retain the
original tagged cell and return a visible diagnostic, rather than approximate
Access `Option Compare Database` or invent action intent. Calling display never
stages a draft/assignment. All74 integrated SIVI frontend tests, check0/0 and
isolated production build pass. The separately gated native reader below now
accepts Wails identity/remount behavior; writer/join/callback/lifecycle remain
unavailable.

### Actual owned Go/frontend transport acceptance

[Go roundtrip regression](../siviparentwire_test.go) obtains one parent original
from the disposable owned context, verifies its twelve projection properties
and lossless JSON roundtrip, and confirms every attached database/runtime YAML
byte is unchanged. Its verbose output emits the original and an independent
context-selection owner as labelled base64 JSON. The dedicated
[frontend wire tests](../frontend/src/siviParentWire.test.cjs) consume that
successful receipt, rather than a second hand-built projection, and check
session initialization, all16 five-domain edits with exact actual expected
cells, immutable raw originals and no display-generated actions.
`npm run test:sivi-wire` requires `VPRO_SIVI_PARENT_WIRE_RECEIPT` pointing to a
successful verbose `TestSIVIParentOriginalWireRoundTrip` receipt; missing/failed
fixture evidence is an explicit error, not a skipped success.

This review also reproduced two sparse-array acceptance gaps against the
retained predecessor: a missing binding slot and a missing unbound raw cell.
Complete shape/original guards now inspect every array slot, including holes,
and tests reject both plus incomplete schemas/rows. Session construction also
requires a nonempty well-formed string editor identity without NUL. Existing
plain test assembly is extracted once for both suites; application policies and
all prior model/transport semantics stay unchanged.

Focused Go race1.371s,74 integrated frontend tests plus2 actual-wire tests,
check0/0 and isolated production build pass. Full Go integration
race692.436s/all packages passes. No new Go application method, binding, mounted
reader/editor, writer or native action is introduced. Actual JSON interoperability
is not Wails dispatch/remount or source callback/join/lifecycle acceptance.

### Mounted owned read-only acceptance

The subsequent source-resolution boundary below adds an immediate YAML source
preference and physical metadata review; parent values remain read-only.

[Public original facade](../siviparentreviewservice.go) adds exported lossless
projection DTOs without changing the private read contract. The independent
literal backend `VPRO_SIVI_PARENT_REVIEW` and frontend build
`VITE_SIVI_PARENT_REVIEW` flags default off, separately from height editing.
Backend reads reuse the owned coordinator/lease/snapshot/schema preflight, not
direct database handles. Malformed flags, disabled requests, stale ownership,
cancelled calls and unsupported physical schemas return explicit errors.

[Read session](../frontend/src/siviParentReadSession.ts) validates unknown wire
results against independent active context/project/plot. It retains cloned
originals outside tab lifetime; Cancel clears originals and errors explicitly,
invalidates late results and cancels the actual Wails promise. Reload is explicit.
Disposal ends ownership. The [panel](../frontend/src/SIVIParentReadPanel.svelte)
renders all77 original source labels in plot/location, site and vegetation
groups with responsive grids, tagged storage and literal source-option display.
Implicit SpeciesListComplete remains visible. It has no editable controls or
Save port, and explains persisted originals versus unsaved FS882 drafts.

Seven enabled native cases verify actual generated dispatch, all77 fields,
invalid FS882 error preservation/remount/Undo, reader remount, post-resize600px
visibility/labels and panel bounds, actual in-flight cancellation under a
disposable SQLite exclusive read barrier with retry, PARENT01-to-PARENT02
ownership replacement and stale-context rejection. A separate disabled backend
phase rejects the opt-in frontend without exposing originals. All16 database/
runtime YAML hashes remain unchanged. Current PID/start/path/HWND and actual
click targets are checked; owned WM_CLOSE is followed by fresh PID absence.
The initial proof skipped Undo while remount reference loading was busy; two
failed proof owners were restored and closed without writes. The corrected
guard waits for readiness and the complete suite passes, rather than resetting
drafts or replaying writes.

Initial focused Go race25.527s/final regression race1.751s and full
race772.916s/all packages pass, as do327 root
frontend tests,79 integrated SIVI tests, check0/0, isolated opt-in/default builds
and independent review. One disposable native fixture/candidate is retained;
the protected default binary/assets/source export are not promoted or changed.
This accepts read-only presentation and owned transport, not Access-equivalent
join/collation, ProjectID metadata, action callbacks/Refresh, parent drafts,
writing/restoration or full FS1333 lifecycle. Resolve those source dependencies
before separately authorizing a mounted editor.

### Bounded source join and ProjectID lifecycle resolution

A new disposable DAO16.0/General1033 database independently verifies1,302
comparisons: all36x36 ASCII letter/digit pairs with case changes, trailing
U+0020, leading/embedded spaces, numeric-looking text and NULL exclusion.
No Access application/form, registry or canonical database was modified.
The [join review](../siviparentjoin.go) implements only the supported nonempty
ASCII alphanumeric TEXT7 domain, with trailing U+0020 and case equivalence.
It scans every physical Env/Admin identity, not only BINARY-filtered rows.
Case/trailing-space aliases and duplicate pairs remain explicit; one exact
physical pair is certified only when no unsupported identity is present.
Unicode, punctuation, empty TEXT, overlength and non-TEXT identities remain
unverified, not approximated with SQLite NOCASE. A positive review is still
not parent write authorization or arbitrary Access collation acceptance.

The [ProjectID source facade](../siviparentreviewservice.go) now exposes owned
choice reads and explicit source switching. Shared
[snapshot ownership](../siviownedread.go) preserves existing parent read/proposal
behavior while reusing lease, pinned snapshot, rollback and profiling for choices
and join review. Metadata reads preflight complete table_xinfo, rejecting hidden/
generated/shadowed identities. Physical duplicate definitions, actual table/
column spelling and raw storage remain intact.

Source options1 Env/2 Master are validated; target metadata is preflighted before
the preference changes. Under the existing YAML mutex, compare-and-set checks
the caller's expected source, retains all other/unknown settings, uses atomic
replacement and rejects cancellation before commit. Invalid source, missing/
unsupported metadata, stale context, collision and failed replacement return
errors without successful-shaped output or data changes. Same-source calls are
semantic no-ops. No plot assignment, audit or hidden metadata creation occurs.

The [source session](../frontend/src/siviParentSourceSession.ts) retains an
independent context/project/plot owner across tab remount and validates actual
wire identity, source, table, signed rowids and complete tagged cells. Failed
parallel reads cancel their outstanding peer; late/disposed responses cannot
repopulate state. Preference commits block close/navigation, cancellation and
owner disposal. Errors survive remount and require explicit observation/reload
before another source change. The panel presents Env/Master source buttons,
original read-only ProjectID/title definitions and join diagnostics; assignment,
metadata edit/create and defective NotInList/Refresh paths stay unavailable.

Focused Go race66.630s and full integration race715.661s/all packages pass;
333 frontend tests, type-check0/0 and isolated opt-in/default builds pass.
Eight initial enabled native cases plus default denial verify source initialization,
duplicate/NULL/empty metadata, ambiguous aliases, blocked in-flight source
commit, stale/collision/invalid rejection, remount, exact YAML restoration and
600px visibility. The successor's three read-only cases verify retained-invalid
source with a blocked peer, explicit cancellation/correction/retry and exact
fixture restoration; successful preference writes were not replayed after the
peer-cancellation correction. Both binaries/assets and their exact receipts are
retained. All16 DB/config hashes restore; current PID/start/path/HWND and fresh
PID absence are checked. Protected delivery and source remain unchanged.
The private directly bound writer now performs those transactional rechecks.
The public/mounted successor below has its own transport/native lifecycle
acceptance; full metadata and source callbacks are not inferred from the
source preference or directly bound writer.

The private [two-field TEXT planner](../siviparenttext.go) is now implemented,
using the accepted shared parent ownership/expected/changed-only/clone loop and
existing nonempty UTF-16 validator. It accepts explicit NULL or literal nonempty
TEXT within25/50 units, omits unchanged invalid history, and does not widen the
scalar/options/categorical domains. [Tests](../siviparenttext_test.go) independently
compare complete single/two-field assignment payloads, exact BMP/astral bounds
and overflow, malformed/new nontext/empty values, historical aliases, clones,
foreign/stale/repeated targets, invalid tails and cancellation/retry. The guessed
cancellation threshold in the new test was corrected to the accepted dynamic
post-projection probe. Focused combined race33.095s passes; independent review
found no significant issues. Full integration race753.107s/all packages passes. Owned nine-field
proposal signatures, public APIs/bindings and parent enabling are unchanged.

The private [eleven-field owned successor](../siviparentread.go) now adds a
separate ordinary-TEXT assignment list to the accepted nine-field proposal.
The existing nine-field method and DTO remain intact through a wrapper that
passes no TEXT edits. One context/coordinator lease and caller-owned snapshot
cover current original reads and all three domains; a third-domain failure
returns no earlier assignments. [Tests](../siviparentstorageproposal_test.go)
independently verify complete eleven-field payloads and domain subsets on
local/external project contexts, old-nine-field equivalence, clones, unchanged
raw history, stale originals, invalid tails/cancellation/retry and no database/
YAML writes. Focused combined race33.937s passes; independent review found no
significant issues. Full integration race742.091s/all packages passes. This is still private read/
planning, not a persistent draft session or source join/write authorization.

## CHARS and next boundary

The CHARS parent is not interchangeable with the normal parent. Its PlotType
handler uses Variant and omits the normal parent's option restore/Refresh.
The normal handler uses String despite a NULL Else branch. Preserve those
differences as evidence; do not copy CHARS parent code merely to show B3/B4/B5.
The existing shared shrub option changes only the A child presentation.

[Source-contract tests](../fs1333source_test.go) pin the nine normal event routes,
77 uniquely owned physical bindings and read-only/unmapped status, alongside
the prior child links, aggregate-height types and extended-child boundary.
They are static/SQLite checks, not Access runtime or complete parent parity.

The private [parent projection](../siviparent.go) and
[owned reader](../siviparentread.go) now derive77 direct bindings from the
packaged source metadata and expose SpeciesListComplete separately as an
implicit target. Complete typed Env/Admin rows retain physical rowids, column
names and declared types. Duplicate physical pairs remain separate; no header,
BOOLEAN normalization, text coercion or editor validation rewrites raw values.
The reader uses the selected project's pinned coordinator, caller-owned read
snapshot and literal context/plot identity, with no public service or binding.

Membership is explicitly `literal-binary-inner-pairs`: both physical text
identities must match the requested plot byte-for-byte. NULL joins and orphans
produce no pairs. This preserves the inner-join shape but **does not establish
Access Unicode collation or DISTINCTROW equivalence for arbitrary historical
duplicates**. The adaptation is labelled in the private result; it must not
be promoted as verified form membership without separate source evidence.
Ambiguous/missing bindings, shadowed rowids, malformed cells, nontext join
identities and substituted views fail explicitly without partial output.

[Disposable fixture tests](../siviparent_test.go) cover physical duplicate
pairs, raw historical invalid BOOLEAN/text/numeric/blob/NULL/empty storage,
clone isolation, literal Unicode/case/space membership, external project paths,
stale contexts, in-flight cancellation/retry and database/config byte invariance.
These are backend contract tests, not native parent acceptance.

### Selective join evidence

A bounded DAO16.0 probe used a disposable copy of the original local
VPro64.accdb (collating order1033), with synthetic tables and no opened forms.
Independent read-only observation verified the prepared rows and two physical
rows per duplicate table, leaving the closed fixture bytes unchanged.

| Prepared text identities | Access inner join observed |
|---|---|
| Exact literal, including quotes/leading and trailing spaces on both sides | Match |
| `Case` / `case`, accented upper/lower case | Match |
| `Space ` / `Space` | Match |
| Composed / decomposed acute accent | Match |
| Leading space on only one side | No match |
| Accented / unaccented text | No match |
| `01` / `1` | No match |
| NULL / NULL | No match |

Selecting `DISTINCTROW e.*,a.*` retained **four identical output pairs** from
two identical physical Env rows and two identical physical Admin rows. This
supports retaining duplicates for that all-table projection rather than
substituting SQL DISTINCT. It does not prove every Access query variant.

The planned empty-text parameter was independently observed as physical NULL
on both sides. Consequently this probe **does not establish empty TEXT join
behavior**; it is not legitimate to count that planned case as accepted.
The first attempt stopped on a stale DAO TableDefs collection; the second
refreshed it and succeeded. Failed evidence is retained, no mutations replayed,
and canonical Access SHA256 remained
`01481b94569c7172f32a812e65365f78cea5e440dd63220d91a37dade5778423`.

These observations confirm that literal BINARY membership differs from Access
for some histories. Do not replace the coordinator's existing views broadly or
approximate a complete Access collation from this small sample. Full parent
promotion needs an explicitly tested comparison/membership policy; the private
raw reader remains labelled as an adaptation and the entrypoint stays disabled.

The next section distinguishes the private writer from its independently gated
public/mounted successor. Keep callback actions, metadata creation and full-form
navigation disabled until their separate contracts and native lifecycle tests pass.
The blocked Long Vegetation SINGLE/Nz/Val calibration is not reopened here.

## Private directly bound writer and typed restoration

The next ten backend gates use the accepted planners without extending source
authorization to callbacks or metadata:

| Gate | Boundary |
|---|---|
| 1 | Re-read normal form/query/module/storage; fourteen directly bound fields only |
| 2 | Recheck bounded ASCII source join, exact physical pair and selected physical SU inside the write transaction |
| 3 | Six nullable SINGLE fields and nullable SV_FloodPlain0/-1 |
| 4 | Two directly bound Est./Meas. TEXT1/2/NULL fields |
| 5 | Nullable/nonempty literal SV_PolygonNumber25 and SV_CanopyComposition50 UTF-16 units |
| 6 | SnowCoverregime1, SV_RootZoneTexture100, SV_AhorizonType5; NULL/empty/unlisted remain distinct |
| 7 | Exact parent audits with child ID=NULL and typed whole-original/committed provenance, in the same transaction |
| 8 | Whole-row/context/schema/alias collisions, cancellation, ownership, trigger rollback, no-op and retry |
| 9 | Typed audited-cell retain/prune restoration; complete committed-pair/audit checks and replay rejection |
| 10 | Focused/full Go race, independent read-only review and sealed source/evidence checkpoint |

The [writer](../siviparentwrite.go) accepts a complete owned original plus four
explicit edit domains. The public positive join review is never reused as write
permission: the writer invokes the same comparison kernel inside its own
transaction on main, preflights original schemas, rechecks physical rowids and
reads selected SU membership there too. External SU is attached read-only;
unfiltered None is explicit. All-table raw before/expected/after comparisons
include unrelated rows, audits and schema, rejecting unexpected trigger effects.
Existing file leases/identity checks cover the final commit boundary. Private
shared transaction tests reject cancellation or file-ownership failure after
all fourteen parent/audit changes have been staged, then verify byte-identical
rollback and successful retry.

New assignments retain existing UTF-16/SINGLE/storage validation. Historical
invalid values are omitted when unchanged; valid correction records their
original typed value for restoration. New BOOLEAN true is -1, while restoration
can recover historical1 without normalizing it. No-op and failed batches do not
append history. BLOB correction is explicitly rejected rather than inventing a
lossy source audit representation.

Audits use the desktop safety adaptation and existing strengths0-3, not an
invented normal-parent BeforeUpdate callback. Parent ID is NULL. Technical
history is created only when actual audits exist and includes complete original/
committed parents and exact typed audited changes. No unaudited deletion becomes
restorable merely because it shares a transaction with an audited value change.

The [restorer](../siviparentrestore.go) checks raw JSON Unicode before decoding,
one complete value, known fields, source-derived target planning, the whole
committed physical pair, exact audit identity/storage and unchanged file owner.
It supports existing case-insensitive _Env/_Admin or project-qualified audit
aliases only; it writes Restore=-1 on retain or removes only verified audit rows
on prune. Unchanged unaudited cells and every child remain untouched. Reopened
context IDs may differ only for the same project/plot and exact physical pair.
Already restored events cannot be replayed.

Focused SIVI race132.748s covers all15 nonempty domain subsets, strengths0-3,
exact astral UTF-16 limits, NULL/empty/option/BOOLEAN semantics, invalid history,
local/external project/SU contexts, schema/alias/collision/trigger failures,
cancellation, recovery and typed restoration. Full integration race894.016s/all
packages passes; independent read-only review found no significant issues.
All ten gates are complete and the source/receipts are sealed in the
[writer checkpoint](../archives/sivi-parent-direct-writer-checkpoint/evidence-manifest.json).
Current machine state is recorded in [WINDOWS_HANDOFF.md](../WINDOWS_HANDOFF.md).
This immutable private checkpoint makes no public/mounted/native claim; those
belong exclusively to the following successor. No protected promotion.

## Public and mounted direct-parent successor

[Strict transport](../siviparenteditservice.go) exposes a separately guarded
editor-original read, Save and typed Restore. Backend parent review and editing
must both be enabled, as must both corresponding frontend build flags; literal
flags default off. Requests explicitly include the complete original and all
four domain arrays. Edits require their ownership, expected and value properties;
unknown/action fields and raw malformed JSON Unicode fail before decoder repair.
The existing review-only original read does not initialize editor controls.

[Persistent writer ownership](../frontend/src/siviParentWriteSession.ts) reuses
the accepted draft parser/proposal and narrows its authorization to fourteen
targets. It preserves raw errors and draft identity through panel remount, blocks
conflicting parent/child operations and rejects dirty/blocked owner replacement.
Invalid drafts block Save/Lock/close; valid correction or Undo clears them.
Only clean original reads are cancellable. Resolved Save/Restore responses retire
their replayable state before shape validation and independent original refresh;
ambiguous acknowledgements or known committed cleanup failures require explicit
recovery, never another mutation. BLOB correction remains unavailable.

The source-labelled panel still has77 bindings, with exactly fourteen single
live controls after explicit load and63 read-only values. Ordinary NULL,
categorical empty TEXT and retaining original storage are separate intents.
Controls use visible associated labels and responsive grouping at1400px/600px,
not exported fixed coordinates.

Focused Go race117.910s, full integration race714.605s/all packages,344 frontend
tests, check0/0, isolated opt-in/default builds and independent read-only review
pass. Disposable native Wails verifies invalid raw draft/remount/Undo/Lock/OS-close/
source replacement guards, actual read cancellation/retry, stale context and
physical alias denial, busy commit guards and exact fourteen-cell Save/audits/
history. The initial verifier incorrectly expected qualified audit aliases and
NULL Restore; `_Env`/`_Admin` and integer0 are the accepted physical representation.
The successful Save was retained and never replayed. Its unfinished native
fourteen-cell prune restoration ran after normal closure and context rotation,
with exact original parent/child/audit restoration and history replay rejection.
A distinct WRITE02 scalar-to-NULL event exercises actual mounted retain Restore,
live committed history/remount, exact one audit retained with Restore=-1, fresh
originals and rejection of its own replay. This is a different plot/value/event,
not repetition of the completed fourteen-field Save.

The actual default build has no review/editing navigation or controls and rejects
all three editor APIs. Editing-disabled backend with opt-in assets cannot expose
live controls. All16 disposable data/config hashes restore after verified owned
closure; typed history and exact before/committed/restored snapshots remain in
the [mounted writer seal](../archives/sivi-parent-mounted-writer-checkpoint/evidence-manifest.json).
The39-file private predecessor, protected binaries/embedded assets, bundled
Sample, original form and canonical Access data remain unchanged.
No Access runtime, default promotion, source callback/Refresh, ProjectID write,
metadata lifecycle, full Unicode collation or complete FS1333 replacement is claimed.

## Private source-action storage

The normal source callback was re-read directly: PlotType captures an Integer,
maps1..5 to Ground/Visual/Note/FS882/Other through a String, assigns the field,
restores the captured option and calls Refresh. Its NULL Else is not widened
into an accepted clear path. Species-list-complete maps1/2/Else through Variant
to true/false/NULL with no Refresh. Form_Current invokes only the separate
display setters. UsysEnv remains the original DISTINCTROW Env/Admin inner join;
clsFormInfo ProjectIdSource remains an independent preference property.

[Private action storage](../siviparentactionwrite.go) uses the existing
[source planner](../siviparentactions.go) without changing its allowlist.
The accepted writer/restorer transaction is shared through a private
history-domain configuration, not a new public generic writer. Direct history
still validates only its fourteen fields. Action history is separately stored
in __VPRO_SIVIParentActionHistory and replans only the two resolved normal source
controls. Raw original/committed pairs, typed audits, aliases, ownership,
selected SU, complete project effects and replay checks are unchanged.

The descriptive SourceRefreshRequired bit identifies a planned normal PlotType
callback (even an explicitly invoked no-op). No Access callback, UI Refresh,
implicit other-draft commit, display audit or form promotion is implemented by
this bit. Source action persistence is an explicit audited desktop transaction,
not proof of the native callback's timing or full lifecycle.

Focused coupled race160.585s and independent read-only review pass. All15 source
choice pairs are tested on local/external fixtures, alongside action subsets/
strengths0-3, historical2 restoration, direct-history isolation, NULL semantics,
retain/prune replay rejection, malformed/wrong-domain history, pre-cancellation,
no-op, complete-original drift and atomic second-audit/BLOB failure/retry.
Unaudited NULL deletion is retained during restoration rather than restored
from the original snapshot. Full integration race768.342s/all packages passes.
Source and receipts are sealed in the
[action storage checkpoint](../archives/sivi-parent-action-storage-checkpoint/evidence-manifest.json).
That private checkpoint does not claim mounted behavior. The successor below
adds it separately; the642-file native predecessor and protected delivery/source
are not replaced.

## Mounted normal source actions

The [public action service](../siviparentactionservice.go) adds strict owned
original/Save/Restore APIs with explicit nullable `option` transport. Unknown
domains/properties, omitted/null required originals/actions, malformed Unicode,
fractional/string options and foreign identities are rejected. All three
independent backend flags (parent review, direct editing, action editing) must
be literal true; frontend flags separately control presentation. Default is off.

The [action session](../frontend/src/siviParentActionWriteSession.ts) reuses
the accepted scoped writer lifecycle. The transport includes only the two
resolved source control identities/physical targets; direct requests still
exclude callbacks. Resolved Save retires drafts/history before checking both
the common acknowledgement and exact source Refresh directive. A malformed/
mismatched directive blocks explicit recovery and cannot replay Save.

Normal Plot Type preserves Grnd/Visual/Note/Full/Other captions with explanatory
Ground/FS882 values. Spp List preserves Comp./Part. and explicitly nullable
selection. Retain original/initialization never stages normalization or audits;
there is no NULL PlotType control. This is not the CHARS Variant handler.
One live control replaces PlotType's original read-only slot; the implicit Spp
List control sits with Vegetation, without a second bound textbox.

Both persistent owners participate in shared Save/Undo/Lock/close/context
guards. Another scope's dirty/invalid/blocked state prevents staging or Undo
through the wrong owner. After commit/recovery, clean peer originals reload
without discarding their typed restoration handle. Native coupled events prove
the direct scalar history survives action Save and action prune, then independently
prunes its own cell. Action restore preserves unaudited direct changes/history.
This audited Save + owned reload is a desktop adaptation, not literal
AfterUpdate/Me.Refresh timing and not an implicit save of other drafts.

Focused SIVI/FS1333 race169.690s, full race770.171s/all packages,353 frontend
tests/check0/0, isolated builds and independent review pass. Actual Wails checks
visible associated labels/options at1400px/600px, zero initialization writes,
remount/Lock/close/mutual ownership, coupled Save/prune, distinct species-only NULL
Save and remounted retain (Restore=-1), replay denial, actual action read
cancellation/retry and default-mounted/backend-off denial. All16 fixture hashes
restore; every owned process exits normally. A navigation-helper stop after
completed coupled restoration required only the unfinished WRITE02 tail under
a fresh context; no successful mutation was repeated.

Source, candidates, typed whole-project/audit/history evidence and cleanup are
sealed in the [mounted action checkpoint](../archives/sivi-parent-action-mounted-checkpoint/evidence-manifest.json).
Full Unicode collation, ProjectID assignment/metadata creation and complete
FS1333 remain unavailable. Protected defaults/assets/data/exports are unchanged;
no Access execution, commit, push or default promotion.

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

Immutable common-field sealing is pending. Remaining work is complete source
presentation of the existing96/98 XL fields,
common-field/event/child integration and the picture workflow. Existing accepted
standalone SIVI evidence remains immutable. Neither the additional-field panel
nor its native proof establishes complete two-page forms/reports parity.

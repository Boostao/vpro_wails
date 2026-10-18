# VPRO delivery and migration plan

**Current workflow status and work order, not an execution chronology.**
Application acceptance baseline: `39c03f34bbab0c74762504f58ce504d34382a164`.
The subsequent `3bf43c4` changes source-status documentation only.
The migration is incomplete; no overall parity percentage is asserted.

## Destination and scope

Deliver one responsive Go/Wails/Svelte desktop that a user can open, navigate,
enter data in and produce the required reports from, with canonical SQLite.
Preserve source data, bindings, labels, meaningful grouping and child links;
adapt presentation rather than reproducing Access pixel coordinates.

The [client scope](docs/CLIENT_SCOPE.md) remains authoritative for retained
requirements and unresolved choices. UI-free R automation, cloud/BECWeb and
wider publication/analysis remain separate visible tracks. Nothing is excluded
merely because it is inconvenient, crossed in an ambiguous screenshot or gated.

## Current readiness

These states describe availability, not confidence percentages:

- **Core:** available in the ordinary experimental build.
- **Preview:** available in the pinned Windows preview below; current and
  explicitly inherited binary acceptance are distinguished.
- **Development:** implemented/verified slices require explicit opt-ins and may
  be absent from that preview. A private kernel is not an end-user workflow.
- **Incomplete:** required workflow or original behavior remains missing,
  unqualified or blocked. A visible menu is not implementation evidence.

| User workflow | Current availability and verified boundary | Remaining completion condition |
| --- | --- | --- |
| Select existing data and browse plots | Core/Preview: original SQLite family, configuration, owned project/SU/hierarchy selection, filtering and context switching | Production onboarding/administration is separate; attachment is not conversion |
| Enter ordinary FS882 data | Core/Preview: 98 mapped writable parent fields; bounded soil/Other/vegetation editors, draft/Save/Undo/Lock/close behavior | Complete remaining source events, calculations and whole linked-form journeys |
| Enter SIVI/CHARS data | Preview: parent panels, mixed child Save/restore, Collected navigation/reopen and explicit child create/delete/restore/creation Undo journeys. Development: wider separately gated species/ID and cross-host combinations | Whole-form callback/navigation equivalence; child creation/deletion is not plot/parent creation |
| Use two-page entry | Preview: complete normal117/CHARS119 parents, linked Humus/Mineral creation/editing, demonstrated vegetation cover/Collected and exact existing-list Species coedits, guards and saved reopen; C/D empty memberships remain read-only. Development: broader Species/metadata/heights and host combinations have separate boundaries | Whole-form/picture integration, populated C/D and remaining field/callback journeys; manual/NotInList, implicit creation and soil deletion/restoration remain unavailable |
| Edit project metadata | Preview: existing explicit record selection/editing. Development: additional creation/template/restoration boundaries | Complete original Add/copy/reference behavior and distinguish project/master storage |
| Work with pictures | Preview: readonly Picture Manager and ordinary linked child, separate owned library/directories, explicit metadata/image selection/viewer. Development: independently gated metadata/lifecycle operations | Original Add/save/default/confirmation/Current timing, mutations and complete two-page/host integration |
| Run profiles, Find and transfers | Preview: explicit project-owned existing-rule Save, ordered readonly preview and revalidated Apply/Next/Previous/Clear navigation. Development: wider rule lifecycle, Find and owned-project Env/SU transfers | Original mapped callbacks/count/scratch/filter side effects, unsupported criteria, external-file mutations and administration |
| Produce priority reports | Preview: inherited Long Environment/Vegetation and Matrix; Summary None/display6/environment/ancillary plus effective common/guide-NULL references and numeric Constancy inspection/publication. Development: other summaries/options/workbooks | Remaining combined/formatted variants, populated guide/attribute data and options/user journeys; original callbacks/printing remain separate |
| Review locations and prepare map output | Preview: explicit saved decimal coordinates, DISTINCT Location review and direct Env/SU Google Earth/KML with duplicate memberships preserved | Preferences, DMS/single-plot callbacks, viewer/remote resources and broader map journeys remain separate |
| Produce labels | Preview: exact saved single-plot card and portable Letter PDF, runtime Arial/calibrated en-US date, guarded once-only export/read-only recovery and retained receipt | Physical printing, Print All, positions/stock, live-form callbacks, Unicode/multiline/overflow output and broader regional calibration |
| Create/import/manage projects | Incomplete: private Access reader/staging/template/DDL groundwork only | Safe end-user creation/import/conversion, merge/splinter/compare and recovery |
| Use the ribbon and auxiliary tools | Partial: some guarded routes and read-only hierarchy/log/location replacements exist | Remaining project/SU/hierarchy administration, standards, Help and auxiliary forms |
| Install/release | Qualified private Windows preview; fourteen default-profile release payloads verified in CI | Current-profile packaging acceptance, platform runtime checks, signing/distribution boundaries |

Details: [capability contracts](docs/FEATURE_CAPABILITIES.md),
[source contracts and receipts](docs/MIGRATION_EVIDENCE.md).
This matrix owns current status; detailed ledgers do not override it by saying
that a narrower preparation or acceptance gate is complete.

## Supported preview profile

The current supported private preview is
`VPRO-Demo-2026-10-18-pictures.zip`,17,588,619 bytes, SHA256
`cf11508f34a5af0aab0f1943f4115be07fefccecfbc372149362122579a7e775`.
Run its `Launch-Demo.exe` on disposable retained data.

Its embedded `package-manifest.json` is the exact profile authority:
56 frontend/46 runtime flags, packaged binary SHA256
`82464aaad993682dcd1e8e9489417ead0e465c9862ea2c342451066b1595f02d`,
source-input SHA256
`d027395dfb5fcce6060b0c57b173f70b80b2094b3e2c3eabb40cf8f5c85b2ea0`.
Do not reconstruct the profile from remembered flag aliases or silently add
new source features. `files[].sha256` owns application/seed hashes; `launcher`
and `walkthrough` pin those separate payloads. Predecessor lineage is explicitly
named, not a substitute for the packaged EXE identity.

Binary source is clean `a6f9bae`. Two existing frontend picture flags and one
backend reading flag extend this private profile; production app/backend/
bindings/defaults are unchanged. Launcher ownership adds exactly three synthetic
resources to the unchanged prior16 seeds: external five-column library and
independent child/manager PNGs. Initial JS1,092,673, down79.00% from5,202,547;
total5,426,894. EXE43,267,072 grows36,864 bytes versus profile54/45.
No new timing/binary-shrink claim.

Current native proof covers Picture Manager and ordinary linked child selection,
physical duplicate-ID/NULL/empty metadata, independent PNG bytes/hash/dimensions,
actual previews/viewers and fields/actions at980/1400. Explicit invalid directory/
filename/missing-file and foreign literal-parent errors pass. A held successful
image Cancel/remount rejects late population; fresh specific CancelClose precedes
correction/release. Reading-off reopen denies both backend APIs while normal117
entry remains available. Two owners close; all19 files/full project and library
snapshots remain exact; zero writes/audits/history.23 clients/launcher races/
readonly coupled race16.291s/check0/0/Vite/Go/exact26 embeds pass.
Independent read-only review accepts with no material findings; all23 ZIP hashes/
final prepare-only pass. Two retained pre-read harness failures continue on the
same owner; native is not first-pass. No new full-root race/Access oracle claim.
Picture mutations/original callbacks/registry discovery/two-page composition,
production import and complete migration remain unqualified.

The preserved54/45 profile ZIP, SHA256
`6034be31ab38ee2ed5dc593409a366fdb791ef3d5f3414b0e0a6775b747dcea5`,
owns one four-text rule8 Save/one history/zero source audits, unchanged389 audits/
counts/Description, explicit78-row lump, zero then P1/P2 matches/eight steps,
actual Apply/Next/Previous/Clear, invalid/held-response guards and saved source
authority reopen.29 clients/race74.402s/two-owner proof are inherited, not rerun.
Profile lifecycle/external grants/Save as SU/species-only/subvarieties and original
mapped callbacks/count/scratch/source zero-filter changes remain unavailable.

The preserved49/45 coordinate/KML ZIP, SHA256
`3e09db767075a2f194b542ca60ade4b22e075231140aa36174b4a248ba08f6de`,
owns one coordinate Save/two audits/one history, Location1 DISTINCT row versus
direct Env/SU2 pairs/KML2 placemarks, exact reopened XML/hash, held review/export/
receipt guards, six gate denials/four saved reopens and five closed owners.
Those mutations/publication were not repeated on56/46. KML readonly recovery,
formal schema/viewer, preferences/DMS/single-plot callbacks remain unqualified.

The preserved45/41 saved-label ZIP, SHA256
`35714f1c79ba444912d218b39fe33e9276fa75d4b19c5d2319cac4e0800235d4`,
owns one P1 PDF/five exact strings/Arial/Letter page, NULL/empty assignments/P3
refusal/retry, held-review Cancel and lost-response readonly recovery, retained
receipt, two closed owners/16 exact files/full snapshots. Its sorted-root and
saved-reopen cleanup failures/corrections remain preserved. Label publication was
not replayed here; physical Print/positions/stock/Print All, live staging and
Unicode/multiline/overflow remain off.

The preserved44/40 Species ZIP, SHA256
`6fc79320bc1ae16bae2e2bb345042dcc15611b12d1ec662fc95f3e132ce705af`,
owns Normal LF1->LF2/Cover1 and CHARS LF2->LF3/Cover5a, two Saves/four audits/two
histories, exact peers/metadata/refs, joint soil guards/recovery/alias/remount,
six denials/three saved reopens. Four owners close;16 files exact each. Synthetic
LF1..LF4 master/personal0/common-seven-cover and list-only scope remain bounded;
manual/alias/NotInList/personal creation, populated personal/duplicate/whole-Unicode,
populated C/D/heights/identity/lifecycle/implicit events remain excluded.
Those mutations were not replayed on54/45.

The preserved43/39 vegetation ZIP, SHA256
`5c3e9ece6accd6fffdfed392e8aae99fa6819d2a54a57276d5cc724525f7f57f`,
owns Normal Cover1/Collected and CHARS Cover5a, two Saves/three audits/two histories,
TEXT1 NULL/C/V, invalid100/narrowed100/overlength, recovery/remount/alias and
two saved gate-off reopens. C/D membership0 with13 read-only physical rows each.
Its combined124 startup cases and corrected bound-editor test keep their identity;
Collected was not replayed here. Three owners close;16 exact files per reopen.

The preserved41/37 reference-report ZIP, SHA256
`b31cc3038bf5742486a84dfc6bff3406da17a5769b7eff568f600555cbc94a5e`,
owns three common/guide-NULL/Constancy publications:39 independent vegetation rows/
names,278 typed cells including60 reviewed environmental cells, numeric Lifeforms
and full provenance. Environment retains kernel calibration; all selected guide
names are NULL,98 derived attribute references are not selected vegetation.
Constancy includes physical LF1 zero (frequency1.00/others0.75/mean1.56); None
drops zero stage entries but retains denominator4. Cancellation/close/ack guards,
references-off denial/ordinary review, two closed owners and exact15 DBs with
declared SU config change/16 files per reopen belong to that binary.
Its inspector/source-mode and reviewed walkthrough corrections remain preserved
without publication replay. These publications were not repeated on54/45.
Use a fresh launcher run for report examples after vegetation edits.

The preserved soil ZIP, SHA256
`4f3f6ed92a9d38cd02bc10e4eafd88dd1af867030a567cbef883f894d9f19fd5`,
owns two explicit creations with18/28 defaults, two Saves/six audits/four histories/
permanent IDs, exact peers, retained normal117/CHARS119 remounts/read-only recovery
and three gate-off reopens. Four owners close;15 other files exact after writes/
16 exact per reopen. These mutations were not replayed on54/45. Positive allocation
is an adaptation, not GenUniqueID compatibility; no implicit creation or soil
deletion/restoration.

The preserved Summary ZIP, SHA256
`507acfd1dc5ee943c261019a504839cb8287fd0bcfde63c8b3a49d9b9904437c`,
owns saved None14/display6/average1/show1, one combined XLSX publication,
108 checked cells (13 independently calculated scientific-name vegetation rows,
30 environmental cells matching the reviewed plan), four memberships and full
very-hidden provenance. Environmental calibration retains kernel lineage.
Duplicate P1 and physical zero-cover P3 remain. Absent master long names yield no
invented ReportSummary unit row. Both owners close;15 databases stay exact
with declared SU/config selection and16 files stay exact in the denial reopen.
Its wire/scientific-name inspector corrections continued without replay.
That no-reference publication was not repeated on54/45; the41/37 predecessor
separately qualifies the three reference-based outputs. Original callbacks and
full formatting/printing remain separate.

The preserved entry-performance ZIP, SHA256
`0de8422f3e7d24d73fe9155ba03b9aefebb3cf3694d3ba09a4e6eb043c9fb76e`,
owns normal117/CHARS119 layouts, a three-scope Save/three audits, saved reopening,
backend denials and20-edit native typing medians8.8/9.4ms. Its immutable reference
snapshots remain; session clean/dirty medians98.85/95.72ms become0.65/0.50ms on
the same6,040,765-byte wire. Those bounded timings were not repeated here.
Its complete-parent predecessor remains exact at SHA256
`d16b9c70c904bc0ad8ac1baf3fcc0384f3abff5f73c9ad9e011cb0ed67c8e214`.

The preserved31/28 startup ZIP, SHA256
`bad5cec8574d6dbdfbf9ceaf9d8157d5c4d856ea1e88176812c77a2d468cacfb`,
owns deferred-module Home/context/plot replacement, failure recovery, close while
loading, parent Save and held child Save/restore/reopen. Its pre-bind interval
remains predicate/compiler coverage, not a deterministically held native interval.

The seed/earlier opt-ins retain the qualified report profile. Its preserved
ZIP, SHA256 `f9d1d26c94b119165cd0bec0511a996933e1b33e7340fd917066cc5751cc9383`,
owns216 Environment values,13 Vegetation pivots, report controls at1400/760/420,
two title Saves/two XLSX publications/independent reopening and cancellation/
stale/no-replace guards. Those mutations were not rerun on this binary. Original
tables/other seed files remain exact; DemoReports retains P1/P1/P2/P3, NULL Strata
and missing names. Select **DemoReports** for Long and the new combined Summary
workbook; the inherited basic Summary inspection also retains original Report.

The first lazy candidate's busy guard was rejected by native cold-load navigation;
the correction distinguishes an unresolved import from editor initialization.
Its zero-write family and later harness-only navigation/selector/history stops
are preserved, not relabelled passing. Metadata review must close explicitly
before navigation. Final qualification supersedes build-time pending metadata.

The immutable23/22 predecessor remains preserved at SHA256
`fcf4104370f00f7305b46aa26c64b0d663f7c22a0089991fd2933a21a585d41b`.
Its parent/metadata, SIVI/CHARS, Summary/log/Matrix and child Save/restore,
Collected/reopen, create/delete/restore/creation-Undo receipts retain their exact
scope. Source and prior gates are retained, not every child mutation rerun on
the new54/45 binary. These are explicit desktop safety adaptations, not original
Access undelete, parent creation or whole-form callback parity.

The current source, that pinned preview and default CI release are distinct.
Seven-job release run39149407850 on `7f4dd23` validates default packaging,
fourteen attached payloads plus manifest and independent API checksums;
it does not qualify an updated preview or every platform's runtime.
This current-main preview deliberately does not enable every accepted opt-in.

## Ranked delivery backlog

Work on one delivery objective at a time. Reconcile existing implementation
before writing another service, state machine, flag or preparation layer.

| Priority | User outcome and bounded work | Completion evidence |
| --- | --- | --- |
| 1 - supported journeys verified | Preserve parent/metadata and child entry/recovery/lifecycle acceptance with its exact inherited23/22 scope. Source/gates are retained; parent Undo/navigation is rechecked on31/28. Consolidate only demonstrably equivalent behavior. Wider host/source combinations remain in priority4. | Exact physical NULL/history/audits, navigation/reopen and acknowledgement/close barriers are sealed; this is not every enabled mutation or whole-form parity |
| 2 - supported subset verified | Preserve Long/Matrix lineage and Summary None/display6/environment/ancillary plus common/guide-NULL and numeric Constancy publications. Other options/combined/formatted variants and printing remain separate. | Independent vegetation/names/membership, reviewed environment, reopened outputs and cancellation/stale/close/acknowledgement barriers sealed; environmental calibration retains kernel lineage |
| 3 - blocked prerequisites | Real-data onboarding/project administration needs production go-mdbtools reader integration, source schema/default/identity qualification and platform/licensing closure. New-project Description/default oracle retry is explicitly closed after two attempts; do not reopen it under another name. Reuse completed F1-F3 and continue independent deliverables. | Disposable complete import/new-project/reopen/recovery before authorization; private scalar staging is not production import and merge/splinter/compare remain separate |
| 4 | Preserve delivered linked-soil, vegetation/existing-list Species, standalone profile Save/preview/navigation and readonly Picture Manager/ordinary child; finish broader Species/field/populated-C/D, picture mutations/two-page integration, wider profile and ribbon workflows. Resolve genuine business choices without inventing callbacks or destructive defects. | Every retained action mapped to an accepted user workflow or an explicitly agreed exclusion |
| 5 | Preserve saved-coordinate Location/KML and saved-label portable PDF; complete required report variants/options, physical labels/printing and final distribution/platform acceptance. | Qualified outputs/devices/stock where required, platform runtime receipts and the exact published supported profile, not CI build success alone |

Priorities organize, not delete, the former thirteen deliverables:
stabilization; F1-F3 context/configuration; ownership/performance; FS882 parent;
SIVI parent/children; SIVI child lifecycle; two-page forms; pictures;
profiles/navigation/transfers; reports/locations; labels; interchange/project
preparation; final replacement acceptance. F1-F3 is complete: do not restart it.
Source contracts retain the detailed outstanding requirements.

## Consolidation and performance rules

- Organize Go implementation and white-box tests together by real domain/
  dependency boundaries. Move frontend test fixtures/helpers out of production
  source when the relevant imports/generators/runners are updated and verified.
- Do not merge source-specific policies, NULL distinctions or physical/application
  identities just to reduce files. Reuse transaction/transport/lifecycle behavior
  only when equivalence or a clearer invariant is demonstrated.
- Application complexity and test volume are different. Preserve independent
  expected results and explicit parity cases; no arbitrary LOC/helper quota.
- Measure startup, representative large-project reads, interaction and bundle/
  binary size on the exact supported profile. Existing bounded fast reads do
  not prove every performance problem is fixed. Current deferred-entry/report
  profile reduces initial transitive JS79.00%; measure startup and larger-project interactions
  separately rather than treating the payload budget as overall performance proof.
- Use named profiles to control combinations. Independent backend denials stay;
  consolidation is not permission to turn all experimental flags on.

## Work and acceptance policy

1. Define the user outcome, owned files, exclusions and checks before a nontrivial
   change. Keep implementation, validation and independent review separate.
2. Run focused tests first; frontend application changes also require tests,
   check and an isolated production build. Generate bindings with `-ts -i`.
3. Obtain independent read-only review for nontrivial application changes.
   Resolve findings and rerun affected checks before accepting the change.
4. Verify Wails bindings/lifecycle with disposable native data, not a browser.
   Validate complete committed data/audits/history and output files independently.
5. Run full Go race validation at coherent application integration boundaries,
   with the explicit Windows budget in the handoff; do not overlap suites.
   Documentation-only changes do not require an 80-minute application rerun.
6. Promote a feature to the supported preview only after its complete user
   journey passes there. A component, kernel or gated source commit is not
   delivery completion. Keep unavailable operations unavailable and visible.
7. Publish coherent authorized commits with explicit scope, normal hooks/signing
   and the required trailer. Never force/amend, discard unrelated work or upload
   private evidence. Checkpoints are handoffs, not the overall stopping condition.

Source investigations must unblock a named delivery outcome. Batch equivalent
cases, use static evidence first and stop broken oracle debugging after twenty
minutes or two attempts. Do not reopen exhausted probes under another label.

## Frozen blockers and verification baseline

- ProjectID completion: two attempts exhausted; no new-ID input/metadata creation
  or Save qualified. Do not widen existing physical-choice assignment.
- Two-page Picture Manager: original buttons lack OnClick bindings; do not invent
  a dormant callback. Existing picture operations are a different boundary.
- Soil deletion: partial6x4 source observation only. One Humus row was removed
  without observed confirmation; intended No was never dispatched. No replay,
  third attempt or destructive desktop grant; Mineral/CHARS/8x6 remain unknown.
- Original Excel recordset formatting, hierarchy diagram, blank public dispatch,
  profile suggestion/ProfileEnv and label-position source units retain their
  documented blockers. Strata/Lifeform Display2 has an original type mismatch.
- Current application baseline passed full race in5003.344s, affected client
  checks and independently reviewed/native-qualified soil creation. This is not
  all combinations/default-demo/whole-source parity.

Exact machine state, hashes and stopped owners:
[Windows handoff](WINDOWS_HANDOFF.md). Portable outcomes:
[migration evidence](docs/MIGRATION_EVIDENCE.md). These records remain intact.

## Memory and history

Start a fresh bounded implementation session from this plan, the handoff and the
relevant source contract, not hundreds of cumulative checkpoint narratives.
Update current status in place. Record technical receipts separately; do not
append another chronology to this work order.

The complete [pre-reset ledger](https://github.com/Boostao/vpro_wails/blob/3bf43c4454bae07bcd482c04f7d665341c0c065e/MIGRATION_PLAN.md)
and [pre-reset README](https://github.com/Boostao/vpro_wails/blob/3bf43c4454bae07bcd482c04f7d665341c0c065e/README.md)
remain immutable in Git. Historical states there are not current availability.

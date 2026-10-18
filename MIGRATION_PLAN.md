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
- **Preview:** exercised in the one pinned Windows preview below.
- **Development:** implemented/verified slices require explicit opt-ins and may
  be absent from that preview. A private kernel is not an end-user workflow.
- **Incomplete:** required workflow or original behavior remains missing,
  unqualified or blocked. A visible menu is not implementation evidence.

| User workflow | Current availability and verified boundary | Remaining completion condition |
| --- | --- | --- |
| Select existing data and browse plots | Core/Preview: original SQLite family, configuration, owned project/SU/hierarchy selection, filtering and context switching | Production onboarding/administration is separate; attachment is not conversion |
| Enter ordinary FS882 data | Core/Preview: 98 mapped writable parent fields; bounded soil/Other/vegetation editors, draft/Save/Undo/Lock/close behavior | Complete remaining source events, calculations and whole linked-form journeys |
| Enter SIVI/CHARS data | Preview: parent panels, mixed child Save/restore, Collected navigation/reopen and explicit child create/delete/restore/creation Undo journeys. Development: wider separately gated species/ID and cross-host combinations | Whole-form callback/navigation equivalence; child creation/deletion is not plot/parent creation |
| Use two-page entry | Preview: complete normal117/CHARS119 parent controls, three-scope Save, guards and saved reopen. Development: metadata, linked soil/vegetation editing and explicit soil creation have separate acceptance | Whole-form/picture integration, remaining callbacks; soil creation and wider child opt-ins are not enabled by parent promotion |
| Edit project metadata | Preview: existing explicit record selection/editing. Development: additional creation/template/restoration boundaries | Complete original Add/copy/reference behavior and distinguish project/master storage |
| Work with pictures | Development: owned reads/metadata and independently gated explicit lifecycle operations | Original Add/save/default/confirmation/Current timing and complete host integration |
| Run profiles, Find and transfers | Development: bounded rule lifecycle/results, explicit navigation, owned-project Env/SU transfers | Unsupported criteria, external-file mutations and administration |
| Produce priority reports | Preview: Long Environment/Vegetation selection, title Save/Undo, preview, reviewed XLSX publication and saved reopening; inherited Summary inspection/Matrix XLSX. Development: additional summaries, options, XLSX/KML | Complete required combined/formatted variants and remaining options/user journeys; printing remains separate |
| Produce labels | Development: saved preview and portable single-label PDF | Physical printing, Print All, positions/stock and broader regional calibration |
| Create/import/manage projects | Incomplete: private Access reader/staging/template/DDL groundwork only | Safe end-user creation/import/conversion, merge/splinter/compare and recovery |
| Use the ribbon and auxiliary tools | Partial: some guarded routes and read-only hierarchy/log/location replacements exist | Remaining project/SU/hierarchy administration, standards, Help and auxiliary forms |
| Install/release | Qualified private Windows preview; fourteen default-profile release payloads verified in CI | Current-profile packaging acceptance, platform runtime checks, signing/distribution boundaries |

Details: [capability contracts](docs/FEATURE_CAPABILITIES.md),
[source contracts and receipts](docs/MIGRATION_EVIDENCE.md).
This matrix owns current status; detailed ledgers do not override it by saying
that a narrower preparation or acceptance gate is complete.

## Supported preview profile

The current supported private preview is
`VPRO-Demo-2026-10-18-entry-performance.zip`,17,523,481 bytes, SHA256
`0de8422f3e7d24d73fe9155ba03b9aefebb3cf3694d3ba09a4e6eb043c9fb76e`.
Run its `Launch-Demo.exe` on disposable retained data.

Its embedded `package-manifest.json` is the exact profile authority:
34 frontend/30 runtime flags, packaged binary SHA256
`56b2a5fe6610899ea24cd39923c5d7e9c5ff49388d1521a200cd87f42fc9f5f6`,
source-input SHA256
`eec3672ad917fcc82a4ea0f9a33afc69b2cf7bcd42418b26ede9232d40ce8a79`.
Do not reconstruct the profile from remembered flag aliases or silently add
new source features. `files[].sha256` owns application/seed hashes; `launcher`
and `walkthrough` pin those separate payloads. Predecessor lineage is explicitly
named, not a substitute for the packaged EXE identity.

Application source is base `67afa71` plus the manifest-pinned reviewed
complete-entry immutable-snapshot delta and tests. The34/30 profile, launcher,
backend, seed and defaults are unchanged.
Initial transitive JS is929,255 bytes, down82.14% from5,202,547; total5,230,739.
The43,061,760-byte EXE is unchanged from the complete-parent preview.
Neither wall-clock startup nor binary shrink is inferred.

Current native acceptance measures normal117/CHARS119 unique labelled parent
fields at980/1400. Invalid drafts block Save/Lock/Home/variant/native close.
Fresh CancelClose acknowledgements precede Undo and held-response release.
One Save commits FieldNumber, SV_CanopyComposition and ProvinceStateTerritory
with three exact audits/one technical history and every physical peer exact.
CHARS refresh and ordinary P2/P1 navigation pass. Saved reopen plus independent
backend review/write denials each leave all sixteen closed files exact.
All four owners are freshly absent; no successful mutation was replayed.
The predecessor's transient Undo selector failure remains preserved and was
continued only after independent zero-write/readiness checks. This candidate's
native run completes without a harness failure.

Focused40 entry cases, three startup-budget cases, launcher race, check0/0,
production Vite/Go builds and exact26-pattern overlay discovery pass;1931 inputs,
21 protected assets and56 canonical input records are exact. Distinct read-only
review finds no significant issues and independently reruns all40 focused cases.
The current report smoke loads options/titles on original Report scope only;
no report preview/publication was replayed. No new full-root race claim.
Complete **parent** entry does not grant whole two-page callbacks/children,
pictures, Source4, identity, implicit creation or soil creation.

Validated detached reference snapshots are now deeply immutable and reused
between actual receipts, rather than cloned on every reactive view. Original/
draft copies, full physical references, NULL distinctions and all source guards
remain. On the same6,040,765-byte wire, median clean/dirty reads98.85/95.72ms
become0.65/0.50ms; a second sequential-process comparison also passes under10ms/
80%-reduction. Native20-edit input/flush medians are8.8/9.4ms normal/CHARS.
No initial-load/transport/SQL speedup is claimed. The prior complete-parent ZIP
remains exact at SHA256
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
and missing names. Select **DemoReports** for Long reports, **Report** for Summary.

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
the new34/30 binary. These are explicit desktop safety adaptations, not original
Access undelete, parent creation or whole-form callback parity.

The current source, that pinned preview and default CI release are distinct.
Seven-job release run38929085479 on `842ca946` validates default packaging;
it does not qualify an updated preview or every platform's runtime.
This current-main preview deliberately does not enable every accepted opt-in.

## Ranked delivery backlog

Work on one delivery objective at a time. Reconcile existing implementation
before writing another service, state machine, flag or preparation layer.

| Priority | User outcome and bounded work | Completion evidence |
| --- | --- | --- |
| 1 - supported journeys verified | Preserve parent/metadata and child entry/recovery/lifecycle acceptance with its exact inherited23/22 scope. Source/gates are retained; parent Undo/navigation is rechecked on31/28. Consolidate only demonstrably equivalent behavior. Wider host/source combinations remain in priority4. | Exact physical NULL/history/audits, navigation/reopen and acknowledgement/close barriers are sealed; this is not every enabled mutation or whole-form parity |
| 2 - supported subset verified | Long Environment/Vegetation title/preview/XLSX/reopen journeys now pass in the pinned profile; preserve inherited Summary/Matrix scope. Other options, combined/formatted variants and printing are not silently completed. | Independent values/membership, saved titles, exact reopened outputs and cancellation/stale/no-replace/close barriers are sealed |
| 3 - blocked prerequisites | Real-data onboarding/project administration needs production go-mdbtools reader integration, source schema/default/identity qualification and platform/licensing closure. New-project Description/default oracle retry is explicitly closed after two attempts; do not reopen it under another name. Reuse completed F1-F3 and continue independent deliverables. | Disposable complete import/new-project/reopen/recovery before authorization; private scalar staging is not production import and merge/splinter/compare remain separate |
| 4 | Finish remaining forms/pictures/profiles/ribbon workflows in client-priority order. Resolve genuine business choices without inventing callbacks or reproducing destructive defects. | Every retained action mapped to an accepted user workflow or an explicitly agreed exclusion |
| 5 | Complete required report variants/options, labels/printing and final distribution/platform acceptance. | Qualified outputs/devices/stock where required, platform runtime receipts and the exact published supported profile, not CI build success alone |

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
  not prove every performance problem is fixed. Deferred entry reduces initial
  transitive JS82.15%; measure wall-clock startup and larger-project interactions
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

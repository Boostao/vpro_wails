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
| Use two-page entry | Development: parent mixed Save, metadata editing, linked soil/vegetation editing and explicit soil creation accepted in qualified profiles | Whole-form/picture integration, remaining callbacks; not included automatically in the pinned preview |
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
`VPRO-Demo-2026-10-18-reports.zip`, 17,506,297 bytes, SHA256
`f9d1d26c94b119165cd0bec0511a996933e1b33e7340fd917066cc5751cc9383`.
Run its `Launch-Demo.exe` on disposable retained data.

Its embedded `package-manifest.json` is the exact profile authority:
31 frontend/28 runtime flags, packaged binary SHA256
`cb7fee2f0ebc666281b3aaec42d61a8c13f0358fed37208cf48c6fcc7a91f0f2`,
source-input SHA256
`0a70abd6fddbfce08ef382acbadffe138cf5c5cfcc82caba85e01c43ec9ed795`.
Do not reconstruct the profile from remembered flag aliases or silently add
new source features. `files[].sha256` owns application/seed hashes; `launcher`
and `walkthrough` pin those separate payloads. Predecessor lineage is explicitly
named, not a substitute for the packaged EXE identity.

Source `bc1490aa` uses existing services/components without new application code.
The private launcher adds six fixed runtime literals and matching independent
environment expectations; complete launcher race tests pass. Only a synthetic
DemoReports_SU table is added: P1 twice, P2 and P3 in DemoA. All17 original tables
and15 other seed files are exact. The original Report SU/default stays intact.
Select **DemoReports** for the Long reports, **Report** for existing Summary.

New-profile native acceptance verifies216 Environment values/72 canonical labels,
thirteen Vegetation rows with presence .5/mean1.5625 and pivots P1=2.5/P2=3.75,
retaining NULL Strata, missing names and the physical four-member denominator.
Report controls at1400/760/420 widths, title draft/Undo/two Saves, two once-only
XLSX publications/independent reopen, same-family saved reopening, core parent
Undo/P1-P2-P1, cancellation/remount, stale/no-replace and fresh native-close
acknowledgements pass. All business tables/audits remain exact; only two declared
titles change after explicit scope selection. Eight owners close normally.
All1930 inputs,21 protected assets and54 canonical inputs match.

Focused Go race,366 client cases, check0/0, isolated Vite/Go production builds and
exact overlay discovery pass. Backend full-race lineage is unchanged; no new
full-root race is claimed. Distinct read-only review finds no significant issues.
The43,027,968-byte EXE is89,600 bytes larger, not a size optimization.

Launcher/profile, empty-unit fixture and DTO/observer expectation failures are
preserved and explicitly resolved. The original empty-unit refusal remains;
only the new synthetic row was corrected. A missed chunked title-Save capture
was recovered from its committed UI receipt and exact closed YAML, never replayed.
The final manifest supersedes stale build-time pending/predecessor metadata.

The immutable23/22 predecessor remains preserved at SHA256
`fcf4104370f00f7305b46aa26c64b0d663f7c22a0089991fd2933a21a585d41b`.
Its parent/metadata, SIVI/CHARS, Summary/log/Matrix and child Save/restore,
Collected/reopen, create/delete/restore/creation-Undo receipts retain their exact
scope. Source and prior gates are retained, not every child mutation rerun on
the new31/28 binary. These are explicit desktop safety adaptations, not original
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
| 3 - next | Real-data onboarding and project administration. Integrate existing go-mdbtools boundaries instead of another Access parser; reuse the original SQLite family and metadata. | Disposable complete import/new-project/reopen/recovery journeys before production authorization; merge/splinter/compare remain separately accounted |
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
  not prove every performance problem is fixed; the initial JS bundle needs work.
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

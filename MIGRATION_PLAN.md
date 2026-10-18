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
| Enter SIVI/CHARS data | Preview/Development: parent editing and separately gated vegetation children, species/Collected/ID/lifecycle composition | Whole-form callback/navigation equivalence; child creation/deletion is not plot/parent creation |
| Use two-page entry | Development: parent mixed Save, metadata editing, linked soil/vegetation editing and explicit soil creation accepted in qualified profiles | Whole-form/picture integration, remaining callbacks; not included automatically in the pinned preview |
| Edit project metadata | Preview: existing explicit record selection/editing. Development: additional creation/template/restoration boundaries | Complete original Add/copy/reference behavior and distinguish project/master storage |
| Work with pictures | Development: owned reads/metadata and independently gated explicit lifecycle operations | Original Add/save/default/confirmation/Current timing and complete host integration |
| Run profiles, Find and transfers | Development: bounded rule lifecycle/results, explicit navigation, owned-project Env/SU transfers | Unsupported criteria, external-file mutations and administration |
| Produce priority reports | Preview subset: Summary inspection and Matrix XLSX. Development: supported Long Environment/Vegetation, summaries, reviewed XLSX/KML and saved options | Complete required combined/formatted variants and complete user report journeys; printing remains separate |
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
`VPRO-Demo-2026-10-18-current-main.zip`, 17,626,821 bytes, SHA256
`fcf4104370f00f7305b46aa26c64b0d663f7c22a0089991fd2933a21a585d41b`.
Run its `Launch-Demo.exe` on disposable retained data.

Its embedded `package-manifest.json` is the exact profile authority:
23 frontend/22 runtime flags, packaged binary SHA256
`d46f1485859b88a6e6e064b496d852688f91a3af0d2213628c2d7db2cad5ed4d`,
source-input SHA256
`cb56ba7414afdc7b961412d1cd3d5d9467312806b02689ece900553c9857e792`.
Do not reconstruct the profile from remembered flag aliases or silently add
new source features. Payload hashes come from `files[].sha256`;
`acceptedCandidateSHA256` identifies predecessor lineage, not this packaged EXE.
Source `58c4723` uses the previous profile, seed and launcher unchanged; the
earlier archive/evidence is preserved. All non-documentation source inputs match
the accepted full-race baseline. Focused client tests, check, isolated Vite/Go
production builds and exact embed discovery pass.

Actual packaged entry verifies Undo, parent/metadata Save/six audits, P1/P2/P1
and reopening the same saved run. Standalone SIVI77, CHARS71/eight panels and
metadata147 controls at1400/760/420 widths pass. All thirty Summary rows match,
including the saved slope; saved-log1070 rows/page50 and one actual reopened
Matrix workbook with leading-zero codes/hidden provenance pass. A stale pre-edit
Summary expectation is retained as a failed assertion, independently corrected
from physical membership; no completed Save/publication was replayed.
Three native owners close normally;1930 inputs,21 assets and49 canonical inputs
match. These are bounded user journeys, not every enabled child mutation.

The current source, that pinned preview and default CI release are distinct.
Seven-job release run38929085479 on `842ca946` validates default packaging;
it does not qualify an updated preview or every platform's runtime.
This current-main preview deliberately does not enable every accepted opt-in.

## Ranked delivery backlog

Work on one delivery objective at a time. Reconcile existing implementation
before writing another service, state machine, flag or preparation layer.

| Priority | User outcome and bounded work | Completion evidence |
| --- | --- | --- |
| 1 - next | Complete the coherent entry journey beyond the qualified parent/metadata path: supported child mutations, Save/Undo, navigation, reopen and recovery. Consolidate FS882/SIVI/two-page orchestration only where behavior is demonstrably equivalent. | Whole-journey acceptance in the supported profile, including NULL/history, competing drafts, cancellation/unknown outcomes and close barriers; missing source behavior stays explicit |
| 2 | Priority reports usable from that profile: select scope/options, preview, publish and reopen supported Long Environment/Vegetation/Summary outputs. | Independent expected values/membership, saved options, exact reopened files, cancellation/stale-approval refusal and visible unsupported variants |
| 3 | Real-data onboarding and project administration. Integrate existing go-mdbtools boundaries instead of another Access parser; reuse the original SQLite family and metadata. | Disposable complete import/new-project/reopen/recovery journeys before production authorization; merge/splinter/compare remain separately accounted |
| 4 | Finish remaining forms/pictures/profiles/ribbon workflows in client-priority order. Resolve genuine business choices without inventing callbacks or reproducing destructive defects. | Every retained action mapped to an accepted user workflow or an explicitly agreed exclusion |
| 5 | Complete required labels/printing and final distribution/platform acceptance. | Qualified devices/stock where required, platform runtime receipts and the exact published supported profile, not CI build success alone |

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

# VPRO migration plan

Current deliverables and availability, not an execution chronology.
[README](README.md) describes capabilities,
[client scope](docs/CLIENT_SCOPE.md) records the agreed tracks, and
[portable evidence](docs/MIGRATION_EVIDENCE.md) distinguishes bounded acceptance
from overall migration completion. The cumulative
[feature capability contract](docs/FEATURE_CAPABILITIES.md) records each
feature-sized revision's entry points, guards and exclusions.

## Target

- Go/Wails/Svelte desktop with SQLite canonical and useful offline workflows.
  Preserve source bindings, labels, logical grouping, child links, event paths,
  physical database families and descriptions. Exported geometry is evidence,
  not a fixed desktop pixel requirement.
- Planned UI-free R automation over related data. R is architectural precedent,
  not Access runtime parity evidence; a second Shiny data-entry UI is not the
  current target.
- Cloud synchronization, public BECMaster/BECWeb, publication products and
  analysis integration remain separate client scope tracks, not implicit desktop
  dependencies or silently excluded deliverables.
- Completion requires a usable, validated workflow or an explicitly agreed
  exclusion. Passing a bounded batch or matching all parent fields does not
  complete the work order.

## Ordered deliverables

| Order | Deliverable | Current boundary | Remaining completion condition |
| --- | --- | --- | --- |
| 1 | Stabilization, ordinary editors and soil | Bounded mapped editor baseline accepted | Preserve nullable metadata, historical values and existing lifecycle behavior |
| 2 | Database/context/configuration | F1-F3 original family, YAML migration, external paths/TEMP views and draft-safe switching complete | Reuse, do not restart the foundation; administration remains separate |
| 3 | Ownership/performance | Pooling, catalogue caching and cancellation implemented | Extract packages/typed catalogue seams only for a demonstrated next-workflow benefit |
| 4 | FS882 parent | 98 mapped fields native-verified writable | Keep source-event and variant gaps explicit |
| 5 | SIVI parent/children | Direct/shared fields, references, source actions, identity, covers/heights/Collected/species and standalone presentation accepted within independent gates | Complete remaining callbacks, calculations/navigation and combined FS1333/SIVI execution |
| 6 | SIVI lifecycle | Creation, physical-row deletion/typed restoration and historical creation Undo accepted with permanent reservations/read-only recovery | Preserve safety adaptations; no implicit Access destructive cleanup or default promotion |
| 7 | Two-page forms | Layouts, common/additional parent fields, reference approval and atomic mixed-field entry accepted | Complete linked-child editing, pictures and unsupported source callbacks, then whole-form acceptance |
| 8 | Pictures | Owned reading, manager/child preview and existing metadata editing accepted | Source-bound Add, allocation/selected-file authority, deletion and typed historical restoration |
| 9 | Profiles/navigation/transfers | Bounded profiles, ownership, Find/Enter and owned-project Env/SU transfers accepted | Remaining criterion semantics, separate-file and administration gaps |
| 10 | Priority reports/locations | Supported environment/vegetation/summary previews, reviewed XLSX/KML and saved preferences accepted | Remaining agreed variants/options and end-to-end report workflows |
| 11 | Label reports | Source-defined owned label preview accepted | Physical printing/Print All, regional date and printer calibration; no speculative enabling |
| 12 | Interchange/project preparation | Fixture-tested private Access reader/staging, table archive and project template/DDL boundaries | Client analysis format and production import/conversion/creator authorization remain separate |
| 13 | Final replacement acceptance | Ongoing | Reconcile every ordered deliverable with verified outcomes or agreed exclusions, then platform/package acceptance |

## Next forms/report work

1. Complete source-bound picture creation. The live manager Add action selects
   one file, stores the literal parent, filename and directory prefix, leaves ID
   to the source engine default and does not copy image bytes. Allocation is a
   separate desktop adaptation; saving a path does not authorize image reads.
2. Complete picture deletion and typed historical restoration with physical
   ownership, complete-row evidence, permanent ID reservations and explicit
   audit policy.
3. Close the remaining two-page/CHARS linked-child and combined FS1333/SIVI
   source-event/navigation/calculation gaps without weakening ordinary editors.
4. Finish remaining supported reports and printing where a safe source contract
   is available. Source4/Access Val calibration is unresolved and stopped;
   unavailable paths must remain disabled rather than be guessed.
5. Perform whole-workflow replacement acceptance and record agreed exclusions.

The private picture-creation draft begun after historical SIVI creation Undo is
not part of the accepted publication snapshot. Focused tests alone do not grant
a registered service, UI, selected-directory authority or native acceptance.

## Invariants and acceptance

- Preserve the original SQLite family, per-project physical names,
  `_table_metadata` descriptions, duplicate definitions and NULL/empty values.
  Derived catalogues never replace canonical data.
- Use immutable contexts, explicit file observations and owned transaction
  leases. Data and history/audits commit together. Test cancellation, collision,
  ownership replacement, rollback, retry and restoration aliases.
- Preserve Access BOOLEAN true=-1, UTF-16 bounds and unchanged historical
  invalid values. Reject malformed raw Unicode before decoder repair; do not
  silently trim, complete, recase or infer source defaults.
- Draft errors and unknown mutation requests survive remounts and block
  unsafe Save/Lock/close/owner changes. Known committed results survive refresh
  failures; recovery must not repeat the mutation.
- Separate source scope/physical validity/reference membership. Source action
  guards compare planned focus/value/context with independent observations.
- Define each implementation contract, owned files, exclusions and measurable
  checks before editing. Keep application complexity separate from test volume;
  helper extraction requires a demonstrated invariant, not a helper quota.
- Focused Go tests first; full `go test -race -timeout 60m ./...` at integration.
  Frontend changes require tests, check and build. Bindings/lifecycle require
  actual Wails with disposable data, not browser preview.
- Nontrivial changes require independent read-only review, resolved findings
  and rerun affected checks. Missing review is an unreviewed checkpoint, not
  acceptance. Native evidence is scoped to exact source/assets and recorded
  owners; completed-response cancellation is not SQL-in-flight proof.
- Continue to the next safe ready deliverable after a checkpoint. Stop for a
  user pause, concrete permission/safety blocker or a decision blocking all safe
  work, not for routine technical verification.

## Publication

Preserve private evidence, binaries, profiles and source copies locally. Commit
reproducible product source/tests/resources, matching bindings and directly
related contracts in coherent feature-sized revisions. Shared root/model/index
changes must match the services in each revision. Validate each intermediate
revision; old receipts are not proof of a differently composed snapshot.

Assemble publication in an isolated integration worktree when live work has
multiple owners. Preserve preceding commits and unfinished work, inspect exact
staged scope and provenance, and do not rewrite or force-push without explicit
authorization. Publish a review branch/PR rather than treating migration
progress as a production release.

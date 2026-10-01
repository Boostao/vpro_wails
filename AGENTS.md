# VPRO migration instructions

## Source and safety

- Build Go/Wails/Svelte with SQLite canonical. DuckDB remains optional.
- Read `C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64_forAI` Forms, Queries,
  Modules and Tables_Def first. Preserve bindings, labels, logical grouping,
  child links and event paths; retain exported geometry as evidence, not a fixed
  desktop pixel-layout requirement. Never rewrite canonical Access/data.
- Access is a selective runtime oracle on disposable copies, not the default
  way to rediscover static properties. R APIs are precedents, not parity proof.
- Keep unavailable workflows disabled. Distinguish mapped storage, implemented
  drafts and verified native behavior. Record adaptations and gaps in README.
- Access reading belongs to go-mdbtools; integrate fixture-tested import
  boundaries rather than duplicating it.
- Preserve the R SQLite database family, per-project physical names and
  `_table_metadata` as native table Description storage, not just a version flag.
  Frozen editor catalogues are read models, never substitutes for that family.
  Complete MIGRATION_PLAN.md foundation F1-F3 before expanding more editors:
  default-init/runtime YAML with explicit lossless JSON migration, SQLite-owned
  external-path contexts/TEMP views and draft-safe switching. R is architectural
  precedent, not parity proof; do not import obsolete paths or require DuckDB
  extensions for offline startup.

## Work organization

- Follow MIGRATION_PLAN.md; WINDOWS_HANDOFF.md holds only current machine state.
  Replace stale status instead of appending another chronology.
- One primary integrates. At most one independent delegated task runs alongside
  it, with exclusive file ownership and a compact contract. No nested agents or
  sibling broadcasts. One owner controls native Access.
- Batch equivalent fields by source properties/events; reuse proven shared
  behavior explicitly and probe differences. Keep exhaustive combinations in
  fast tests. Time-box broken oracle debugging to 20 minutes or two attempts.
- Keep one current baseline and candidate, one active fixture per workflow, and
  compact data/audit differences plus representative visuals. Preserve existing
  evidence; archive closed builds with hashes before any later deletion.
- Stop work on user pause. Cleanup/replanning is not write enablement or permission
  to launch another native suite.

## Validation and invariant checks

- Focused Go tests first, then `go test -race ./...` at integration. Frontend changes
  require tests, `npm run check` and `npm run build`. Bindings/lifecycle need Wails,
  not browser preview. Use disposable data/config/profile directories.
- Mutations and audits share transactions; test cancellation, collision,
  ownership, rollback, retry and restoration aliases. Reserve deleted child IDs.
- Normalize Access BOOLEAN reads explicitly; preserve true=-1 in storage/audit.
- Preserve unchanged historical invalid values by omitting unchanged assignments.
  Reject malformed/new overlength text and raw JSON Unicode before decoder repair;
  Access bounds are UTF-16 units. Do not silently trim, complete or change case.
- Draft errors must survive tab remounts and block Save/Lock/close; clear them on
  valid correction, Save or Undo with the appropriate editor/original identity.
- Reference NULL/empty metadata and duplicate definitions remain distinguishable.
  Availability, physical validity and membership are different decisions.
- Never inherit phantom option audits, rejected-save history commits, destructive
  restoration or implicit identity changes as desktop requirements.
- Source action guards compare planned focus/value/context with independent
  observations. Capture source state before fallible host COM reads; unknown
  dialogs stop. IsLoaded is not Form view, and helper comparison must preserve
  literals while accounting only for explicitly allowed VBA identifier recasing.
- Verify current PID/start/path and owned windows, not remembered IDs. Restore
  changed fixture code/bindings/settings and seal cleanup; retain unknowns honestly.

Detailed behavioral evidence is in docs/FS882-6x4XL-access-contract.md. Historical
forensic instructions and execution notes remain in the ignored maintenance
archive; consult them for a specific issue, not as a growing default prompt.

## Learnings

- Build SQLite file URIs from resolved absolute paths with proper URI escaping
  (`net/url` in Go; `Path.as_uri()` in Python). Raw paths containing `#` can drop
  `mode=ro` and accidentally create a truncated-name file; prove readonly behavior
  with quotes, spaces, Unicode and URI delimiters. Verify wrapped path labels
  for clipping as well as visibility before accepting same-name disambiguation.
- With yaml.v3, use a map alias (`type configValues = map[string]any`) when nested
  sections are asserted as `map[string]any`; a named map type can propagate into
  nested decoding and invalidate those assertions. Test real Windows config
  replacement with a handle that denies FILE_SHARE_DELETE, including rollback,
  staging cleanup and retry; first installation must not overwrite a concurrent
  target. Keep failed persistence from changing effective service preferences.
- FS882 presentation should preserve field relationships, labels and meaningful
  row/group order, not Access's tiny controls or absolute coordinates. Use the
  Shiny `mod_fs882_6x4.R` layout as a grouping precedent and the existing Svelte/
  Tailwind styling with responsive CSS Grid: related fields side-by-side when wide,
  then stacked with their labels when narrow. Keep one live control per field and
  unchanged validation/lifecycle behavior; widening the window alone does not fix
  fixed-size inputs. Do not enable unavailable workflows during a presentation pass.
- Native layout checks must use actual visibility (`checkVisibility()`), not just
  nonzero rectangles: descendants of closed `details` can retain measurable boxes.
  Table cells use their column headers and row-specific accessible names; ordinary
  fields need visible associated labels. After a tab remount, wait for reference
  loading and Save readiness before clicking, without resetting drafts or replaying
  completed writes. Keep routine guidance below the fields, never safety feedback.

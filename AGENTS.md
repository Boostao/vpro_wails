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
  Foundation F1-F3 is complete: reuse default-init/runtime YAML with explicit
  lossless JSON migration, SQLite-owned external-path contexts/TEMP views and
  draft-safe switching. Do not restart that infrastructure. R is architectural
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
- Continue autonomously until the migration/work order is complete, not until
  a fixed number of gates, batches or checkpoints is reached. Do not use a
  gate-batch task as the overall completion condition. Record each verified
  result, select the next safe ready backlog item and continue without another
  "resume". Checkpoints preserve evidence and context; they are not stopping
  points. Before declaring completion, reconcile all ordered deliverables with
  verified outcomes or explicitly agreed exclusions.
- Perform tests, independent review and disposable native acceptance yourself.
  Do not ask the user to verify routine technical results or approve each next
  step. For genuine unresolved client choices, preserve source behavior or keep
  the affected workflow unavailable, record the question and continue independent
  deliverables. Stop only for a user pause, a concrete safety/permission blocker
  or a decision that blocks all remaining safe work; never invent approval or parity.

## Complexity, review and commits

- Assess application core and tests separately. Test volume is not application
  complexity; retain explicit parity cases and independent expected results.
  Do not target a helper count or split functions merely to reduce line counts.
- Extract a shared guard/helper only for demonstrated equivalent behavior or a
  clearer invariant boundary. Search prior art first; keep operation-specific
  errors, cancellation, lease ownership, transaction cleanup and NULL/empty
  distinctions visible. Test helpers and their callers; avoid helpers that
  reproduce the implementation to calculate test expectations.
- Before a nontrivial implementation/refactoring batch, define its contract,
  owned files, exclusions, acceptance checks and completion condition. Track
  implementation, validation and independent review as separate dependent tasks.
- Independent read-only review is a default acceptance gate for nontrivial
  application changes, not contingent on another user request. Use one reviewer
  distinct from the implementer, with the exact diff/base and bounded contract;
  ask for correctness, source alignment and missing acceptance coverage.
  No nested agents, overlapping writers or native actions by the reviewer.
  Small documentation-only checks can be direct unless delegation is requested.
- Passing tests does not replace review. Resolve findings and rerun affected
  checks before acceptance; obtain a focused follow-up for substantive fixes.
  Record reviewer disposition, validation evidence and remaining unknowns.
  If review is unavailable, checkpoint honestly as unreviewed, not accepted.
- When a commit is authorized, inspect status and intended staged diff, separate
  app/test/docs from private evidence or generated artifacts, and use the existing
  commit convention and required co-author trailer. Stage explicit intended paths
  unless the invoked commit workflow requires otherwise; preserve unrelated work.
  Use non-interactive commands without bypassing hooks/signing, then verify the
  commit and worktree status. No amendment or push without explicit authorization.

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

- `GenUniqueID()` in a DAO TableDef field DefaultValue is an Access database-engine
  random Long ID expression, not a missing VBA module function. Read actual field/
  index metadata and the DAO DefaultValue contract before searching more modules.
  Preserve AutoIncr/default/index distinctions; this alone does not prove allocator
  compatibility, collision/retry/deleted-ID behavior or grant desktop creation.

- Svelte deep-reactive rows are proxies, not cloneable plain objects. Use
  `$state.snapshot` inside the guarded operation before serializing a reviewed
  row. Extracted-function tests with plain objects cannot prove this path;
  exercise an actual client-runtime proxy, in that runtime's object realm.

- SQLite BINARY collation alone does not establish literal text ownership:
  column affinity can still coerce numeric or leading-zero identities. For a
  literal source scope, require text storage and byte-exact comparison with
  `CAST(column AS BLOB)=CAST(? AS BLOB)`; test INTEGER/NUMERIC/REAL as well as
  TEXT/BLOB/untyped columns without changing legacy readers implicitly.

- Nullable/mutable application-ID editing must check parent and peer read DTOs,
  not just its writer. A legacy numeric-ID reader must never invent ID=0 for
  NULL; return explicit unavailable ordinary rows/actions/totals while independent
  physical-row views continue, and propagate unrelated read failures. Scope
  restoration acknowledgements to the actual audit policy: ID-only technical
  history requires a history ID but zero source audit pruning.

- Collision/restoration witnesses must include complete physical peer rows, not
  only rowid and a mutable ID. SQLite can reuse a deleted rowid: a replacement
  peer with the same ID but different Species/PlotNumber would pass a reduced
  witness. Test actual automatic rowid reuse and reject changed peer snapshots
  without pruning history; permit explicit retry only after exact source recovery.

- Trace active VBA control flow before planning callback effects. For example,
  `UpdateMetadataSppList` jumps unconditionally to `MyExit` before opening or
  editing metadata; its named purpose and dormant AddNew/Edit body are not
  write requirements. Preserve the no-op, not invented rows, columns or audits.

- Typed history changes are the audited subset, not necessarily the complete
  committed plan. At audit strength1, a NULL addition can commit unaudited while
  another changed field is audited. Record the full typed source plan separately
  when proving mixed provenance, but restore only the audited subset; never reject
  legitimate unaudited differences or implicitly restore them.

- SQLite can contain malformed UTF-8 even in TEXT columns (for example
  `CAST(X'FF' AS TEXT)`). A direct string scan must reject it before constructing
  typed reference metadata; a nonselectable diagnostic is not sufficient because
  later JSON serialization repairs the bytes. Keep valid empty/overlength text
  distinct from malformed source text, and test read-error correction/retry.

- Insert new top-level functions beside a unique adjacent declaration, not a
  function signature plus an ambiguous closing brace: a patch can match the
  first inner block and silently nest the insertion. Prefer a separate cohesive
  file when the boundary is unclear; formatting/compilation must check the actual
  scope before further implementation.

- Closed build archives containing Go overlays must be outside package discovery
  (an underscore-prefixed directory or an existing separate module). A copied
  `main.go` with relative embed paths is evidence, not a runnable application
  package. Preserve the archive/manifest bytes when moving it; verify `go list`
  and full-suite discovery rather than treating new archive setup errors as
  unrelated pre-existing failures.

- ACE/DAO fixed-decimal Format first converts DOUBLE to fifteen significant
  decimal digits, then rounds half-away; `math.Round(value*scale)` is not an
  equivalent helper. Probe adjacent DOUBLEs as well as exact-looking midpoints:
  source `1.005` becomes `1.01` at two decimals and the DOUBLE just below `0.15`
  becomes `0.2` at one decimal. Reuse the shared calibrated decimal formatter;
  source species criteria compare formatted values, not raw pre-format numbers.

- Workbook collision guards must match the XLSX library's own name equivalence.
  Excelize resolves sheets with `strings.EqualFold`; `strings.ToLower` keys miss
  Unicode pairs such as `S`/long-s and can silently reuse/overwrite an existing
  worksheet. Compare all allocated and reserved names with EqualFold and require
  zero-output refusal in tests.

- Client captions establish scope intent; unmarked screenshots are candidate
  retain, not approved as-is parity, and colour-only marks need a legend before
  excluding functionality. Help/manual explain domain intent, not current Access
  execution or new constraints on historical data. The current target is a
  Go/Wails desktop plus UI-free R automation; historical Shiny/cloud outcomes
  remain separate scope tracks in docs/CLIENT_SCOPE.md.

- After changing C sources nested under an amalgamating cgo file, force a fresh
  build (`go test -a -race ./...`); `-count=1` alone can reuse stale included C
  objects. Do not clear shared machine caches. MinGW CRT encoding tests must
  pass environment variables at child-process startup rather than assume Go
  `os.Setenv` changes are visible to C `getenv`.
- The Windows root race suite exceeds Go's default ten-minute runner timeout
  and can exceed 45 minutes. Use an explicit integration budget
  (`go test -race -timeout 60m ./...`) and retain timeout receipts as failed
  runner evidence, not accepted tests.
- Generate Wails TypeScript bindings with `-ts -i`, preserving the repository's
  interface-style nullable-array contracts; `-ts` alone generates classes and
  changes those contracts. Do not repair consumers to accommodate the wrong mode.
- Go 1.25 can reuse stale package-index embed patterns across different source
  overlays; `-a` does not repair discovery. For isolated overlay builds, use
  `GODEBUG=goindex=0` and verify `go list` reports the exact planned `EmbedPatterns`
  before compiling; never clear shared caches or replace accepted frontend assets.
- Shared preference editors must retain per-context drafts, draft validation and
  irreversible mutation receipts separately. Validation blocks Save/native close
  while keeping correction/Undo live; acknowledgement must not erase receipts.
  In Svelte input handlers, capture the current DOM value before validating rather
  than assuming `bind:value` has already updated its reactive variable.
- SQLite REAL storage can normalize binary64 negative zero even when the Go
  parameter preserves its sign. Lossless import staging must compare independently
  reopened tagged storage and real bits, refusing unsupported normalization rather
  than claiming scalar adaptation alone proves exact database preservation.
- PowerShell evidence JSON must contain projected primitives, not extended
  objects: use `Select-String ... | ForEach-Object { $_.Line }` before
  `ConvertTo-Json`. Serializing MatchInfo/PSPath objects can recursively include
  provider/assembly graphs and inflate a compact receipt into tens of megabytes.
- Windows Go path-based `os.Stat` can defer physical file-ID loading until
  `os.SameFile`, so it does not freeze identity across rename/replacement.
  Capture planned identities with an opened handle's `Stat`, surface stat/close
  errors, and test replacement before staging as well as after staging.
- Native formatting must check complete output capacity, including NUL, before
  trimming or copying a suffix. Test the actual bound-column path at the minimum
  buffer size and size-1, not only the Go bridge's large fixed buffer; truncated
  fractional timestamps must become sticky errors, never successful partial dates.
- Scoped operations already hold the context's operation read lease; pool borrows
  must reuse it rather than recursively taking the same RWMutex read lock, which
  can deadlock behind a queued switch writer. Only unscoped read borrows acquire
  their own lifetime lease; test release, queued switch and owned shutdown.
- Holding the SQLite owner's mutex blocks new pool borrows, not writes through
  already-borrowed pools: ordinary header/child transactions execute outside
  that mutex. Independent completed snapshots prove observation-time equality,
  not source equality at a later file link. Preserve committed publication
  results after postcommit drift/cleanup errors; never wrap publication in a
  snapshot helper that zeroes its result on transaction-cleanup errors.
- A persistent pinned SQLite coordinator must use caller-owned read snapshot
  cleanup (`beginReadSnapshot`): database/sql's automatic rollback on a cancelled
  transaction context can discard that connection and lose all attachments/TEMP
  views. Keep every query on the request context, check cancellation before
  commit and surface rollback errors. Verify in-flight cancellation followed
  by a successful read, not only cancellation before acquiring a lease.
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
  CSS visibility does not imply viewport intersection. After resizing, scroll the
  intended controls/image into view, assert viewport intersection and containment,
  then capture; use separate scrolled segments when vertical scrolling is intended.
  Require every planned selector to resolve; an empty list must not pass an
  `every`/`any` assertion vacuously. Measure interactive buttons/disclosures as
  well as data fields; measure disclosed fields only after actually opening them.
  Table cells use their column headers and row-specific accessible names; ordinary
  fields need visible associated labels. After a tab remount, wait for reference
  loading and Save readiness before clicking, without resetting drafts or replaying
  completed writes. Keep routine guidance below the fields, never safety feedback.
- Keep immutable reviewed DTOs in Svelte `$state.raw`, or explicitly snapshot them
  at a transport boundary before defensive `structuredClone`; cloning a reactive
  `$state` proxy fails in WebView2 even when plain-object unit tests pass.
- Long approval hashes and destination paths can expand an implicit CSS Grid
  track beyond its container. Use `minmax(0, 1fr)` and appropriate wrapping for
  report panels; `width: 100%` and border-box inputs alone do not fix that overflow.
- WebView2-hosted JavaScript confirmations can block CDP, including the mouse
  response that opened them. For example, observe the exact `Delete this
  SubOtherXL record?` message and OK/Cancel controls through the current owned
  window's UI Automation descendants, then invoke only that planned decision;
  unknown dialogs stop. Continue from the last sealed mutation checkpoint,
  never replay a successful create/save to recover a blocked harness.
- Raw-Unicode validation and Go JSON `DisallowUnknownFields` do not reject
  duplicate object properties. For a strict request, check tokenized exact keys
  and decoded-key collisions before struct decoding: `"title"` repeated or
  escaped as `"\u0074itle"` must not silently replace an earlier value.
- After awaited client-side result validation, recheck the request generation
  before assigning UI state. Large XML validation must yield so Cancel/remount/
  close guards can run; test a held validation yield in actual Wails, release it
  after remount and prove the old result cannot populate the new panel.
- Frontend mocks must match the production wire, not just their validator.
  Share a strict-Go round-trip fixture for boundaries such as metadata columns
  (`declaredType`, not `type`). Go tests can resolve test-only package helpers;
  also compile production and inspect binding-generation warnings.
- Large Wails requests use UTF-8 byte chunks; a native fetch guard that examines
  only string bodies can miss a successful Save. Reassemble chunks by their
  upload ID and hold the final actual response. If capture was missed, inspect
  owned physical history/data first and never replay the completed mutation.

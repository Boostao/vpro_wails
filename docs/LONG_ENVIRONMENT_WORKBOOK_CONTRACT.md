# Long Environment workbook contract

## Scope and original source

This is the separately default-off XLSX publication of the existing Long
Environment report, not a SQLite table dump, a Summary Environment report or
complete Access/Excel execution parity.

Read-only source evidence is in the local `VPro64_forAI` exports:

- `Modules/V7mdlReportsEnv.txt`: `EnvReport`, transposition, four title rows,
  worksheet naming, `SortSheets` and print setup.
- `Modules/V7mdlUtility.txt`: `SheetNameCheck`.
- `Forms/USysEnvReportOptions.txt`: the existing report/title event path.

The existing [owned report](../environmentreport.go) supplies 67 Env/Admin
fields and five heading rows, original labels and selected-SU memberships.
Its conflict, orphan and reference-name rules are unchanged. Profile navigation
does not narrow the selected SU. External selected-SU databases are supported.
None and dynamic hierarchy are unavailable.

## Workbook output

[Preparation](../environmentworkbook.go) uses Excelize v2.10.1, declared in the
existing Go module. Each original report unit has a worksheet:

1. Literal report title.
2. `Environment Table`.
3. `Site Unit - <original code>`.
4. Uniquely resolved original long name, or blank when missing.
5. The 72 source labels vertically, with plot observations in subsequent columns.

Original section headings remain grouped and styled. The first column and first
five rows are frozen. Useful widths replace Access's tiny controls/fonts.
Missing names and duplicate NULL candidates remain distinguishable in source
metadata; duplicate NULLs are not a conflicting name. Conflicting or unsupported
name candidates are refused, not resolved with an invented `First()`.

Source print settings are retained: portrait, no printed gridlines, page-count
right header, date right footer, .55-inch side margins, .5-inch other margins,
fit to one page tall and unconstrained width. The print-date token is dynamic
when a spreadsheet program prints; it does not change prepared file bytes.
Sheets sort by uppercase name. Go Unicode uppercase/ordinal comparison is an
explicit deterministic adaptation, not a claim of VBA locale/collation parity.
The review mapping remains in original report-unit order independently of
physical worksheet order.

Worksheet names follow source 31-UTF-16-unit truncation and replacement of
`: / \ [ ] * ?` with `-`; empty codes use the source zero-based `NoName<index>`.
Split-surrogate truncation, apostrophe edges, provenance-name conflicts and
case-insensitive collisions are explicit errors. There are no automatic suffixes,
case changes to source codes, overwritten sheets or omitted plots.

Text uses explicit string cells: formula-shaped values are not formulas.
NULL is blank in the visible sheet; empty text is an emitted text value.
Original typed values, NULL/empty distinctions, name candidates and diagnostics
are retained reversibly as chunked hexadecimal report JSON in the very-hidden
`_VPRO_Source` worksheet. This is preservation metadata, not access control or a
copy of every raw table. The approval separately covers all four raw physical
tables, including columns not displayed in the report.

Invalid XML text, malformed Unicode, text beyond 32767 UTF-16 units, more than
16384 columns, BLOB values and INTEGER values exceeding 15 decimal digits are
refused without repair. REAL cells use float64 numeric emission; spreadsheet
display/numeric conventions must not replace the retained typed source.
Fixed document timestamps make identical preparation byte-deterministic.

## Owned review and publication

[The service](../environmentworkbookservice.go) exposes `GetReview` and
`ExportReviewed`. Raw JSON requests use existing strict Unicode/shape guards.
The review includes owned context/paths, report, unit-to-sheet mapping, SHA256,
byte length and source approval. Publication requires that exact approval/title
and an explicit literal `.xlsx` destination.

The owned mutex remains held across fresh physical snapshots and the shared
[no-replace publisher](../artifactpublication.go). Original Env, Admin, selected
SU and MasterSiteUnitList are reread before final linking and after known
publication. Drift in an unreported column invalidates approval too.
Existing files are never replaced. No database, audit, scratch table or report
preference is written; Excel is neither launched nor automated.

Known receipts use the shared `published`, `published-with-errors` and
`not-published` classification. A cleanup/drift/cancellation error after final
publication retains the known file/hash and explicit do-not-replay feedback.
Lost or malformed transport acknowledgements are unknown, never no-file defaults.

[The panel](../frontend/src/LongEnvironmentReport.svelte) shares its existing
read generation, cancellation, title/preferences, navigation and native-close
guards. Approved review objects are immutable raw Svelte state, so defensive
cloning at the publication boundary does not encounter reactive proxies.
Publication cannot be cancelled and holds the barrier until acknowledgement.
Acknowledgement invalidates prepared approval; unknown errors survive remount.
Title edits invalidate review. Receipts and safety feedback stay above guidance.
The [shared publication session](../frontend/src/publicationSession.ts) also
preserves existing KML publication behavior.

Independent flags, both off by default:

- Presentation: `VITE_LONG_ENVIRONMENT_WORKBOOK=true`, within
  `VITE_LONG_ENVIRONMENT_REPORT=true`.
- Runtime: `VPRO_LONG_ENVIRONMENT_WORKBOOK=true`.

## Validation state

- Coupled workbook/publisher/KML Go race: 11.818s.
- Complete frontend: 569 tests (131 pretest + 438), check zero errors/warnings.
- Isolated enabled/build-off frontend and production Wails builds pass;
  protected production assets were not used as an output directory.
- Interface bindings: 300 packages, 26 services, 209 methods, 193 models.
- Twelve actual Wails cases across four normally closed acceptance owners:
  1400/600px visible labels/contained fields; reviewed file bytes/print settings;
  no-replace collision; held irreversible publication/navigation/native-close;
  lost acknowledgement/acknowledged unknown remount; independent build/runtime
  denial; held cancellable review/stale suppression/remount/retry.
- Two 47941-byte files independently match reviewed SHA256, 11 source sheets,
  51 plot observations, all 72 labels, print metadata and lossless typed report.
  The fixture's conflicting Summary SU is preserved; the UI explicitly selects
  the existing Sample SU. Only that disposable configuration selection changes,
  and original configuration plus all 16 fixture/protected hashes restore.
- Three earlier zero-output candidates are normally closed and hash-archived:
  initial narrow-layout checks and the reactive-proxy publication refusal.
  No publication was replayed during recovery. Harness tail corrections used
  observed cancellation/unknown-alert labels, not data changes.

Fixture: `evidence/private/native-environment-workbook`. Driver:
`<session>/files/environment-workbook-native.py`. Completed modes must not be
replayed. No live Wails/Access/Excel owner remains.

Independent read-only review found no significant issues. Full all-package Go
race passes with fresh root 1206.963s against 668 unchanged source identities.
The [immutable integration successor](MIGRATION_EVIDENCE.md#snapshot-environment-workbook-desktop-checkpoint)
verifies the accepted attribute predecessor, closed candidate hashes, final
restoration, output identities and every archived hash/size. Production promotion
and the wider forms/reports migration remain outside this bounded workflow.

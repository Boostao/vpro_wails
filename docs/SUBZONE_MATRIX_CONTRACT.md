# Subzone Matrix of Units

## Current boundary

The internal pure planner and private owned snapshot reader are independently
reviewed implementation prerequisites, not an enabled report. A private
streamed XLSX preparation draft passes focused workbook checks but still needs
independent review. There is no service, desktop entry or publication path.
The ribbon entry remains unavailable. Original callback, Excel formatting and
complete report parity are not claimed.

The planner lives in
[`internal/reports/subzonematrix`](../internal/reports/subzonematrix), separate
from Wails orchestration. Tests remain beside their Go package; compact
independent native expected data lives in its `testdata` directory.

## Source behavior

Canonical exports identify `Sb12OnAction` / `sb12btn03` / `SubZoneMatrix`.
The ribbon caption is **Subzone Matrix of Units**. The active implementation
reads the current hierarchy, not the site-unit table mentioned in the ribbon
description. Registry `CurrHierarchy` defaults to `None`.

The three dynamically created queries:

1. Select hierarchy `Level=11`, `Left(Name,10)` as Unit, `Mid(Name,11)` as SS,
   and Parent.
2. Drive a left join from preserved `USysSubzoneMatrixLabels.MyUnit` to Unit.
   Pivot unit is `Left(MyUnit,9)`. NULL Parent produces NULL Value; a nonnull
   missing parent does not erase SS.
3. Left join Parent to hierarchy ID, group by parent Name and pivot Unit,
   using **unordered `First(Value)`**.

`BGC unit` and `Len` do not participate. Duplicate label rows are not
definitions of additional output columns. No-label input yields zero rows
and only the Name field; a missing hierarchy with nonempty labels yields
one all-NULL row. NULL/empty pivot values produce the `<>` heading.

The source exports `qrySubzoneMatrix3` as Excel97 `.xls`, with field names.
It rotates the first row 90 degrees, autofits columns, centers/bolds the body,
freezes at B2 and clears A1. It reuses or creates Excel, opens the exported
file, makes Excel visible and saves. Those host/dialog actions are not
implemented by the pure planner.

## Controlled evidence and deliberate refusals

Eleven extracted-query scenarios executed in a fresh disposable ACE1033
database. Ordinary and duplicate-label results have independent explicit
expected matrices. The compact [source fixture](../internal/reports/subzonematrix/testdata/source.json)
retains complete synthetic inputs and native outputs.

In one ambiguous cell, native First selected `04`, not the insertion-first
`01`. The planner therefore refuses differing candidates, including NULL
versus text, rather than selecting a physical row or inventing a stable source
representative. Equivalent byte-identical candidates are safe to collapse.
Case/trailing-space aliases with differing displayed row or column spelling
also refuse; distinct pivots with the same displayed heading and a pivot
colliding with Name refuse.

An additional 996 probes exercise 95 printable ASCII characters at ten
positions, omission controls and 36 preserved-label variants. All 1658 native
equality pairs match case-insensitive ASCII comparison with trailing-space
equivalence. The [comparison fixture](../internal/reports/subzonematrix/testdata/comparison.json)
contains the independently read native pairs, not expectations calculated by
the planner. This is bounded calibration, not whole-Unicode or general locale
equivalence. Comparison keys outside printable ASCII refuse. Opaque SS suffixes
can retain valid Unicode because they are not comparison keys.

Rows and columns use explicit technical ASCII-key ordering, NULL first.
This is an adaptation, not original effective locale ordering.
NULL and empty data values remain distinct.

## Bounds and ownership

- At most 100,000 physical hierarchy rows and 100,000 physical labels.
- At most 64 MiB aggregate source Name/MyUnit UTF-8 bytes.
- At most 254 pivot columns and 1,000,000 pivot cells.
- Exceeding a bound returns an explicit error and no partial matrix.
- The planner checks cancellation throughout preparation/join work.
- Inputs are not mutated; returned pointers and slices do not alias inputs.
- Duplicate hierarchy IDs refuse instead of guessing a parent.
- Referenced parent names/groups are resolved once per parent ID, not rescanned
  per child. A 32-MiB parent with 99,999 equivalent children must produce its
  single cell within a five-second focused work budget.

These bounds are implementation limits, not newly inferred source validation
rules. No text is trimmed, recased or truncated in returned data.
Comparison normalization is confined to internal keys.

The [private reader](../subzonematrix.go) reuses the owned context lease and
caller-owned pinned snapshot. It reads the selected hierarchy and preserved
VPro64 labels, never substituting a project or SU. Complete tagged originals
retain unused BLOB, BOOLEAN -1, REAL and NULL/empty values. ID/Parent require
original signed32 INTEGER storage; Level requires signed16 INTEGER storage.
No scalar is coerced. Retired/None contexts and invalid storage/schema refuse;
correction and actual in-flight cancellation followed by pinned retry pass.

Reader preflight bounds include **all** ordinary physical columns:
1..64 columns per source, 100,000 rows per source, 2,000,000 combined original
cells and 64 MiB combined storage bytes. Shared `table_xinfo` validation rejects
generated/hidden columns and shadowed rowid aliases before `table_info` or
materialization. Exact thresholds and overbounds are tested, including unused
oversized STORED generated BLOBs.

Independent reviews resolved repeated-parent work amplification and omitted
generated-column accounting. The planner's fanout regression passes in 0.57s;
the corrected reader's focused race, vet and actual `production,sqliteonly`
compile pass. An initial validation driver's `sqlite_only` spelling did not
select that profile; its retained binary is default-profile evidence only.
Corrected discovery explicitly confirms the SQLite-only driver and excludes
the default driver. Neither review grants integration or desktop acceptance.

## Remaining acceptance

Before enabling a workflow, integrate the owned hierarchy/VPro64 snapshot
reader with explicit physical storage/schema/byte guards, complete originals,
context cancellation and source provenance. Reuse the existing XLSX text and
publication boundaries. The preparation draft uses `.xlsx`, literal string
cells, rotated headings, centered/bold/bottom body, B2 freeze and blank A1.
It deliberately uses 8..80-character heuristic widths and a 105-point header,
not native Excel AutoFit metrics. A hidden typed-matrix/scope/original-hash
sidecar distinguishes NULL/empty visible blanks; it is not a full source
database archive or restore format. No `.xls` or Excel-host parity is claimed.
Streamed worksheet data uses a private per-operation temporary directory.
Preparation explicitly removes it after closing Excelize and surfaces removal
failures, returning no workbook bytes even if serialization otherwise succeeded.
This compensates for the pinned library discarding stream close/removal errors.
Acceptance exercises exactly 1,000,000 pivot cells, an independently observed
spill above the library's 16 MiB threshold, cancellation after that spill and
actual Windows handles denying deletion. Stream size observations use opened
handle statistics; Windows directory entries can retain stale sizes while the
writer is open. Independent review accepted the cleanup fix and private
preparation only; all ten validation-input hashes, logs and the production
binary were verified. The next service batch extracts the same original-hash
calculation without changing its bytes and revalidates that dependency.
Add retained desktop request/close guards, binding
validation, independent review and disposable Wails acceptance. Do not infer
those outcomes from pure-package tests or extracted DAO queries.

## Reviewed service and desktop candidate

The service candidate uses its own literal `VPRO_SUBZONE_MATRIX_WORKBOOK`
backend flag. Independent review accepted the unregistered service boundary.
The next desktop candidate registers it with matching interface-mode bindings
and a separate literal `VITE_SUBZONE_MATRIX_WORKBOOK` frontend/menu flag.
Both remain default-off; registration alone does not grant native/integration
acceptance. CamelCase preview DTOs contain source
scope/hash, columns and at most 100 rows, with explicit NULL pointers and
non-NULL arrays. Preview paging reads/plans the original matrix but does not
serialize XLSX; workbook-invalid text can still be reviewed as literal source
text. Full XLSX review is a separate operation returning a 25-row first page,
exact workbook hash/length and approval hash.

Strict requests reject missing/NULL fields, unknown or duplicate exact/escaped
keys, malformed Unicode and invalid paging bounds. Approval binds context,
physical paths, hierarchy, complete tagged-original hash, rendered workbook
hash and a versioned format domain. Publication reuses the context operation
lease, owned mutex, caller-owned snapshots and existing no-overwrite publisher.
The snapshot body is extracted, not recursively invoked through another lease.
Independent complete-original observations before linking and after publication
detect drift at those observation times; they do not prove continuous source
immutability during the interval. A committed file keeps its irreversible
receipt after drift/cleanup/cancellation errors and must not be replayed.

Focused tests cover owned paging/NULL/literal text, exact 100-row boundaries,
DTO ownership, repeat-review determinism, actual Go JSON round-trip, file hash,
collision, complete unrendered-column drift, retired context, strict requests,
snapshot failure/cancellation/retry and retained postcommit outcomes. Service
validation and independent review pass. A shared actual-Go wire fixture is
consumed by Go strict decoding and frontend tests, including all three
publication statuses. Disposable generation normalizes only displayed context/
fixture path labels, not data or NULL/empty fields; workbook/approval hashes
are actual Go output and are not stable across different physical fixture paths.

The desktop candidate uses raw immutable DTO state, guarded paged reads and
separate full workbook review. Request generations reject late Cancel/remount
responses; changed complete source between pages requires a first-page reload.
Per-context publication sessions reuse the shared irreversible receipt owner,
survive remount, block context/native close until acknowledged and preserve
the outcome after acknowledgement. The root subscribes independently so
unmounting a panel cannot release a publication barrier.
Focused96 coupled frontend cases and check0/0 pass; isolated default/enabled
builds preserve accepted frontend assets. A first binding-generation attempt
used the system Go1.27 and emitted source-processing warnings; it is retained,
not accepted. Regeneration with the qualified Go1.25 on PATH and `-ts -i`
has zero warnings and changes exactly the service, model and index bindings.
Independent desktop review finds no significant issues and verifies all 1,814
source and 19 protected asset hashes before/after review. Actual Wails default,
frontend-off/backend-off and enabled modes pass on one disposable fixture.
The 31-row matrix preserves orphan NULL-name groups, empty suffixes, literal
formula-looking text, NULL/empty pivots and first/tail paging. UI XLSX output
matches reviewed hash/length and independently reopened worksheet XML; a
separate planned collision refuses without changing that completed output.
Publication receipts block navigation/close, survive acknowledgement/remount
and do not recreate replayable reviews. Held actual reads pass Cancel/remount,
late-result rejection and fresh retry. Both held-read and retained-receipt
native close requests require the exact cancellation acknowledgement before
releasing their barrier. At three widths, actual visibility, viewport
intersection and containment pass for every planned control, caption/header,
wrapped path/hash and associated destination label. The complete 16-file
family/configuration stays unchanged. Five native owners close normally.
All1,963 registered frontend executions pass. Seven existing root harnesses
explicitly seed/pass the new inactive publication peer and preserve their prior
guard expectations. Independent follow-up verifies exactly seven test-only and
five documentation-status deltas against the native-accepted candidate, with
all1,814 input hashes and19 protected assets checked before/after; no application
or binding change occurred.

Full default-profile `go test -race -timeout 120m ./...` passes exit0,
root4172.205s, with all1,814 inputs,2,439 sealed artifacts and package/embed
discovery unchanged. All five original native owners are freshly absent and the
source freeze is released. These counts describe the frozen candidate; this
documentation-only integration disposition follows that accepted receipt.
No original Excel-host formatting, callback or whole-report parity is granted
by these portable desktop results.

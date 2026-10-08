# Access reader and canonical-value boundary

## Scope and current status

Access reading belongs to go-mdbtools, not a second VPRO parser. The original
local checkout remains unchanged. A separate Windows candidate builds the
existing bundled MDBTools/fakeglib with real GNU libiconv; public MDB/ACCDB and
a byte-identical read-only VPRO service-pack fixture pass fresh C/race tests.
Unicode paths, malformed paths, truncation, corrupt row offsets and explicit
source/conversion failures have focused coverage.

Independent review closed two corrected defects: failed compressed Unicode
conversion left an unterminated buffer consumed by C-string callers, and
non-UTF-8 output encodings bypassed the Go API's UTF-8 string contract. A third
correction is also closed: decoded embedded U+0000 fails explicitly rather than
silently truncating at a C-string boundary. Fresh raw conversion and bound-caller
tests cover Jet3/Jet4 embedded/NUL-only text rejection, genuine empty strings and
ordinary terminators. Final focused1.490s, full public/VPRO-copy1.799s,
race19.274s and vet pass. Independent source/test inspection closes all three
findings; this is not a broader parser-safety or Linux-runtime claim. Input Jet3
charset selection is distinct from output encoding. Forced `-a` builds are
required after nested bundled C changes.

The next bounded memo/OLE correction reproduced corrupt declared-four-byte Jet3
chains returning partial `AB` with `Next()==true` and `Err()==nil`. Native
offset/chunk/length/cycle checks and sticky caller errors now reject incomplete
full values. Genuine NULL/empty/inline/multi-chunk values remain covered.
Fresh focused3.231s, full public/VPRO-copy5.706s, race22.664s and vet pass;
primary forced-C-build consumer1.467s still round-trips all932 complete native
cells through untyped and declared-affinity SQLite. Independent memo/direct-caller
review found no significant issues; it inspected all six changed files against
the receipt and did not rerun tests. The prior reader seal is not acceptance of
this correction; its separate62-file exact successor now verifies the16-file
complete-row predecessor and88-file reader baseline. Jet4 chains, legacy
flags and alternative `MDB_COPY_OLE` remain unverified. Existing 32 KiB memo
input and 32 MiB Go OLE limits are retained, not general source-size support.
No general corruption guarantee is claimed.
The Jet4 follow-up adds94 synthetic native/bound checks for4096-byte geometry,
UCS-2LE/compressed Unicode and exact OLE bytes. Empty/inline/multi-chunk values,
assembly before text conversion, malformed encodings and bad/cyclic/incomplete
chains pass exact expected-byte/error checks. Final focused2.418s/full6.728s/
race24.214s/vet and native consumer1.440s pass; independent review found no
significant issues and verified production source unchanged. Public ACCDB65
rows provided no reusable long-value header. This closes bounded synthetic
coverage only, not actual ACCDB chained-caller or source-VPRO parity; two focused
attempts are closed. No permission to retry closed Access probes or assume
legacy flag semantics follows.

The disposable external consumer uses a local module replacement and
process-local include/link/runtime dependency paths. No absolute replacement,
new reader dependency or import flag has been added to the VPRO module.
Linux runtime, broader corruption handling and LGPL/libiconv redistribution
remain unverified. No native Access/COM or production import has run.

## Implemented internal value adapter

`internal/accessimport.SQLiteValue` accepts only the documented go-mdbtools
native value types:

| Reader value | Canonical parameter |
|---|---|
| nil | SQLite NULL |
| bool | signed64 -1 for true, 0 for false |
| int64 | unchanged exact integer |
| float64 | unchanged finite real; no hemisphere/range inference |
| string | unchanged valid UTF-8 text, including empty/whitespace/NUL |
| []byte | detached binary, including non-NULL zero-length BLOB |
| time.Time | reader-neutral UTC date text without a timezone suffix |

The adapter can preserve an already valid Go string containing NUL. This does
not establish native-reader support: its C-string transport must explicitly
reject embedded NUL until every boundary becomes length-aware.

Malformed UTF-8, nonfinite reals, unknown numeric/object types, non-reader
timezone representations and unsupported calendar ranges fail explicitly
without a successful/partial value. Dates are not shifted into a new timezone.
Decimal/currency and GUID strings remain strings; no parsing, rounding,
trimming, case completion or Unicode repair is performed.

Focused race1.340s covers exact signed64 boundaries, real signed zero, detached
binary, historical literal text and neutral dates. Actual disposable SQLite
storage tests distinguish NULL/empty text/empty BLOB and measure true=-1,
not merely a boolean-shaped proxy. A fresh forced-build isolated native-reader
consumer race1.452s independently round-trips all932 Shippers/Products/Employees
fixture cells through actual SQLite storage, including eight native true values
stored as -1 and five NULL cells; its Access source hash is unchanged.
Fresh full application `go test -race -timeout 45m ./...` passes
(root821.469s, adapter1.446s, all packages). No earlier reader or DDL receipt is
substituted for validation of these additions.

This adapter authorizes no file or database mutation. It is not an Access
decoder, schema upgrader, table copier, ownership coordinator or public API.

## Private value-storage affinity plan

`internal/accessimport.PlanColumns` consumes ordered native-reader column
names/types. It retains their literal names and order, validates UTF-8 and NUL
boundaries, and rejects unknown types and SQLite ASCII-case collisions without
renaming or returning partial plans. Non-ASCII case pairs remain distinct, as in
SQLite; whitespace and punctuation are not trimmed or treated as SQL.

| Native type | Planned SQLite affinity |
|---|---|
| boolean, integer | INTEGER |
| real | REAL |
| decimal, datetime, text, guid | TEXT |
| binary | BLOB |

Decimal/currency strings deliberately use TEXT, not NUMERIC: preserving a Go
string parameter is insufficient if the destination silently converts it.
Dates remain reader-neutral text. Tests measure actual SQLite storage, including
signed64 limits, high-precision decimal text, numeric-looking GUID/text and empty
BLOB. The isolated reader consumer now verifies complete native rows against
these declared affinities as well as the untyped scalar staging cells; all932
cells, eight true=-1 values and five NULLs round-trip (race1.479s).
An initial consumer harness inserted a row before all parameters were collected;
it failed before successful typed insertion. The corrected complete-row check
passes; the first failure receipt is retained.

The plan generates no SQL and establishes no original constraints, lengths,
defaults, autonumber/identity policy, relationships, indexes or nullable metadata.
It is a private staging-value plan, not a canonical VPRO schema migration or
permission to overwrite existing tables. Cancellation before, during and at
completion returns no plan.
Full production integration passes (root823.064s, all packages); final focused
race1.344s separately covers the test-only cancellation cases added during that
run. Production planner code was unchanged between those validations.

## Complete native row boundary

`internal/accessimport.SQLiteRow` composes the column plan and scalar adapter.
It requires one source cell per observed column and exact native type
correspondence: BOOLEAN is a Go bool, integer an int64, real a float64,
decimal/text/GUID a string, datetime a time.Time and binary a byte slice.
Already-normalized boolean integers and numeric-looking text are not accepted
as substitutes for source types. NULL remains permitted because source
nullability/required constraints have not been observed.

The complete result is detached before any storage operation. A malformed
schema, missing/extra cell, mismatched type, invalid scalar or initial/in-flight/
final cancellation returns no row, even when earlier cells were valid.
Errors retain the column identity without changing literal values.
Focused race1.405s passes; the actual native consumer now uses this complete-row
adapter and verifies all932 cells against both untyped staging and planned typed
SQLite tables (race1.506s), including eight true=-1 values and five NULLs.
Fresh full application integration passes (root852.401s, adapter1.374s,
all packages). The complete-row source was unchanged throughout that run.

This does not transact a table, follow links, materialize metadata, publish
files, authorize destination overwrites or bypass project ownership.

## Required before an enabled importer

A separate Windows reader timestamp candidate has reproduced upstream
fractional-second loss and negative-date failure using disposable MDB/ACCDB
cells. Source binary64 days do not establish fixed millisecond precision:
day45000 has an approximately628.643ns representable step. The candidate prefers
whole seconds only when they reconstruct the source double, otherwise
reconstructible nanoseconds, and explicitly rejects unrepresentable/nonfinite
values. Negative OLE day/fraction aliases map to neutral UTC calendar values;
this does not preserve distinct raw aliases as separate `time.Time` identities.

Corrected native/public-Go focused MDB3.891s/ACCDB1.647s, clone full10.163s/
race28.134s/vet and primary932-cell SQLite consumer1.442s pass. Independent
review closed the native small-bind-buffer formatting defect with40 actual
direct/bound-column cases: fractional `.123456633` needs30 bytes including NUL,
canonical `.5` needs22, ordinary seconds20 and date-only11; size-1 explicitly
fails without truncated success. Exact synthetic native-to-SQLite integration
also passes1.450s: the reader's123456633ns becomes TEXT
`2023-03-15 00:00:00.123456633`, unchanged through all scalar/schema/complete-row
boundaries and real SQLite storage. Only one planned eight-byte cell of a
disposable public-MDB copy differs; canonical/public fixtures remain unchanged.
Exact archival acceptance is next. No Access runtime/display,
DATETIME Extended or Linux claim follows, and the production dependency remains
unchanged. The accepted earlier adapter cannot recover fractions already lost
by an uncorrected reader.

- Observe local versus linked tables without automatically following links.
- Inventory source names/types and retain missing/NULL/empty descriptions where
  the reader can distinguish them. Its current plain Description string is not
  proof of that distinction; do not silently complete metadata.
- Apply and verify the private value-storage affinity plan in an owned staging
  boundary. It does not authorize canonical VPRO schema reconstruction; original
  constraints and identity/storage policy require separate source evidence.
- Preserve physical rows and independently verify schema, storage, counts and
  representative values. Use the corrected native timestamp boundary only
  after its platform/source ownership prerequisites are met; unrepresentable
  subnanosecond values explicitly fail, rather than losing precision silently.
- Import only into an exclusively owned new/disposable destination with
  transactions, cancellation, rollback, retry, collision and publication tests.
  Errors from inventory, reading, conversion, close and validation must block
  publication; rejected attempts must not report success.
- Headless automation must not bypass desktop leases/audits or mutate a live
  desktop project. No cross-process ownership guarantee is implied by this
  isolated consumer.

Publication, canonical-data replacement, legacy conversion and the companion
R package remain separate unavailable deliverables.

## Static legacy conversion boundary

The exported `V7mdlImportVProVer07.ImportVPro07` is a version-specific conversion,
not a generic lossless table copy. It links seven source tables under temporary
`USysConv` aliases, creates a destination table set, and runs separate Env,
Admin, Audit, Humus, Mineral, Other and Veg INSERT statements. Admin is projected
from the old Env table, including `PlotNumber` to `Plot` and `StrataCoverTotal`;
the final Env assignment uses renamed SV fields. Metadata is linked, but this
routine has no Metadata INSERT. Earlier constructed Env SQL is overwritten
before execution; it is evidence, not a second executed statement.

Those dynamic VBA statements do not establish blanket eight-table conversion,
nullable metadata preservation or defaults/identity parity. The routine suppresses
warnings, has its error-handler activation commented out and reports success
after cleanup; transactional rollback and rejected-attempt success suppression
must be established independently, not inherited.

The separate exported `USysImportVtab` form routes its three import buttons to
`ImportEnvData`, `ImportVtabVeg` and `ImportSortingInst`, not this VPRO07 routine.
These distinct workflows remain disabled. Source table definitions, original
column relationships and each version's actual event/module path must be mapped
before canonical conversion. No Access application or source data was modified
to make these static observations.

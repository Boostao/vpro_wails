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

The separate additive reader `DB.TableDescriptions()` now observes catalog-local
`MSysObjects.LvProp` without loading linked physical targets. A complete parsed
supported sequence distinguishes recorded table-scope Description entries from
missing entries. Unavailable metadata stays unavailable: absent, NULL and
zero-length LvProp are not separated. Ordered duplicates, block/entry/name
positions, native type/flag bytes and detached raw value bytes are retained
before compatibility lookup; named column blocks do not leak into table
descriptions. Existing `Table.Description` compatibility is unchanged.

NULL state, DAO type semantics and authored/inherited/default definition origin
remain explicitly unknown. Zero raw bytes are neither NULL proof nor confirmed
empty-string semantics. This is not an entire lossless property envelope.
Malformed/truncated/unsupported native property sequences fail explicitly and
prevent partial successful inventory/scan; untested format variants remain gaps.

Focused1.302s, fresh clone full11.412s/race28.930s/vet and independent
inspection-only review pass. The reviewer verified11 candidate hashes and35
unchanged pre-hashed files but ran no runtime tests. Primary native consumer
1.544s independently checks nine public table observations, literal Shippers
description/type10/flags1/positions, unknown semantic states, compatibility and
detached bytes, alongside932 exact native-to-SQLite cells and123456633ns date
preservation. MDB/ACCDB/source-copy hashes remain unchanged. Actual linked-table
and unavailable-metadata fixtures, DAO NULL semantics and Linux are unverified.
The [78-file exact description successor](../archives/access-reader-description-checkpoint/evidence-manifest.json)
verifies its19-file application predecessor,76 timestamp evidence files and44
reader baseline files, with370 application Go hashes unchanged. No production
importer/dependency is added.

The additive `DB.NativeColumns(table)` candidate reuses the existing native
table-definition parser through an opt-in observation path. It preserves
native source-column-number order, decoded names including duplicates, native
type codes/MDBTools Access-backend labels and raw size/flag bytes before BOOL
normalization. These are observations, not DDL or constraints: size units,
required/nullability/defaults/identity and decimal precision/scale remain
unavailable. Native backend labels differ from legacy Scan affinity labels.
Unsupported/incomplete/malformed observations fail without partial success;
links are not followed. The bounded supported count is1..256 columns.
Fresh clone full16.623s/race34.040s/vet and primary exact three-column source-byte
consumer1.503s pass, alongside932 cells,9 descriptions and exact fractional
timestamps. All three fixtures remain unchanged. Independent review found no
significant issues, verified46 baseline/38 unchanged reader files and10 candidate
hashes, and ran focused native tests1.636s without skips. The bounded observation
boundary is sealed in the
[81-file exact successor](../archives/access-reader-columns-checkpoint/evidence-manifest.json),
verifying18 application predecessor/78 description evidence/46 reader baseline
files and372 unchanged application Go hashes. Arbitrary corrupted
formats and Linux remain unverified. No
production dependency/importer is enabled.

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
The [exact timestamp successor](../archives/access-reader-datetime-checkpoint/evidence-manifest.json)
retains76 files, verifies its22-file application predecessor and42 reader
baseline files from the62-file memo/18-file Jet4 seals, and confirms368 application
Go hashes unchanged. No Access runtime/display,
DATETIME Extended or Linux claim follows, and the production dependency remains
unchanged. The accepted earlier adapter cannot recover fractions already lost
by an uncorrected reader.

- Observe local versus linked tables without automatically following links.
- Inventory source names/types and use catalog-local observation provenance
  rather than the compatibility Description string as evidence. Preserve
  unavailable/unknown states; prove semantic NULL/empty and linked-object
  behavior before claiming those distinctions or silently completing metadata.
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

## Accepted private owned in-memory table staging

[`PrepareTable`](../internal/accessimport/staging.go) applies the accepted
column/complete-row adapters to a separately owned in-memory SQLite database.
It consumes the supplied native reader and closes it exactly once, including
invalid arguments, cancellation, row/type failures and budget rejection.
The isolated consumer adapts go-mdbtools' named `Row` type without decoding it
again; no reader dependency or absolute replacement enters the application.

Callers must supply positive row and explicit byte limits. The supported native
column count is1..256. The image budget is16384..2147483647 bytes, enforced by a
4096-byte SQLite page budget and the final serialized length. This bounds the
staging image, not total process memory or a fallible native `Next` call's runtime.
Source table/column names remain literal bound metadata, never executable SQL.
Internal `stage_rows` uses generated `c0..cN` columns and a staging-only physical
ordinal; `stage_columns` retains ordered literal names, reader types and planned
affinities. This avoids source `rowid`/`_rowid_`/`oid` collisions without repairing
names or pretending the generated ordinal is an original Access identity.

All schema/rows are created in one transaction. Source read and close must
succeed before its in-memory commit. The detached serialized SQLite image is
reopened independently and its source identity, ordered column mapping, actual
storage schema, physical row order/count and every tagged stored type/value are
checked against the complete adapted input. SHA256 includes integer/real bits,
NULL/text/BLOB tags, lengths and row boundaries; empty BLOB/text/NULL and duplicate
physical rows remain distinct. SQLite normalizes binary64 negative zero in REAL
storage, so that source value is explicitly refused rather than silently repaired.
The output SHA256 covers the exact serialized image, not source authorization.
Every read/close/rollback/storage/verification/cancellation error clears the whole
result. No image is returned before both owned SQLite connections are closed.

Focused adapter/staging race1.411s/vet passes, including exact one-row/16384-byte
and256-column thresholds, rejection just beyond bounds, cancellation/read/close
failure identities, retry, concurrent isolation and independently modified images.
The actual isolated go-mdbtools consumer race1.559s independently reads five images
from three unchanged fixtures: MDB932 cells/89 rows (eight true=-1/five NULL),
public ACCDB845 cells/65 rows and disposable VPRO service-pack ACCDB5 cells/one row.
This is1782 measured cells/155 physical rows, not a VPRO project import.
Independent review identified a final-hash cancellation gap. A post-hash context
check and deterministic cancellation-during-hash zero-result test correct it.
Independent focused closure race1.352s and exact five-file hashes confirm
resolution; the initial reviewer also ran uncached package race/vet and the actual
three-fixture consumer. Fresh all-package race integration passes
root954.209s/staging1.248s, with397 Go files unchanged throughout the run.
Five actual returned images are also captured once in exclusively owned session
evidence, separately from application preparation. Read-only Python SQLite
independently checks their exact hashes, integrity, schema, literal mappings,
155 ordered rows/1782 cells and the MDB eight true=-1/five NULL values; image and
source hashes remain unchanged. These harness evidence writes are not publication.
The [exact staging successor](../archives/access-import-staging-checkpoint/evidence-manifest.json)
preserves reviewed source, consumer, five actual images, independent image checks
and integration evidence while verifying the359-file preference predecessor,
accepted reader source/fixture identities and protected defaults.

The image is private staging, not a canonical schema, original metadata/default/
constraint/identity reconstruction or a client interchange-format decision.
The caller still owns source-file identity, local-versus-linked authorization and
coherent multi-table observation; this row interface cannot establish those facts.
There is no destination file, publication, active context, audit mutation, native
Access/COM action or production import flag.

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

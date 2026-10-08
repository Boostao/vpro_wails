# Private typed table CSV boundary

## Status and source

The private Go codec in [tablecsv.go](../tablecsv.go) prepares a versioned
roundtrip boundary for canonical SQLite tables and companion R automation.
An independently gated owned Wails review now exposes this in-memory document;
it is not file publication/import, a SQLite database reconstructor, an RDS writer,
TurboVeg support or an Access converter. No file or database is written by the
codec or review. Destination publication and companion-package integration
remain separate deliverables.

Client screenshot pages8-9 request consolidated table CSV/RDS and TurboVeg
interchange. Original `USysExportToR.btnExportTables_Click` calls
`V7mdlExportToR1.BuildRAll`: it writes analysis-oriented `.veg`, `.env` and
`.suh` products, performs vegetation grouping and replaces NULL/empty environment
values with periods. Those concatenated legacy records are not a lossless table
format and are not inherited as roundtrip requirements. Original table
definitions and `_table_metadata` remain authoritative. This boundary reuses
the existing physical tagged table/cell models and canonical cell validator;
it does not duplicate Access parsing or field-edit validation.

## Version1 shape

An in-memory document contains CSV bytes and a manifest with:

- `version`: exactly1.
- `table`: literal physical name; no inferred project, role or SQL selection.
- `columns`: ordered original names and declared types.
- `rowIds`: ordered, unique canonical signed64 decimal strings, never floats.
- `storage`: one complete ordered SQLite storage-tag array per physical row.
- `descriptions`: explicit physical Description candidates, each with `rowId`
  and a canonical tagged `value`. NULL, empty and duplicate values are retained.
  Absence is an explicit empty array, not a missing or NULL array.
- `sha256`: lowercase SHA256 of the exact CSV bytes. This detects accidental
  corruption; it is not authentication or permission to import.

The first CSV record contains the original column names. Remaining records
contain one value per column:

| Storage tag | CSV field after ordinary CSV parsing |
|---|---|
| `null` | Empty string |
| `text` | JSON string scalar, including its JSON quotes |
| `integer` | Exact signed64 decimal string |
| `real` | Finite float64 roundtrip decimal, including negative zero |
| `blob` | Exact lowercase hexadecimal bytes, including empty bytes |

Text uses JSON escaping so CRLF, LF, quotes, commas, NUL, whitespace and Unicode
are not repaired by CSV line-ending normalization. A reader must parse CSV
without numeric or missing-value inference, then decode text JSON scalars and
apply the manifest's storage tags. A NULL value is not the text `"NULL"`, a period,
an empty text value or an empty BLOB. Signed64 integers and physical IDs must not
pass through R doubles.

Single-column NULL and empty-BLOB records are emitted as quoted empty CSV fields,
not blank lines that CSV readers can skip. Empty tables retain the header and
explicit empty row/storage arrays. Column names containing carriage returns or
NUL are explicitly unsupported, not renamed. Invalid tagged cells, nonfinite
reals, malformed UTF-8 and unpaired JSON Unicode escapes are rejected.

Decoding verifies the checksum, header, physical identities, complete row/storage
cardinalities and trailing EOF. Errors and cancellation return zero output, not
partial success. Caller-owned schema, descriptions, input cells and returned
rows do not share mutable pointers across the codec boundary.

## Validation and limits

[Tests](../tablecsv_test.go) cover independent SQLite physical rows, exact signed64
limits and IDs beyond2^53, NULL/empty/BLOB distinctions, CRLF/NUL/Unicode text,
negative real zero, duplicate nullable descriptions, empty tables, single-column
empty records, corruption/extra/missing rows, malformed Unicode and cancellation
before and during processing. Focused race passes1.185s. Independent review
found the single-column blank-record defect; it was fixed, regression-tested and
the blocking finding closed. Full integration `go test -race -timeout 20m ./...`
passes (root883.751s, all packages). That immutable kernel receipt does not
validate the subsequent owned reader, facade or frontend.

This format is a private migration preparation, not client approval of a final
plain-value CSV format. Indexes, defaults, constraints, triggers and writable
import policy are not reconstructed from column declared types. No scope,
membership, identity mutation or field-validation permission follows from a
valid document. RDS, legacy analysis exports and TurboVeg require their own
source/format contracts.

## Owned native review successor

[GetProjectTableCSVReview](../tablecsvservice.go) uses the existing context
operation lease and pinned read-only SQLite snapshot. Scope is exactly one of
the current project's eight physical core tables (Admin, Audit, Env, Humus,
Metadata, Mineral, Other, Veg), not a selected SU, current plot or profile.
Support tables, views, differently cased names and arbitrary SQL are rejected.
Table values and physical Description candidates share the same read snapshot.
The separate `descriptionMetadataPresent` flag distinguishes an absent physical
metadata table from a present table with no matching rows; NULL, empty and
duplicate candidates remain distinct. Ownership, cancellation or cleanup errors
return no review, rather than a partial/success-shaped result.

Backend `VPRO_TABLE_CSV_REVIEW` accepts only literal `true`/`false`, defaults off
and does not inherit any SIVI flag. Frontend `VITE_TABLE_CSV_REVIEW=true` separately
enables navigation. Actual generated bindings carry the complete typed manifest
and UTF-8 CSV string. [Transport validation](../frontend/src/projectTableCSV.ts)
detaches input before asynchronous checksum work, verifies the planned context,
project/path/table, complete typed shape, CSV grammar/values and exact SHA256,
then returns a narrowed snapshot. Cancellation and tab/context teardown invalidate
both request and checksum completion. No download/import/publication control exists.

Focused owned/facade race2.083s, all376 frontend tests, check0 errors/warnings,
independent review with no significant defects and isolated enabled/default
production Wails builds pass. Sixteen native cases across four fresh owners
verify default denial, independent backend-off refusal, all eight physical
tables against independent SQLite/CSV observations, actual WebView SHA256,
visible labels at1400/600px, nullable Description details, busy navigation/native
close, cancellation during checksum, stale remount suppression and retry.
Existing FS882 Cancel/Discard retain or remove the appropriate draft; invalid
input disables Save and continue. Successful pre-existing writes were not replayed.
All16 fixture file hashes restore after each owned close; protected defaults and
canonical/source data remain unchanged. Native setup initially rejected duplicate
metadata under the original `table_name` PRIMARY KEY and rolled back; that
constraint was preserved. Native NULL/empty/escaped descriptions use separate
original rows; duplicate provenance remains covered by the fast fixture tests.
Two read-only harness retries corrected expected transport keys and readiness,
not application code or business writes.

Fresh full Go integration for this successor passes (root826.279s, all packages);
883.751s remains the kernel predecessor. No default promotion or full
export/import acceptance follows from native review acceptance.

## Access reader platform

Access reading remains the responsibility of `meztez/go-mdbtools`. The existing
local404b4116 reader remains unchanged. A separate disposable Windows candidate
now builds bundled MDBTools/fakeglib with local GNU libiconv and passes public
MDB/ACCDB, copied VPRO, Unicode-path, corruption and conversion tests. Independent
review closed three defects; embedded source-text NUL is explicitly rejected.
The [native scalar boundary](ACCESS_IMPORT_CONTRACT.md) and isolated consumer
preserve Access true=-1 without introducing an application reader dependency,
duplicate parser, source rewrite or elevated machine installation.
Access import stays disabled until a supported,
fixture-tested reader boundary is available.

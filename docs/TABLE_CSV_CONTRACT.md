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

## Private single-artifact preparation

A private in-memory container is implemented for the existing version1 CSV
document. Two loose files cannot be published atomically as one operation;
the preparation uses a conventional ZIP Store container with exact `table.csv`
and `manifest.json` members so a future owned no-replace writer can publish one
complete artifact. This is an implementation adaptation, not client approval
of ZIP delivery, a plain-value CSV/RDS replacement or enabled publication.

The container must preserve exact CSV bytes and every supplied manifest field,
reject ambiguous members/JSON, verify integrity and accept an explicit positive
caller byte budget. No compression, extraction to disk, inferred metadata or
database reconstruction is authorized. Existing description candidates do not
prove the physical metadata table exists: the owned review's separate presence
flag and full original schema are still outside a generic version1 document.
The private `encodeTableCSVBundle` and `decodeTableCSVBundle` validate the
existing codec's document, snapshot all caller-owned bytes/manifest slices and
Description pointers before fallible context checks, and return no partial
result on invalid input or cancellation. The decoder takes an explicit positive
byte budget and rejects unsupported or ambiguous members, headers, integrity,
JSON properties and raw Unicode before repair. Existing table identities,
storage tags, literal text and description candidates are not inferred or
renamed. These APIs still authorize no file/database write.

Final corrected bundle plus predecessor codec race5.278s and package vet pass.
Regression tests independently inspect ZIP bytes, enforce exact budget versus
size-1, preserve deterministic roundtrips, cover malformed/ambiguous members/
manifest/CRC and mutate caller aliases at each context checkpoint. Independent
review is closed; full integration passes root833.343s/all packages.
This is not accepted publication.

Bundle review identified and closed a metadata correction: JSON
`declaredType:null` was silently decoded as the valid empty declared type.
Raw validation now rejects null/non-string values before typed decoding while
explicit `""` remains valid and roundtrip-preserved. Final focused race5.278s
and vet pass; the5.096s receipt is the retained pre-correction state. Independent
reviewer inspects only the fix/direct tests and reports zero unresolved findings.
Reviewer runtime tests lacked the proper CGO environment; supplied implementation
receipts are the runtime evidence. Fresh full integration passes root833.343s/
all packages with unchanged bundle source. No file publication or final client-
format approval is claimed.

The [exact private bundle checkpoint](../archives/table-csv-bundle-checkpoint/evidence-manifest.json)
retains16 source/review/validation files, verifies its18-file predecessor and
records364 Go integration hashes. It does not enable publication.

A separate private fresh-path publisher uses the existing atomic no-replace
hard-link pattern. It stages one validated artifact in an observed existing
parent, verifies current owned identities/bytes, preserves collision targets
and unowned aliases, and surfaces cancellation/cleanup errors.
Successful linking is the commit boundary: later errors report committed
status without deleting the published target or encouraging replay. Tests are
disposable; this helper alone establishes no project lease, source-reobservation,
cross-process data coordination, desktop action or client output-format approval.
The private writer draft passes eight test groups, coupled CSV/bundle race5.567s
and vet. Two Windows symlink cases are privilege-skipped, not runtime-verified;
requested0600 does not establish Windows POSIX-like owner-only ACLs.
Independent review closed a Windows identity gap: path `os.Stat` deferred
physical file-ID acquisition and could accept a replacement parent introduced
during encoding before staging. Parent acceptance now freezes identity from an
opened directory handle, propagates stat/close errors and checks that identity
before stage creation. A real Windows regression reproduces the old failure
and now rejects replacement while preserving original/unowned contents.
Final nine publication groups/coupled race5.672s and vet pass; reviewer reports
zero unresolved findings and independently runs replacement regressions with
Windows race1.199s. Fresh full integration passes root828.253s/all packages;
source remained unchanged throughout. No public publication is claimed.
Path observations remain neither a hostile handle-relative filesystem guarantee
nor a project/source ownership coordinator.
The [accepted publication checkpoint](../archives/table-csv-publication-checkpoint/evidence-manifest.json)
retains27 exact files, verifies its16-file bundle predecessor and records366
Go integration hashes.

## Private source-owned publication candidate

The unexported `publishOwnedProjectTableCSV` composes the existing owned review
and no-replace publisher. It detaches the expected review before fallible
callbacks, checks literal context/project/path/table identities, and compares
the complete document plus the separate physical metadata-presence bit.
Independent read transactions finish before staging, immediately before the
publisher's final artifact/filesystem checks, and after committed publication.
Read/transaction-cleanup errors before linking block publication; observable
source drift or cleanup errors after linking preserve `Published`, path and
hash with an explicit do-not-replay error. No business rows or audits are written.

The context operation lease pins the selection. Holding the source owner's
mutex serializes owner-mutex writers and new pool borrows, but ordinary
header/child writers can already hold borrowed pools and execute transactions
outside that mutex. External connections are likewise not excluded. The
guarantee is an equal, independently fresh, completed prelink snapshot, not
source equality at the instantaneous hard link or detection of every transient
change. Postcommit observation does not turn this into cross-process atomicity.
The generic version1 artifact still omits physical metadata-table presence and
full original schema constraints; comparing that presence during orchestration
does not add it to the bundle.

Candidate focused CSV/owned-publication/bundle race passes12.798s with237
cases/subcases; package vet passes. Independent inspection-only review reports
zero significant findings and verifies the baseline diffs, callback ordering,
alias detachment and committed-result preservation; reviewer runtime tests
were not executed. Fresh full integration passes root849.089s/all packages
against368 captured Go source hashes. Candidate receipts under
`archives/table-csv-owned-publication-checkpoint` are unsealed implementation
evidence, not acceptance. The helper remains private/unwired: no feature flag,
public method, desktop/native export or final ZIP/CSV/RDS approval is added.
The reviewed private source-owned successor is ready for exact archival
acceptance; its source-observation limitations remain unchanged.

## Access reader platform

Access reading remains the responsibility of `meztez/go-mdbtools`. The existing
local404b4116 reader remains unchanged. A separate disposable Windows candidate
now builds bundled MDBTools/fakeglib with local GNU libiconv and passes public
MDB/ACCDB, copied VPRO, Unicode-path, corruption and conversion tests. Independent
review closed three defects; embedded source-text NUL is explicitly rejected.
The [native scalar boundary](ACCESS_IMPORT_CONTRACT.md) and isolated consumer
preserve Access true=-1 without introducing an application reader dependency,
duplicate parser, source rewrite or elevated machine installation.
Access import stays disabled until complete owned staging/schema/metadata and
transactional publication boundaries are verified; tested reader/value kernels
do not authorize a conversion or canonical-data replacement.

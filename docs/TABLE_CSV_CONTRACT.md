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

The [exact private bundle checkpoint](MIGRATION_EVIDENCE.md#snapshot-table-csv-bundle-checkpoint)
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
The [accepted publication checkpoint](MIGRATION_EVIDENCE.md#snapshot-table-csv-publication-checkpoint)
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
The [reviewed private source-owned successor](MIGRATION_EVIDENCE.md#snapshot-table-csv-owned-publication-reviewed-checkpoint)
retains22 exact files, verifies all27 publication-predecessor files and records368
unchanged integration source hashes. Its source-observation limitations remain
unchanged.

## Private owned archive successor

[Owned archive preparation](../tablecsvownedarchive.go) preserves the physical
Description metadata-presence bit inside the artifact, rather than only comparing
it while publishing. ZIP Store still has exactly `table.csv` and `manifest.json`.
The latter is now a separately identified envelope (the document body below
is abbreviated, not a decodable fixture):

```json
{
  "format": "vpro-owned-table-csv",
  "version": 1,
  "descriptionMetadataPresent": false,
  "document": { "...": "the unchanged complete version1 table manifest" }
}
```

The envelope's version is independent of the table document's version. Metadata
absence and present-but-empty produce different bytes; NULL, empty and duplicate
physical candidates remain in the complete nested document. Absent metadata with
nonempty candidates is rejected, never repaired. The boolean is required and
must be a literal JSON boolean; missing, NULL and inferred defaults are invalid.
This does not capture the entire metadata table, its constraints, or database
indexes/defaults/triggers. It is not a SQLite database reconstructor.

The unchanged generic version1 bundle and this owned envelope deliberately reject
each other's manifests; no historical artifact is silently upgraded or assigned
metadata-presence authority. Both reuse the same deterministic ZIP writer,
strict bounded member/header/CRC checks and raw Unicode/object-shape validation.
Owned publication reuses the existing context lease, fresh source observations,
checked read cleanup and no-replace single-artifact writer. Its existing
already-borrowed/external-writer limitations remain unchanged.

ZIP delivery is a provisional engineering choice while the user is unavailable,
not client approval of an ordinary analysis CSV/RDS format. This successor is
private: no desktop binding, feature flag, download/import control or native
export behavior is enabled. Final focused legacy/shared/owned race12.453s and
package vet pass. Tests independently inspect the envelope and cover absent/
present-empty/nullable duplicate states, malformed and missing properties,
Unicode, integrity/budget refusal, every cancellation checkpoint, detached
aliases, collisions, source drift, clean retry and preserved postcommit warnings.
Fresh full Go race integration passes root962.800s/all packages against408
pinned Go files and192 unchanged frontend/dependency identities. Primary boundary
inspection and tests are recorded; no new independent-review or native-export
claim is made. The
[private owned archive checkpoint](MIGRATION_EVIDENCE.md#snapshot-table-csv-owned-archive-checkpoint)
retains exact code, source and receipts and verifies the39-file scope predecessor.
The next vertical is separately gated desktop review/approval, destination
picker and irreversible publication receipt/lifecycle handling.

## Private source-bound approval facade

[Archive approval preparation](../tablecsvarchiveservice.go) provides a private,
unregistered facade for that desktop successor. Its independent literal
`VPRO_TABLE_CSV_ARCHIVE_EXPORT` parser defaults off and does not inherit the
existing read-only review flag. The service is not registered in the application;
there are no new Wails bindings or UI/native export controls.

`GetTableCSVArchiveReview` accepts exactly `{"table":"literal core table"}`.
It returns the complete owned table review plus the separate source approval,
archive SHA256, archive byte count, explicit format and envelope version.
The domain-separated approval hash binds the complete detached owned review:
context/project/path, original ordered schema, physical row IDs/storage tags,
exact CSV bytes and nullable/duplicate Description candidates plus physical
metadata presence. Identical CSV is not sufficient approval when metadata or
ownership changes. This checksum is neither authentication nor import permission.

`ExportReviewedTableCSVArchive` requires exactly `table`, `approvalHash` and
`destination` strings. Invalid UTF-8, raw unpaired JSON Unicode, missing/NULL
properties, duplicate/escaped aliases, case aliases and extra fields are rejected
before decoder repair. Identities and destination are explicit nonempty Unicode
without NUL; nothing is trimmed, case-normalized or inferred. Publication first
reads the owned source again and compares the exact lowercase64-hex approval,
then uses the already-tested owned archive writer and fresh pre/postlink
observations. A new review is required after observable source/schema/metadata
drift. A stale context or arbitrary/support/view table never grants publication.

Errors before entering publication remain explicit request errors. Once the
publisher returns, its outcome is a typed `published`, `published-with-errors`
or `not-published` receipt retaining requested destination, observed canonical
path, artifact SHA256 and error text. A cleanup/drift error after linking must not
become an ordinary RPC failure or lose the committed artifact. Requested paths
can retain Windows short-name spelling while observed paths use the canonical
long-name spelling. No implicit extension, replacement, launch or preference
save is performed.

The prior KML strict request decoder and publication-receipt logic are extracted
as shared helpers with their existing behavior and public DTOs preserved.
Focused coupled race16.047s and package vet pass, covering all eight physical
core tables, raw ownership/schema/metadata/row drift, stale and malformed
approvals, every hash cancellation checkpoint, detached aliases, collision,
read cleanup refusal/retry, real committed cleanup warning and existing KML
regressions. An initial test incorrectly compared canonical long paths to
requested Windows short paths; corrected tests independently resolve the actual
path while checking the literal request separately. A misplaced test-function
insertion was corrected before the passing receipt. Both failed receipts remain
retained; no application failure or native acceptance is inferred from them.
Fresh full Go race integration passes root982.703s/all packages against410
pinned Go files/192 unchanged frontend identities and the35-file owned archive
predecessor. The
[private approval checkpoint](MIGRATION_EVIDENCE.md#snapshot-table-csv-archive-approval-checkpoint)
retains exact code/source/failed and passing receipts. This is primary-tested
private preparation, not independent-review or native desktop acceptance.

## Independently gated desktop migration archive

The registered [archive service](../tablecsvarchiveservice.go) now has actual
generated Wails bindings and an independently default-off desktop surface in
[the table review panel](../frontend/src/ProjectTableCSVReview.svelte).
Backend `VPRO_TABLE_CSV_ARCHIVE_EXPORT` accepts only literal `true`/`false`.
Frontend export controls require exactly `VITE_TABLE_CSV_ARCHIVE_EXPORT=true`;
the existing table-review navigation also requires `VITE_TABLE_CSV_REVIEW=true`.
The archive backend does not inherit `VPRO_TABLE_CSV_REVIEW`: native proof
confirms archive review with that original backend flag off. Production defaults
are not promoted.

Explicit preparation displays the exact whole physical table, schema, value CSV,
metadata presence/candidates, source approval and archive hash/size/format.
Frontend validation detaches before asynchronous CSV checksum validation and
checks planned context/project/path/table identities. It does not claim to
independently derive the server's source-approval or ZIP hash. A literal
destination textarea and optional native `Dialogs.SaveFile` chooser are separate
from the reviewed Export action; neither typing nor choosing a file publishes.
Picker cancellation preserves the prior literal destination, creates no file
and releases its close/navigation barrier. Picker errors remain explicit.
There is no extension completion, silent text repair, replacement or viewer
launch. This is a migration archive, not an ordinary analysis CSV or RDS product.

[Persistent publication sessions](../frontend/src/tableCSVArchiveExport.ts)
retain every known and unknown attempt across acknowledgement, reload, tab
remount and subsequent attempts. Unknown transport after invoked publication
does not invent committed/changed booleans. Acknowledgement releases the barrier
and invalidates approval without erasing evidence or automatically replaying a
write. Publication is not cancellable or tracked as a read; held writes and
unacknowledged outcomes block navigation/native close. Literal owned identity
checks prevent receipt reassignment to another project/path.

Primary frontend validation passes438 tests (10 pretest plus428 runner cases),
check0 errors/warnings and isolated opt-in/default-off builds. Vite retains its
existing large-chunk warning; no build error or protected-dist replacement.
Actual production Wails builds use verified exclusive embed overlays. Twenty
native cases across four fresh normally closed owners verify:

- Independent frontend-default-off and backend-off gates and actual bindings.
- Exact owned Env review against independent physical SQLite/CSV observations.
- One actually visible destination control and associated label at1400/600px.
- Cancellation of the actual owned standard Windows SaveFile picker.
- Exact two-member ZIP bytes/manifest/hash/size at the explicit destination.
- Collision refusal with the original artifact unchanged.
- Noncancellable held responses after real commits, navigation/native-close
  barriers, known warnings and unknown transport outcomes with historical
  receipts retained across acknowledgement/remount; known receipts also survive
  subsequent native attempts. Fast session tests cover unknown receipts through
  subsequent attempts without another native write.
- Existing FS882 valid/invalid draft Cancel/Discard/Undo and disabled invalid
  Save behavior, without replaying a successful business write.

Four planned export files are retained. The native warning/unknown cases
instrument only delivery of a real committed response; actual source-cleanup
warnings are separately exercised by Go facade tests, not claimed as native
Access faults. All16 source/config fixture hashes restore after every close;
no SQLite/audit/config/canonical/registry or protected-default writes occur.
Fresh full desktop Go race integration passes root931.623s/all packages against
410 pinned Go files and195 frontend/dependency identities. The
[desktop archive checkpoint](MIGRATION_EVIDENCE.md#snapshot-table-csv-archive-desktop-checkpoint)
retains exact builds, four outputs, native/source/validation receipts and verifies
the36-file approval predecessor. Primary/native verification and frontend
implementation-agent tests are recorded; no separate independent-review claim
is made. No import/RDS/TurboVeg, database
reconstruction or final client plain-analysis format approval follows.

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

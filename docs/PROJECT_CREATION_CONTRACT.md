# New-project source boundary

## Status

Source-mapped preparation and a private owned read-only template observer for
project administration, not an enabled new-project writer or an Access runtime-
parity claim. Existing context ownership, YAML and file-publication helpers are
precedents to reuse; they are not permission to overwrite a project or infer
template defaults.

Client screenshot page5 identifies project administration as a priority.
Original ribbon `V7mdlRibbonOnAction.Sb03OnAction`, control `sb03btn01`, and
`frmMainMenuFloat.cmbCurrProject_Change`, choice `New`, both call
`V7mdlCreateTables.CreateTableSet`. Historical version converters also call it;
they do not authorize those converters or legacy imports in the desktop.

## Original behavior and adaptations required

`CreateTableSet` asks for a literal name and destination database, checks an
existing project `_Env` table in both current and destination databases, and
copies eight source tables. It does not copy Sample data or create SU, hierarchy,
profile, theme or herbarium tables implicitly.

| Destination suffix | Original physical template | Retained SQLite columns | Indexes |
|---|---|---:|---:|
| Env | USysEnvTable |112|3|
| Veg | USysVegTable |44|3|
| Other | USysOtherTable |11|1|
| Humus | USysSoilHumusTable |18|1|
| Mineral | USysSoilMineralTable |28|1|
| Audit | USysAudit |11|1|
| Metadata | USysMetadataTable |75|4|
| Admin | USysAdminTable |20|1|

All eight retained templates are physically empty. Env.PlotNumber and Admin.Plot
have unique indexes. Every source index and its uniqueness must survive explicit
destination-name mapping; creating only columns is incomplete. Source SQL
exports contain Access-specific omitted defaults and `GenUniqueID()` expressions.
They are evidence, not executable SQLite SQL or permission to invent defaults.

The retained immutable [VPro64 template database](../resources/database-family/VPro64.db)
has SHA256 `4f299ffb69fd5d90ba7fd454ac200a935153acc762bfe45112659a8adc672c8b`.
It has no `_table_metadata` table. That absence must remain distinguishable from
an existing metadata table with no matching rows, NULL descriptions or empty
descriptions. Do not infer template versions from column counts or frozen editor
catalogues. Exact read-only template/schema/index evidence is retained privately.

After copying, `V7mdlAudit.LogNewProject` writes three records to the new project's
Audit table:

1. `Table = NewProject`, literal new project and current user, source timestamp.
2. `Table = USysAllSpecs`, with the observed reference version in AfterEdit.
3. `Table = USysTableOfLists`, with its observed reference version in AfterEdit.

This procedure has no audit-strength condition. The unchanged active project's
audit is not its destination. Actual implicit Boolean defaults and timestamp
storage are not established by this static trace; a desktop plan must make
those assignments explicit and verify the full new-file snapshot atomically.
Reference descriptions must be observed independently, not silently completed.
Original `AllSpecsVersion` and `TableOfListsVersion` read the linked table-object
Description of `USysAllSpecs` and `USysTableOfLists`, respectively. Their source
error paths return `"Unknown"`; this success-shaped fallback is not inherited.
A physical referenced-table description is not automatically proof of the
linked-object property. Retain provenance/absence/NULL/empty/duplicates and
resolve ambiguity before planning creation audits, without guessing a version.

`V7mdlRelationships.CreateRelationship` then requests Env.PlotNumber links to
Veg, Humus, Mineral, Other and Audit, using update/delete cascade attributes.
The Admin link targets Admin.Plot with an additional attribute1. Metadata links
use the ProjectID/StartDate pair with numeric attribute2. The function forcibly
sets `IsExtended = False` before its extended relationship block; that dormant
block is not an effective new-project requirement. Parent mutation/cascade
workflows require separate child/audit integrity acceptance, not automatic
inheritance of destructive behavior.

The original copies temporary tables into the running application, transfers
them, deletes temporary tables and links the destination. Its failure handling
does not provide a desktop atomicity guarantee. A replacement must instead
stage a new owned SQLite file, preserve source/default bytes, verify schema,
indexes, rows and creation audits, then publish without overwriting or silently
switching the active context. Cancellation, destination races/collisions,
ownership changes, rollback, retry and cleanup must be tested.

Names are subject to the existing desktop literal family-name policy, with an
explicitly documented stricter adaptation to the source's advisory prompt.
Do not trim or recase names, change existing identities, use Sample as a new
project, or interpret a missing destination as overwrite permission.

## Private owned template observation

[projectcreationtemplates.go](../projectcreationtemplates.go) observes all eight
literal physical VPro64 template tables in one existing context-owned pinned
read-only snapshot. It preserves ordered columns/explicit empty rows, raw table,
index and trigger definitions (including NULL implicit-index SQL), constraints
and defaults as evidence, and physical Description metadata/candidates. The
observed original column/index cardinalities are required; they are not schema
authentication or permission to execute the observed DDL.

The [shared schema reader](../sqliteschema.go) extracts the prior SIVI assignment
schema query without changing literal BINARY filtering/order or history JSON/
digest representation. No SQL text is executed by this new observation surface.
No destination name/file, audit rows, relationships, new context, public method,
binding, UI or feature enablement is introduced.

Focused/coupled Go race51.423s passes. Tests independently compare original
table/index SQL and explicit empty shapes, detach caller mutations, distinguish
absent/present-empty/nullable/duplicate descriptions with extra typed cells,
reject missing/views/nonempty/extra-column/malformed/index-deficient templates,
and verify ownership/stale context/cancellation after partial observation and
while waiting for the held snapshot mutex, followed by retry. A golden digest
protects the existing SIVI history format; its coupled assignment suite passes.
Independent review found no significant defects; fresh full integration passes
(root804.895s, all packages). No creator or Access runtime-parity acceptance follows from this
preparatory reader.

A selective DAO description/default probe on a disposable canonical-core copy
was time-boxed and rejected: the unstarted core has no `USysAllSpecs` linked
object, so the probe failed before property/default reads or cancelled AddNew.
No application/startup/source module/link change or row commit occurred. The
DAO child closed, the copy was exclusively readable, and source/copy hashes
remain identical. This is cleanup evidence, not a native version/default claim.
Runtime linked-property and implicit Boolean/date behavior remain unresolved;
no `"Unknown"` substitute or inferred version is accepted.

## Source evidence

| Original module | SHA256 |
|---|---|
| V7mdlCreateTables.txt | `deb11daad28e1c01002f86a34a0a55daeb499526f87ddb1781223edf755a56d0` |
| V7mdlAudit.txt | `01a97135b413aedd26cebe092ba387139c046aa09efd100551cc7ea842836471` |
| V7mdlRelationships.txt | `00f391ed0f7901b76b15d3045f8a9b352034e4edd29c0f096b7acae508c8f42d` |

Forms, callbacks, table definitions and the retained physical templates were
read without Access execution, source mutation or a native application owner.
Save as, splinter, merge, compare, arbitrary attachment and standards/cloud
distribution remain distinct administration workflows.

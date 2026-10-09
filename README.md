# VPRO Desktop

An experimental Access-to-Go/Wails v3/Svelte 5 migration with SQLite as the
canonical desktop store. **This is not yet a complete or production-approved
Access replacement.** Use disposable projects when evaluating write workflows.
The planned R companion is UI-free automation, not a second entry UI.

## Current capabilities

The table distinguishes implemented and bounded native-verified workflows from
complete form/report parity. A feature flag does not establish source authority
or enable its independently guarded peers.

| Area | Implemented boundary | Remaining limitations |
| --- | --- | --- |
| Foundation | Original SQLite database family, retained JSON-to-YAML preference migration, owned external-path contexts, offline TEMP views, draft-safe switching and explicit recovery | Conversion, broad project administration and multiwindow coordination remain incomplete |
| Ordinary FS882 entry | 98 mapped parent fields; Other, Humus, Mineral and vegetation fields; responsive source grouping, persistent validation, transactional audits, Lock and native-close guards | Not every source event, calculation, child workflow or form variant |
| Two-page entry | Source layouts, distinct normal/CHARS policies, common/additional parent fields, reference approval and atomic mixed-field Save/typed restoration | Complete linked children, pictures and unsupported callbacks remain open |
| SIVI / FS1333 | Owned parent/reference review, direct/shared editors, bounded source actions, physical ProjectID, heights/covers, Collected/species, combined child editing and standalone presentation | Independently opt-in; full combined Access execution and remaining navigation/calculation behavior are not established |
| SIVI lifecycle | Source-bound creation, exact physical-row deletion/typed restoration, historical creation Undo, permanent identity reservations and read-only lost-receipt recovery | Independently opt-in desktop safety adaptations, not inherited destructive Access cleanup |
| Pictures | Optional owned library, linked metadata review, separately authorized previews, manager/child presentation and existing directory/name editing | Add/delete/historical restoration and arbitrary selected-directory authority are not enabled |
| Profiles and navigation | Reviewed ordered profiles, rule lifecycle, scoped Find/Enter, file ownership and Save as SU | Arbitrary administration and broader restoration/criterion semantics remain incomplete |
| Environment/SU transfer | Independently reviewed forward/reverse owned-project changes with transactional provenance and drift guards | Separate-file SU and personal-definition writes remain unavailable |
| Long Environment | Owned 72-row preview, saved title preferences and reviewed no-replace XLSX publication | Independently opt-in; not Excel automation or every environment report |
| Summary Environment | Normal-SU layer/lifeform/species previews, saved options and separately gated reviewed workbook variants | Hierarchy and wider report modes remain unavailable |
| Vegetation and summaries | Long Vegetation layer/quality/None/Lifeform/typed Strata/Code previews, supported unlumped XLSX; standalone lifeform/species-attribute summaries and combined workbook | Source4/Access Val calibration, lumping and unsupported report variants remain unavailable |
| Locations and labels | Owned location review, KML preparation/no-replace publication, saved preferences and source-defined label preview | Remote icons/viewer launch, physical printing/Print All and printer/date calibration remain unverified |
| Interchange preparation | Fixture-tested Access scalar/column/row/staging boundaries and owned table archive review/publication | Not an enabled production Access importer, canonical conversion or final client analysis-format decision |

See [migration deliverables](MIGRATION_PLAN.md), [client scope](docs/CLIENT_SCOPE.md),
[per-feature capability boundaries](docs/FEATURE_CAPABILITIES.md) and the workflow
contracts below for exact boundaries. Earlier field counts or test totals are not
permission to promote defaults.

## Architecture and safety

- Preserve VPro64/VLists/VUser/VMetaData/VMessageBoard, per-project physical table
  names and `_table_metadata` descriptions. Derived editor catalogues are read
  models, not replacement databases. [Sample](resources/Sample.db) is the
  retained SQLite seed, SHA256
  `e63f0c2a051761701bdad3c81bcfde4067ab322883c7e4ac84a3541ae8578ad8`.
- `config.init.yml` initializes persistent YAML. Legacy preference import is
  explicit and lossless; source files remain retained. Failed persistence must
  preserve prior bytes and effective settings. Do not import obsolete paths.
- SQLite owns readonly attachments, TEMP views and context leases. DuckDB is
  optional; offline startup must not require downloading extensions.
- Mutations and their source audits/technical provenance share transactions.
  Check cancellation, literal ownership, collisions, rollback, retry and
  replacement. A known commit followed by refresh failure is not a failed write
  to repeat; unknown acknowledgements require the workflow's read-only recovery.
- Preserve NULL versus empty text, duplicate reference definitions, Access
  BOOLEAN true=-1, UTF-16 bounds and unchanged historical invalid assignments.
  Do not silently trim, recase, infer source defaults or repair raw Unicode.
- Persistent draft errors block Save, Lock, owner disposal and native close,
  while keeping correction and permitted Undo available. Configuration failures
  open explicit recovery rather than silently changing paths.

## Build and validate

Use the dependency versions in [go.mod](go.mod) and
[frontend/package.json](frontend/package.json). Wails is v3 beta.26 and requires
the platform's native development prerequisites and a CGO-capable compiler.

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.26
cd frontend
npm ci
npm test
npm run check
npm run build
cd ..
go test -race -timeout 60m ./...
wails3 dev
```

On Windows use `npm.cmd`. The development window uses Vite at
`http://127.0.0.1:9245/`. Browser preview alone cannot validate Go bindings or
native lifecycle behavior. Release builds use `wails3 build`; building is not
production acceptance.

The complete Windows race suite can exceed 45 minutes. Use an explicit runner
budget; a timeout is failed validation evidence, not an accepted test run.

Generate nullable interface-style bindings with:

```sh
wails3 generate bindings -f '-tags production' -ts -i
```

Use focused tests during development and full race integration before
acceptance. Native validation must use disposable `VPRO_DATA_DIR` and
`VPRO_CONFIG_DIR` directories. A compiled or enabled experimental feature is not
a verified native workflow.

## Feature flags and evidence

Ordinary mapped entry editors are generally default-on; experimental SIVI,
two-page, picture, report, interchange and administration surfaces remain
independently default-off. Frontend `VITE_*` flags are build-time presentation
choices, not backend permissions. Runtime `VPRO_*` grants remain separate.
Consult each workflow contract and its actual flag declaration before enabling.

[Portable evidence summaries](docs/MIGRATION_EVIDENCE.md) record bounded results,
source identities, exclusions and private receipt hashes. Native binaries,
Access copies, closed project data, screenshots, profiles and bulk checkpoint
archives stay outside Git. Their absence upstream is deliberate, not a reason
to manufacture parity or remove evidence locally.

## Workflow contracts

- [Ordinary FS882](docs/FS882-6x4XL-access-contract.md)
- [SIVI / FS1333](docs/FS1333-parent-access-contract.md)
- [Two-page entry and pictures](docs/FS882-two-page-access-contract.md)
- [Long Environment XLSX](docs/LONG_ENVIRONMENT_WORKBOOK_CONTRACT.md)
- [Long Vegetation XLSX](docs/LONG_VEGETATION_WORKBOOK_CONTRACT.md)
- [Summary Environment and detail reports](docs/SITE_UNIT_DETAIL_REPORT_CONTRACT.md)
- [Lifeform summary](docs/LIFEFORM_SUMMARY_CONTRACT.md) and
  [combined workbook](docs/LIFEFORM_WORKBOOK_CONTRACT.md)
- [Species attributes](docs/SPECIES_ATTRIBUTE_SUMMARY_CONTRACT.md)
- [Locations/KML](docs/PLOT_LOCATION_CONTRACT.md)
- [Label preview](docs/PLOT_LABEL_PREVIEW_CONTRACT.md)
- [Table archives](docs/TABLE_CSV_CONTRACT.md),
  [Access import preparation](docs/ACCESS_IMPORT_CONTRACT.md) and
  [project-creation preparation](docs/PROJECT_CREATION_CONTRACT.md)

Current work remains in [MIGRATION_PLAN.md](MIGRATION_PLAN.md).
[WINDOWS_HANDOFF.md](WINDOWS_HANDOFF.md) is a current operational handoff, not
a release history or a portable evidence archive.

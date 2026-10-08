# VPRO desktop migration

Go/Wails/Svelte provides the desktop boundary; SQLite is canonical and DuckDB is
optional. The predecessor's reviewed data-family, context, reference and editing
boundaries are retained. Unavailable workflows remain unavailable.

## Current capability contract

Long Vegetation now has an **owned Wails preview UI and quality-qualified layer
criteria** for an explicitly selected site unit. The UI remains default-off:
`VITE_LONG_VEGETATION_REPORT` must be the literal `true`. Owned context and source
scope checks remain required; this is not a source mutation or printing grant.
See [the preview](frontend/src/LongVegetationReport.svelte),
[the service](vegetationreportservice.go) and
[quality semantics](vegetationreportquality.go).

[Feature capabilities](docs/FEATURE_CAPABILITIES.md) is the cumulative source
contract for this revision. Each entry states its implemented or private-only
boundary, entry/gate locators and exclusions. It is a capability read model, not
a chronology, test log or claim that a default-off surface is generally enabled.
Later capabilities must be read from that table rather than inferred from an
early preview or preparation entry.

## Safety boundaries

- Preserve the SQLite database family, per-project physical names and native
  `_table_metadata` Description storage.
- Use owned contexts, literal physical witnesses, retained validation and
  operation-specific transaction/history rules. NULL and empty stay distinct.
- Private Access adapters and prepared images are not public import workflows;
  project-template preparation is not installed project creation.
- No source4, physical printing, picture creation or unavailable callback is
  enabled by this capability catalogue.
- Native acceptance and publication authorization are separate gates. Source
  contracts and passing unit tests do not constitute new native parity claims.

## Build and focused validation

Use existing dependencies and disposable data/configuration for tests. In
`frontend`, run `npm run test`, `npm run check` and `npm run build`. Go changes
require focused behavioral tests; `go test -run '^$' ./...` checks compilation
only. Keep native lifecycle verification separate from browser previews.

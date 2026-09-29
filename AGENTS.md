# VPRO Wails migration

## Scope and evidence

- Build the desktop application in Go/Wails/Svelte; use Access SaveAsText in `../VPRO_ACCESS/VPro64_forAI`, the installed Access application on the Win11 VM, and the R package in `../vpro` as behavioral references. Do not write to canonical Access source or production databases.
- For ambiguous behavior, run reproducible probes on disposable VM copies and record source form/procedure, inputs, observed data changes, and failure cases. R APIs and tests are precedents, not proof of Access parity.
- SQLite is canonical storage. Keep the experimental DuckDB coordinator optional until a workflow requires cross-database queries and offline extension packaging is verified.
- The separate `go-mdbtools` effort owns Access-file reading. Integrate it only behind fixture-validated import boundaries; do not duplicate its implementation here.

## Agent handoffs

- Assign one bounded workflow per agent. Read-only evidence agents may trace Access events or compare existing code; implementation agents must have exclusive Go or frontend file ownership and an agreed service contract.
- Hand off a compact checklist: source path and event, observed VM result, intended desktop behavior, test fixture, and `implemented | missing` status. The primary agent reconciles conflicting evidence, integrates changes, and verifies the native app.
- Do not enable navigation to a placeholder or enable writes until its actual workflow and failure behavior have been verified.

## Validation

- Run focused Go tests for changed services, then `go test -race ./...`; run `npm run check` and `npm run build` in `frontend` for frontend changes. Verify binding-dependent interactions in the Wails window, not only the browser preview.
- Keep data mutations transactional, test cancellation/collision/rollback paths, and use disposable project copies. Record incomplete parity explicitly in `README.md`.
# Feature capabilities

This cumulative table describes source-level capability boundaries in this
revision. It is not execution chronology, test history, a production-enablement
list or independent native acceptance evidence. The predecessor's existing
boundaries remain in force.

**Default-off** means an implemented surface still requires its literal frontend
or independent service opt-in and all operation-specific ownership checks.
**Private** means a preparation, reader, kernel or contract boundary, not a public
desktop workflow. Gate/entry locators identify actual implementation, source
contracts and structured resources; a visible control or file is not permission
to mutate canonical data.

Read entries cumulatively. A preparation row does not grant a later operation;
later explicitly implemented capabilities retain their own separate gates.
Unavailable source4, physical printing, picture creation and dormant callbacks
remain unavailable. Native verification and publication are separate approvals.

| Capability | State and source contract | Guarded boundary and exclusions | Entry, service, flag and resource locators |
| --- | --- | --- | --- |
| Validate Long Vegetation presentation and quality | **Default-off Wails preview** — Owned selected-SU Long Vegetation layer preview and quality criteria; preserves physical fanout, NULL statistics and reference provenance. | UI requires literal VITE_LONG_VEGETATION_REPORT=true; backend requires owned context and explicit SU. No non-layer modes, workbook export, source mutation or printing is granted by this row. | [`frontend/src/LongVegetationReport.svelte`](../frontend/src/LongVegetationReport.svelte); [`vegetationreportservice.go`](../vegetationreportservice.go); [`vegetationreportquality.go`](../vegetationreportquality.go); [`environmentreport.go`](../environmentreport.go); [`environmentreportservice.go`](../environmentreportservice.go); [`reportunitnames.go`](../reportunitnames.go); [`scopedplotlookup.go`](../scopedplotlookup.go); [`sqlitecontext.go`](../sqlitecontext.go) |

---
description: "Native desktop parity acceptance and asynchronous close witnesses"
applyTo: "**"
---

## Learnings

- After posting an owned native close request, wait for the specific native-close
  cancellation acknowledgement before correcting a draft or releasing a held
  operation. A generic "Wait for the current operation" assertion can match an
  earlier navigation warning while native close is still queued; use the exact
  closing message (or an independently observed CancelClose receipt), then
  reverify the owned PID/start/path before changing the barrier.

- Validate unsupported joined grouping representatives before aggregation and
  threshold filtering, not only in final report rows. Separate A/a byte buckets
  with cover2 can both disappear at threshold3 while native collation would have
  combined them; keep refusal checks scoped to actual joins so unrelated master
  rows cannot veto output. Reopened workbook checks must resolve the exact reviewed
  data sheets, not treat ReportSummary or hidden approval/source sheets as data.

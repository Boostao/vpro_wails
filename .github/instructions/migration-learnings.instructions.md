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

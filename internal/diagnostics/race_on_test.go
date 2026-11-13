//go:build race

package diagnostics

import "time"

const synthesisLatencyBudget = 200 * time.Millisecond

//go:build !race

package diagnostics

import "time"

const synthesisLatencyBudget = 50 * time.Millisecond

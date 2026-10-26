package radar

import "math"

// pollIntervalBounds: user-chosen fetch frequency (add + edit). Matches portable import (60..86400).
const (
	pollIntervalMin = 60
	pollIntervalMax = 86400
)

// clampPollInterval snaps a requested seconds value into [60, 86400]. Non-finite / non-positive → 0 (caller uses default).
func clampPollInterval(n float64) int {
	if math.IsNaN(n) || math.IsInf(n, 0) || n <= 0 {
		return 0
	}
	if n < pollIntervalMin {
		return pollIntervalMin
	}
	if n > pollIntervalMax {
		return pollIntervalMax
	}
	return int(math.Round(n))
}

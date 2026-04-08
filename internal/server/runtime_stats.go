package server

import (
	"fmt"
	"strings"
)

func humanBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}

	divisor := uint64(unit)
	suffixes := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	index := 0
	for value >= divisor*unit && index < len(suffixes)-1 {
		divisor *= unit
		index++
	}
	return fmt.Sprintf("%.1f %s", float64(value)/float64(divisor), suffixes[index])
}

func joinNonEmpty(values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, " | ")
}

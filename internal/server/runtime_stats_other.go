//go:build !linux

package server

import (
	"os"
	"runtime"
	"strconv"
)

func collectServerRuntimeRows() []metricRow {
	rows := make([]metricRow, 0, 3)
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		rows = append(rows, metricRow{Name: "hostname", Value: hostname})
	}
	rows = append(rows, metricRow{Name: "goos", Value: runtime.GOOS})
	rows = append(rows, metricRow{Name: "gomaxprocs", Value: strconv.Itoa(runtime.GOMAXPROCS(0))})
	return rows
}

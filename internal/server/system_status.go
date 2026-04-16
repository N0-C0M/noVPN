package server

import "time"

type systemStatusSnapshot struct {
	ObservedAt       time.Time `json:"observed_at"`
	Hostname         string    `json:"hostname,omitempty"`
	Ready            bool      `json:"ready"`
	CPUCores         int       `json:"cpu_cores"`
	Load1            float64   `json:"load_1"`
	Load5            float64   `json:"load_5"`
	Load15           float64   `json:"load_15"`
	MemoryUsedBytes  uint64    `json:"memory_used_bytes"`
	MemoryTotalBytes uint64    `json:"memory_total_bytes"`
	DiskUsedBytes    uint64    `json:"disk_used_bytes"`
	DiskTotalBytes   uint64    `json:"disk_total_bytes"`
	UptimeSeconds    uint64    `json:"uptime_seconds"`
	RootPath         string    `json:"root_path,omitempty"`
}

//go:build !linux

package server

import (
	"os"
	"runtime"
	"strings"
	"time"
)

func collectSystemStatusSnapshot(rootPath string, ready bool) (systemStatusSnapshot, error) {
	hostname, _ := os.Hostname()
	return systemStatusSnapshot{
		ObservedAt: time.Now().UTC(),
		Hostname:   hostname,
		Ready:      ready,
		CPUCores:   runtime.NumCPU(),
		RootPath:   firstNonEmptyServer(strings.TrimSpace(rootPath), "/"),
	}, nil
}

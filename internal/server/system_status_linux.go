//go:build linux

package server

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func collectSystemStatusSnapshot(rootPath string, ready bool) (systemStatusSnapshot, error) {
	snapshot := systemStatusSnapshot{
		ObservedAt: time.Now().UTC(),
		Ready:      ready,
		CPUCores:   runtime.NumCPU(),
		RootPath:   firstNonEmptyServer(strings.TrimSpace(rootPath), "/"),
	}
	hostname, _ := os.Hostname()
	snapshot.Hostname = hostname

	if load1, load5, load15, err := readLoadAverages(); err == nil {
		snapshot.Load1 = load1
		snapshot.Load5 = load5
		snapshot.Load15 = load15
	}
	if total, used, err := readMemoryUsage(); err == nil {
		snapshot.MemoryTotalBytes = total
		snapshot.MemoryUsedBytes = used
	}
	if total, used, err := readDiskUsage(snapshot.RootPath); err == nil {
		snapshot.DiskTotalBytes = total
		snapshot.DiskUsedBytes = used
	}
	if uptime, err := readUptimeSeconds(); err == nil {
		snapshot.UptimeSeconds = uptime
	}
	return snapshot, nil
}

func readLoadAverages() (float64, float64, float64, error) {
	payload, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}
	fields := strings.Fields(string(payload))
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("unexpected /proc/loadavg payload")
	}
	load1, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, 0, 0, err
	}
	load5, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, 0, 0, err
	}
	load15, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return 0, 0, 0, err
	}
	return load1, load5, load15, nil
}

func readMemoryUsage() (uint64, uint64, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	values := make(map[string]uint64)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		fields := strings.Fields(strings.TrimSpace(parts[1]))
		if len(fields) == 0 {
			continue
		}
		value, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		values[strings.TrimSpace(parts[0])] = value * 1024
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, err
	}

	total := values["MemTotal"]
	available := values["MemAvailable"]
	if available == 0 {
		available = values["MemFree"] + values["Buffers"] + values["Cached"]
	}
	if total == 0 {
		return 0, 0, fmt.Errorf("MemTotal is missing")
	}
	if available > total {
		available = total
	}
	return total, total - available, nil
}

func readDiskUsage(rootPath string) (uint64, uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(rootPath, &stat); err != nil {
		return 0, 0, err
	}
	total := stat.Blocks * uint64(stat.Bsize)
	available := stat.Bavail * uint64(stat.Bsize)
	if available > total {
		available = total
	}
	return total, total - available, nil
}

func readUptimeSeconds() (uint64, error) {
	payload, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(payload))
	if len(fields) == 0 {
		return 0, fmt.Errorf("unexpected /proc/uptime payload")
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}
	if value < 0 {
		value = 0
	}
	return uint64(value), nil
}

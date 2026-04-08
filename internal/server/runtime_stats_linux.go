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

func collectServerRuntimeRows() []metricRow {
	rows := make([]metricRow, 0, 8)

	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		rows = append(rows, metricRow{Name: "hostname", Value: hostname})
	}

	rows = append(rows, metricRow{Name: "gomaxprocs", Value: strconv.Itoa(runtime.GOMAXPROCS(0))})

	if uptime, err := linuxUptime(); err == nil {
		rows = append(rows, metricRow{Name: "uptime", Value: uptime.Round(time.Second).String()})
	}

	if load, err := linuxLoadAverage(); err == nil {
		rows = append(rows, metricRow{Name: "load_avg", Value: load})
	}

	if memory, err := linuxMemoryUsage(); err == nil {
		rows = append(rows, metricRow{Name: "memory", Value: memory})
	}

	if disk, err := linuxRootDiskUsage(); err == nil {
		rows = append(rows, metricRow{Name: "disk_root", Value: disk})
	}

	if traffic, err := linuxNetworkTraffic(); err == nil {
		rows = append(rows, metricRow{Name: "network_total", Value: traffic})
	}

	return rows
}

func linuxUptime() (time.Duration, error) {
	payload, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(payload))
	if len(fields) == 0 {
		return 0, fmt.Errorf("empty /proc/uptime")
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}
	return time.Duration(seconds * float64(time.Second)), nil
}

func linuxLoadAverage() (string, error) {
	payload, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(payload))
	if len(fields) < 3 {
		return "", fmt.Errorf("invalid /proc/loadavg")
	}
	return fmt.Sprintf("%s %s %s", fields[0], fields[1], fields[2]), nil
}

func linuxMemoryUsage() (string, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return "", err
	}
	defer file.Close()

	var totalKB uint64
	var availableKB uint64

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		value, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			continue
		}

		switch parts[0] {
		case "MemTotal:":
			totalKB = value
		case "MemAvailable:":
			availableKB = value
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if totalKB == 0 {
		return "", fmt.Errorf("MemTotal not found")
	}

	usedKB := totalKB - availableKB
	return fmt.Sprintf(
		"%s / %s used (%.0f%%)",
		humanBytes(usedKB*1024),
		humanBytes(totalKB*1024),
		(float64(usedKB)/float64(totalKB))*100,
	), nil
}

func linuxRootDiskUsage() (string, error) {
	var stats syscall.Statfs_t
	if err := syscall.Statfs("/", &stats); err != nil {
		return "", err
	}

	total := stats.Blocks * uint64(stats.Bsize)
	free := stats.Bavail * uint64(stats.Bsize)
	used := total - free
	if total == 0 {
		return "", fmt.Errorf("disk size is zero")
	}

	return fmt.Sprintf(
		"%s / %s used (%.0f%%)",
		humanBytes(used),
		humanBytes(total),
		(float64(used)/float64(total))*100,
	), nil
}

func linuxNetworkTraffic() (string, error) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return "", err
	}
	defer file.Close()

	var rx uint64
	var tx uint64
	scanner := bufio.NewScanner(file)
	lineIndex := 0
	for scanner.Scan() {
		lineIndex++
		if lineIndex <= 2 {
			continue
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		if name == "lo" {
			continue
		}

		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}

		rxValue, err := strconv.ParseUint(fields[0], 10, 64)
		if err == nil {
			rx += rxValue
		}
		txValue, err := strconv.ParseUint(fields[8], 10, 64)
		if err == nil {
			tx += txValue
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	return joinNonEmpty("rx "+humanBytes(rx), "tx "+humanBytes(tx)), nil
}

package monitor

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type SystemStats struct {
	LoadAverage string `json:"load_average"`
	MemoryUsage string `json:"memory_usage"`
	Uptime      string `json:"uptime"`
	Hostname    string `json:"hostname"`
	Kernel      string `json:"kernel"`
	DiskUsage   string `json:"disk_usage"`
}

func GetSystemStats() (SystemStats, error) {
	stats := SystemStats{}

	// Hostname
	hostname, _ := os.Hostname()
	stats.Hostname = hostname

	// Kernel
	kernel, _ := os.ReadFile("/proc/sys/kernel/osrelease")
	stats.Kernel = strings.TrimSpace(string(kernel))

	// Load Average
	load, err := os.ReadFile("/proc/loadavg")
	if err == nil {
		fields := strings.Fields(string(load))
		if len(fields) >= 3 {
			stats.LoadAverage = fmt.Sprintf("%s, %s, %s", fields[0], fields[1], fields[2])
		}
	} else {
		stats.LoadAverage = "N/A"
	}

	// Memory Usage
	memInfo, err := os.ReadFile("/proc/meminfo")
	if err == nil {
		stats.MemoryUsage = parseMemInfo(string(memInfo))
	} else {
		stats.MemoryUsage = "N/A"
	}

	// Uptime
	uptimeData, err := os.ReadFile("/proc/uptime")
	if err == nil {
		fields := strings.Fields(string(uptimeData))
		if len(fields) > 0 {
			uptimeSecs, _ := strconv.ParseFloat(fields[0], 64)
			stats.Uptime = formatUptime(uptimeSecs)
		}
	} else {
		stats.Uptime = "N/A"
	}

	// Disk Usage (Simplified using df if available)
	stats.DiskUsage = "N/A"
	out, err := exec.Command("df", "-h", "/").Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		if len(lines) >= 2 {
			fields := strings.Fields(lines[1])
			if len(fields) >= 5 {
				stats.DiskUsage = fmt.Sprintf("%s / %s (%s)", fields[2], fields[1], fields[4])
			}
		}
	}

	return stats, nil
}

func parseMemInfo(content string) string {
	lines := strings.Split(content, "\n")
	var total, available float64

	for _, line := range lines {
		if strings.HasPrefix(line, "MemTotal:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				total, _ = strconv.ParseFloat(parts[1], 64)
			}
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				available, _ = strconv.ParseFloat(parts[1], 64)
			}
		}
	}

	if total == 0 {
		return "Unknown"
	}

	used := total - available
	usedGB := used / 1024 / 1024
	totalGB := total / 1024 / 1024
	percent := (used / total) * 100

	return fmt.Sprintf("%.1f/%.1f GB (%.0f%%)", usedGB, totalGB, percent)
}

func formatUptime(seconds float64) string {
	days := int(seconds) / 86400
	hours := (int(seconds) % 86400) / 3600
	minutes := (int(seconds) % 3600) / 60
	
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

package monitor

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type SystemStats struct {
	LoadAverage    string `json:"load_average"`
	MemoryUsage    string `json:"memory_usage"`
	Uptime         string `json:"uptime"`
	Hostname       string `json:"hostname"`
	Kernel         string `json:"kernel"`
	DiskUsage      string `json:"disk_usage"`
	CompactionMode string `json:"compaction_mode"`
	GatewayStatus  string `json:"gateway_status"`
	GatewayPID     int    `json:"gateway_pid"`
	GatewayUptime  string `json:"gateway_uptime"`
	GatewayMemory  string `json:"gateway_memory"`
}

func GetSystemStats() (SystemStats, error) {
	stats := SystemStats{
		GatewayStatus: "offline",
	}

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

	// Compaction Mode (Mocked for now)
	stats.CompactionMode = "Auto"

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

	// ── Gateway health (Learned from script) ──
	// Find PID for openclaw-gateway (might be named 'openclaw' or match command line)
	// The script used: ps aux | grep 'openclaw-gateway' | grep -v grep | awk '{print $2}'
	// We'll search for 'openclaw' or 'openclaw-gateway'
	pgrep, err := exec.Command("pgrep", "-f", "openclaw").Output()
	if err == nil {
		pids := strings.Fields(string(pgrep))
		if len(pids) > 0 {
			pid := pids[0] // Take first matching PID
			pidInt, _ := strconv.Atoi(pid)
			stats.GatewayPID = pidInt
			stats.GatewayStatus = "online"

			// Get etime and rss
			ps, err := exec.Command("ps", "-p", pid, "-o", "etime=,rss=").Output()
			if err == nil {
				fields := strings.Fields(string(ps))
				if len(fields) >= 2 {
					stats.GatewayUptime = fields[0]
					rssKB, _ := strconv.ParseFloat(fields[1], 64)
					if rssKB > 1048576 {
						stats.GatewayMemory = fmt.Sprintf("%.1f GB", rssKB/1048576)
					} else if rssKB > 1024 {
						stats.GatewayMemory = fmt.Sprintf("%.0f MB", rssKB/1024)
					} else {
						stats.GatewayMemory = fmt.Sprintf("%.0f KB", rssKB)
					}
				}
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

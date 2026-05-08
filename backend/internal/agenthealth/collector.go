package agenthealth

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Health struct {
	CPUPercent     float64
	MemPercent     float64
	MemUsedBytes   uint64
	MemTotalBytes  uint64
	DiskFreeBytes  uint64
	DiskTotalBytes uint64
	UptimeSeconds  uint64
}

var prevIdle, prevTotal uint64
var lastCPU time.Time

// Collect gathers system health metrics from /proc.
func Collect() *Health {
	h := &Health{}

	// CPU from /proc/stat
	if data, err := os.ReadFile("/proc/stat"); err == nil {
		h.CPUPercent = parseCPU(string(data))
	}

	// Memory from /proc/meminfo
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		h.MemTotalBytes, h.MemUsedBytes, h.MemPercent = parseMem(string(data))
	}

	// Disk from statfs on /
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err == nil {
		h.DiskTotalBytes = stat.Blocks * uint64(stat.Bsize)
		h.DiskFreeBytes = stat.Bavail * uint64(stat.Bsize)
	}

	// Uptime from /proc/uptime
	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) > 0 {
			if v, err := strconv.ParseFloat(parts[0], 64); err == nil {
				h.UptimeSeconds = uint64(v)
			}
		}
	}

	return h
}

func parseCPU(stat string) float64 {
	// Find the aggregate "cpu " line (not "cpu0", "cpu1", etc.)
	lines := strings.Split(stat, "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return 0
		}
		// fields: cpu, user, nice, system, idle, iowait, irq, softirq, steal
		var vals [8]uint64
		for i := 0; i < 8 && i+1 < len(fields); i++ {
			vals[i], _ = strconv.ParseUint(fields[i+1], 10, 64)
		}
		idle := vals[3] + vals[4] // idle + iowait
		total := vals[0] + vals[1] + vals[2] + vals[3] + vals[4] + vals[5] + vals[6] + vals[7]
		now := time.Now()

		if prevTotal > 0 {
			totalDiff := total - prevTotal
			idleDiff := idle - prevIdle
			if totalDiff > 0 {
				delta := now.Sub(lastCPU).Seconds()
				if delta > 0 {
					prevIdle = idle
					prevTotal = total
					lastCPU = now
					return (1.0 - float64(idleDiff)/float64(totalDiff)) * 100.0
				}
			}
		}
		prevIdle = idle
		prevTotal = total
		lastCPU = now
		return 0
	}
	return 0
}

func parseMem(meminfo string) (total, used uint64, percent float64) {
	var memTotal, memFree, buffers, cached, sReclaimable uint64
	lines := strings.Split(meminfo, "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		v, _ := strconv.ParseUint(parts[1], 10, 64)
		switch parts[0] {
		case "MemTotal:":
			memTotal = v
		case "MemFree:":
			memFree = v
		case "Buffers:":
			buffers = v
		case "Cached:":
			cached = v
		case "SReclaimable:":
			sReclaimable = v
		}
	}
	memTotal *= 1024
	memFree *= 1024
	buffers *= 1024
	cached *= 1024
	sReclaimable *= 1024

	used = memTotal - memFree - buffers - cached - sReclaimable
	if memTotal > 0 {
		percent = float64(used) / float64(memTotal) * 100.0
	}
	return memTotal, used, percent
}

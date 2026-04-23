package services

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// SystemStats holds all system resource information.
type SystemStats struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemTotal    uint64  `json:"mem_total"`
	MemUsed     uint64  `json:"mem_used"`
	MemPercent  float64 `json:"mem_percent"`
	DiskTotal   uint64  `json:"disk_total"`
	DiskUsed    uint64  `json:"disk_used"`
	DiskPercent float64 `json:"disk_percent"`
	Uptime      string  `json:"uptime"`
	UptimeSecs  float64 `json:"uptime_secs"`
	LoadAvg1    float64 `json:"load_avg_1"`
	LoadAvg5    float64 `json:"load_avg_5"`
	LoadAvg15   float64 `json:"load_avg_15"`
	Hostname    string  `json:"hostname"`
	OS          string  `json:"os"`
	Arch        string  `json:"arch"`
	NumCPU      int     `json:"num_cpu"`
}

// SysInfoService reads system metrics from /proc (Linux only).
type SysInfoService struct {
	prevIdle  uint64
	prevTotal uint64
}

// NewSysInfoService creates a new system info service.
func NewSysInfoService() *SysInfoService {
	s := &SysInfoService{}
	// Prime the CPU measurement
	s.readCPU()
	time.Sleep(100 * time.Millisecond)
	return s
}

// GetStats returns current system statistics.
func (s *SysInfoService) GetStats() SystemStats {
	hostname, _ := os.Hostname()

	stats := SystemStats{
		CPUUsage: s.getCPUUsage(),
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		NumCPU:   runtime.NumCPU(),
	}

	// Memory
	s.getMemInfo(&stats)

	// Disk
	s.getDiskInfo(&stats)

	// Uptime
	s.getUptime(&stats)

	// Load average
	s.getLoadAvg(&stats)

	return stats
}

func (s *SysInfoService) readCPU() (idle, total uint64) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return 0, 0
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 8 || fields[0] != "cpu" {
		return 0, 0
	}

	var values []uint64
	for _, f := range fields[1:] {
		v, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			continue
		}
		values = append(values, v)
	}

	if len(values) < 4 {
		return 0, 0
	}

	idle = values[3]
	for _, v := range values {
		total += v
	}
	return idle, total
}

func (s *SysInfoService) getCPUUsage() float64 {
	idle, total := s.readCPU()
	defer func() {
		s.prevIdle = idle
		s.prevTotal = total
	}()

	if s.prevTotal == 0 {
		return 0
	}

	idleDelta := float64(idle - s.prevIdle)
	totalDelta := float64(total - s.prevTotal)
	if totalDelta == 0 {
		return 0
	}

	return (1.0 - idleDelta/totalDelta) * 100.0
}

func (s *SysInfoService) getMemInfo(stats *SystemStats) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return
	}

	var memTotal, memAvailable uint64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			memTotal = val * 1024 // Convert from kB to bytes
		case "MemAvailable:":
			memAvailable = val * 1024
		}
	}

	stats.MemTotal = memTotal
	stats.MemUsed = memTotal - memAvailable
	if memTotal > 0 {
		stats.MemPercent = float64(stats.MemUsed) / float64(memTotal) * 100
	}
}

func (s *SysInfoService) getDiskInfo(stats *SystemStats) {
	var statfs syscall.Statfs_t
	if err := syscall.Statfs("/", &statfs); err != nil {
		return
	}

	stats.DiskTotal = statfs.Blocks * uint64(statfs.Bsize)
	stats.DiskUsed = (statfs.Blocks - statfs.Bfree) * uint64(statfs.Bsize)
	if statfs.Blocks > 0 {
		stats.DiskPercent = float64(stats.DiskUsed) / float64(stats.DiskTotal) * 100
	}
}

func (s *SysInfoService) getUptime(stats *SystemStats) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return
	}
	secs, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return
	}

	stats.UptimeSecs = secs
	d := int(secs) / 86400
	h := (int(secs) % 86400) / 3600
	m := (int(secs) % 3600) / 60

	if d > 0 {
		stats.Uptime = fmt.Sprintf("%dd %dh %dm", d, h, m)
	} else if h > 0 {
		stats.Uptime = fmt.Sprintf("%dh %dm", h, m)
	} else {
		stats.Uptime = fmt.Sprintf("%dm", m)
	}
}

func (s *SysInfoService) getLoadAvg(stats *SystemStats) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return
	}
	stats.LoadAvg1, _ = strconv.ParseFloat(fields[0], 64)
	stats.LoadAvg5, _ = strconv.ParseFloat(fields[1], 64)
	stats.LoadAvg15, _ = strconv.ParseFloat(fields[2], 64)
}

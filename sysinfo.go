package main

import (
	"os/exec"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// SysInfo holds a snapshot of system resource usage.
type SysInfo struct {
	RAMUsed  uint64  `json:"ram_used"`
	RAMTotal uint64  `json:"ram_total"`
	CPUPct   float64 `json:"cpu_pct"`
	DiskUsed uint64  `json:"disk_used"`
	DiskTotal uint64 `json:"disk_total"`

	// GPU fields — populated only if nvidia-smi is available
	GPUName      string `json:"gpu_name,omitempty"`
	GPUVRAMUsed  uint64 `json:"gpu_vram_used,omitempty"`
	GPUVRAMTotal uint64 `json:"gpu_vram_total,omitempty"`
}

// GetSysInfo returns a current resource snapshot.
func GetSysInfo() SysInfo {
	info := SysInfo{}

	// RAM
	if v, err := mem.VirtualMemory(); err == nil {
		info.RAMUsed = v.Used
		info.RAMTotal = v.Total
	}

	// CPU — 1-second sample
	if pcts, err := cpu.Percent(time.Second, false); err == nil && len(pcts) > 0 {
		info.CPUPct = pcts[0]
	}

	// Disk (root partition)
	if u, err := disk.Usage("/"); err == nil {
		info.DiskUsed = u.Used
		info.DiskTotal = u.Total
	}

	// GPU — best effort via nvidia-smi
	info.GPUName, info.GPUVRAMUsed, info.GPUVRAMTotal = queryNvidiaGPU()

	return info
}

// queryNvidiaGPU parses nvidia-smi output. Returns zero values if unavailable.
func queryNvidiaGPU() (name string, vramUsed, vramTotal uint64) {
	out, err := exec.Command("nvidia-smi",
		"--query-gpu=name,memory.used,memory.total",
		"--format=csv,noheader,nounits",
	).Output()
	if err != nil {
		return "", 0, 0
	}

	line := strings.TrimSpace(string(out))
	parts := strings.Split(line, ", ")
	if len(parts) < 3 {
		return "", 0, 0
	}

	name = strings.TrimSpace(parts[0])
	vramUsed = parseMiB(parts[1])
	vramTotal = parseMiB(parts[2])
	return
}

// parseMiB converts a MiB string (e.g. "3812") to bytes.
func parseMiB(s string) uint64 {
	s = strings.TrimSpace(s)
	var v uint64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			v = v*10 + uint64(c-'0')
		}
	}
	return v * 1024 * 1024
}

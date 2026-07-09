package bench

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

// resourceMonitor samples HOST-level CPU utilization (/proc/stat) and memory
// usage (/proc/meminfo) while a cell is being measured. Host-level is the
// honest scope here: the client, Postgres, and SpiceDB share one machine, so
// per-process numbers would hide the engine-side cost.
type resourceMonitor struct {
	stop chan struct{}
	done chan struct{}

	cpuSum, cpuMax float64
	memSum, memMax float64 // used MB
	samples        int
}

// ResourceUsage is the per-cell aggregate attached to results.
type ResourceUsage struct {
	CPUAvgPct    float64 `json:"cpu_avg_pct"`
	CPUMaxPct    float64 `json:"cpu_max_pct"`
	MemUsedAvgMB float64 `json:"mem_used_avg_mb"`
	MemUsedMaxMB float64 `json:"mem_used_max_mb"`
}

func startResourceMonitor(interval time.Duration) *resourceMonitor {
	m := &resourceMonitor{stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(m.done)
		prevBusy, prevTotal, ok := readCPUStat()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-m.stop:
				return
			case <-ticker.C:
				if busy, total, ok2 := readCPUStat(); ok && ok2 && total > prevTotal {
					pct := 100 * float64(busy-prevBusy) / float64(total-prevTotal)
					m.cpuSum += pct
					if pct > m.cpuMax {
						m.cpuMax = pct
					}
					prevBusy, prevTotal = busy, total
				}
				if usedMB, ok2 := readMemUsedMB(); ok2 {
					m.memSum += usedMB
					if usedMB > m.memMax {
						m.memMax = usedMB
					}
				}
				m.samples++
			}
		}
	}()
	return m
}

// Stop halts sampling and returns the aggregate (zero-valued off Linux/procfs).
func (m *resourceMonitor) Stop() ResourceUsage {
	close(m.stop)
	<-m.done
	if m.samples == 0 {
		return ResourceUsage{}
	}
	n := float64(m.samples)
	return ResourceUsage{
		CPUAvgPct:    m.cpuSum / n,
		CPUMaxPct:    m.cpuMax,
		MemUsedAvgMB: m.memSum / n,
		MemUsedMaxMB: m.memMax,
	}
}

// readCPUStat returns cumulative (busy, total) jiffies from /proc/stat.
func readCPUStat() (busy, total uint64, ok bool) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return 0, 0, false
	}
	fields := strings.Fields(sc.Text()) // "cpu user nice system idle iowait irq softirq steal ..."
	if len(fields) < 8 || fields[0] != "cpu" {
		return 0, 0, false
	}
	var vals []uint64
	for _, s := range fields[1:] {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return 0, 0, false
		}
		vals = append(vals, v)
	}
	for _, v := range vals {
		total += v
	}
	idle := vals[3]
	if len(vals) > 4 {
		idle += vals[4] // + iowait
	}
	return total - idle, total, true
}

// readMemUsedMB returns (MemTotal − MemAvailable) in MB from /proc/meminfo.
func readMemUsedMB() (float64, bool) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, false
	}
	defer f.Close()
	var totalKB, availKB float64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if v, found := strings.CutPrefix(line, "MemTotal:"); found {
			totalKB, _ = strconv.ParseFloat(strings.Fields(v)[0], 64)
		} else if v, found := strings.CutPrefix(line, "MemAvailable:"); found {
			availKB, _ = strconv.ParseFloat(strings.Fields(v)[0], 64)
		}
		if totalKB > 0 && availKB > 0 {
			break
		}
	}
	if totalKB == 0 || availKB == 0 {
		return 0, false
	}
	return (totalKB - availKB) / 1024, true
}

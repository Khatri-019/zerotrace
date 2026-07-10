package enricher

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	bootTimeOnce sync.Once
	bootTimeNs   int64 // Unix nanoseconds at which the kernel booted
)

// BootTimeNs returns the Unix nanosecond timestamp at which this Linux system
// booted, read once from /proc/stat (btime field) and cached forever.
//
// eBPF ktime timestamps are nanoseconds since boot (CLOCK_MONOTONIC).
// To convert to wall-clock Unix nanoseconds: wallNs = BootTimeNs() + ktimeNs
func BootTimeNs() int64 {
	bootTimeOnce.Do(func() {
		ns, err := readBootTimeNs()
		if err != nil {
			// Fallback: estimate from current time minus uptime
			bootTimeNs = fallbackBootTimeNs()
		} else {
			bootTimeNs = ns
		}
	})
	return bootTimeNs
}

// MonotonicToUnixNs converts a BPF ktime_get_ns() value (nanoseconds since
// boot) to a Unix nanosecond timestamp suitable for span StartTimeNs/EndTimeNs.
func MonotonicToUnixNs(ktimeNs uint64) int64 {
	return BootTimeNs() + int64(ktimeNs)
}

// readBootTimeNs reads the boot time from /proc/stat.
// The "btime" line contains seconds since Unix epoch at which the system booted.
func readBootTimeNs() (int64, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, fmt.Errorf("open /proc/stat: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "btime ") {
			var btimeSec int64
			if _, err := fmt.Sscanf(line, "btime %d", &btimeSec); err != nil {
				return 0, fmt.Errorf("parse btime: %w", err)
			}
			return btimeSec * int64(time.Second), nil
		}
	}
	return 0, fmt.Errorf("btime not found in /proc/stat")
}

// fallbackBootTimeNs estimates boot time using /proc/uptime.
// Less accurate than /proc/stat but works as a last resort.
func fallbackBootTimeNs() int64 {
	f, err := os.Open("/proc/uptime")
	if err != nil {
		return 0
	}
	defer f.Close()

	var uptimeSec float64
	if _, err := fmt.Fscanf(f, "%f", &uptimeSec); err != nil {
		return 0
	}
	uptimeNs := int64(uptimeSec * float64(time.Second))
	return time.Now().UnixNano() - uptimeNs
}

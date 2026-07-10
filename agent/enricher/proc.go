package enricher

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// GetProcessName reads the process name from /proc/<pid>/comm.
func GetProcessName(pid int) (string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return "", fmt.Errorf("read comm for pid %d: %w", pid, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// GetRemoteAddr reads the remote address of a TCP socket for a given fd
// by scanning /proc/<pid>/net/tcp for the matching inode.
// Falls back to an empty string on error rather than panicking.
func GetRemoteAddr(pid, fd int) (string, error) {
	// Read the inode of the fd from /proc/<pid>/fd/<fd>
	fdLink := fmt.Sprintf("/proc/%d/fd/%d", pid, fd)
	target, err := os.Readlink(fdLink)
	if err != nil {
		return "", fmt.Errorf("readlink %s: %w", fdLink, err)
	}
	// target looks like "socket:[<inode>]"
	if !strings.HasPrefix(target, "socket:[") {
		return "", fmt.Errorf("fd %d is not a socket (got %s)", fd, target)
	}
	var inode uint64
	if _, err := fmt.Sscanf(target, "socket:[%d]", &inode); err != nil {
		return "", fmt.Errorf("parse inode from %s: %w", target, err)
	}

	return lookupTCPInode(pid, inode)
}

// lookupTCPInode scans /proc/<pid>/net/tcp for a line whose inode matches,
// then decodes the remote address from that line.
func lookupTCPInode(pid int, inode uint64) (string, error) {
	tcpFiles := []string{
		fmt.Sprintf("/proc/%d/net/tcp", pid),
		"/proc/net/tcp",
	}

	for _, path := range tcpFiles {
		addr, err := scanTCPFile(path, inode)
		if err == nil && addr != "" {
			return addr, nil
		}
	}
	return "", fmt.Errorf("inode %d not found in /proc/net/tcp", inode)
}

// scanTCPFile parses /proc/net/tcp looking for a line with the given inode.
// Each line has the format:
//
//	sl  local_address  rem_address  st  tx_queue:rx_queue  tr:tm->when  retrnsmt  uid  timeout  inode ...
func scanTCPFile(path string, inode uint64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Scan() // skip header line
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		var lineInode uint64
		if _, err := fmt.Sscanf(fields[9], "%d", &lineInode); err != nil {
			continue
		}
		if lineInode == inode {
			return decodeTCPAddr(fields[2])
		}
	}
	return "", fmt.Errorf("inode not found")
}

// decodeTCPAddr converts a hex "XXXXXXXX:PPPP" address from /proc/net/tcp
// to a "a.b.c.d:port" string.
func decodeTCPAddr(hex string) (string, error) {
	parts := strings.Split(hex, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid addr: %s", hex)
	}
	var ip uint32
	var port uint16
	if _, err := fmt.Sscanf(parts[0], "%08X", &ip); err != nil {
		return "", err
	}
	if _, err := fmt.Sscanf(parts[1], "%04X", &port); err != nil {
		return "", err
	}
	// /proc/net/tcp stores IPs in host byte order on little-endian systems
	b0 := ip & 0xff
	b1 := (ip >> 8) & 0xff
	b2 := (ip >> 16) & 0xff
	b3 := (ip >> 24) & 0xff
	return fmt.Sprintf("%d.%d.%d.%d:%d", b0, b1, b2, b3, port), nil
}

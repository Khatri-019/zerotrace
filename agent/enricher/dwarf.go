package enricher

import (
	"bufio"
	"debug/elf"
	"fmt"
	"os"
	"strings"
)

// ENDBR64 is the 4-byte Intel CET "End Branch 64" instruction that begins
// every function in CET-compiled shared libraries (Ubuntu 22.04+).
// The kernel uprobe infrastructure cannot plant a breakpoint on this opcode
// (errno 524 = ENOTSUPP on the perf_uprobe PMU).
// Skipping 4 bytes lands us on the actual first real instruction.
const endbr64Len = 4

// FindFunctionOffset returns the file offset of funcName inside the ELF binary
// at binaryPath, adjusted for ENDBR64 preambles that block uprobe attachment.
//
// The offset returned is suitable for use with link.UprobeOptions.Offset.
func FindFunctionOffset(binaryPath, funcName string) (uint64, error) {
	f, err := elf.Open(binaryPath)
	if err != nil {
		return 0, fmt.Errorf("open elf %s: %w", binaryPath, err)
	}
	defer f.Close()

	var symAddr uint64
	var found bool

	// 1. Static symbol table
	syms, err := f.Symbols()
	if err == nil {
		for _, sym := range syms {
			if sym.Name == funcName && elf.ST_TYPE(sym.Info) == elf.STT_FUNC {
				symAddr = sym.Value
				found = true
				break
			}
		}
	}

	// 2. Dynamic symbol table (shared libraries strip .symtab)
	if !found {
		dynsyms, err := f.DynamicSymbols()
		if err == nil {
			for _, sym := range dynsyms {
				if sym.Name == funcName && elf.ST_TYPE(sym.Info) == elf.STT_FUNC {
					symAddr = sym.Value
					found = true
					break
				}
			}
		}
	}

	if !found {
		return 0, fmt.Errorf("symbol %q not found in %s", funcName, binaryPath)
	}

	// Convert virtual address → file offset by walking section headers.
	fileOffset, err := vAddrToFileOffset(f, symAddr)
	if err != nil {
		// Fall back to using the raw virtual address (works on non-PIE binaries)
		fileOffset = symAddr
	}

	// Check if the function begins with ENDBR64 (f3 0f 1e fa).
	// If so, advance 4 bytes to skip past it — otherwise the kernel will
	// refuse to attach (errno 524: "possible trap insn").
	adjusted, err := skipENDBR64(binaryPath, fileOffset)
	if err == nil {
		fileOffset = adjusted
	}

	return fileOffset, nil
}

// vAddrToFileOffset converts an ELF virtual address to its file offset
// by finding the section that contains it and computing the delta.
func vAddrToFileOffset(f *elf.File, vaddr uint64) (uint64, error) {
	for _, sec := range f.Sections {
		if sec.Type == elf.SHT_NULL {
			continue
		}
		if vaddr >= sec.Addr && vaddr < sec.Addr+sec.Size {
			return (vaddr - sec.Addr) + sec.Offset, nil
		}
	}
	// Try program headers (for stripped binaries without section headers)
	for _, prog := range f.Progs {
		if prog.Type != elf.PT_LOAD {
			continue
		}
		if vaddr >= prog.Vaddr && vaddr < prog.Vaddr+prog.Filesz {
			return (vaddr - prog.Vaddr) + prog.Off, nil
		}
	}
	return 0, fmt.Errorf("vaddr 0x%x not found in any section or program header", vaddr)
}

// skipENDBR64 reads the 4 bytes at fileOffset in path and returns
// fileOffset+4 if they match the ENDBR64 opcode, otherwise returns fileOffset.
func skipENDBR64(path string, fileOffset uint64) (uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		return fileOffset, err
	}
	defer f.Close()

	buf := make([]byte, 4)
	n, err := f.ReadAt(buf, int64(fileOffset))
	if err != nil || n < 4 {
		return fileOffset, fmt.Errorf("short read")
	}

	// ENDBR64: f3 0f 1e fa
	if buf[0] == 0xf3 && buf[1] == 0x0f && buf[2] == 0x1e && buf[3] == 0xfa {
		return fileOffset + endbr64Len, nil
	}
	return fileOffset, nil
}

// GetOpenSSLLibPath scans /proc/<pid>/maps to find the libssl.so path loaded
// in that process. Returns the first match found.
func GetOpenSSLLibPath(pid int) (string, error) {
	mapsPath := fmt.Sprintf("/proc/%d/maps", pid)
	f, err := os.Open(mapsPath)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", mapsPath, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// Lines look like: "7f1a2b3c4000-7f1a2b3c5000 r-xp 00000000 08:01 12345 /lib/x86_64-linux-gnu/libssl.so.3"
		if strings.Contains(line, "libssl.so") {
			fields := strings.Fields(line)
			if len(fields) >= 6 {
				return fields[5], nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scanning %s: %w", mapsPath, err)
	}
	return "", fmt.Errorf("libssl.so not found in /proc/%d/maps", pid)
}

// GetOpenSSLProcList returns all PIDs whose memory maps include libssl.so.
// This is used by the uprobe scanner to know which processes to instrument.
func GetOpenSSLProcList() ([]int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("readdir /proc: %w", err)
	}

	var pids []int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		var pid int
		if _, err := fmt.Sscanf(e.Name(), "%d", &pid); err != nil {
			continue // Not a PID directory
		}
		mapsPath := fmt.Sprintf("/proc/%d/maps", pid)
		if hasLibSSL(mapsPath) {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

// hasLibSSL returns true if libssl.so appears in the given maps file.
func hasLibSSL(mapsPath string) bool {
	f, err := os.Open(mapsPath)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "libssl.so") {
			return true
		}
	}
	return false
}

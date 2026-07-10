package main

import (
	"fmt"

	"github.com/zerotrace/zerotrace/agent/enricher"
)

func main() {
	lib := "/usr/lib/x86_64-linux-gnu/libssl.so.3"

	for _, sym := range []string{"SSL_write", "SSL_read"} {
		off, err := enricher.FindFunctionOffset(lib, sym)
		if err != nil {
			fmt.Printf("ERROR %s: %v\n", sym, err)
			continue
		}
		fmt.Printf("%s → file offset: 0x%x (%d)\n", sym, off, off)
	}

	// Also verify boot time
	bootNs := enricher.BootTimeNs()
	fmt.Printf("Boot time (Unix ns): %d\n", bootNs)
	fmt.Printf("Boot time (seconds): %d\n", bootNs/1e9)
}

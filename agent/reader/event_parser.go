package reader

import (
	"errors"
)

func ParseEventType(data []byte) (uint8, error) {
    if len(data) < 32 { // minimum struct size
        return 0, errors.New("event too small")
    }
    // event_type is typically after standard headers
    // For tcp_event_t, it's at offset 32 (8+4+4+8+4+2+2+4+4 = 40 bytes)
    // Actually we should match the exact struct definition from common.h
    return data[len(data)-17], nil // Needs actual offset based on C structs
}
// Full implementation will unpack the bytes into Go structs matching common.h

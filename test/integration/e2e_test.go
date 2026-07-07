package integration

import "testing"

func TestEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}
	// Stub for e2e
}

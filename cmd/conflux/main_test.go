package main

import "testing"

// TestMainRuns exercises the main entrypoint to increase coverage for cmd/conflux.
func TestMainRuns(t *testing.T) {
	// Call main() — in normal operation this should return without calling os.Exit.
	main()
}

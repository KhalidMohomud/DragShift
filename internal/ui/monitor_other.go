//go:build !darwin

package ui

import "fmt"

// startEventMonitor is a stub for non-macOS platforms.
func startEventMonitor(ch chan<- []string) error {
	return fmt.Errorf("DragShift: shift+drag event monitoring is only supported on macOS")
}

// stopEventMonitor is a no-op on non-macOS platforms.
func stopEventMonitor() {}

//go:build darwin

package ui

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices
#include "wheel_darwin.h"
*/
import "C"

import (
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/ZakariyaSharif/dragshift/internal/converter"
)

//export goGetTargetsForFile
func goGetTargetsForFile(cPath *C.char) *C.char {
	path := C.GoString(cPath)
	targets := converter.TargetsFor(path)
	if len(targets) == 0 {
		return nil
	}
	return C.CString(strings.Join(targets, ","))
}

//export goPerformConversion
func goPerformConversion(cPaths *C.char, cTarget *C.char) {
	paths := strings.Split(C.GoString(cPaths), "\n")
	target := C.GoString(cTarget)
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		println("[DragShift] Converting " + filepath.Base(p) + " → ." + target + " ...")
		created, err := converter.Convert(p, target)
		if err != nil {
			println("[DragShift] ✗ Error: " + err.Error())
		} else {
			for _, c := range created {
				println("[DragShift] ✓ Saved: " + c)
			}
		}
	}
}

// startEventMonitor starts the macOS global event monitor.
func startEventMonitor() error {
	C.startNativeEventMonitor()
	return nil
}

// stopEventMonitor tears down the macOS global event monitor.
func stopEventMonitor() {
	C.stopNativeEventMonitor()
}

// Force the linker to keep the cgo export symbol alive.
var _ = unsafe.Pointer(nil)

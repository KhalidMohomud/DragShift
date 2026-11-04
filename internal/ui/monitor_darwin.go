//go:build darwin

package ui

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices
#include "wheel_darwin.h"
*/
import "C"

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
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
	var ok, failed int
	var lastErr string
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		println("[DragShift] Converting " + filepath.Base(p) + " → ." + target + " ...")
		created, err := converter.Convert(p, target)
		if err != nil {
			println("[DragShift] ✗ Error: " + err.Error())
			failed++
			lastErr = err.Error()
		} else {
			for _, c := range created {
				println("[DragShift] ✓ Saved: " + c)
			}
			ok++
		}
	}
	if failed > 0 {
		notify("Conversion failed", lastErr)
	} else if ok > 0 {
		notify("Converted to ."+target, "Saved next to the original file")
	}
}

// notify shows a macOS notification so results are visible without a terminal.
func notify(title, msg string) {
	if len(msg) > 200 {
		msg = msg[:200] + "…"
	}
	script := "display notification " + strconv.Quote(msg) + " with title \"DragShift\" subtitle " + strconv.Quote(title)
	_ = exec.Command("osascript", "-e", script).Run()
}

// startEventMonitor starts the macOS global event monitor.
func startEventMonitor() error {
	C.startNativeEventMonitor()
	return nil
}

// StopEventMonitor tears down the macOS global event monitor.
func stopEventMonitor() {
	C.stopNativeEventMonitor()
}

// OpenFileDialog opens the file and format choosers in a separate AppleScript
// process. DragShift is a menu-bar app, so AppKit modal panels can otherwise
// be hidden behind Finder or immediately dismissed during activation changes.
func OpenFileDialog() {
	go func() {
		fmt.Println("[DragShift] 📂 Opening file picker...")
		filePath, err := runAppleScript(`
activate
set selectedFile to choose file with prompt "Select a file to convert:"
return POSIX path of selectedFile
`)
		if err != nil {
			printDialogError("file selection", err)
			return
		}

		targets := converter.TargetsFor(filePath)
		if len(targets) == 0 {
			fmt.Printf("[DragShift] ✗ Unsupported file type: %s\n", filepath.Ext(filePath))
			return
		}
		fmt.Printf("[DragShift] ✅ Selected: %s\n", filePath)

		formatScript := "activate\nset formats to {" + appleScriptList(targets) + "}\n" +
			"set chosen to choose from list formats with prompt \"Choose an output format:\" " +
			"default items {item 1 of formats} with title \"DragShift\"\n" +
			"if chosen is false then error number -128\n" +
			"return item 1 of chosen\n"
		target, err := runAppleScript(formatScript)
		if err != nil {
			printDialogError("format selection", err)
			return
		}
		target = strings.ToLower(strings.TrimSpace(target))

		fmt.Printf("[DragShift] 🚀 Converting %s → .%s ...\n", filepath.Base(filePath), target)
		created, err := converter.Convert(filePath, target)
		if err != nil {
			fmt.Printf("[DragShift] ✗ Error: %v\n", err)
			notify("Conversion failed", err.Error())
			return
		}
		for _, output := range created {
			fmt.Printf("[DragShift] ✓ Saved: %s\n", output)
		}
		notify("Converted to ."+target, "Saved next to the original file")
	}()
}

func runAppleScript(script string) (string, error) {
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(out))
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("%s", message)
	}
	return strings.TrimSpace(string(out)), nil
}

func appleScriptList(values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = strconv.Quote(strings.ToUpper(value))
	}
	return strings.Join(quoted, ", ")
}

func printDialogError(action string, err error) {
	message := err.Error()
	if strings.Contains(strings.ToLower(message), "user canceled") || strings.Contains(message, "-128") {
		fmt.Printf("[DragShift] (%s cancelled)\n", action)
		return
	}
	fmt.Printf("[DragShift] ✗ %s failed: %s\n", action, message)
}

// Force the linker to keep the cgo export symbol alive.
var _ = unsafe.Pointer(nil)

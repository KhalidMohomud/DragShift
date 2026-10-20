// Package ui provides the macOS menu bar application for DragShift.
//
// It uses systray for a persistent menu bar icon and CGo-based event
// monitoring to detect Shift+drag gestures. When a file is dragged while
// holding Shift, available conversion formats are shown as menu items.
package ui

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ZakariyaSharif/dragshift/internal/converter"
	"github.com/getlantern/systray"
)

// App is the top-level DragShift application.
type App struct {
	mu sync.Mutex

	// formatItems holds the dynamically-created systray menu items for
	// each available target format. They are replaced every time a new
	// set of dragged files is detected.
	formatItems []*systray.MenuItem

	// draggedFiles stores the file paths detected from the most recent
	// Shift+drag gesture.
	draggedFiles []string

	// quit is closed when the user selects "Quit" from the menu bar.
	quit chan struct{}
}

// NewApp returns a new App ready to be Run.
func NewApp() *App {
	return &App{
		quit: make(chan struct{}),
	}
}

// Run starts the systray event loop and the Shift+drag event monitor.
// It blocks until the user quits the application.
func (a *App) Run() error {
	systray.Run(a.onReady, a.onExit)
	return nil
}

// Stop signals the application to shut down gracefully.
func (a *App) Stop() {
	select {
	case <-a.quit:
		// already closed
	default:
		close(a.quit)
	}
	systray.Quit()
}

// onReady is called by systray once the host menu bar is available.
func (a *App) onReady() {
	systray.SetTitle("⇧")
	systray.SetTooltip("DragShift — Hold ⇧ while dragging a file to convert it")

	fmt.Println("\n=======================================================")
	fmt.Println("🚀 DragShift is RUNNING!")
	fmt.Println("=======================================================")
	fmt.Println("• Menu Bar: Look for [ ⇧ ] in your Mac menu bar (top right corner).")
	fmt.Println("• How to use:")
	fmt.Println("  1. In Finder, drag any file (e.g. image, video, audio, pdf)")
	fmt.Println("  2. Hold the Shift (⇧) key while dragging")
	fmt.Println("  3. Release mouse, then click [ ⇧ ] in the menu bar to convert")
	fmt.Println("• To stop DragShift, press Ctrl+C in this terminal.")
	fmt.Println("=======================================================\n")

	mConvert := systray.AddMenuItem("Select File to Convert...", "Choose a file to convert")

	systray.AddSeparator()

	// Static informational item (disabled so it can't be clicked).
	info := systray.AddMenuItem("Or: Hold ⇧ while dragging a file in Finder", "")
	info.Disable()

	systray.AddSeparator()

	mQuit := systray.AddMenuItem("Quit", "Quit DragShift")

	// Start the native macOS event monitor in the background.
	go a.startMonitor()

	// Listen for menu actions.
	go func() {
		for {
			select {
			case <-mConvert.ClickedCh:
				fmt.Println("[DragShift] 'Select File to Convert...' clicked from menu bar.")
				OpenFileDialog()
			case <-mQuit.ClickedCh:
				a.Stop()
				return
			case <-a.quit:
				return
			}
		}
	}()
}

// onExit is called by systray when the event loop terminates.
func (a *App) onExit() {
	stopEventMonitor()
}

// startMonitor kicks off the platform-specific Shift+drag event monitor.
func (a *App) startMonitor() {
	if err := startEventMonitor(); err != nil {
		log.Printf("event monitor error: %v", err)
	}
}

// onFilesDetected is called when Shift+drag files are detected.
// It rebuilds the systray menu to show available target formats.
func (a *App) onFilesDetected(files []string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.draggedFiles = files

	// Remove previous dynamic format items.
	for _, item := range a.formatItems {
		item.Hide()
	}
	a.formatItems = nil

	if len(files) == 0 {
		return
	}

	// Determine available target formats from the first file's extension.
	ext := strings.ToLower(filepath.Ext(files[0]))
	formats := formatsForExtension(ext)

	if len(formats) == 0 {
		noFmt := systray.AddMenuItem("No conversions available", "")
		noFmt.Disable()
		a.formatItems = append(a.formatItems, noFmt)
		return
	}

	header := systray.AddMenuItem(fmt.Sprintf("Convert %d file(s):", len(files)), "")
	header.Disable()
	a.formatItems = append(a.formatItems, header)

	fmt.Println("[DragShift] Available target formats:")
	for _, f := range formats {
		fmt.Printf("   → Click '%s' under [ ⇧ ] in menu bar\n", f)
		item := systray.AddMenuItem(fmt.Sprintf("  → %s", f), fmt.Sprintf("Convert to %s", f))
		a.formatItems = append(a.formatItems, item)

		// Capture for goroutine.
		targetFmt := f
		go a.waitForFormatClick(item, targetFmt)
	}
}

// waitForFormatClick listens for a click on a format menu item and
// triggers the conversion.
func (a *App) waitForFormatClick(item *systray.MenuItem, targetFmt string) {
	select {
	case <-item.ClickedCh:
		a.mu.Lock()
		files := make([]string, len(a.draggedFiles))
		copy(files, a.draggedFiles)
		a.mu.Unlock()

		a.runConversion(files, targetFmt)
	case <-a.quit:
	}
}

// runConversion invokes the converter package for each file, converting
// it to the requested format.
func (a *App) runConversion(files []string, targetFmt string) {
	var anyOK, anyFail bool
	for _, f := range files {
		log.Printf("converting %s → %s", f, targetFmt)
		created, err := converter.Convert(f, targetFmt)
		if err != nil {
			log.Printf("✗ conversion failed for %s: %v", filepath.Base(f), err)
			anyFail = true
			continue
		}
		for _, c := range created {
			log.Printf("✓ created %s", filepath.Base(c))
		}
		anyOK = true
	}

	// Flash a status icon in the menu bar, then reset after 3 seconds.
	if anyFail && !anyOK {
		systray.SetTitle("✗")
	} else if anyOK {
		systray.SetTitle("✓")
	}
	go func() {
		time.Sleep(3 * time.Second)
		systray.SetTitle("⇧")
	}()
}

// formatsForExtension returns the list of formats a file with the given
// extension can be converted to. It delegates to the converter package.
func formatsForExtension(ext string) []string {
	// Create a dummy path with the extension to query the converter.
	return converter.TargetsFor("file" + ext)
}

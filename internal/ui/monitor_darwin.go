//go:build darwin

package ui

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

// _monitorRunning guards the NSEvent global monitor lifecycle.
static id _globalMonitor = nil;
static BOOL _monitorRunning = NO;

// Forward declaration of the Go callback (CGo generates 'char*', not 'const char*').
extern void goShiftDragCallback(char *paths);

// startNativeEventMonitor installs an NSEvent global monitor that watches
// for left-mouse-drag events while the Shift modifier is held. When such
// a gesture ends (mouse-up) it reads file URLs from the drag pasteboard
// and forwards them to Go via goShiftDragCallback.
static void startNativeEventMonitor(void) {
    if (_monitorRunning) return;
    _monitorRunning = YES;

    // We need a running NSApplication for global event monitors.
    [NSApplication sharedApplication];

    NSEventMask mask = NSEventMaskLeftMouseDragged | NSEventMaskLeftMouseUp;

    // Track whether we saw a shift-drag so we only fire once per gesture.
    __block BOOL sawShiftDrag = NO;

    _globalMonitor = [NSEvent addGlobalMonitorForEventsMatchingMask:mask
        handler:^(NSEvent *event) {
            if (event.type == NSEventTypeLeftMouseDragged) {
                if (event.modifierFlags & NSEventModifierFlagShift) {
                    sawShiftDrag = YES;
                }
                return;
            }

            // Mouse-up after a shift-drag.
            if (event.type == NSEventTypeLeftMouseUp && sawShiftDrag) {
                sawShiftDrag = NO;

                // Read file URLs from the general pasteboard. macOS puts
                // dragged file URLs on the general pasteboard when a drag
                // session begins from Finder.
                NSPasteboard *pb = [NSPasteboard generalPasteboard];
                NSArray<NSURL *> *urls = [pb readObjectsForClasses:@[[NSURL class]]
                                                           options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}];
                if (urls.count == 0) {
                    // Fallback: try the drag pasteboard.
                    pb = [NSPasteboard pasteboardWithName:NSPasteboardNameDrag];
                    urls = [pb readObjectsForClasses:@[[NSURL class]]
                                             options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}];
                }
                if (urls.count == 0) return;

                // Build a newline-separated list of file paths.
                NSMutableArray<NSString *> *pathList = [NSMutableArray array];
                for (NSURL *url in urls) {
                    if (url.isFileURL) {
                        [pathList addObject:url.path];
                    }
                }
                if (pathList.count == 0) return;

                NSString *joined = [pathList componentsJoinedByString:@"\n"];
                goShiftDragCallback((char *)[joined UTF8String]);
            }
        }];

    // Run the run-loop so the monitor receives events.
    // This call blocks; it is expected to be called from a goroutine.
    [[NSRunLoop currentRunLoop] run];
}

// stopNativeEventMonitor removes the global event monitor.
static void stopNativeEventMonitor(void) {
    if (_globalMonitor != nil) {
        [NSEvent removeMonitor:_globalMonitor];
        _globalMonitor = nil;
    }
    _monitorRunning = NO;
}
*/
import "C"

import (
	"strings"
	"unsafe"
)

// fileChan is the channel that startEventMonitor provides to the caller.
// The C callback writes into it.
var fileChan chan<- []string

//export goShiftDragCallback
func goShiftDragCallback(cPaths *C.char) {
	if fileChan == nil {
		return
	}
	paths := C.GoString(cPaths)
	if paths == "" {
		return
	}
	files := strings.Split(paths, "\n")

	// Non-blocking send; drop if the channel is full.
	select {
	case fileChan <- files:
	default:
	}
}

// startEventMonitor starts the macOS global event monitor for Shift+drag
// gestures. Detected file paths are sent to ch. This function blocks
// until the monitor is stopped.
func startEventMonitor(ch chan<- []string) error {
	fileChan = ch
	C.startNativeEventMonitor()
	return nil
}

// stopEventMonitor tears down the macOS global event monitor.
func stopEventMonitor() {
	C.stopNativeEventMonitor()
}

// Force the linker to keep the cgo export symbol alive.
var _ = unsafe.Pointer(nil)

//go:build darwin

package ui

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices

#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>

// _monitorRunning guards the NSEvent global monitor lifecycle.
static id _globalMonitor = nil;
static BOOL _monitorRunning = NO;

// Forward declaration of the Go callback (CGo generates 'char*', not 'const char*').
extern void goShiftDragCallback(char *paths);

static void checkInputMonitoringPermission(void) {
    if (!CGPreflightListenEventAccess()) {
        CGRequestListenEventAccess();
        printf("\n=======================================================\n");
        printf("⚠️  macOS INPUT MONITORING PERMISSION REQUIRED\n");
        printf("DragShift needs permission to detect the Shift key while dragging.\n");
        printf("1. Open System Settings > Privacy & Security > Input Monitoring\n");
        printf("2. Turn ON Terminal (or your current terminal app)\n");
        printf("3. Restart DragShift (press Ctrl+C, then 'make run')\n");
        printf("=======================================================\n\n");
        fflush(stdout);
    } else {
        printf("[DragShift] ✓ Input Monitoring permission: OK\n");
        fflush(stdout);
    }
}

// startNativeEventMonitor installs an NSEvent global monitor that watches
// for left-mouse-drag events while the Shift modifier is held. When such
// a gesture ends (mouse-up) it reads file URLs from the drag pasteboard
// and forwards them to Go via goShiftDragCallback.
static void startNativeEventMonitor(void) {
    if (_monitorRunning) return;
    _monitorRunning = YES;

    dispatch_async(dispatch_get_main_queue(), ^{
        checkInputMonitoringPermission();

        NSEventMask mask = NSEventMaskLeftMouseDragged | NSEventMaskLeftMouseUp;
        __block BOOL sawShiftDrag = NO;

        _globalMonitor = [NSEvent addGlobalMonitorForEventsMatchingMask:mask
            handler:^(NSEvent *event) {
                if (event.type == NSEventTypeLeftMouseDragged) {
                    if (event.modifierFlags & NSEventModifierFlagShift) {
                        if (!sawShiftDrag) {
                            printf("[DragShift] ⇧ Shift+drag active... release mouse when ready.\n");
                            fflush(stdout);
                        }
                        sawShiftDrag = YES;
                    }
                    return;
                }

                // Mouse-up after a shift-drag.
                if (event.type == NSEventTypeLeftMouseUp && sawShiftDrag) {
                    sawShiftDrag = NO;

                    // Read file URLs from drag pasteboard first, then general pasteboard.
                    NSPasteboard *pb = [NSPasteboard pasteboardWithName:NSPasteboardNameDrag];
                    NSArray<NSURL *> *urls = [pb readObjectsForClasses:@[[NSURL class]]
                                                             options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}];
                    if (urls.count == 0) {
                        pb = [NSPasteboard generalPasteboard];
                        urls = [pb readObjectsForClasses:@[[NSURL class]]
                                                 options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}];
                    }
                    if (urls.count == 0) {
                        printf("[DragShift] ℹ️ Mouse released, but no file URLs were found in drag pasteboard.\n");
                        fflush(stdout);
                        return;
                    }

                    // Build a newline-separated list of file paths.
                    NSMutableArray<NSString *> *pathList = [NSMutableArray array];
                    for (NSURL *url in urls) {
                        if (url.isFileURL) {
                            [pathList addObject:url.path];
                        }
                    }
                    if (pathList.count == 0) return;

                    printf("[DragShift] 🎯 Detected %lu file(s):\n", (unsigned long)pathList.count);
                    for (NSString *p in pathList) {
                        printf("   • %s\n", [p UTF8String]);
                    }
                    fflush(stdout);

                    NSString *joined = [pathList componentsJoinedByString:@"\n"];
                    goShiftDragCallback((char *)[joined UTF8String]);
                }
            }];

        if (_globalMonitor == nil) {
            printf("[DragShift] ⚠️ Warning: Failed to register global event monitor. Grant Input Monitoring permission in System Settings.\n");
            fflush(stdout);
        } else {
            printf("[DragShift] ✓ Shift+drag monitor is ACTIVE and ready.\n");
            fflush(stdout);
        }
    });
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
// gestures. Detected file paths are sent to ch.
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

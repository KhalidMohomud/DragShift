#import "wheel_darwin.h"
#import <ApplicationServices/ApplicationServices.h>
#import <math.h>
#import <stdlib.h>

#define R_OUT 128.0
#define R_IN 40.0
#define MARGIN 10.0
#define GAP_DEG 2.5

// Forward declarations of Go callbacks from CGo
extern char* goGetTargetsForFile(char *path);
extern void goPerformConversion(char *paths, char *target);

static inline int wedge_index(CGFloat dx, CGFloat dy, int n) {
    CGFloat dist = hypot(dx, dy);
    if (dist < R_IN || dist > R_OUT || n <= 0) {
        return -1;
    }
    CGFloat rad = atan2(dy, dx);
    CGFloat deg = rad * 180.0 / M_PI;
    CGFloat cw = fmod(90.0 - deg + 360.0, 360.0);
    int idx = (int)(cw / (360.0 / (CGFloat)n));
    if (idx < 0) idx = 0;
    if (idx >= n) idx = n - 1;
    return idx;
}

@interface DragShiftWheelView : NSView <NSDraggingDestination>
@property (nonatomic, strong) NSArray<NSString *> *formats;
@property (nonatomic, strong) NSArray<NSString *> *filePaths;
@property (nonatomic, assign) NSInteger hotIndex;
@end

@implementation DragShiftWheelView

- (instancetype)initWithFrame:(NSRect)frame formats:(NSArray<NSString *> *)formats files:(NSArray<NSString *> *)files {
    self = [super initWithFrame:frame];
    if (self) {
        _formats = formats;
        _filePaths = files;
        _hotIndex = -1;
        [self registerForDraggedTypes:@[NSPasteboardTypeFileURL]];
    }
    return self;
}

- (BOOL)isFlipped {
    return NO;
}

- (NSPoint)centerPoint {
    NSRect b = [self bounds];
    return NSMakePoint(NSMidX(b), NSMidY(b));
}

- (NSInteger)indexForPoint:(NSPoint)pt {
    NSPoint c = [self centerPoint];
    return wedge_index(pt.x - c.x, pt.y - c.y, (int)_formats.count);
}

- (void)setHotIndex:(NSInteger)idx {
    if (_hotIndex != idx) {
        _hotIndex = idx;
        [self setNeedsDisplay:YES];
    }
}

- (void)drawRect:(NSRect)dirtyRect {
    [super drawRect:dirtyRect];

    NSPoint c = [self centerPoint];
    NSUInteger n = _formats.count;
    if (n == 0) return;
    CGFloat span = 360.0 / (CGFloat)n;

    NSMutableParagraphStyle *style = [[NSMutableParagraphStyle alloc] init];
    style.alignment = NSTextAlignmentCenter;

    NSDictionary *textAttrs = @{
        NSFontAttributeName: [NSFont boldSystemFontOfSize:14],
        NSForegroundColorAttributeName: [NSColor whiteColor],
        NSParagraphStyleAttributeName: style
    };

    for (NSUInteger i = 0; i < n; i++) {
        CGFloat hi = 90.0 - (CGFloat)i * span - GAP_DEG;
        CGFloat lo = 90.0 - (CGFloat)(i + 1) * span + GAP_DEG;

        NSBezierPath *path = [NSBezierPath bezierPath];
        [path appendBezierPathWithArcWithCenter:c radius:R_OUT startAngle:lo endAngle:hi clockwise:NO];
        [path appendBezierPathWithArcWithCenter:c radius:R_IN startAngle:hi endAngle:lo clockwise:YES];
        [path closePath];

        if ((NSInteger)i == _hotIndex) {
            [[NSColor systemOrangeColor] setFill];
        } else {
            [[NSColor colorWithWhite:0.10 alpha:0.90] setFill];
        }
        [path fill];

        // Format label in wedge
        CGFloat midRad = (90.0 - ((CGFloat)i + 0.5) * span) * M_PI / 180.0;
        CGFloat rm = (R_IN + R_OUT) / 2.0;
        CGFloat tx = c.x + rm * cos(midRad);
        CGFloat ty = c.y + rm * sin(midRad);

        NSString *fmtText = [_formats[i] uppercaseString];
        NSRect textRect = NSMakeRect(tx - 38, ty - 10, 76, 20);
        [fmtText drawInRect:textRect withAttributes:textAttrs];
    }

    // Center circle
    NSBezierPath *centerCircle = [NSBezierPath bezierPathWithOvalInRect:NSMakeRect(c.x - R_IN, c.y - R_IN, R_IN * 2, R_IN * 2)];
    [[NSColor colorWithWhite:0.18 alpha:0.95] setFill];
    [centerCircle fill];

    NSString *hint = (_hotIndex >= 0 && (NSUInteger)_hotIndex < n) ? [_formats[_hotIndex] uppercaseString] : @"⇧";
    NSDictionary *centerAttrs = @{
        NSFontAttributeName: [NSFont boldSystemFontOfSize:17],
        NSForegroundColorAttributeName: [NSColor colorWithWhite:1.0 alpha:0.9],
        NSParagraphStyleAttributeName: style
    };
    [hint drawInRect:NSMakeRect(c.x - 35, c.y - 12, 70, 24) withAttributes:centerAttrs];
}

// Drag destination protocol
- (NSDragOperation)draggingEntered:(id<NSDraggingInfo>)sender {
    NSPoint pt = [self convertPoint:[sender draggingLocation] fromView:nil];
    NSInteger idx = [self indexForPoint:pt];
    [self setHotIndex:idx];
    return idx >= 0 ? NSDragOperationCopy : NSDragOperationNone;
}

- (NSDragOperation)draggingUpdated:(id<NSDraggingInfo>)sender {
    NSPoint pt = [self convertPoint:[sender draggingLocation] fromView:nil];
    NSInteger idx = [self indexForPoint:pt];
    [self setHotIndex:idx];
    return idx >= 0 ? NSDragOperationCopy : NSDragOperationNone;
}

- (void)draggingExited:(id<NSDraggingInfo>)sender {
    [self setHotIndex:-1];
}

- (BOOL)prepareForDragOperation:(id<NSDraggingInfo>)sender {
    NSPoint pt = [self convertPoint:[sender draggingLocation] fromView:nil];
    return [self indexForPoint:pt] >= 0;
}

- (BOOL)performDragOperation:(id<NSDraggingInfo>)sender {
    NSPoint pt = [self convertPoint:[sender draggingLocation] fromView:nil];
    NSInteger idx = [self indexForPoint:pt];
    if (idx < 0 || (NSUInteger)idx >= _formats.count) return NO;

    NSString *chosenFormat = _formats[idx];

    NSPasteboard *pb = [sender draggingPasteboard];
    NSArray<NSURL *> *urls = [pb readObjectsForClasses:@[[NSURL class]]
                                             options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}];
    NSMutableArray<NSString *> *paths = [NSMutableArray array];
    for (NSURL *u in urls) {
        if (u.isFileURL) [paths addObject:u.path];
    }
    if (paths.count == 0 && _filePaths.count > 0) {
        paths = [_filePaths mutableCopy];
    }

    if (paths.count > 0) {
        NSString *joined = [paths componentsJoinedByString:@"\n"];
        goPerformConversion((char *)[joined UTF8String], (char *)[chosenFormat UTF8String]);
    }

    [[self window] orderOut:nil];
    return YES;
}

@end

// Global references for panel
static NSPanel *_dropPanel = nil;
static DragShiftWheelView *_wheelView = nil;
static id _globalMonitor = nil;
static BOOL _monitorRunning = NO;

static void showDropWheel(NSArray<NSString *> *files, NSArray<NSString *> *formats) {
    if (formats.count == 0 || files.count == 0) return;

    if (_dropPanel != nil) {
        [_dropPanel orderOut:nil];
        _dropPanel = nil;
        _wheelView = nil;
    }

    CGFloat size = 2.0 * (R_OUT + MARGIN);
    NSPoint p = [NSEvent mouseLocation];
    NSScreen *scr = [NSScreen mainScreen];
    NSRect scrFrame = scr ? [scr frame] : NSMakeRect(0, 0, 1920, 1080);

    CGFloat x = MIN(MAX(p.x - size / 2.0, 4.0), scrFrame.size.width - size - 4.0);
    CGFloat y = MIN(MAX(p.y - size / 2.0, 4.0), scrFrame.size.height - size - 4.0);

    _dropPanel = [[NSPanel alloc] initWithContentRect:NSMakeRect(x, y, size, size)
                                            styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel
                                              backing:NSBackingStoreBuffered
                                                defer:NO];
    [_dropPanel setLevel:kCGOverlayWindowLevel];
    [_dropPanel setOpaque:NO];
    [_dropPanel setBackgroundColor:[NSColor clearColor]];
    [_dropPanel setHasShadow:NO];
    [_dropPanel setHidesOnDeactivate:NO];

    _wheelView = [[DragShiftWheelView alloc] initWithFrame:NSMakeRect(0, 0, size, size)
                                                   formats:formats
                                                     files:files];
    [_dropPanel setContentView:_wheelView];

    [_dropPanel setAlphaValue:0.0];
    [_dropPanel orderFrontRegardless];
    [[_dropPanel animator] setAlphaValue:1.0];
}

static void hideDropWheel(void) {
    if (_dropPanel != nil) {
        [_dropPanel orderOut:nil];
        _dropPanel = nil;
        _wheelView = nil;
    }
}

static void checkInputMonitoringPermission(void) {
    if (!CGPreflightListenEventAccess()) {
        CGRequestListenEventAccess();
        printf("\n=======================================================\n");
        printf("⚠️  macOS INPUT MONITORING PERMISSION REQUIRED\n");
        printf("DragShift needs permission to detect the Shift key while dragging.\n");
        printf("1. Open System Settings > Privacy & Security > Input Monitoring\n");
        printf("2. Turn ON Terminal (or your terminal app)\n");
        printf("3. Restart DragShift\n");
        printf("=======================================================\n\n");
        fflush(stdout);
    } else {
        printf("[DragShift] ✓ Input Monitoring permission: OK\n");
        fflush(stdout);
    }
}

static NSArray<NSString *> *getSelectedFilesInFinder(void) {
    NSString *src = @"tell application \"Finder\"\n"
                    @"set sel to selection as alias list\n"
                    @"set res to \"\"\n"
                    @"repeat with itemRef in sel\n"
                    @"    set res to res & POSIX path of itemRef & linefeed\n"
                    @"end repeat\n"
                    @"return res\n"
                    @"end tell";
    NSAppleScript *as = [[NSAppleScript alloc] initWithSource:src];
    NSDictionary *err = nil;
    NSAppleEventDescriptor *desc = [as executeAndReturnError:&err];
    if (desc != nil) {
        NSString *outStr = [desc stringValue];
        if (outStr.length > 0) {
            NSArray *lines = [outStr componentsSeparatedByCharactersInSet:[NSCharacterSet newlineCharacterSet]];
            NSMutableArray *res = [NSMutableArray array];
            for (NSString *l in lines) {
                if (l.length > 0 && [[NSFileManager defaultManager] fileExistsAtPath:l]) {
                    [res addObject:l];
                }
            }
            return res;
        }
    }
    return @[];
}

void startNativeEventMonitor(void) {
    if (_monitorRunning) return;
    _monitorRunning = YES;

    dispatch_async(dispatch_get_main_queue(), ^{
        checkInputMonitoringPermission();

        NSEventMask mask = NSEventMaskLeftMouseDragged | NSEventMaskLeftMouseUp;
        __block BOOL sawShiftDrag = NO;

        _globalMonitor = [NSEvent addGlobalMonitorForEventsMatchingMask:mask
            handler:^(NSEvent *event) {
                if (event.type == NSEventTypeLeftMouseDragged) {
                    BOOL shiftPressed = (event.modifierFlags & NSEventModifierFlagShift) != 0;
                    if (shiftPressed) {
                        sawShiftDrag = YES;
                        if (_dropPanel == nil) {
                            // Fetch dragged files from pasteboard
                            NSPasteboard *pb = [NSPasteboard pasteboardWithName:NSPasteboardNameDrag];
                            NSArray<NSURL *> *urls = [pb readObjectsForClasses:@[[NSURL class]]
                                                                     options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}];
                            if (urls.count == 0) {
                                pb = [NSPasteboard generalPasteboard];
                                urls = [pb readObjectsForClasses:@[[NSURL class]]
                                                         options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}];
                            }
                            NSMutableArray<NSString *> *files = [NSMutableArray array];
                            if (urls.count > 0) {
                                for (NSURL *u in urls) {
                                    if (u.isFileURL) [files addObject:u.path];
                                }
                            }
                            if (files.count == 0) {
                                NSArray<NSString *> *finderSel = getSelectedFilesInFinder();
                                if (finderSel.count > 0) {
                                    [files addObjectsFromArray:finderSel];
                                }
                            }

                            if (files.count > 0) {
                                char *cTargets = goGetTargetsForFile((char *)[files[0] UTF8String]);
                                if (cTargets != NULL) {
                                    NSString *str = [NSString stringWithUTF8String:cTargets];
                                    free(cTargets);
                                    if (str.length > 0) {
                                        NSArray<NSString *> *fmts = [str componentsSeparatedByString:@","];
                                        printf("\n[DragShift] 🎡 Shift+Drag: Showing conversion wheel for %s!\n",
                                               [files[0].lastPathComponent UTF8String]);
                                        fflush(stdout);
                                        showDropWheel(files, fmts);
                                    }
                                }
                            }
                        } else {
                            // Update hover state
                            NSPoint mouseLoc = [NSEvent mouseLocation];
                            NSRect frame = [_dropPanel frame];
                            CGFloat dx = mouseLoc.x - (frame.origin.x + frame.size.width / 2.0);
                            CGFloat dy = mouseLoc.y - (frame.origin.y + frame.size.height / 2.0);
                            int idx = wedge_index(dx, dy, (int)_wheelView.formats.count);
                            [_wheelView setHotIndex:idx];
                        }
                    } else {
                        if (_dropPanel != nil) {
                            hideDropWheel();
                        }
                    }
                    return;
                }

                // Mouse up: convert if dropped on a wedge
                if (event.type == NSEventTypeLeftMouseUp) {
                    if (_dropPanel != nil && sawShiftDrag) {
                        sawShiftDrag = NO;
                        NSPoint mouseLoc = [NSEvent mouseLocation];
                        NSRect frame = [_dropPanel frame];
                        CGFloat dx = mouseLoc.x - (frame.origin.x + frame.size.width / 2.0);
                        CGFloat dy = mouseLoc.y - (frame.origin.y + frame.size.height / 2.0);
                        int idx = wedge_index(dx, dy, (int)_wheelView.formats.count);

                        if (idx >= 0 && (NSUInteger)idx < _wheelView.formats.count) {
                            NSString *fmt = _wheelView.formats[idx];
                            NSString *joined = [_wheelView.filePaths componentsJoinedByString:@"\n"];
                            printf("[DragShift] 🎯 Dropped on [%s]!\n", [fmt.uppercaseString UTF8String]);
                            fflush(stdout);
                            goPerformConversion((char *)[joined UTF8String], (char *)[fmt UTF8String]);
                        }

                        dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.15 * NSEC_PER_SEC)), dispatch_get_main_queue(), ^{
                            hideDropWheel();
                        });
                    }
                    sawShiftDrag = NO;
                }
            }];

        if (_globalMonitor == nil) {
            printf("[DragShift] ⚠️ Warning: Failed to register global event monitor. Grant Input Monitoring permission in System Settings.\n");
            fflush(stdout);
        } else {
            printf("[DragShift] ✓ Shift+drag wheel monitor is ACTIVE and ready.\n");
            fflush(stdout);
        }
    });
}

void stopNativeEventMonitor(void) {
    if (_globalMonitor != nil) {
        [NSEvent removeMonitor:_globalMonitor];
        _globalMonitor = nil;
    }
    hideDropWheel();
    _monitorRunning = NO;
}

void openFileDialogAndConvert(void) {
    printf("[DragShift] 📂 'Select File to Convert...' triggered. Opening file chooser...\n");
    fflush(stdout);

    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.15 * NSEC_PER_SEC)), dispatch_get_main_queue(), ^{
        [NSApp activateIgnoringOtherApps:YES];

        NSOpenPanel *panel = [NSOpenPanel openPanel];
        [panel setCanChooseFiles:YES];
        [panel setCanChooseDirectories:NO];
        [panel setAllowsMultipleSelection:YES];
        [panel setMessage:@"Select files you want to convert:"];
        [panel setPrompt:@"Choose"];
        [panel setLevel:NSFloatingWindowLevel];

        NSModalResponse resp = [panel runModal];
        printf("[DragShift] File dialog closed. Response code: %ld\n", (long)resp);
        fflush(stdout);

        NSArray<NSURL *> *urls = (resp == NSModalResponseOK) ? [panel URLs] : @[];
        NSMutableArray<NSString *> *files = [NSMutableArray array];
        for (NSURL *u in urls) {
            if (u.isFileURL) [files addObject:u.path];
        }

        // If NSOpenPanel was dismissed or didn't get files, try AppleScript fallback
        if (files.count == 0 && resp != NSModalResponseCancel) {
            NSString *script = @"tell application \"System Events\"\n"
                               @"activate\n"
                               @"set f to choose file with prompt \"Select a file to convert:\"\n"
                               @"return POSIX path of f\n"
                               @"end tell";
            NSAppleScript *as = [[NSAppleScript alloc] initWithSource:script];
            NSDictionary *err = nil;
            NSAppleEventDescriptor *desc = [as executeAndReturnError:&err];
            if (desc != nil) {
                NSString *p = [desc stringValue];
                if (p.length > 0) {
                    [files addObject:p];
                }
            }
        }

        if (files.count == 0) {
            printf("[DragShift] (No files selected)\n");
            fflush(stdout);
            return;
        }

        char *cTargets = goGetTargetsForFile((char *)[files[0] UTF8String]);
        if (cTargets == NULL) {
            [NSApp activateIgnoringOtherApps:YES];
            NSAlert *noTargetAlert = [[NSAlert alloc] init];
            [noTargetAlert setMessageText:@"No Formats Available"];
            [noTargetAlert setInformativeText:[NSString stringWithFormat:@"File format .%@ cannot be converted.", [files[0] pathExtension]]];
            [noTargetAlert runModal];
            return;
        }

        NSString *str = [NSString stringWithUTF8String:cTargets];
        free(cTargets);
        NSArray<NSString *> *fmts = [str componentsSeparatedByString:@","];
        if (fmts.count == 0) return;

        // Prompt user with format choices
        [NSApp activateIgnoringOtherApps:YES];
        NSAlert *alert = [[NSAlert alloc] init];
        [alert setMessageText:[NSString stringWithFormat:@"Convert %@", [files[0] lastPathComponent]]];
        [alert setInformativeText:@"Select target format:"];

        for (NSString *fmt in fmts) {
            [alert addButtonWithTitle:[fmt uppercaseString]];
        }
        [alert addButtonWithTitle:@"Cancel"];

        NSModalResponse alertResp = [alert runModal];
        NSInteger buttonIndex = alertResp - NSAlertFirstButtonReturn;

        if (buttonIndex >= 0 && (NSUInteger)buttonIndex < fmts.count) {
            NSString *chosenFormat = fmts[buttonIndex];
            NSString *joined = [files componentsJoinedByString:@"\n"];
            printf("[DragShift] 🚀 Converting via dialog to [%s]...\n", [chosenFormat.uppercaseString UTF8String]);
            fflush(stdout);
            goPerformConversion((char *)[joined UTF8String], (char *)[chosenFormat UTF8String]);
        }
    });
}

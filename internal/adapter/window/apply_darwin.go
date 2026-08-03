//go:build darwin

package window

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework AppKit
#import <Cocoa/Cocoa.h>
#import <dispatch/dispatch.h>

static void set_ignores_mouse_events(int ignore) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSApplication *app = [NSApplication sharedApplication];
        NSWindow *window = [app mainWindow];
        if (window == nil) {
            window = [app keyWindow];
        }
        if (window == nil) {
            window = [[app windows] firstObject];
        }
        if (window) {
            [window setIgnoresMouseEvents:(BOOL)ignore];
        }
    });
}
*/
import "C"

import "interagent/internal/port"

func applyMode(mode port.OverlayMode) error {
	ignore := 0
	if mode == port.OverlayModeClickThrough {
		ignore = 1
	}
	C.set_ignores_mouse_events(C.int(ignore))
	return nil
}

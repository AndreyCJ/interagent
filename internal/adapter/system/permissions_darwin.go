//go:build darwin

package system

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework AVFoundation -framework CoreGraphics -framework ApplicationServices -framework Foundation

#import <AVFoundation/AVFoundation.h>
#import <CoreGraphics/CoreGraphics.h>
#import <ApplicationServices/ApplicationServices.h>
#import <dispatch/dispatch.h>

// Microphone authorization status: 3 == AVAuthorizationStatusAuthorized.
static int ia_mic_status(void) {
    return (int)[AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeAudio];
}

// Triggers the microphone permission prompt and waits for the user's answer.
// Returns 1 if granted, 0 otherwise. Blocks up to 30s.
static int ia_mic_request(void) {
    __block int granted = 0;
    dispatch_semaphore_t sem = dispatch_semaphore_create(0);
    [AVCaptureDevice requestAccessForMediaType:AVMediaTypeAudio completionHandler:^(BOOL ok) {
        granted = ok ? 1 : 0;
        dispatch_semaphore_signal(sem);
    }];
    dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, 30 * NSEC_PER_SEC));
    return granted;
}

// Shows the Accessibility trust prompt (kAXTrustedCheckOptionPrompt) once.
// Returns 1 if trusted after the prompt, 0 otherwise.
static int ia_accessibility_request(void) {
    CFStringRef key = CFSTR("AXTrustedCheckOptionPrompt");
    CFTypeRef val = kCFBooleanTrue;
    CFDictionaryRef options = CFDictionaryCreate(NULL, (const void **)&key, &val, 1,
                                                 &kCFTypeDictionaryKeyCallBacks,
                                                 &kCFTypeDictionaryValueCallBacks);
    Boolean trusted = AXIsProcessTrustedWithOptions(options);
    CFRelease(options);
    return trusted ? 1 : 0;
}
*/
import "C"

import (
	"errors"
	"os/exec"

	"interagent/internal/adapter/portal"
	"interagent/internal/port"
)

// Permissions queries and mutates the macOS TCC grants backing the app's
// permissions (ADR-008). Screen recording is checked via CGPreflight, the
// microphone via AVFoundation, Accessibility via AX API.
type Permissions struct{}

func NewPermissions(_ *portal.ScreenCast, _ func() error) *Permissions { return &Permissions{} }

func (p *Permissions) Status(perm port.Permission) (bool, error) {
	switch perm {
	case port.PermissionMicrophone:
		return C.ia_mic_status() == 3, nil
	case port.PermissionScreenCapture:
		return bool(C.CGPreflightScreenCaptureAccess()), nil
	case port.PermissionAccessibility:
		return C.AXIsProcessTrusted() != 0, nil
	default:
		return false, errors.New("unknown permission: " + string(perm))
	}
}

func (p *Permissions) Request(perm port.Permission) error {
	switch perm {
	case port.PermissionMicrophone:
		C.ia_mic_request()
		return nil
	case port.PermissionScreenCapture:
		C.CGRequestScreenCaptureAccess()
		return nil
	case port.PermissionAccessibility:
		C.ia_accessibility_request()
		return nil
	default:
		return errors.New("unknown permission: " + string(perm))
	}
}

func (p *Permissions) OpenSettings(perm port.Permission) error {
	url, ok := settingsURL(perm)
	if !ok {
		return errors.New("unknown permission: " + string(perm))
	}
	return exec.Command("open", url).Run()
}

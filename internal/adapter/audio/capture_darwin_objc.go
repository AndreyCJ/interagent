//go:build darwin

// Objective-C glue for the audio capture adapters. The delegates and session
// helpers live here so the preamble compiles exactly once; the Go callback
// bridges are exported in capture_darwin.go (cgo would otherwise duplicate the
// ObjC class symbols into _cgo_export.c and the link would fail).
package audio

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework AVFoundation -framework CoreMedia -framework CoreVideo -framework ScreenCaptureKit -framework Foundation

#import <AVFoundation/AVFoundation.h>
#import <CoreMedia/CoreMedia.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>
#import <dispatch/dispatch.h>
#import <stdint.h>

// Callbacks from ObjC delegate threads into Go (registered per capture).
typedef void (*go_audio_cb)(void *user, float *data, int len);
typedef void (*go_stop_cb)(void *user, char *desc);

// Go callback bridges (exported in capture_darwin.go; declared here so the
// delegate setters can reference them from the preamble).
void iaGoAudioCallback(void *user, float *data, int len);
void iaSystemStopCallback(void *user, char *desc);

@interface IAMicDelegate : NSObject <AVCaptureAudioDataOutputSampleBufferDelegate>
@property (nonatomic, assign) void *goUser;
@property (nonatomic, assign) go_audio_cb goCb;
@end

@implementation IAMicDelegate
- (void)captureOutput:(AVCaptureOutput *)captureOutput
  didOutputSampleBuffer:(CMSampleBufferRef)sampleBuffer
  fromConnection:(AVCaptureConnection *)connection {
    if (!self.goCb) return;
    CMBlockBufferRef block = CMSampleBufferGetDataBuffer(sampleBuffer);
    if (!block) return;
    size_t len = 0;
    char *ptr = NULL;
    CMBlockBufferGetDataPointer(block, 0, NULL, &len, &ptr);
    if (!ptr || len < 4) return;
    self.goCb(self.goUser, (float *)(void *)ptr, (int)(len / 4));
}
@end

// SystemCaptureDelegate receives SCStream audio sample buffers.
@interface IASystemDelegate : NSObject <SCStreamOutput, SCStreamDelegate>
@property (nonatomic, assign) void *goUser;
@property (nonatomic, assign) go_audio_cb goCb;
// Asynchronous stop/error surfacing (e.g. missing Screen Recording permission).
@property (nonatomic, assign) void *stopUser;
@property (nonatomic, assign) go_stop_cb stopCb;
@end

@implementation IASystemDelegate
- (void)stream:(SCStream *)stream
    didOutputSampleBuffer:(CMSampleBufferRef)sampleBuffer
    ofType:(SCStreamOutputType)type {
    if (type != SCStreamOutputTypeAudio || !self.goCb) return;
    CMBlockBufferRef block = CMSampleBufferGetDataBuffer(sampleBuffer);
    if (!block) return;
    size_t len = 0;
    char *ptr = NULL;
    CMBlockBufferGetDataPointer(block, 0, NULL, &len, &ptr);
    if (!ptr || len < 4) return;
    self.goCb(self.goUser, (float *)(void *)ptr, (int)(len / 4));
}
- (void)stream:(SCStream *)stream didStopWithError:(NSError *)error {
    // surface asynchronously via the captured channel (see below)
    if (self.stopCb) {
        NSString *desc = error ? [error localizedDescription] : @"stream stopped";
        self.stopCb(self.stopUser, (char *)[desc UTF8String]);
    }
}
@end

void *ia_mic_session(void) { return (void *)CFBridgingRetain([[AVCaptureSession alloc] init]); }
void ia_mic_release(void *p) { CFBridgingRelease(p); }
void ia_system_release(void *p) { CFBridgingRelease(p); }

// Delegate factories: CFBridgingRetain hands an extra +1 retain to Go; the
// matching *_release helpers drop it on Stop.
void *iamic_new(void) { return (void *)CFBridgingRetain([[IAMicDelegate alloc] init]); }
void iamic_release(void *p) { CFBridgingRelease(p); }
void *iasystem_new(void) { return (void *)CFBridgingRetain([[IASystemDelegate alloc] init]); }
void iasystem_release(void *p) { CFBridgingRelease(p); }

void iamic_set_cb(void *delegate, uintptr_t user) {
    IAMicDelegate *d = (__bridge IAMicDelegate *)delegate;
    d.goUser = (void *)user;
    d.goCb = (go_audio_cb)iaGoAudioCallback;
}

void iasystem_set_cb(void *delegate, uintptr_t user) {
    IASystemDelegate *d = (__bridge IASystemDelegate *)delegate;
    d.goUser = (void *)user;
    d.goCb = (go_audio_cb)iaGoAudioCallback;
}

void iasystem_set_stop(void *delegate, uintptr_t user) {
    IASystemDelegate *d = (__bridge IASystemDelegate *)delegate;
    d.stopUser = (void *)user;
    d.stopCb = (go_stop_cb)iaSystemStopCallback;
}

void iasystem_clear_stop(void *delegate) {
    IASystemDelegate *d = (__bridge IASystemDelegate *)delegate;
    d.stopUser = NULL;
    d.stopCb = NULL;
}

// Builds an AVCaptureSession from the default audio input and starts it.
// Returns 0 on success, -1 on failure (message written into errBuf).
int iamic_start_session(void *session, void *delegate, char *errBuf, int errLen) {
    AVCaptureSession *s = (__bridge AVCaptureSession *)session;
    IAMicDelegate *d = (__bridge IAMicDelegate *)delegate;

    AVCaptureDevice *device = [AVCaptureDevice defaultDeviceWithMediaType:AVMediaTypeAudio];
    if (!device) {
        snprintf(errBuf, errLen, "no default audio input device");
        return -1;
    }
    NSError *err = nil;
    AVCaptureDeviceInput *input = [AVCaptureDeviceInput deviceInputWithDevice:device error:&err];
    if (!input) {
        snprintf(errBuf, errLen, "cannot create audio input: %s",
                 err ? [[err localizedDescription] UTF8String] : "unknown error");
        return -1;
    }
    AVCaptureAudioDataOutput *output = [[AVCaptureAudioDataOutput alloc] init];
    output.audioSettings = @{
        AVFormatIDKey: @(kAudioFormatLinearPCM),
        AVNumberOfChannelsKey: @1,
        AVLinearPCMBitDepthKey: @32,
        AVLinearPCMIsFloatKey: @YES,
        AVLinearPCMIsBigEndianKey: @NO,
        AVLinearPCMIsNonInterleavedKey: @NO,
    };
    [output setSampleBufferDelegate:d
                             queue:dispatch_get_global_queue(DISPATCH_QUEUE_PRIORITY_HIGH, 0)];

    if ([s canAddInput:input]) {
        [s addInput:input];
    } else {
        snprintf(errBuf, errLen, "session cannot add audio input");
        return -1;
    }
    if ([s canAddOutput:output]) {
        [s addOutput:output];
    } else {
        snprintf(errBuf, errLen, "session cannot add audio output");
        return -1;
    }
    [s startRunning];
    return 0;
}

void iamic_stop_session(void *session) {
    if (!session) return;
    AVCaptureSession *s = (__bridge AVCaptureSession *)session;
    if (s.running) {
        [s stopRunning];
    }
}

// Creates an audio-only SCStream (all system audio) and starts capture.
// Blocks until the async SCShareableContent / startCapture chain resolves
// (or times out after 10s). Returns a retained SCStream, or NULL.
void *iasystem_start(void *delegate, char *errBuf, int errLen) {
    __block SCStream *stream = nil;
    __block NSString *errDesc = nil;
    dispatch_semaphore_t sem = dispatch_semaphore_create(0);

    [SCShareableContent getShareableContentWithCompletionHandler:^(SCShareableContent *content, NSError *error) {
        if (error) {
            errDesc = [NSString stringWithFormat:@"SCError %ld (%@): %@ | userInfo=%@",
                       (long)error.code, error.domain, error.localizedDescription, error.userInfo];
            dispatch_semaphore_signal(sem);
            return;
        }
        // Include all applications instead of excluding an empty window list:
        // with `initWithDisplay:excludingWindows:@[]` some macOS versions start
        // the stream but never deliver buffers (see ADR-012).
        SCContentFilter *filter = [[SCContentFilter alloc] initWithDisplay:content.displays.firstObject
                                                       includingApplications:content.applications
                                                            exceptingWindows:@[]];
        if (!filter) {
            errDesc = @"no shareable display found";
            dispatch_semaphore_signal(sem);
            return;
        }
        SCDisplay *display = content.displays.firstObject;
        SCStreamConfiguration *config = [[SCStreamConfiguration alloc] init];
        // Real pixel dimensions are required: a 1x1 audio-only config makes the
        // daemon fail to materialize the stream (SCError 1003 kCGErrorInvalidConnection,
        // "The stream is nil") on macOS 26 (see ADR-012).
        config.width = display.width > 0 ? display.width : 1920;
        config.height = display.height > 0 ? display.height : 1080;
        config.sampleRate = 48000;
        config.channelCount = 2;
        config.queueDepth = 8;
        config.capturesAudio = YES;
        config.excludesCurrentProcessAudio = NO;
        config.showsCursor = NO;

        stream = [[SCStream alloc] initWithFilter:filter
                                    configuration:config
                                         delegate:(__bridge id<SCStreamDelegate>)delegate];
        NSError *outErr = nil;
        BOOL added = [stream addStreamOutput:(__bridge id<SCStreamOutput>)delegate
                                        type:SCStreamOutputTypeAudio
                          sampleHandlerQueue:dispatch_get_global_queue(DISPATCH_QUEUE_PRIORITY_HIGH, 0)
                                       error:&outErr];
        if (!added) {
            errDesc = outErr ? [NSString stringWithFormat:@"SCError %ld (%@): %@ | userInfo=%@",
                                        (long)outErr.code, outErr.domain, outErr.localizedDescription, outErr.userInfo]
                             : @"cannot add audio stream output";
            dispatch_semaphore_signal(sem);
            return;
        }
        [stream startCaptureWithCompletionHandler:^(NSError *err) {
            if (err) {
                errDesc = [NSString stringWithFormat:@"SCError %ld (%@): %@ | userInfo=%@",
                           (long)err.code, err.domain, err.localizedDescription, err.userInfo];
                // Tear the stream down so a failed start does not leave a
                // half-open session registered with the SCK service (which
                // makes later startCapture calls fail with 1003).
                [stream stopCaptureWithCompletionHandler:nil];
                stream = nil;
            }
            dispatch_semaphore_signal(sem);
        }];
    }];

    dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC));
    if (errDesc) {
        snprintf(errBuf, errLen, "%s", [errDesc UTF8String]);
        return NULL;
    }
    if (!stream) {
        snprintf(errBuf, errLen, "timed out starting stream");
        return NULL;
    }
    return (void *)CFBridgingRetain(stream);
}

void iasystem_stop(void *stream) {
    if (!stream) return;
    SCStream *s = (__bridge SCStream *)stream;
    [s stopCaptureWithCompletionHandler:nil];
}

*/
import "C"

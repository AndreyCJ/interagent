//go:build darwin

package audio

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework AVFoundation -framework CoreMedia -framework CoreVideo -framework ScreenCaptureKit -framework Foundation

#import <stdint.h>

// ObjC capture helpers, implemented in capture_darwin_objc.go. Declared here so
// the Go side can call them. This preamble intentionally defines no ObjC classes:
// cgo copies this preamble into _cgo_export.c (because of the //export functions
// below), and global ObjC class symbols would otherwise be linked twice.
void *ia_mic_session(void);
void ia_mic_release(void *p);
void ia_system_release(void *p);
void *iamic_new(void);
void iamic_release(void *p);
void iamic_set_cb(void *delegate, uintptr_t user);
int iamic_start_session(void *session, void *delegate, char *errBuf, int errLen);
void iamic_stop_session(void *session);
void *iasystem_new(void);
void iasystem_release(void *p);
void iasystem_set_cb(void *delegate, uintptr_t user);
void iasystem_set_stop(void *delegate, uintptr_t user);
void iasystem_clear_stop(void *delegate);
void *iasystem_start(void *delegate, char *errBuf, int errLen);
void iasystem_stop(void *stream);
*/
import "C"

import (
	"errors"
	"sync"
	"unsafe"

	"interagent/internal/port"
)

var (
	cbMu      sync.Mutex
	callbacks = map[unsafe.Pointer]func([]float32){}
)

//export iaGoAudioCallback
func iaGoAudioCallback(user unsafe.Pointer, data *C.float, length C.int) {
	cbMu.Lock()
	cb := callbacks[user]
	cbMu.Unlock()
	if cb == nil || length <= 0 || data == nil {
		return
	}
	samples := unsafe.Slice((*float32)(unsafe.Pointer(data)), int(length))
	cb(samples)
}

//export iaSystemStopCallback
func iaSystemStopCallback(user unsafe.Pointer, desc *C.char) {
	s := (*SystemCapture)(user)
	s.mu.Lock()
	started := s.started
	s.mu.Unlock()
	if !started {
		return
	}
	msg := C.GoString(desc)
	select {
	case s.stopErr <- errors.New("system capture stopped: " + msg):
	default:
	}
}

type MicrophoneCapture struct {
	mu       sync.Mutex
	session  unsafe.Pointer
	delegate unsafe.Pointer
	userPtr  unsafe.Pointer
	onChunk  func([]byte)
	started  bool
	stopErr  chan error
}

func NewMicrophoneCapture() *MicrophoneCapture {
	return &MicrophoneCapture{stopErr: make(chan error, 1)}
}

func (m *MicrophoneCapture) Start(onChunk func([]byte)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return nil
	}
	if onChunk == nil {
		return errors.New("microphone capture: nil onChunk")
	}

	session := C.ia_mic_session()
	if session == nil {
		return errors.New("avcapture: session init failed")
	}
	delegate := C.iamic_new()
	if delegate == nil {
		C.ia_mic_release(session)
		return errors.New("avcapture: mic delegate init failed")
	}
	C.iamic_set_cb(delegate, C.uintptr_t(uintptr(unsafe.Pointer(m))))

	userPtr := unsafe.Pointer(m)
	cbMu.Lock()
	callbacks[userPtr] = func(frames []float32) { m.pump(frames) }
	cbMu.Unlock()

	m.session = session
	m.delegate = delegate
	m.userPtr = userPtr
	m.onChunk = onChunk

	var errBuf [512]C.char
	if C.iamic_start_session(session, delegate, &errBuf[0], C.int(len(errBuf))) != 0 {
		msg := C.GoString(&errBuf[0])
		if msg == "" {
			msg = "avcapture: session start failed"
		}
		C.iamic_release(delegate)
		C.ia_mic_release(session)
		cbMu.Lock()
		delete(callbacks, userPtr)
		cbMu.Unlock()
		m.session = nil
		m.delegate = nil
		m.userPtr = nil
		m.onChunk = nil
		return errors.New(msg)
	}

	m.started = true
	return nil
}

func (m *MicrophoneCapture) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.started {
		return nil
	}
	C.iamic_stop_session(m.session)
	C.iamic_release(m.delegate)
	C.ia_mic_release(m.session)
	cbMu.Lock()
	delete(callbacks, m.userPtr)
	cbMu.Unlock()
	m.session = nil
	m.delegate = nil
	m.userPtr = nil
	m.onChunk = nil
	m.started = false
	return nil
}

func (m *MicrophoneCapture) Devices() ([]port.AudioDevice, error) { return nil, nil }
func (m *MicrophoneCapture) SetDevice(id string) error            { return nil }

// pump delivers captured float32 frames to onChunk as little-endian bytes.
func (m *MicrophoneCapture) pump(frames []float32) {
	if m.onChunk == nil {
		return
	}
	mono := toMonoFloat32(frames, 1)
	m.onChunk(float32ToBytes(mono))
}

type SystemCapture struct {
	mu       sync.Mutex
	stream   unsafe.Pointer
	delegate unsafe.Pointer
	userPtr  unsafe.Pointer
	onChunk  func([]byte)
	started  bool
	stopErr  chan error
}

func NewSystemCapture() *SystemCapture {
	return &SystemCapture{stopErr: make(chan error, 1)}
}

func (s *SystemCapture) Start(onChunk func([]byte)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}
	if onChunk == nil {
		return errors.New("system capture: nil onChunk")
	}

	select {
	case <-s.stopErr:
	default:
	}

	delegate := C.iasystem_new()
	if delegate == nil {
		return errors.New("screencapturekit: system delegate init failed")
	}
	C.iasystem_set_cb(delegate, C.uintptr_t(uintptr(unsafe.Pointer(s))))
	C.iasystem_set_stop(delegate, C.uintptr_t(uintptr(unsafe.Pointer(s))))

	userPtr := unsafe.Pointer(s)
	cbMu.Lock()
	callbacks[userPtr] = func(frames []float32) { s.pump(frames) }
	cbMu.Unlock()

	s.delegate = delegate
	s.userPtr = userPtr
	s.onChunk = onChunk

	var errBuf [512]C.char
	stream := C.iasystem_start(delegate, &errBuf[0], C.int(len(errBuf)))
	if stream == nil {
		msg := C.GoString(&errBuf[0])
		if msg == "" {
			msg = "screencapturekit: stream start failed"
		}
		C.iasystem_release(delegate)
		cbMu.Lock()
		delete(callbacks, userPtr)
		cbMu.Unlock()
		s.delegate = nil
		s.userPtr = nil
		s.onChunk = nil
		return errors.New(msg)
	}

	s.stream = stream
	s.started = true
	return nil
}

func (s *SystemCapture) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return nil
	}
	C.iasystem_clear_stop(s.delegate)
	C.iasystem_stop(s.stream)
	C.iasystem_release(s.delegate)
	C.ia_system_release(s.stream)
	cbMu.Lock()
	delete(callbacks, s.userPtr)
	cbMu.Unlock()
	s.stream = nil
	s.delegate = nil
	s.userPtr = nil
	s.onChunk = nil
	s.started = false
	return nil
}

func (s *SystemCapture) Devices() ([]port.AudioDevice, error) { return nil, nil }
func (s *SystemCapture) SetDevice(id string) error            { return nil }

func (s *SystemCapture) pump(frames []float32) {
	if s.onChunk == nil {
		return
	}
	mono := toMonoFloat32(frames, 1)
	s.onChunk(float32ToBytes(mono))
}

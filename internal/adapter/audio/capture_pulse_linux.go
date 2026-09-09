//go:build linux && cgo

package audio

/*
#cgo CFLAGS: -D_GNU_SOURCE
#cgo pkg-config: libpulse-simple libpulse
#cgo LDFLAGS: -lpulse-simple -lpulse

#include <pulse/pulseaudio.h>
#include <pulse/simple.h>
#include <pulse/error.h>
#include <stdlib.h>
#include <string.h>

// --- source enumeration via the libpulse threaded mainloop ---
//
// libpulse has no PA_SOURCE_DEFAULT flag: the default source is only known
// through pa_server_info.default_source_name, so we fetch it first and compare
// source names against it. Every callback signals the threaded mainloop: the
// pa_threaded_mainloop_wait() loop below only wakes on pa_threaded_mainloop_signal.

typedef struct {
	char **names;
	char **descriptions;
	int   *defaults;
	int    count;
	int    capacity;
	int    done;
	int    err;
	char                 *default_name;
	pa_threaded_mainloop *ml;
} ia_source_state;

void ia_source_cb(pa_context *c, const pa_source_info *i, int eol, void *userdata) {
	(void)c;
	ia_source_state *s = userdata;
	if (eol > 0) { s->done = 1; pa_threaded_mainloop_signal(s->ml, 0); return; }
	if (i == NULL) { s->err = 1; s->done = 1; pa_threaded_mainloop_signal(s->ml, 0); return; }
	if (s->count == s->capacity) {
		int nc = s->capacity ? s->capacity * 2 : 8;
		char **n = realloc(s->names, sizeof(char*) * nc);
		if (!n) { s->err = 1; s->done = 1; pa_threaded_mainloop_signal(s->ml, 0); return; }
		s->names = n;
		char **d = realloc(s->descriptions, sizeof(char*) * nc);
		if (!d) { s->err = 1; s->done = 1; pa_threaded_mainloop_signal(s->ml, 0); return; }
		s->descriptions = d;
		int *f = realloc(s->defaults, sizeof(int) * nc);
		if (!f) { s->err = 1; s->done = 1; pa_threaded_mainloop_signal(s->ml, 0); return; }
		s->defaults = f;
		s->capacity = nc;
	}
	char *nm = strdup(i->name);
	if (!nm) { s->err = 1; s->done = 1; pa_threaded_mainloop_signal(s->ml, 0); return; }
	char *ds = strdup(i->description);
	if (!ds) { free(nm); s->err = 1; s->done = 1; pa_threaded_mainloop_signal(s->ml, 0); return; }
	s->names[s->count] = nm;
	s->descriptions[s->count] = ds;
	s->defaults[s->count] = (s->default_name && strcmp(i->name, s->default_name) == 0) ? 1 : 0;
	s->count++;
	pa_threaded_mainloop_signal(s->ml, 0);
}

void ia_server_info_cb(pa_context *c, const pa_server_info *info, void *userdata) {
	(void)c;
	ia_source_state *s = userdata;
	if (info == NULL || info->default_source_name == NULL) { s->err = 1; s->done = 1; }
	else {
		char *dup = strdup(info->default_source_name);
		if (!dup) { s->err = 1; s->done = 1; }
		else s->default_name = dup;
	}
	pa_threaded_mainloop_signal(s->ml, 0);
}

void ia_context_state_cb(pa_context *c, void *userdata) {
	(void)c;
	pa_threaded_mainloop *ml = userdata;
	pa_threaded_mainloop_signal(ml, 0);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

const pulseAppName = "interagent"

type pulseSimple struct {
	simple *C.pa_simple
}

func init() {
	openPulseReader = openPulseReaderCGO
	listPulseSources = listPulseSourcesCGO
}

func openPulseReaderCGO(device string, sampleRate, chunkBytes int) (pulseReader, error) {
	spec := C.pa_sample_spec{
		format:   C.PA_SAMPLE_FLOAT32LE,
		rate:     C.uint32_t(sampleRate),
		channels: 1,
	}
	var dev *C.char
	if device != "" {
		dev = C.CString(device)
		defer C.free(unsafe.Pointer(dev))
	}
	app := C.CString(pulseAppName)
	defer C.free(unsafe.Pointer(app))
	stream := C.CString("record")
	defer C.free(unsafe.Pointer(stream))

	var cerr C.int
	ps := C.pa_simple_new(nil, app, C.PA_STREAM_RECORD, dev, stream, &spec, nil, nil, &cerr)
	if ps == nil {
		return nil, fmt.Errorf("pulse open: %s", C.GoString(C.pa_strerror(cerr)))
	}
	return &pulseSimple{simple: ps}, nil
}

func (p *pulseSimple) Read(buf []byte) (int, error) {
	if p.simple == nil || len(buf) == 0 {
		return 0, nil
	}
	var cerr C.int
	if n := C.pa_simple_read(p.simple, unsafe.Pointer(&buf[0]), C.size_t(len(buf)), &cerr); n < 0 {
		return 0, fmt.Errorf("pulse read: %s", C.GoString(C.pa_strerror(cerr)))
	}
	return len(buf), nil
}

func (p *pulseSimple) Close() error {
	if p.simple != nil {
		C.pa_simple_free(p.simple)
		p.simple = nil
	}
	return nil
}

func listPulseSourcesCGO() ([]pulseSourceInfo, error) {
	mainloop := C.pa_threaded_mainloop_new()
	if mainloop == nil {
		return nil, errors.New("pulse: failed to allocate mainloop")
	}
	defer C.pa_threaded_mainloop_free(mainloop)

	api := C.pa_threaded_mainloop_get_api(mainloop)
	name := C.CString(pulseAppName + "-devices")
	defer C.free(unsafe.Pointer(name))
	ctx := C.pa_context_new(api, name)
	if ctx == nil {
		return nil, errors.New("pulse: failed to allocate context")
	}
	defer C.pa_context_unref(ctx)

	C.pa_context_set_state_callback(ctx, C.pa_context_notify_cb_t(C.ia_context_state_cb), unsafe.Pointer(mainloop))

	C.pa_threaded_mainloop_lock(mainloop)
	if C.pa_context_connect(ctx, nil, C.PA_CONTEXT_NOFLAGS, nil) < 0 {
		C.pa_threaded_mainloop_unlock(mainloop)
		return nil, errors.New("pulse: context connect failed")
	}
	if C.pa_threaded_mainloop_start(mainloop) < 0 {
		C.pa_threaded_mainloop_unlock(mainloop)
		return nil, errors.New("pulse: mainloop start failed")
	}
	defer C.pa_threaded_mainloop_stop(mainloop)
	// Defers run LIFO, so unlock (registered last) runs before stop. stop()
	// takes its own lock; per libpulse docs it must be called after unlocking —
	// the worker's poll re-locks the mutex to exit, so stopping while the
	// recursive reference is still held deadlocks the join.
	defer C.pa_threaded_mainloop_unlock(mainloop)

	for {
		state := C.pa_context_get_state(ctx)
		if state == C.PA_CONTEXT_READY || state == C.PA_CONTEXT_FAILED || state == C.PA_CONTEXT_TERMINATED {
			break
		}
		C.pa_threaded_mainloop_wait(mainloop)
	}
	if C.pa_context_get_state(ctx) != C.PA_CONTEXT_READY {
		return nil, fmt.Errorf("pulse: context %s", C.GoString(C.pa_strerror(C.pa_context_errno(ctx))))
	}

	state := C.ia_source_state{}
	state.ml = mainloop
	C.pa_context_get_server_info(ctx, C.pa_server_info_cb_t(C.ia_server_info_cb), unsafe.Pointer(&state))
	for state.default_name == nil && state.err == 0 {
		C.pa_threaded_mainloop_wait(mainloop)
	}
	if state.err != 0 {
		releaseSourceState(&state)
		return nil, errors.New("pulse: default source lookup failed")
	}

	C.pa_context_get_source_info_list(ctx, C.pa_source_info_cb_t(C.ia_source_cb), unsafe.Pointer(&state))
	for state.done == 0 && state.err == 0 {
		C.pa_threaded_mainloop_wait(mainloop)
	}
	if state.err != 0 {
		releaseSourceState(&state)
		return nil, errors.New("pulse: source listing failed")
	}

	names := unsafe.Slice(state.names, int(state.count))
	descs := unsafe.Slice(state.descriptions, int(state.count))
	defaults := unsafe.Slice(state.defaults, int(state.count))
	out := make([]pulseSourceInfo, 0, int(state.count))
	for i := 0; i < int(state.count); i++ {
		out = append(out, pulseSourceInfo{
			Name:        C.GoString(names[i]),
			Description: C.GoString(descs[i]),
			IsDefault:   defaults[i] != 0,
		})
	}
	releaseSourceState(&state)
	return out, nil
}

func releaseSourceState(s *C.ia_source_state) {
	if s.default_name != nil {
		C.free(unsafe.Pointer(s.default_name))
	}
	names := unsafe.Slice(s.names, int(s.count))
	descs := unsafe.Slice(s.descriptions, int(s.count))
	for i := 0; i < int(s.count); i++ {
		C.free(unsafe.Pointer(names[i]))
		C.free(unsafe.Pointer(descs[i]))
	}
	C.free(unsafe.Pointer(s.names))
	C.free(unsafe.Pointer(s.descriptions))
	C.free(unsafe.Pointer(s.defaults))
}

//go:build linux && cgo

package audio

/*
#cgo CFLAGS: -D_REENTRANT -I/usr/include/pipewire-0.3 -I/usr/include/spa-0.2
#cgo LDFLAGS: -lpipewire-0.3
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <pipewire/pipewire.h>
#include <spa/param/audio/raw.h>
#include <spa/param/audio/format-utils.h>
#include <spa/param/param.h>

void iaGoPWChunk(void *ptr, int size, int channels);

typedef struct {
	struct pw_main_loop *loop;
	struct pw_context   *context;
	struct pw_core      *core;
	struct pw_stream    *stream;
	struct spa_hook      stream_listener;
} ia_pw_state;

static void ia_stream_process(void *data) {
	struct pw_stream *stream = data;
	struct pw_buffer *b = pw_stream_dequeue_buffer(stream);
	if (b == NULL)
		return;
	struct spa_buffer *buf = b->buffer;
	struct spa_data *d = &buf->datas[0];
	if (d && d->data && d->chunk && d->chunk->size > 0)
		iaGoPWChunk(d->data, d->chunk->size, 2);
	pw_stream_queue_buffer(stream, b);
}

static const struct pw_stream_events ia_stream_events = {
	PW_VERSION_STREAM_EVENTS,
	.destroy = NULL,
	.control_info = NULL,
	.param_changed = NULL,
	.process = ia_stream_process,
};

static ia_pw_state *ia_pw_new(void) {
	ia_pw_state *st = calloc(1, sizeof(ia_pw_state));
	if (st == NULL) return NULL;
	st->loop = pw_main_loop_new(NULL);
	if (st->loop == NULL) { free(st); return NULL; }
	return st;
}

static int ia_pw_connect(ia_pw_state *st, int fd, uint32_t node_id, uint32_t sample_rate) {
	struct pw_properties *props = pw_properties_new(
		PW_KEY_APP_ID, "interagent",
		PW_KEY_APP_NAME, "interagent",
		NULL);
	// pw_context_new takes ownership of props on every path (no double free):
	// verified against PipeWire 1.6.8 src/pipewire/context.c — the pointer is
	// stored directly (this->properties = properties, line 452) and freed by
	// pw_properties_free on the calloc-fail path (line 409) and by
	// pw_context_destroy (line 672) on both error_cleanup and normal teardown.
	st->context = pw_context_new(pw_main_loop_get_loop(st->loop), props, 0);
	if (st->context == NULL) { close(fd); return -1; }
	st->core = pw_context_connect_fd(st->context, fd, NULL, 0);
	if (st->core == NULL) { close(fd); return -2; }

	st->stream = pw_stream_new(st->core, "interagent-screencast",
		pw_properties_new(PW_KEY_MEDIA_NAME, "system audio", NULL));
	if (st->stream == NULL) return -3;

	uint8_t buffer[512];
	struct spa_pod_builder b = SPA_POD_BUILDER_INIT(buffer, sizeof(buffer));
	struct spa_pod_frame f[2];
	spa_pod_builder_push_object(&b, &f[0], SPA_TYPE_OBJECT_Format, SPA_PARAM_Format);
	spa_pod_builder_add(&b,
		SPA_FORMAT_mediaType,      SPA_POD_Int(SPA_MEDIA_TYPE_audio),
		SPA_FORMAT_mediaSubtype,   SPA_POD_Int(SPA_MEDIA_SUBTYPE_raw),
		SPA_FORMAT_AUDIO_format,   SPA_POD_Id(SPA_AUDIO_FORMAT_F32),
		SPA_FORMAT_AUDIO_rate,     SPA_POD_Int(sample_rate),
		SPA_FORMAT_AUDIO_channels, SPA_POD_Int(2),
		0);
	struct spa_pod *pod = spa_pod_builder_pop(&b, &f[0]);
	const struct spa_pod *params = pod;

	pw_stream_add_listener(st->stream, &st->stream_listener, &ia_stream_events, st->stream);

	// From the previous connect on, the fd lives in the wire connection and is
	// closed by pw_core_disconnect (core.h: "the socket will be closed
	// automatically on disconnect or error"). Analog -3/-4 teardown cascades
	// through ia_pw_free -> pw_core_disconnect.
	if (pw_stream_connect(st->stream, PW_DIRECTION_INPUT, node_id,
			PW_STREAM_FLAG_AUTOCONNECT | PW_STREAM_FLAG_MAP_BUFFERS,
			&params, 1) < 0) {
		return -4;
	}
	return 0;
}

static void ia_pw_run(ia_pw_state *st) {
	pw_main_loop_run(st->loop);
}

static void ia_pw_quit(ia_pw_state *st) {
	if (st->loop) pw_main_loop_quit(st->loop);
}

static void ia_pw_free(ia_pw_state *st) {
	if (st->stream) {
		pw_stream_disconnect(st->stream);
		pw_stream_destroy(st->stream);
	}
	if (st->core) pw_core_disconnect(st->core);
	if (st->context) pw_context_destroy(st->context);
	if (st->loop) pw_main_loop_destroy(st->loop);
	free(st);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

// pwConsumer is the live PipeWire audio consumer for a portal ScreenCast node.
type pwConsumer struct {
	state     *C.ia_pw_state
	started   bool
	closed    bool
	closeOnce sync.Once
	done      chan struct{}
	onChunk   func([]byte)
}

// activeChunk funnels stream data from the C process callback thread. Guarded
// by consumerMu; only one consumer streams per process (matches darwin).
var (
	consumerMu  sync.Mutex
	activeChunk func([]byte)
)

//export iaGoPWChunk
func iaGoPWChunk(ptr unsafe.Pointer, size, channels C.int) {
	if size <= 0 || channels <= 0 {
		return
	}
	consumerMu.Lock()
	cb := activeChunk
	var out []byte
	if cb != nil {
		out = make([]byte, int(size))
		copy(out, unsafe.Slice((*byte)(ptr), int(size)))
	}
	consumerMu.Unlock()
	if out == nil || cb == nil {
		return
	}
	if int(channels) == 2 {
		out = float32ToBytes(toMonoFloat32(bytesToFloat32(out), 2))
	}
	cb(out)
}

func newPWConsumer(fd, nodeID, sampleRate int, onChunk func([]byte)) (*pwConsumer, error) {
	state := C.ia_pw_new()
	if state == nil {
		syscall.Close(fd) // no C object exists; the fd is still ours
		return nil, errors.New("pipewire: out of memory")
	}
	if err := int(C.ia_pw_connect(state, C.int(fd), C.uint32_t(nodeID), C.uint32_t(sampleRate))); err != 0 {
		C.ia_pw_free(state)
		return nil, fmt.Errorf("pipewire connect: errno=%d", err)
	}
	// fd ownership moved into the wire context on successful connect.
	return &pwConsumer{state: state, done: make(chan struct{}), onChunk: onChunk}, nil
}

// Start spawns the single pw_main_loop_run goroutine. It is idempotent: a
// second Start on an already-started consumer is a no-op (exactly one loop
// goroutine owns the done channel), and Start after Close is refused.
func (c *pwConsumer) Start() error {
	consumerMu.Lock()
	defer consumerMu.Unlock()
	if c.closed {
		return errors.New("pw consumer: already closed")
	}
	if c.started {
		return nil
	}
	if c.state == nil {
		return errors.New("pw consumer: not connected")
	}
	activeChunk = c.onChunk
	c.started = true
	go func() {
		defer close(c.done)
		C.ia_pw_run(c.state)
	}()
	return nil
}

// Close terminates the consumer. For a started consumer it quits the main
// loop and waits for the run goroutine to return before freeing the C state.
// For a consumer whose Start was never called there is no goroutine to own
// done, so Close completes the channel itself and returns immediately: nothing
// is running, so there is nothing to quit, and waiting on done would hang
// forever (the pre-fix bug). Teardown of a connected-but-never-started
// consumer only frees the C state (no loop iteration ever ran).
func (c *pwConsumer) Close() error {
	var closeErr error
	c.closeOnce.Do(func() {
		consumerMu.Lock()
		c.closed = true
		started := c.started
		if started {
			activeChunk = nil
		}
		consumerMu.Unlock()

		if !started {
			if c.done != nil {
				close(c.done)
			}
			if c.state != nil {
				C.ia_pw_free(c.state)
			}
			return
		}

		C.ia_pw_quit(c.state)
		<-c.done
		C.ia_pw_free(c.state)
	})
	return closeErr
}

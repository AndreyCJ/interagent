//go:build linux && !cgo

package audio

import "errors"

type pwConsumer struct{}

func newPWConsumer(fd, nodeID, sampleRate int, onChunk func([]byte)) (*pwConsumer, error) {
	return nil, errors.New("system sound capture requires cgo (build with CGO_ENABLED=1)")
}

func (c *pwConsumer) Start() error { return errors.New("pw consumer not built") }
func (c *pwConsumer) Close() error { return nil }

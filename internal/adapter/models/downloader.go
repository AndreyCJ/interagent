// Package models downloads and verifies local inference model artifacts
// (whisper ggml models and the Silero VAD model) into the models directory.
package models

import (
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//go:embed checksums.txt
var checksumsFile string

type spec struct {
	url      string
	sha256   string
	fileName string
}

type Store struct {
	client *http.Client
	dir    string
	specs  map[string]spec
}

func New(dir string) *Store {
	s := &Store{
		client: http.DefaultClient,
		dir:    dir,
		specs:  map[string]spec{},
	}
	for _, line := range strings.Split(strings.TrimSpace(checksumsFile), "\n") {
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		s.add(parts[0], parts[1])
	}
	return s
}

func (s *Store) add(name, sha256 string) {
	switch name {
	case "ggml-base.bin":
		s.specs["ggml-base"] = spec{
			url:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin",
			sha256:   sha256,
			fileName: "ggml-base.bin",
		}
	case "ggml-silero-v6.2.0.bin":
		s.specs["silero-vad"] = spec{
			url:      "https://huggingface.co/ggml-org/whisper-vad/resolve/main/ggml-silero-v6.2.0.bin",
			sha256:   sha256,
			fileName: "ggml-silero-v6.2.0.bin",
		}
	case "ggml-tiny.bin":
		s.specs["ggml-tiny"] = spec{
			url:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin",
			sha256:   sha256,
			fileName: "ggml-tiny.bin",
		}
	case "ggml-small.bin":
		s.specs["ggml-small"] = spec{
			url:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin",
			sha256:   sha256,
			fileName: "ggml-small.bin",
		}
	}
}

func (s *Store) specFor(model string) (spec, error) {
	sp, ok := s.specs[model]
	if !ok {
		return spec{}, fmt.Errorf("unknown model %q", model)
	}
	return sp, nil
}

func (s *Store) Status(model string) (bool, string, error) {
	sp, err := s.specFor(model)
	if err != nil {
		return false, "", err
	}
	path := filepath.Join(s.dir, sp.fileName)
	fi, err := os.Stat(path)
	if err != nil {
		return false, path, nil
	}
	return fi.Size() > 0, path, nil
}

func (s *Store) Download(model string, onProgress func(received, total int64)) error {
	sp, err := s.specFor(model)
	if err != nil {
		return err
	}
	dest := filepath.Join(s.dir, sp.fileName)
	part := dest + ".part"

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	// Resume: reuse any existing partial file.
	var existing int64
	if fi, err := os.Stat(part); err == nil {
		existing = fi.Size()
	}

	req, err := http.NewRequest(http.MethodGet, sp.url, nil)
	if err != nil {
		return err
	}
	if existing > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existing))
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("download %s: http %d", sp.fileName, resp.StatusCode)
	}

	total := existing
	if resp.StatusCode == http.StatusPartialContent {
		// Content-Range: bytes <from>-<to>/<total>
		cr := resp.Header.Get("Content-Range")
		if _, err := fmt.Sscanf(cr, "bytes %d-%d/%d", new(int64), new(int64), &total); err != nil {
			return fmt.Errorf("download %s: bad Content-Range %q", sp.fileName, cr)
		}
	} else if cl := resp.ContentLength; cl > 0 {
		total = cl
	}

	f, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	received := existing
	buf := make([]byte, 256*1024)
	hash := newSHA256()
	// Hash the pre-existing bytes too, so the final checksum covers the whole file.
	hashFromFile(hash, part, existing)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				return werr
			}
			hash.Write(buf[:n])
			received += int64(n)
			if onProgress != nil {
				onProgress(received, total)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			return rerr
		}
	}
	if err := f.Close(); err != nil {
		return err
	}

	if got := fmt.Sprintf("%x", hash.Sum(nil)); got != sp.sha256 {
		return fmt.Errorf("checksum mismatch for %s: got %s want %s", sp.fileName, got, sp.sha256)
	}
	return os.Rename(part, dest)
}

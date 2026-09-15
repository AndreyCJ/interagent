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
	client     *http.Client
	dir        string
	voxtypeDir string
	specs      map[string]spec
}

func New(dir string) *Store {
	s := &Store{
		client:     http.DefaultClient,
		dir:        dir,
		voxtypeDir: voxtypeModelsDir(),
		specs:      map[string]spec{},
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
	case "ggml-large-v3.bin":
		s.specs["ggml-large-v3"] = spec{
			url:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3.bin",
			sha256:   sha256,
			fileName: "ggml-large-v3.bin",
		}
	case "ggml-large-v3-turbo.bin":
		s.specs["ggml-large-v3-turbo"] = spec{
			url:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo.bin",
			sha256:   sha256,
			fileName: "ggml-large-v3-turbo.bin",
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

	if s.adoptExternal(dest, sp) {
		return nil
	}

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

// adoptExternal links a matching model file found in an external models
// directory (voxtype) into s.dir so it is reused instead of re-downloaded.
// It returns true when dest now exists as a result.
func (s *Store) adoptExternal(dest string, sp spec) bool {
	if s.voxtypeDir == "" {
		return false
	}
	src := filepath.Join(s.voxtypeDir, sp.fileName)
	if _, err := os.Stat(src); err != nil {
		return false
	}
	h := newSHA256()
	if err := hashFile(h, src); err != nil {
		return false
	}
	if fmt.Sprintf("%x", h.Sum(nil)) != sp.sha256 {
		return false
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return false
	}
	if err := os.Link(src, dest); err == nil {
		return true
	}
	if err := copyFile(src, dest); err != nil {
		return false
	}
	return true
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(dst)
		return err
	}
	return out.Close()
}

// voxtypeModelsDir returns the directory where voxtype keeps its whisper
// models. INTERAGENT_VOXTYPE_MODELS_DIR overrides it; otherwise XDG data home
// (defaulting to ~/.local/share) is used.
func voxtypeModelsDir() string {
	if d := os.Getenv("INTERAGENT_VOXTYPE_MODELS_DIR"); d != "" {
		return d
	}
	base := ""
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		base = xdg
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(os.TempDir(), "voxtype", "models")
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "voxtype", "models")
}

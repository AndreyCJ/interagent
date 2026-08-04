package models

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func checksum(t *testing.T, data []byte) string {
	t.Helper()
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// testModel describes one artifact under test: model key -> spec seed.
type testModel struct {
	fileName string
	data     []byte
	url      string
}

func newTestStore(t *testing.T, dir string, models map[string]testModel, client *http.Client) *Store {
	t.Helper()
	specs := make(map[string]spec)
	for key, m := range models {
		specs[key] = spec{fileName: m.fileName, url: m.url, sha256: checksum(t, m.data)}
	}
	return &Store{dir: dir, client: client, specs: specs}
}

// rangeHandler serves a file, honoring Range requests (resume support).
func rangeHandler(t *testing.T, data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "" {
			w.Header().Set("Content-Length", fmt.Sprint(len(data)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
			return
		}
		var from int
		if _, err := fmt.Sscanf(rangeHeader, "bytes=%d-", &from); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", from, len(data)-1, len(data)))
		w.Header().Set("Content-Length", fmt.Sprint(len(data)-from))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(data[from:])
	}
}

func TestStore_Status_NotInstalled(t *testing.T) {
	store := newTestStore(t, t.TempDir(), map[string]testModel{
		"ggml-base": {fileName: "ggml-base.bin"},
	}, http.DefaultClient)
	installed, path, err := store.Status("ggml-base")
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if installed {
		t.Error("Status() = installed=true for missing file")
	}
	if path == "" {
		t.Error("Status() returned empty path")
	}
}

func TestStore_Status_Installed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ggml-base.bin"), []byte("model"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := newTestStore(t, dir, map[string]testModel{
		"ggml-base": {fileName: "ggml-base.bin"},
	}, http.DefaultClient)
	installed, path, err := store.Status("ggml-base")
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if !installed {
		t.Error("Status() = installed=false for existing file")
	}
	if path != filepath.Join(dir, "ggml-base.bin") {
		t.Errorf("Status() path = %q", path)
	}
}

func TestStore_Status_UnknownModel(t *testing.T) {
	store := newTestStore(t, t.TempDir(), nil, http.DefaultClient)
	if _, _, err := store.Status("nope"); err == nil {
		t.Fatal("Status(unknown) should return an error")
	}
}

func TestStore_Download_Success(t *testing.T) {
	payload := []byte("fake ggml-base.bin payload")
	srv := httptest.NewServer(rangeHandler(t, payload))
	defer srv.Close()
	dir := t.TempDir()
	store := newTestStore(t, dir, map[string]testModel{
		"ggml-base": {fileName: "ggml-base.bin", data: payload, url: srv.URL},
	}, srv.Client())

	var mu sync.Mutex
	lastProgress := int64(0)
	finalReported := false
	err := store.Download("ggml-base", func(received, total int64) {
		mu.Lock()
		defer mu.Unlock()
		if received < lastProgress {
			t.Errorf("progress went backwards: %d -> %d", lastProgress, received)
		}
		lastProgress = received
		if received == total {
			finalReported = true
		}
	})
	if err != nil {
		t.Fatalf("Download() error: %v", err)
	}
	mu.Lock()
	if !finalReported {
		t.Error("Download() never reported received == total")
	}
	mu.Unlock()

	data, err := os.ReadFile(filepath.Join(dir, "ggml-base.bin"))
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(data) != string(payload) {
		t.Error("downloaded file content mismatch")
	}
	if _, err := os.Stat(filepath.Join(dir, "ggml-base.bin.part")); !os.IsNotExist(err) {
		t.Error(".part file should be renamed away after success")
	}
}

func TestStore_Download_ChecksumMismatch_Error(t *testing.T) {
	payload := []byte("payload")
	srv := httptest.NewServer(rangeHandler(t, payload))
	defer srv.Close()
	dir := t.TempDir()
	store := newTestStore(t, dir, map[string]testModel{
		"ggml-base": {fileName: "ggml-base.bin", data: []byte("different"), url: srv.URL},
	}, srv.Client())

	if err := store.Download("ggml-base", func(int64, int64) {}); err == nil {
		t.Fatal("Download() should error on checksum mismatch")
	}
	if _, err := os.Stat(filepath.Join(dir, "ggml-base.bin")); !os.IsNotExist(err) {
		t.Error("mismatched file must not be left at the final name")
	}
}

func TestStore_Download_ResumeFromPartial(t *testing.T) {
	payload := []byte("0123456789abcdefghij")
	dir := t.TempDir()
	// Pre-existing .part with the first 10 bytes (resume point).
	if err := os.WriteFile(filepath.Join(dir, "ggml-base.bin.part"), payload[:10], 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(rangeHandler(t, payload))
	defer srv.Close()
	store := newTestStore(t, dir, map[string]testModel{
		"ggml-base": {fileName: "ggml-base.bin", data: payload, url: srv.URL},
	}, srv.Client())

	if err := store.Download("ggml-base", func(int64, int64) {}); err != nil {
		t.Fatalf("Download() error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "ggml-base.bin"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != string(payload) {
		t.Errorf("resumed download corrupt: got %q", data)
	}
}

func TestStore_Download_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	dir := t.TempDir()
	store := newTestStore(t, dir, map[string]testModel{
		"ggml-base": {fileName: "ggml-base.bin", data: []byte("x"), url: srv.URL},
	}, srv.Client())

	if err := store.Download("ggml-base", func(int64, int64) {}); err == nil {
		t.Fatal("Download() should return HTTP error")
	}
}

func TestChecksumsFile_Format(t *testing.T) {
	data, err := os.ReadFile("checksums.txt")
	if err != nil {
		t.Fatalf("read checksums.txt: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("checksums.txt must have exactly 2 lines, got %d", len(lines))
	}
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) != 2 || len(parts[1]) != 64 {
			t.Errorf("malformed checksum line: %q", line)
		}
	}
}

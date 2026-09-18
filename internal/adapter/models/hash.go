package models

import (
	"crypto/sha256"
	"hash"
	"io"
	"os"
)

func newSHA256() hash.Hash { return sha256.New() }

// hashFile feeds the whole file at path into h.
func hashFile(h hash.Hash, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	return nil
}

// hashFromFile feeds the first size bytes of path into h (used so a resumed
// .part still produces a full-file checksum).
func hashFromFile(h hash.Hash, path string, size int64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.CopyN(h, f, size); err != nil && err != io.EOF {
		return err
	}
	return nil
}

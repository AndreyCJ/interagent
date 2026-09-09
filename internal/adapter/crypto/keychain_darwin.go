//go:build darwin

package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os/exec"
	"strings"
)

const (
	keychainService = "com.interagent.keys"
	keychainAccount = "master"
)

var securityCmd = exec.Command

type Keychain struct{}

func NewKeychain() *Keychain { return &Keychain{} }

func (k *Keychain) GetOrCreate() ([]byte, error) {
	key, err := k.find()
	if err == nil {
		return key, nil
	}
	if !isItemNotFound(err) {
		return nil, err
	}
	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := k.add(key); err != nil {
		return nil, err
	}
	return key, nil
}

func (k *Keychain) find() ([]byte, error) {
	out, err := securityCmd("security", "find-generic-password", "-s", keychainService, "-a", keychainAccount, "-w").Output()
	if err != nil {
		return nil, err
	}
	return hex.DecodeString(strings.TrimSpace(string(out)))
}

func (k *Keychain) add(key []byte) error {
	return securityCmd("security", "add-generic-password", "-U", "-s", keychainService, "-a", keychainAccount, "-w", hex.EncodeToString(key)).Run()
}

func isItemNotFound(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 44
}

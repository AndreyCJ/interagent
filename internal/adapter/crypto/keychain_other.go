//go:build !darwin

package crypto

import "crypto/rand"

type Keychain struct{ key []byte }

func NewKeychain() *Keychain { return &Keychain{} }

func (k *Keychain) GetOrCreate() ([]byte, error) {
	if k.key != nil {
		return k.key, nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	k.key = key
	return key, nil
}

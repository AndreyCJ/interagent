package crypto

import (
	"testing"
)

type memKeyStore struct{ key []byte }

func (m *memKeyStore) GetOrCreate() ([]byte, error) {
	if m.key != nil {
		return m.key, nil
	}
	m.key = make([]byte, 32)
	for i := range m.key {
		m.key[i] = byte(i)
	}
	return m.key, nil
}

func TestAESGCM_RoundTrip(t *testing.T) {
	c, err := NewCrypto(&memKeyStore{})
	if err != nil {
		t.Fatalf("NewCrypto() returned error: %v", err)
	}
	enc, err := c.Encrypt("sk-secret-123")
	if err != nil {
		t.Fatalf("Encrypt() returned error: %v", err)
	}
	if enc == "sk-secret-123" {
		t.Error("Encrypt() must not return plaintext")
	}
	dec, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt() returned error: %v", err)
	}
	if dec != "sk-secret-123" {
		t.Errorf("Decrypt() = %q, want %q", dec, "sk-secret-123")
	}
}

func TestAESGCM_Decrypt_WrongKey_Fails(t *testing.T) {
	a := NewAESGCM(make([]byte, 32))
	b := NewAESGCM(append([]byte{1}, make([]byte, 31)...))
	enc, err := a.Encrypt("secret")
	if err != nil {
		t.Fatalf("Encrypt() returned error: %v", err)
	}
	if _, err := b.Decrypt(enc); err == nil {
		t.Error("Decrypt() with wrong key should fail")
	}
}

func TestAESGCM_Encrypt_IsRandomized(t *testing.T) {
	a := NewAESGCM(make([]byte, 32))
	e1, _ := a.Encrypt("same")
	e2, _ := a.Encrypt("same")
	if e1 == e2 {
		t.Error("two encryptions of the same plaintext must differ (random nonce)")
	}
}

func TestAESGCM_Decrypt_Garbage_Fails(t *testing.T) {
	a := NewAESGCM(make([]byte, 32))
	if _, err := a.Decrypt("not-base64!!"); err == nil {
		t.Error("Decrypt() of garbage should fail")
	}
}

//go:build darwin

package crypto

import (
	"os/exec"
	"testing"
)

func TestKeychain_GetOrCreate_RegeneratesOnItemNotFound(t *testing.T) {
	orig := securityCmd
	t.Cleanup(func() { securityCmd = orig })
	securityCmd = func(name string, args ...string) *exec.Cmd {
		if len(args) > 0 && args[0] == "add-generic-password" {
			return exec.Command("true")
		}
		return exec.Command("sh", "-c", "exit 44")
	}

	k := NewKeychain()
	key, err := k.GetOrCreate()
	if err != nil {
		t.Fatalf("GetOrCreate() should regenerate on errSecItemNotFound (exit 44), got error: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("key length = %d, want 32", len(key))
	}
}

func TestKeychain_GetOrCreate_DoesNotRegenerateOnOtherError(t *testing.T) {
	orig := securityCmd
	t.Cleanup(func() { securityCmd = orig })
	var addCalls int
	securityCmd = func(name string, args ...string) *exec.Cmd {
		if len(args) > 0 && args[0] == "add-generic-password" {
			addCalls++
		}
		return exec.Command("sh", "-c", "exit 1")
	}

	k := NewKeychain()
	if _, err := k.GetOrCreate(); err == nil {
		t.Fatal("GetOrCreate() should return an error on a non-not-found find failure")
	}
	if addCalls != 0 {
		t.Errorf("add-generic-password called %d times; must not regenerate on non-not-found errors", addCalls)
	}
}

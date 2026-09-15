package main

import (
	"errors"
	"testing"

	"interagent/internal/port"
)

type fakePermissions struct {
	status     map[port.Permission]bool
	requested  []port.Permission
	grantOnReq map[port.Permission]bool
}

func (f *fakePermissions) Status(p port.Permission) (bool, error) {
	return f.status[p], nil
}

func (f *fakePermissions) Request(p port.Permission) error {
	f.requested = append(f.requested, p)
	if f.grantOnReq[p] {
		f.status[p] = true
	}
	return nil
}

func (f *fakePermissions) OpenSettings(p port.Permission) error { return nil }

func TestEnsureListeningPermissions(t *testing.T) {
	t.Run("proceeds without request when granted", func(t *testing.T) {
		p := &fakePermissions{status: map[port.Permission]bool{
			port.PermissionScreenCapture: true,
		}}
		if err := ensureListeningPermissions(p); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(p.requested) != 0 {
			t.Errorf("requested %v, want none", p.requested)
		}
	})

	t.Run("requests screen when not granted and proceeds on grant", func(t *testing.T) {
		p := &fakePermissions{
			status:     map[port.Permission]bool{},
			grantOnReq: map[port.Permission]bool{port.PermissionScreenCapture: true},
		}
		if err := ensureListeningPermissions(p); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(p.requested) != 1 || p.requested[0] != port.PermissionScreenCapture {
			t.Errorf("requested %v, want [screen-recording]", p.requested)
		}
	})

	t.Run("errors when the request does not grant", func(t *testing.T) {
		p := &fakePermissions{status: map[port.Permission]bool{}}
		err := ensureListeningPermissions(p)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "screen recording permission required for system sound" {
			t.Errorf("error = %q, want %q", err.Error(), "screen recording permission required for system sound")
		}
	})
}

func TestPermissionErrorPayload_WireContract(t *testing.T) {
	payload := permissionErrorPayload(port.PermissionScreenCapture, errors.New("picker cancelled"))
	if payload["stage"] != "permission" {
		t.Errorf("stage = %v, want permission", payload["stage"])
	}
	if payload["permission"] != "screen-recording" {
		t.Errorf("permission = %v, want screen-recording", payload["permission"])
	}
	if payload["error"] != "picker cancelled" {
		t.Errorf("error = %v, want picker cancelled", payload["error"])
	}
	if _, ok := payload["error"].(string); !ok {
		t.Errorf("error payload = %T, want string", payload["error"])
	}
}

type fakeModels struct {
	ensured []string
	failOn  map[string]error
}

func (f *fakeModels) Ensure(model string) error {
	f.ensured = append(f.ensured, model)
	if err := f.failOn[model]; err != nil {
		return err
	}
	return nil
}

func TestEnsureSTTModels(t *testing.T) {
	t.Run("rejects empty model", func(t *testing.T) {
		m := &fakeModels{}
		if err := ensureSTTModels(m, ""); err == nil {
			t.Fatal("expected error for empty STT model")
		}
		if len(m.ensured) != 0 {
			t.Errorf("ensured %v, want none", m.ensured)
		}
	})

	t.Run("ensures store keys and VAD in order", func(t *testing.T) {
		m := &fakeModels{}
		if err := ensureSTTModels(m, "base"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"ggml-base", "silero-vad"}
		if len(m.ensured) != len(want) {
			t.Fatalf("ensured %v, want %v", m.ensured, want)
		}
		for i := range want {
			if m.ensured[i] != want[i] {
				t.Errorf("ensured[%d] = %q, want %q", i, m.ensured[i], want[i])
			}
		}
	})

	t.Run("ensures ggml-large-v3 store key for large-v3", func(t *testing.T) {
		m := &fakeModels{}
		if err := ensureSTTModels(m, "large-v3"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"ggml-large-v3", "silero-vad"}
		if len(m.ensured) != len(want) {
			t.Fatalf("ensured %v, want %v", m.ensured, want)
		}
		for i := range want {
			if m.ensured[i] != want[i] {
				t.Errorf("ensured[%d] = %q, want %q", i, m.ensured[i], want[i])
			}
		}
	})

	t.Run("returns on ensure failure and stops", func(t *testing.T) {
		m := &fakeModels{failOn: map[string]error{"ggml-base": errors.New("download failed")}}
		if err := ensureSTTModels(m, "base"); err == nil {
			t.Fatal("expected error, got nil")
		}
		if len(m.ensured) != 1 {
			t.Errorf("ensured %v, want stop after first failure", m.ensured)
		}
	})
}

func TestSTTModelKey(t *testing.T) {
	cases := map[string]string{
		"tiny":           "ggml-tiny",
		"base":           "ggml-base",
		"small":          "ggml-small",
		"large-v3":       "ggml-large-v3",
		"large-v3-turbo": "ggml-large-v3-turbo",
		"ggml-base":      "ggml-base",
		"silero-vad":     "silero-vad",
		"some-custom":    "some-custom",
	}
	for in, want := range cases {
		if got := sttModelKey(in); got != want {
			t.Errorf("sttModelKey(%q) = %q, want %q", in, got, want)
		}
	}
}

//go:build linux

package portal

import (
	"os"
	"syscall"
	"testing"

	"github.com/godbus/dbus/v5"
)

// openEnsureFD reports whether fd is still a valid open file descriptor.
func isOpenFD(t *testing.T, fd int) bool {
	t.Helper()
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFD, 0)
	return errno == 0
}

// CloseThenCloseReason is the double-close regression guard for Ruling 1: the
// fd must be released by exactly one owner. TakeFD hands ownership to the
// PipeWire consumer; Stream.Close must not close the taken fd.
func TestStream_TakeFD_ThenClose_LeavesFDForCaller(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer func() { _ = w.Close() }()
	fd := int(r.Fd())

	fake := &fakeStreamConn{}
	st := &Stream{
		SessionPath: dbus.ObjectPath("/org/freedesktop/portal/desktop/session/1/9"),
		conn:        fake,
		fd:          fd,
	}
	if got := st.TakeFD(); got != fd {
		t.Fatalf("TakeFD() = %d, want %d", got, fd)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close() after TakeFD: %v", err)
	}
	if !isOpenFD(t, fd) {
		t.Fatal("TakeFD+Close closed the caller's fd: double-close defect")
	}
	if fake.sessionCloseCalls != 1 {
		t.Errorf("session Close calls = %d, want 1", fake.sessionCloseCalls)
	}
}

// TestStream_Close_ClosesOwnedFD asserts the normal contract: when the Stream
// still owns the fd, Close must release it exactly once.
func TestStream_Close_ClosesOwnedFD(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer func() { _ = w.Close() }()
	fd := int(r.Fd())

	fake := &fakeStreamConn{}
	st := &Stream{conn: fake, fd: fd}
	if err := st.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}
	if isOpenFD(t, fd) {
		t.Fatal("Close() left the owned fd open")
	}
}

// TestStream_Close_NoConn_StillClosesFD covers the seam-less path (conn nil):
// releasing the fd must not depend on the dbus connection being present.
func TestStream_Close_NoConn_StillClosesFD(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer func() { _ = w.Close() }()
	fd := int(r.Fd())

	st := &Stream{conn: nil, fd: fd}
	if err := st.Close(); err != nil {
		t.Fatalf("Close() with nil conn: %v", err)
	}
	if isOpenFD(t, fd) {
		t.Fatal("Close() with nil conn left the owned fd open")
	}
}

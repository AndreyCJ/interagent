package portal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

func TestParseStreams_Valid(t *testing.T) {
	nodeID := uint32(42)
	props := map[string]dbus.Variant{
		"source_type": dbus.MakeVariant(uint32(2)),
		"size":        dbus.MakeVariant("1920x1080"),
	}
	// godbus decodes a(ua{sv}) into []interface{} of structs-as-[]interface{}.
	v := []interface{}{[]interface{}{nodeID, props}}

	gotID, gotProps, err := parseStreams(v)
	if err != nil {
		t.Fatalf("parseStreams: %v", err)
	}
	if gotID != nodeID {
		t.Errorf("node id = %d, want %d", gotID, nodeID)
	}
	if st := gotProps["source_type"].Value(); st != uint32(2) {
		t.Errorf("source_type = %v, want 2", st)
	}
}

func TestParseStreams_Empty(t *testing.T) {
	if _, _, err := parseStreams([]interface{}{}); err == nil {
		t.Fatal("parseStreams(empty) = nil error, want error")
	}
}

func TestResolveStreams_UnwrapsStoredVariant(t *testing.T) {
	// The Response body arrives as map[string]dbus.Variant; the streams value
	// must be unwrapped (Variant.Value()) before parseStreams sees it.
	results := map[string]dbus.Variant{
		"streams": dbus.MakeVariant([]interface{}{
			[]interface{}{uint32(7), map[string]dbus.Variant{
				"source_type": dbus.MakeVariant(uint32(2)),
			}},
		}),
		"restore_token": dbus.MakeVariant("rt-y"),
	}
	nodeID, props, token, err := resolveStreams(results)
	if err != nil {
		t.Fatalf("resolveStreams: %v", err)
	}
	if nodeID != 7 {
		t.Errorf("node id = %d, want 7", nodeID)
	}
	if st := props["source_type"].Value(); st != uint32(2) {
		t.Errorf("source_type = %v, want 2", st)
	}
	if token != "rt-y" {
		t.Errorf("restore_token = %q, want rt-y", token)
	}
}

func TestResolveStreams_MissingStreams(t *testing.T) {
	if _, _, _, err := resolveStreams(map[string]dbus.Variant{}); err == nil {
		t.Fatal("resolveStreams(empty results) = nil error, want error")
	}
}

func TestResolveStreams_MissingTokenOK(t *testing.T) {
	results := map[string]dbus.Variant{
		"streams": dbus.MakeVariant([]interface{}{
			[]interface{}{uint32(1), map[string]dbus.Variant{}},
		}),
	}
	if _, _, token, err := resolveStreams(results); err != nil {
		t.Fatalf("resolveStreams without token: %v", err)
	} else if token != "" {
		t.Errorf("token = %q, want empty", token)
	}
}

func TestBuildSelectOptions_PersistMode2AndTypes(t *testing.T) {
	opts := buildSelectOptions(true, "tok-123")
	if v, ok := opts["persist_mode"]; !ok || v.Value().(uint32) != 2 {
		t.Errorf("persist_mode = %v, want 2", opts["persist_mode"])
	}
	if v, ok := opts["types"]; !ok || v.Value().(uint32) != 3 {
		t.Errorf("types = %v, want 3 (MONITOR|WINDOW)", opts["types"])
	}
	if v, ok := opts["restore_token"]; !ok || v.Value().(string) != "tok-123" {
		t.Errorf("restore_token absent or wrong: %v", opts["restore_token"])
	}
}

func TestBuildSelectOptions_NoPersistNoToken(t *testing.T) {
	opts := buildSelectOptions(false, "")
	if _, ok := opts["persist_mode"]; ok {
		t.Errorf("persist_mode set for non-persist request: %v", opts["persist_mode"])
	}
	if _, ok := opts["restore_token"]; ok {
		t.Errorf("restore_token set when empty: %v", opts["restore_token"])
	}
}

func TestWaitResponse_ReturnsStatusAndResults(t *testing.T) {
	ch := make(chan *dbus.Signal, 1)
	ch <- &dbus.Signal{
		Name: "org.freedesktop.portal.Request.Response",
		Body: []interface{}{
			uint32(0),
			map[string]dbus.Variant{"restore_token": dbus.MakeVariant("rt-x")},
		},
	}
	status, results, err := waitResponse(context.Background(), ch)
	if err != nil {
		t.Fatalf("waitResponse: %v", err)
	}
	if status != 0 {
		t.Errorf("status = %d, want 0", status)
	}
	if rt := results["restore_token"].Value().(string); rt != "rt-x" {
		t.Errorf("restore_token = %q, want rt-x", rt)
	}
}

func TestWaitResponse_SkipsOtherPaths(t *testing.T) {
	ch := make(chan *dbus.Signal, 2)
	ch <- &dbus.Signal{
		Name: "org.freedesktop.portal.Request.Response",
		Path: dbus.ObjectPath("/org/freedesktop/portal/desktop/request/1/1"),
		Body: []interface{}{uint32(0), map[string]dbus.Variant{}},
	}
	ch <- &dbus.Signal{
		Name: "org.freedesktop.portal.Request.Response",
		Path: dbus.ObjectPath("/org/freedesktop/portal/desktop/request/1/2"),
		Body: []interface{}{
			uint32(0),
			map[string]dbus.Variant{"restore_token": dbus.MakeVariant("rt-z")},
		},
	}
	want := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/1/2")
	_, results, err := waitResponsePath(context.Background(), ch, want)
	if err != nil {
		t.Fatalf("waitResponsePath: %v", err)
	}
	if rt := results["restore_token"].Value().(string); rt != "rt-z" {
		t.Errorf("restore_token = %q, want rt-z (matched wrong request)", rt)
	}
}

func TestWaitResponse_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := waitResponse(ctx, make(chan *dbus.Signal))
	if err == nil {
		t.Fatal("waitResponse(cancelled ctx) = nil error, want error")
	}
}

func TestSessionHandleFrom_StringAndPath(t *testing.T) {
	res := map[string]dbus.Variant{
		"session_handle": dbus.MakeVariant("/org/freedesktop/portal/desktop/session/1/2"),
	}
	if got := sessionHandleFrom(res); got != "/org/freedesktop/portal/desktop/session/1/2" {
		t.Errorf("sessionHandleFrom(string) = %q", got)
	}
	res["session_handle"] = dbus.MakeVariant(dbus.ObjectPath("/org/freedesktop/portal/desktop/session/1/3"))
	if got := sessionHandleFrom(res); got != "/org/freedesktop/portal/desktop/session/1/3" {
		t.Errorf("sessionHandleFrom(ObjectPath) = %q", got)
	}
}

func TestSessionHandleFrom_Missing(t *testing.T) {
	if got := sessionHandleFrom(map[string]dbus.Variant{}); got != "" {
		t.Errorf("sessionHandleFrom(missing) = %q, want empty", got)
	}
}

func TestCallRequestHandle_SingleReply(t *testing.T) {
	obj := &fakeBusObject{call: &dbus.Call{Body: []interface{}{dbus.ObjectPath("/r/1")}}}
	got, err := callRequestHandle(obj, "org.freedesktop.portal.ScreenCast.CreateSession")
	if err != nil {
		t.Fatalf("callRequestHandle: %v", err)
	}
	if got != "/r/1" {
		t.Errorf("request handle = %q, want /r/1", got)
	}
}

func TestCallRequestHandle_ClassicTwoFieldReply(t *testing.T) {
	// Older portal replies replying (oo) request+session; request is first.
	obj := &fakeBusObject{call: &dbus.Call{Body: []interface{}{
		dbus.ObjectPath("/r/1"),
		dbus.ObjectPath("/s/1"),
	}}}
	got, err := callRequestHandle(obj, "org.freedesktop.portal.ScreenCast.CreateSession")
	if err != nil {
		t.Fatalf("callRequestHandle: %v", err)
	}
	if got != "/r/1" {
		t.Errorf("request handle = %q, want /r/1", got)
	}
}

func TestCallRequestHandle_Errors(t *testing.T) {
	obj := &fakeBusObject{call: &dbus.Call{Err: dbus.Error{Name: "org.freedesktop.portal.Error.Failed"}}}
	if _, err := callRequestHandle(obj, "x"); err == nil {
		t.Fatal("callRequestHandle(error reply) = nil err, want error")
	}

	obj = &fakeBusObject{call: &dbus.Call{Body: []interface{}{"not-a-path"}}}
	if _, err := callRequestHandle(obj, "x"); err == nil {
		t.Fatal("callRequestHandle(garbage body) = nil err, want error")
	}
}

// fakeBusObject implements just enough of dbus.BusObject to feed
// callRequestHandle.
type fakeBusObject struct {
	call *dbus.Call
}

func (f *fakeBusObject) Call(method string, _ dbus.Flags, args ...interface{}) *dbus.Call {
	return f.call
}
func (f *fakeBusObject) CallWithContext(_ context.Context, method string, _ dbus.Flags, args ...interface{}) *dbus.Call {
	return f.call
}
func (f *fakeBusObject) Go(method string, _ dbus.Flags, ch chan *dbus.Call, args ...interface{}) *dbus.Call {
	if ch != nil {
		ch <- f.call
	}
	return f.call
}
func (f *fakeBusObject) GoWithContext(_ context.Context, method string, _ dbus.Flags, ch chan *dbus.Call, args ...interface{}) *dbus.Call {
	if ch != nil {
		ch <- f.call
	}
	return f.call
}
func (f *fakeBusObject) AddMatchSignal(iface, member string, options ...dbus.MatchOption) *dbus.Call {
	return f.call
}
func (f *fakeBusObject) RemoveMatchSignal(iface, member string, options ...dbus.MatchOption) *dbus.Call {
	return f.call
}
func (f *fakeBusObject) GetProperty(p string) (dbus.Variant, error) { return dbus.Variant{}, nil }
func (f *fakeBusObject) StoreProperty(p string, value interface{}) error {
	return nil
}
func (f *fakeBusObject) SetProperty(p string, v interface{}) error { return nil }
func (f *fakeBusObject) Destination() string                       { return "" }
func (f *fakeBusObject) Path() dbus.ObjectPath                     { return "" }

func TestRestoreToken_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	old := restoreTokenPath
	restoreTokenPath = func() string { return filepath.Join(dir, "tok") }
	defer func() { restoreTokenPath = old }()

	if err := saveRestoreToken("rt-abcd"); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := loadRestoreToken()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != "rt-abcd" {
		t.Errorf("loaded = %q, want rt-abcd", got)
	}
}

func TestRestoreToken_EmptyRefused(t *testing.T) {
	dir := t.TempDir()
	old := restoreTokenPath
	restoreTokenPath = func() string { return filepath.Join(dir, "tok") }
	defer func() { restoreTokenPath = old }()

	if err := saveRestoreToken(""); err == nil {
		t.Fatal("saveRestoreToken('') = nil, want error")
	}
}

// fakeStreamConn implements dbusConn for Stream tests, counting session Close
// calls via a counting BusObject. Only Object is exercised by Stream.Close.
type fakeStreamConn struct {
	sessionCloseCalls int
}

func (f *fakeStreamConn) Object(dest string, path dbus.ObjectPath) dbus.BusObject {
	return &countingBusObject{calls: &f.sessionCloseCalls}
}
func (f *fakeStreamConn) Signal(ch chan<- *dbus.Signal)               {}
func (f *fakeStreamConn) RemoveSignal(ch chan<- *dbus.Signal)         {}
func (f *fakeStreamConn) AddMatchSignal(...dbus.MatchOption) error    { return nil }
func (f *fakeStreamConn) RemoveMatchSignal(...dbus.MatchOption) error { return nil }
func (f *fakeStreamConn) NameHasOwner(name string) (bool, error)      { return true, nil }
func (f *fakeStreamConn) Close() error                                { return nil }

type countingBusObject struct {
	fakeBusObject
	calls *int
}

func (c *countingBusObject) Call(method string, _ dbus.Flags, args ...interface{}) *dbus.Call {
	*c.calls++
	return &dbus.Call{}
}

func TestStream_TakeFD_TransfersOwnership(t *testing.T) {
	st := &Stream{fd: 7}
	if fd := st.TakeFD(); fd != 7 {
		t.Fatalf("TakeFD() = %d, want 7", fd)
	}
	if fd := st.FD(); fd != -1 {
		t.Errorf("FD() after TakeFD = %d, want -1 (fd no longer owned by Stream)", fd)
	}
	if fd := st.TakeFD(); fd != -1 {
		t.Errorf("second TakeFD() = %d, want -1 (fd already taken)", fd)
	}
	// Close after TakeFD must still destroy the session but not sever the fd.
	if err := st.Close(); err != nil {
		t.Errorf("Close() after TakeFD: %v", err)
	}
}

func TestStream_Close_Idempotent_SkipsTakenFD(t *testing.T) {
	fake := &fakeStreamConn{}
	st := &Stream{
		SessionPath: dbus.ObjectPath("/org/freedesktop/portal/desktop/session/1/9"),
		conn:        fake,
		fd:          -1,
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}
	if fake.sessionCloseCalls != 1 {
		t.Errorf("session Close calls = %d, want 1", fake.sessionCloseCalls)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("second Close(): %v", err)
	}
	if fake.sessionCloseCalls != 1 {
		t.Errorf("session Close calls after idempotent re-Close = %d, want 1", fake.sessionCloseCalls)
	}
}

// TestLiveScreenCastGrant exercises the honest wire flow against the real
// session bus: portal presence -> picker negotiation -> stream attempts.
// It is gated behind INTERAGENT_PORTAL_INTEGRATION=1 and tolerates the human
// dialog not being answered (the portal may deny/never resolve the picker);
// the assertions are about the wire negotiation, not the user's click.
func TestLiveScreenCastGrant(t *testing.T) {
	if os.Getenv("INTERAGENT_PORTAL_INTEGRATION") == "" {
		t.Skip("set INTERAGENT_PORTAL_INTEGRATION=1 to run live portal tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	sc := NewScreenCast()
	defer func() { _ = sc.Close() }()

	if !sc.Available(ctx) {
		t.Skipf("portal %s has no owner on this session bus — not the desktop bus?", portalName)
	}

	err := sc.Grant(ctx)
	switch {
	case err == nil:
		if !sc.Granted() {
			t.Error("Granted() = false after a successful Grant")
		}
		st, openErr := sc.OpenStream(ctx)
		if openErr != nil {
			t.Errorf("OpenStream after successful Grant: %v", openErr)
			return
		}
		if st.FD() <= 0 {
			t.Errorf("stream FD = %d, want > 0", st.FD())
		}
		if st.SessionPath == "" {
			t.Error("stream SessionPath is empty")
		}
		if err := st.Close(); err != nil {
			t.Errorf("stream Close: %v", err)
		}
	case errors.Is(err, context.DeadlineExceeded):
		t.Logf("Grant blocked at the portal picker (no human interaction); negotiation reached the response wait: %v", err)
	case errors.Is(err, context.Canceled):
		t.Logf("Grant cancelled: %v", err)
	case strings.Contains(strings.ToLower(err.Error()), "cancel"):
		t.Logf("Grant returned the honest user-cancel error: %v", err)
	default:
		t.Errorf("Grant failed unexpectedly at wire level: %v", err)
	}
}

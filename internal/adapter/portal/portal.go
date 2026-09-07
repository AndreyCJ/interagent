// Package portal wraps the freedesktop XDG Desktop Portal D-Bus interfaces
// (ScreenCast / Session) used on Linux for system-audio capture and the
// matching permission grants. Honest negotiation: Availability -> Grant
// (portal picker) -> Start -> Ready -> Streams (PipeWire fd). Pure Go via
// godbus — no cgo.
package portal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	portalName      = "org.freedesktop.portal.Desktop"
	portalPath      = dbus.ObjectPath("/org/freedesktop/portal/desktop")
	screenCastIface = "org.freedesktop.portal.ScreenCast"
	requestIface    = "org.freedesktop.portal.Request"
	sessionIface    = "org.freedesktop.portal.Session"

	selectMonitor = uint32(1)
	selectWindow  = uint32(2)
)

// dbusConn is the slice of *dbus.Conn the portal needs; replaced by fakes in
// tests and by sessionConn at runtime.
type dbusConn interface {
	Object(dest string, path dbus.ObjectPath) dbus.BusObject
	Signal(ch chan<- *dbus.Signal)
	RemoveSignal(ch chan<- *dbus.Signal)
	AddMatchSignal(options ...dbus.MatchOption) error
	RemoveMatchSignal(options ...dbus.MatchOption) error
	NameHasOwner(name string) (bool, error)
	Close() error
}

// sessionConn adapts *dbus.Conn to the seam. godbus v5 does not expose
// NameHasOwner on the connection type, so the owner probe is done via the bus
// daemon's GetNameOwner method.
type sessionConn struct {
	*dbus.Conn
}

func (c *sessionConn) NameHasOwner(name string) (bool, error) {
	var owner string
	err := c.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, name).Store(&owner)
	if err == nil {
		return true, nil
	}
	var dErr dbus.Error
	if errors.As(err, &dErr) {
		switch dErr.Name {
		case "org.freedesktop.DBus.Error.NameHasNoOwner", "org.freedesktop.DBus.Error.ServiceUnknown":
			return false, nil
		}
	}
	return false, err
}

var connect = func() (dbusConn, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}
	return &sessionConn{Conn: conn}, nil
}

// restoreTokenPath is overridable in tests.
var restoreTokenPath = func() string {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = os.TempDir()
		}
		dir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dir, "interagent", "screencast-restore-token")
}

func loadRestoreToken() (string, error) {
	b, err := os.ReadFile(restoreTokenPath())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func saveRestoreToken(tok string) error {
	if tok == "" {
		return errors.New("refusing to persist an empty restore token")
	}
	p := restoreTokenPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(tok), 0o600)
}

func randSuffix() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "nonce"
	}
	return hex.EncodeToString(b)
}

// ScreenCast coordinates one portal ScreenCast user session.
type ScreenCast struct {
	mu      sync.Mutex
	conn    dbusConn
	granted bool
}

func NewScreenCast() *ScreenCast {
	return &ScreenCast{}
}

// NewScreenCastConn is for tests and pre-created connections.
func NewScreenCastConn(conn dbusConn) *ScreenCast {
	return &ScreenCast{conn: conn}
}

func (s *ScreenCast) connOrNew() (dbusConn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		c, err := connect()
		if err != nil {
			return nil, fmt.Errorf("session dbus: %w", err)
		}
		s.conn = c
	}
	return s.conn, nil
}

func (s *ScreenCast) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil {
		err := s.conn.Close()
		s.conn = nil
		return err
	}
	return nil
}

// Available reports whether the desktop exposes the ScreenCast portal. Honest:
// false only when the portal is genuinely absent or unreachable.
func (s *ScreenCast) Available(ctx context.Context) bool {
	conn, err := s.connOrNew()
	if err != nil {
		return false
	}
	has, err := conn.NameHasOwner(portalName)
	return err == nil && has
}

// Granted reports whether a screen-cast session has been granted this run OR a
// persisted restore token exists. Honest: it never returns true for nothing.
func (s *ScreenCast) Granted() bool {
	s.mu.Lock()
	granted := s.granted
	s.mu.Unlock()
	if granted {
		return true
	}
	tok, err := loadRestoreToken()
	return err == nil && tok != ""
}

// Grant runs the full ScreenCast flow (the portal picker). On success it
// records a run grant and persists the returned restore token (best-effort when
// the backend provides one). The portal picker IS the consent dialog.
func (s *ScreenCast) Grant(ctx context.Context) error {
	st, err := s.open(ctx, "")
	if err != nil {
		return err
	}
	token := st.RestoreToken
	if err := st.Close(); err != nil {
		return err
	}
	s.mu.Lock()
	s.granted = true
	s.mu.Unlock()
	if token != "" {
		_ = saveRestoreToken(token)
	}
	return nil
}

// OpenStream acquires a live ScreenCast audio stream. It reuses the persisted
// restore token when available (restores the prior selection without a picker);
// otherwise the portal picker is shown. Any new restore token is persisted.
func (s *ScreenCast) OpenStream(ctx context.Context) (*Stream, error) {
	tok, _ := loadRestoreToken()
	st, err := s.open(ctx, tok)
	if err != nil {
		return nil, err
	}
	if st.RestoreToken != "" {
		_ = saveRestoreToken(st.RestoreToken)
	}
	s.mu.Lock()
	s.granted = true
	s.mu.Unlock()
	return st, nil
}

// open runs the honest portal negotiation over one session:
// CreateSession -> Response (session_handle) -> SelectSources -> Response
// (selection) -> Start -> Response (streams) -> OpenPipeWireRemote.
//
// The modern portal replies to CreateSession with only the Request handle (o);
// the session handle is delivered in that request's Response results. Every
// step answers on its OWN request path, so we match Request::Response broadly
// BEFORE CreateSession (no response can be missed) and the waiter rejects
// signals not addressed to the request path we are waiting for.
func (s *ScreenCast) open(ctx context.Context, restoreToken string) (*Stream, error) {
	conn, err := s.connOrNew()
	if err != nil {
		return nil, fmt.Errorf("screen record: %w", err)
	}

	sigCh := make(chan *dbus.Signal, 8)
	conn.Signal(sigCh)
	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface(requestIface),
		dbus.WithMatchMember("Response"),
		dbus.WithMatchSender(portalName),
	); err != nil {
		conn.RemoveSignal(sigCh)
		return nil, fmt.Errorf("screen record dbus match: %w", err)
	}
	defer conn.RemoveMatchSignal(
		dbus.WithMatchInterface(requestIface),
		dbus.WithMatchMember("Response"),
		dbus.WithMatchSender(portalName),
	)
	defer conn.RemoveSignal(sigCh)

	portalObj := conn.Object(portalName, portalPath)

	sessionToken := "ia_session_" + randSuffix()
	reqToken := "ia_request_" + randSuffix()
	opts := map[string]dbus.Variant{
		"handle_token":         dbus.MakeVariant(reqToken),
		"session_handle_token": dbus.MakeVariant(sessionToken),
	}

	createReq, err := callRequestHandle(portalObj, screenCastIface+".CreateSession", opts)
	if err != nil {
		return nil, fmt.Errorf("screen record create session: %w", err)
	}

	responseCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	status, results, err := waitResponsePath(responseCtx, sigCh, createReq)
	if err != nil {
		return nil, fmt.Errorf("screen record create session response: %w", err)
	}
	if status != 0 {
		return nil, errors.New("screen record create session was rejected")
	}
	sessionPath := sessionHandleFrom(results)
	if sessionPath == "" {
		return nil, errors.New("screen record create session: no session_handle in response")
	}

	// Clean up the half-open session on any later failure.
	streamed := false
	defer func() {
		if !streamed {
			_ = conn.Object(portalName, sessionPath).Call(sessionIface+".Close", 0).Err
		}
	}()

	selectReq, err := callRequestHandle(portalObj, screenCastIface+".SelectSources", sessionPath, buildSelectOptions(true, restoreToken))
	if err != nil {
		return nil, fmt.Errorf("screen record select sources: %w", err)
	}
	status, _, err = waitResponsePath(responseCtx, sigCh, selectReq)
	if err != nil {
		return nil, fmt.Errorf("screen record selection: %w", err)
	}
	if status != 0 {
		return nil, errors.New("screen record selection was cancelled")
	}

	startReq, err := callRequestHandle(portalObj, screenCastIface+".Start", sessionPath, "", map[string]dbus.Variant{})
	if err != nil {
		return nil, fmt.Errorf("screen record start: %w", err)
	}
	status, results, err = waitResponsePath(responseCtx, sigCh, startReq)
	if err != nil {
		return nil, fmt.Errorf("screen record response: %w", err)
	}
	if status != 0 {
		return nil, errors.New("screen record start was rejected")
	}
	nodeID, props, token, err := resolveStreams(results)
	if err != nil {
		return nil, fmt.Errorf("screen record streams: %w", err)
	}

	var newFD dbus.UnixFD
	if err := portalObj.Call(screenCastIface+".OpenPipeWireRemote", 0, sessionPath, map[string]dbus.Variant{}).Store(&newFD); err != nil {
		return nil, fmt.Errorf("screen record open pipewire: %w", err)
	}

	streamed = true
	return &Stream{
		SessionPath:  sessionPath,
		NodeID:       nodeID,
		Props:        props,
		RestoreToken: token,
		conn:         conn,
		fd:           int(newFD),
	}, nil
}

// callRequestHandle invokes a portal method whose reply is (o request_handle)
// and returns the request handle. Lenient decode: portals have replied with
// both (o) and (oo) across versions; the request handle is always first.
func callRequestHandle(obj dbus.BusObject, method string, args ...interface{}) (dbus.ObjectPath, error) {
	call := obj.Call(method, 0, args...)
	if call.Err != nil {
		return "", call.Err
	}
	if len(call.Body) < 1 {
		return "", errors.New("empty reply")
	}
	h, ok := call.Body[0].(dbus.ObjectPath)
	if !ok {
		return "", errors.New("malformed request handle in reply")
	}
	return h, nil
}

// sessionHandleFrom extracts the session ObjectPath from a CreateSession
// Response results map. Portals deliver it as a string or an ObjectPath.
func sessionHandleFrom(results map[string]dbus.Variant) dbus.ObjectPath {
	v, ok := results["session_handle"]
	if !ok {
		return ""
	}
	switch h := v.Value().(type) {
	case string:
		return dbus.ObjectPath(h)
	case dbus.ObjectPath:
		return h
	}
	return ""
}

// Stream is an open portal ScreenCast audio node plus its PipeWire fd.
type Stream struct {
	SessionPath  dbus.ObjectPath
	NodeID       uint32
	Props        map[string]dbus.Variant
	RestoreToken string
	conn         dbusConn
	fd           int
	mu           sync.Mutex
	closed       bool
}

func (st *Stream) FD() int { return st.fd }

// Close destroys the underlying portal session (DestroySession semantics via
// org.freedesktop.portal.Session.Close) and releases the PipeWire fd. Idempotent.
func (st *Stream) Close() error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.closed {
		return nil
	}
	st.closed = true
	var firstErr error
	if st.conn != nil {
		if err := st.conn.Object(portalName, st.SessionPath).Call(sessionIface+".Close", 0).Err; err != nil {
			firstErr = err
		}
	}
	if st.fd > 0 {
		if err := closeFD(st.fd); firstErr == nil {
			firstErr = err
		}
		st.fd = -1
	}
	return firstErr
}

func buildSelectOptions(persist bool, restoreToken string) map[string]dbus.Variant {
	opts := map[string]dbus.Variant{
		"types":    dbus.MakeVariant(selectMonitor | selectWindow),
		"multiple": dbus.MakeVariant(false),
	}
	if persist {
		opts["persist_mode"] = dbus.MakeVariant(uint32(2))
	}
	if restoreToken != "" {
		opts["restore_token"] = dbus.MakeVariant(restoreToken)
	}
	return opts
}

// parseStreams unpacks the Response "streams" value, which godbus decodes as a
// []interface{} of structs-as-[]interface{}{ nodeID uint32, props a{sv} }.
func parseStreams(v any) (uint32, map[string]dbus.Variant, error) {
	arr, ok := v.([]interface{})
	if !ok || len(arr) == 0 {
		return 0, nil, errors.New("no streams in response")
	}
	first, ok := arr[0].([]interface{})
	if !ok || len(first) != 2 {
		return 0, nil, errors.New("malformed stream entry")
	}
	id, ok := first[0].(uint32)
	if !ok {
		return 0, nil, errors.New("malformed stream node id")
	}
	props, ok := first[1].(map[string]dbus.Variant)
	if !ok {
		return 0, nil, errors.New("malformed stream props")
	}
	return id, props, nil
}

// resolveStreams extracts the first stream plus restore token from a Start
// Response results map. The "streams" value arrives as a dbus.Variant wrapping
// the a(ua{sv}) payload, so it must be unwrapped (.Value()) before parseStreams.
func resolveStreams(results map[string]dbus.Variant) (uint32, map[string]dbus.Variant, string, error) {
	v, ok := results["streams"]
	if !ok {
		return 0, nil, "", errors.New("no streams in response")
	}
	nodeID, props, err := parseStreams(v.Value())
	if err != nil {
		return 0, nil, "", err
	}
	var token string
	if tv, ok := results["restore_token"]; ok {
		if tok, ok := tv.Value().(string); ok {
			token = tok
		}
	}
	return nodeID, props, token, nil
}

// waitResponse blocks until the portal Request::Response signal arrives or ctx
// is cancelled. Response body is (u status, a{sv} results).
func waitResponse(ctx context.Context, ch chan *dbus.Signal) (uint32, map[string]dbus.Variant, error) {
	return waitResponsePath(ctx, ch, "")
}

// waitResponsePath is waitResponse that additionally rejects signals whose
// request path differs from want. The portal emits one Response per
// CreateSession/SelectSources/Start (each on its own path); passing the Start
// request path selects the wire step that carries the streams.
func waitResponsePath(ctx context.Context, ch chan *dbus.Signal, want dbus.ObjectPath) (uint32, map[string]dbus.Variant, error) {
	for {
		select {
		case <-ctx.Done():
			return 0, nil, ctx.Err()
		case sig, ok := <-ch:
			if !ok {
				return 0, nil, errors.New("dbus signal channel closed")
			}
			if sig.Name != requestIface+".Response" {
				continue
			}
			if want != "" && sig.Path != want {
				continue
			}
			if len(sig.Body) < 2 {
				continue
			}
			status, ok1 := sig.Body[0].(uint32)
			results, ok2 := sig.Body[1].(map[string]dbus.Variant)
			if ok1 && ok2 {
				return status, results, nil
			}
		}
	}
}

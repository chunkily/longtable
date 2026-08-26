// Package listener serves an HTTP handler on an address that can change
// while the program keeps running.
//
// It exists so that changing `addr` doesn't mean restarting the process.
// A double-clicked binary has no supervisor: if it exits, nothing brings
// it back, and a Host who changed a setting is left with a browser tab
// that will never load and no window to read an error in. Re-executing
// is worse — on Windows the console the process owns goes with it.
//
// See planning/backlog/rebind-without-dropping-the-server.md.
package listener

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// ErrNeedsRestart says a change is legitimate but cannot be made while
// running, so the caller should store it and say a restart is needed
// rather than treat it as a failure. See Rebind.
var ErrNeedsRestart = errors.New("this address can only be changed by restarting")

// shutdownGrace is how long a replaced listener gets to finish what it
// was already doing. Nothing waits on it — the new listener is already
// accepting by then — so this only bounds how long a stale connection
// may linger.
const shutdownGrace = 5 * time.Second

// Swappable is an http.Handler being served on an address that can be
// changed.
type Swappable struct {
	handler http.Handler
	fatal   chan error

	mu sync.Mutex
	// requested is the address as it was asked for (":8080"), which is
	// what a setting holds and what an unchanged value compares against.
	requested string
	// bound is what it actually resolved to ("[::]:8080"), which is what
	// a caller should print and what a port comparison should use.
	bound  string
	server *http.Server
}

// Start binds addr and begins serving.
//
// A failure here is fatal to the caller by design: a server that cannot
// listen where it was told to has not started, and pretending otherwise
// leaves a Host looking for it at an address nothing is on.
func Start(addr string, handler http.Handler) (*Swappable, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	s := &Swappable{
		handler:   handler,
		fatal:     make(chan error, 1),
		requested: addr,
		bound:     ln.Addr().String(),
		server:    &http.Server{Handler: handler},
	}
	go s.run(s.server, ln)
	return s, nil
}

// Rebind moves to a new address without dropping the current one until
// the new one is accepting.
//
// **The new address is bound before the old one is released**, and that
// ordering is the whole point. If the bind fails — a port already in use
// is the likely case — nothing has been given up: the old listener is
// still serving, and the caller gets an error to show whoever asked.
// Shut down first and a failed bind leaves a Host with *neither*
// address, no admin page to fix it from, and only the CLI left.
//
// It also means a caller can respond immediately. By the time Rebind
// returns, the new listener is accepting, so a redirect to it cannot
// arrive early.
//
// Three outcomes worth handling separately:
//
//   - nil — now serving on next.
//   - ErrNeedsRestart — a legitimate change that can't be made live.
//     Store it and say so.
//   - anything else — could not bind. Nothing changed; don't store it
//     either, or the next start will fail on the same address.
func (s *Swappable) Rebind(next string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Either spelling of where we already are is a no-op. A caller may
	// hold the address as it was asked for (":8080", which is what a
	// setting stores) or as it resolved ("[::]:8080", which is what Addr
	// reports), and neither is a request to change anything. Comparing
	// only the first sent the second down the same-port path below and
	// answered "restart to stay where you are".
	if next == s.requested || next == s.bound {
		return nil
	}

	// Same port, different host — ":8080" to "127.0.0.1:8080" — cannot be
	// bound before releasing, because the two overlap. Linux refuses the
	// second bind outright; Windows allows it and then hands loopback to
	// the more specific listener, which is how a Host once ended up with
	// the admin page on localhost and their rooms on the LAN address
	// (see internal/admin). Doing it the other way round, shutdown first,
	// would reintroduce exactly the "neither address" failure this whole
	// type exists to prevent. So it is refused, honestly, as a restart.
	if samePort(next, s.bound) {
		return ErrNeedsRestart
	}

	ln, err := net.Listen("tcp", next)
	if err != nil {
		return fmt.Errorf("couldn't listen on %s: %w", next, err)
	}

	old := s.server
	s.server = &http.Server{Handler: s.handler}
	s.requested, s.bound = next, ln.Addr().String()
	go s.run(s.server, ln)

	// Released in the background: the new listener is already accepting,
	// so nothing is waiting on this, and a request still in flight on the
	// old one gets a moment to finish. Doing it inline would also
	// deadlock a caller that is itself being served by the listener being
	// replaced.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		_ = old.Shutdown(ctx)
	}()

	return nil
}

// Addr is the address currently being served, as it resolved.
func (s *Swappable) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bound
}

// Fatal carries a serve error that ended a listener for a reason other
// than being replaced. Nothing is sent when a rebind retires the old
// server, which is an ordinary end rather than a failure.
func (s *Swappable) Fatal() <-chan error { return s.fatal }

func (s *Swappable) run(srv *http.Server, ln net.Listener) {
	err := srv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return
	}
	// Non-blocking: the first failure is the interesting one, and a
	// second must not wedge a goroutine forever if nobody is reading.
	select {
	case s.fatal <- err:
	default:
	}
}

// samePort reports whether two listen addresses want the same port.
func samePort(a, b string) bool {
	_, aPort, aErr := net.SplitHostPort(a)
	_, bPort, bErr := net.SplitHostPort(b)
	return aErr == nil && bErr == nil && aPort == bPort
}

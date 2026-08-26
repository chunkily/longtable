package listener

import (
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// answering is a handler that says which server answered, so a test can
// tell the new listener from the old.
func answering(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, name)
	})
}

// start brings one up on an ephemeral port, so these tests never fight
// each other or anything else on the machine.
func start(t *testing.T, name string) *Swappable {
	t.Helper()

	s, err := Start("127.0.0.1:0", answering(name))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	return s
}

// reach reports what the address answers, or "" if nothing does.
func reach(t *testing.T, addr string) string {
	t.Helper()

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + addr + "/")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return string(body)
}

// freePort takes a port and gives it straight back, so the number is one
// nothing is listening on.
func freePort(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

func TestRebind_MovesToTheNewAddress(t *testing.T) {
	s := start(t, "longtable")
	before := s.Addr()

	if got := reach(t, before); got != "longtable" {
		t.Fatalf("the original address answered %q", got)
	}

	next := freePort(t)
	if err := s.Rebind(next); err != nil {
		t.Fatalf("Rebind: %v", err)
	}

	// Accepting by the time Rebind returned — no wait, no poll. That is
	// what lets a caller redirect straight away without the browser
	// arriving before anything is listening.
	if got := reach(t, next); got != "longtable" {
		t.Fatalf("the new address answered %q, want longtable", got)
	}
	if s.Addr() != next {
		t.Errorf("Addr() = %q, want %q", s.Addr(), next)
	}

	// And the old one stops, though not necessarily instantly — it is
	// given a moment to finish what it was doing.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if reach(t, before) == "" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Error("the old address is still answering")
}

// The rule the whole type exists for. A bind that fails must cost
// nothing: shut down first and a Host is left with no game, no admin
// page, and only the CLI to recover from.
func TestRebind_KeepsTheOldAddressWhenTheNewOneIsTaken(t *testing.T) {
	s := start(t, "longtable")
	before := s.Addr()

	// Something else is already on the address being moved to.
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer taken.Close()

	err = s.Rebind(taken.Addr().String())
	if err == nil {
		t.Fatal("rebinding onto a taken port succeeded")
	}
	if errors.Is(err, ErrNeedsRestart) {
		t.Fatalf("a taken port was reported as needing a restart: %v", err)
	}

	if got := reach(t, before); got != "longtable" {
		t.Fatalf("the old address answered %q after a failed rebind — the server was dropped", got)
	}
	if s.Addr() != before {
		t.Errorf("Addr() = %q, want the old %q", s.Addr(), before)
	}
}

// Same port, different host. It cannot be bound before releasing because
// the two overlap, and doing it the other way round is the failure
// above. Refused honestly instead.
func TestRebind_RefusesASamePortChange(t *testing.T) {
	s := start(t, "longtable")
	before := s.Addr()

	_, port, err := net.SplitHostPort(before)
	if err != nil {
		t.Fatalf("SplitHostPort: %v", err)
	}

	if err := s.Rebind(":" + port); !errors.Is(err, ErrNeedsRestart) {
		t.Fatalf("err = %v, want ErrNeedsRestart", err)
	}
	if got := reach(t, before); got != "longtable" {
		t.Errorf("the server stopped answering after a refused rebind: %q", got)
	}
}

func TestRebind_ToTheSameAddressDoesNothing(t *testing.T) {
	s := start(t, "longtable")
	before := s.Addr()

	if err := s.Rebind(before); err != nil {
		t.Fatalf("rebinding to the current address: %v", err)
	}
	if got := reach(t, before); got != "longtable" {
		t.Errorf("answered %q after a no-op rebind", got)
	}
}

// A retired listener ends with ErrServerClosed, which is an ordinary end
// rather than a failure. Reporting it would take down a server that is
// working perfectly, at the moment a Host changed a setting.
func TestRebind_DoesNotReportTheRetiredListenerAsAFailure(t *testing.T) {
	s := start(t, "longtable")

	if err := s.Rebind(freePort(t)); err != nil {
		t.Fatalf("Rebind: %v", err)
	}

	select {
	case err := <-s.Fatal():
		t.Fatalf("a rebind reported a fatal error: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
}

// A server that cannot listen where it was told has not started, and
// saying otherwise sends a Host looking for it somewhere nothing is.
func TestStart_FailsOnAnAddressItCannotHave(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer taken.Close()

	if _, err := Start(taken.Addr().String(), answering("longtable")); err == nil {
		t.Fatal("starting on a taken port succeeded")
	}
}

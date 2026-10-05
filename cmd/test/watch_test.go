package main

import (
	"testing"

	"github.com/kevinnadar22/ledis/internal/resp"
)

func TestWatchRequiresAtLeastOneKey(t *testing.T) {
	sess := newTestSession(t)
	got := sess.Execute(makeCommand("WATCH"))
	if got != "-ERR WATCH requires at least one key\r\n" {
		t.Fatalf("got %q", got)
	}
}

func TestWatchReturnsOK(t *testing.T) {
	sess := newTestSession(t)
	got := sess.Execute(makeCommand("WATCH", "foo"))
	if got != "+OK\r\n" {
		t.Fatalf("got %q, want +OK\\r\\n", got)
	}
}

func TestWatchMultipleKeys(t *testing.T) {
	sess := newTestSession(t)
	got := sess.Execute(makeCommand("WATCH", "a", "b"))
	if got != "+OK\r\n" {
		t.Fatalf("got %q", got)
	}
}

func TestUnwatchNoArgsReturnsOK(t *testing.T) {
	sess := newTestSession(t)
	sess.Execute(makeCommand("WATCH", "foo"))
	got := sess.Execute(makeCommand("UNWATCH"))
	if got != "+OK\r\n" {
		t.Fatalf("got %q", got)
	}
}

func TestUnwatchWithKeyArgsReturnsError(t *testing.T) {
	sess := newTestSession(t)
	got := sess.Execute(makeCommand("UNWATCH", "foo"))
	if got != "-ERR wrong number of arguments for UNWATCH\r\n" {
		t.Fatalf("got %q", got)
	}
}

// Redis: if a watched key is changed by another client before EXEC, EXEC returns null.
func TestExecNilWhenOtherClientModifiesWatchedKey(t *testing.T) {
	srv := newTestServer(t)
	watcher := newTestSessionFromServer(srv)
	other := newTestSessionFromServer(srv)

	watcher.Execute(makeCommand("WATCH", "foo"))
	watcher.Execute(makeCommand("MULTI"))
	watcher.Execute(makeCommand("SET", "foo", "from-tx"))

	other.Execute(makeCommand("SET", "foo", "from-other"))

	got := watcher.Execute(makeCommand("EXEC"))
	if got != resp.EncodeNil() {
		t.Fatalf("EXEC: got %q, want null bulk string %q", got, resp.EncodeNil())
	}
	if got := watcher.Execute(makeCommand("GET", "foo")); got != "$10\r\nfrom-other\r\n" {
		t.Fatalf("GET foo: got %q (transaction should not have applied)", got)
	}
}

func TestWatchesClearedAfterExec(t *testing.T) {
	srv := newTestServer(t)
	sess := newTestSessionFromServer(srv)
	other := newTestSessionFromServer(srv)

	sess.Execute(makeCommand("WATCH", "foo"))
	sess.Execute(makeCommand("MULTI"))
	sess.Execute(makeCommand("SET", "foo", "first"))
	sess.Execute(makeCommand("EXEC"))

	other.Execute(makeCommand("SET", "foo", "second"))
	sess.Execute(makeCommand("MULTI"))
	sess.Execute(makeCommand("SET", "foo", "third"))
	got := sess.Execute(makeCommand("EXEC"))
	want := resp.EncodeArrayOfReplies([]string{"+OK\r\n"})
	if got != want {
		t.Fatalf("EXEC without WATCH should not abort; got %q, want %q", got, want)
	}
	if got := sess.Execute(makeCommand("GET", "foo")); got != "$5\r\nthird\r\n" {
		t.Fatalf("GET foo: got %q", got)
	}
}

func TestExecAppliesWhenWatchedKeyUnchanged(t *testing.T) {
	srv := newTestServer(t)
	sess := newTestSessionFromServer(srv)

	sess.Execute(makeCommand("WATCH", "foo"))
	sess.Execute(makeCommand("MULTI"))
	sess.Execute(makeCommand("SET", "foo", "kept"))
	got := sess.Execute(makeCommand("EXEC"))
	want := resp.EncodeArrayOfReplies([]string{"+OK\r\n"})
	if got != want {
		t.Fatalf("EXEC: got %q, want %q", got, want)
	}
	if got := sess.Execute(makeCommand("GET", "foo")); got != "$4\r\nkept\r\n" {
		t.Fatalf("GET foo: got %q", got)
	}
}

// UNWATCH should only drop the current client's watches, not every client on the server.
func TestUnwatchOnlyAffectsCurrentSession(t *testing.T) {
	srv := newTestServer(t)
	sessA := newTestSessionFromServer(srv)
	sessB := newTestSessionFromServer(srv)

	sessA.Execute(makeCommand("WATCH", "shared"))
	sessB.Execute(makeCommand("WATCH", "shared"))
	sessA.Execute(makeCommand("UNWATCH"))

	sessB.Execute(makeCommand("MULTI"))
	sessB.Execute(makeCommand("SET", "shared", "b-val"))
	other := newTestSessionFromServer(srv)
	other.Execute(makeCommand("SET", "shared", "other"))

	got := sessB.Execute(makeCommand("EXEC"))
	if got != resp.EncodeNil() {
		t.Fatalf("sessB EXEC should abort when watched key changed; got %q", got)
	}
}

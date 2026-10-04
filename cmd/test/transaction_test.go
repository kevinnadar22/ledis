package main

import (
	"testing"

	"github.com/kevinnadar22/ledis/internal/resp"
)

func TestTransactionMultiExec(t *testing.T) {
	srv := newTestServer(t)
	sess := newTestSessionFromServer(srv)

	if got := sess.Execute(makeCommand("MULTI")); got != "+OK\r\n" {
		t.Fatalf("MULTI: got %q, want +OK\\r\\n", got)
	}
	if got := sess.Execute(makeCommand("SET", "txkey", "txval")); got != "+QUEUED\r\n" {
		t.Fatalf("SET in MULTI: got %q, want +QUEUED\\r\\n", got)
	}
	execRes := sess.Execute(makeCommand("EXEC"))
	if execRes != resp.EncodeArrayOfReplies([]string{"+OK\r\n"}) {
		t.Fatalf("EXEC: got %q", execRes)
	}
	got := sess.Execute(makeCommand("GET", "txkey"))
	if got != "$5\r\ntxval\r\n" {
		t.Fatalf("GET after EXEC: got %q", got)
	}
}

func TestTransactionDiscard(t *testing.T) {
	srv := newTestServer(t)
	sess := newTestSessionFromServer(srv)

	sess.Execute(makeCommand("MULTI"))
	sess.Execute(makeCommand("SET", "discardme", "1"))
	if got := sess.Execute(makeCommand("DISCARD")); got != "+OK\r\n" {
		t.Fatalf("DISCARD: got %q", got)
	}
	if got := sess.Execute(makeCommand("GET", "discardme")); got != "$-1\r\n" {
		t.Fatalf("GET after DISCARD: expected key absent, got %q", got)
	}
	if got := sess.Execute(makeCommand("EXEC")); got != "-ERR EXEC without MULTI\r\n" {
		t.Fatalf("EXEC after DISCARD: got %q", got)
	}
}

func TestTransactionExecWithoutMulti(t *testing.T) {
	sess := newTestSession(t)
	got := sess.Execute(makeCommand("EXEC"))
	if got != "-ERR EXEC without MULTI\r\n" {
		t.Errorf("got %q", got)
	}
}

func TestTransactionNestedMulti(t *testing.T) {
	sess := newTestSession(t)
	sess.Execute(makeCommand("MULTI"))
	got := sess.Execute(makeCommand("MULTI"))
	if got != "-ERR MULTI calls can not be nested\r\n" {
		t.Errorf("got %q", got)
	}
}

func TestTransactionExecAbortAfterErrorInMulti(t *testing.T) {
	sess := newTestSession(t)
	sess.Execute(makeCommand("MULTI"))
	got := sess.Execute(makeCommand("FOO"))
	if got != "-ERR unknown command 'FOO'\r\n" {
		t.Fatalf("unknown in MULTI: got %q", got)
	}
	got = sess.Execute(makeCommand("EXEC"))
	want := "-EXECABORT Transaction discarded because of previous errors\r\n"
	if got != want {
		t.Fatalf("EXEC: got %q, want %q", got, want)
	}
}

func TestTransactionSubscribeNotAllowedInMulti(t *testing.T) {
	sess := newTestSession(t)
	sess.Execute(makeCommand("MULTI"))
	got := sess.Execute(makeCommand("SUBSCRIBE", "chan"))
	if got != "-ERR command not allowed in transaction\r\n" {
		t.Errorf("got %q", got)
	}
}

func TestTransactionMultipleQueuedCommands(t *testing.T) {
	srv := newTestServer(t)
	sess := newTestSessionFromServer(srv)

	sess.Execute(makeCommand("MULTI"))
	sess.Execute(makeCommand("SET", "a", "1"))
	sess.Execute(makeCommand("INCR", "a"))
	execRes := sess.Execute(makeCommand("EXEC"))
	wantExec := resp.EncodeArrayOfReplies([]string{"+OK\r\n", ":2\r\n"})
	if execRes != wantExec {
		t.Fatalf("EXEC: got %q, want %q", execRes, wantExec)
	}
	if got := sess.Execute(makeCommand("GET", "a")); got != "$1\r\n2\r\n" {
		t.Fatalf("GET a: got %q", got)
	}
}

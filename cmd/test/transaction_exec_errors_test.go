package main

import (
	"testing"

	"github.com/kevinnadar22/ledis/internal/resp"
)

// MULTI → SET a → queue-time error → EXECABORT; key a must not exist.
func TestExecAbortSetA(t *testing.T) {
	srv := newTestServer(t)
	sess := newTestSessionFromServer(srv)

	sess.Execute(makeCommand("MULTI"))
	if got := sess.Execute(makeCommand("SET", "a", "1")); got != "+QUEUED\r\n" {
		t.Fatalf("SET a: got %q, want +QUEUED\\r\\n", got)
	}
	if got := sess.Execute(makeCommand("NOTACOMMAND")); got != "-ERR unknown command 'NOTACOMMAND'\r\n" {
		t.Fatalf("queue-time error: got %q", got)
	}

	got := sess.Execute(makeCommand("EXEC"))
	want := "-EXECABORT Transaction discarded because of previous errors\r\n"
	if got != want {
		t.Fatalf("EXEC: got %q, want %q", got, want)
	}
	if got := sess.Execute(makeCommand("GET", "a")); got != "$-1\r\n" {
		t.Fatalf("GET a: got %q — SET a must not have run", got)
	}
}

// Queue-time error: bad command while in MULTI → EXECABORT and no queued command runs.
func TestExecAbortQueueTimeErrorNothingApplied(t *testing.T) {
	srv := newTestServer(t)
	sess := newTestSessionFromServer(srv)

	sess.Execute(makeCommand("MULTI"))
	if got := sess.Execute(makeCommand("SET", "pending", "never")); got != "+QUEUED\r\n" {
		t.Fatalf("SET: got %q, want +QUEUED\\r\\n", got)
	}
	if got := sess.Execute(makeCommand("NOTACOMMAND")); got != "-ERR unknown command 'NOTACOMMAND'\r\n" {
		t.Fatalf("queue-time error: got %q", got)
	}

	got := sess.Execute(makeCommand("EXEC"))
	want := "-EXECABORT Transaction discarded because of previous errors\r\n"
	if got != want {
		t.Fatalf("EXEC: got %q, want %q", got, want)
	}
	if got := sess.Execute(makeCommand("GET", "pending")); got != "$-1\r\n" {
		t.Fatalf("GET pending: got %q — queued SET must not have run", got)
	}
}

// Runtime error: valid at queue time, fails at EXEC; other commands in the batch still apply.
func TestExecRuntimeErrorPartialSuccess(t *testing.T) {
	srv := newTestServer(t)
	sess := newTestSessionFromServer(srv)

	srv.DB().Set("bad", "not-an-integer")

	sess.Execute(makeCommand("MULTI"))
	sess.Execute(makeCommand("SET", "good", "1"))
	sess.Execute(makeCommand("INCR", "bad"))
	sess.Execute(makeCommand("SET", "also", "2"))

	got := sess.Execute(makeCommand("EXEC"))
	want := resp.EncodeArrayOfReplies([]string{
		"+OK\r\n",
		"-ERR value is not an integer or out of range\r\n",
		"+OK\r\n",
	})
	if got != want {
		t.Fatalf("EXEC: got %q, want %q", got, want)
	}
	if got := sess.Execute(makeCommand("GET", "good")); got != "$1\r\n1\r\n" {
		t.Fatalf("GET good: got %q", got)
	}
	if got := sess.Execute(makeCommand("GET", "also")); got != "$1\r\n2\r\n" {
		t.Fatalf("GET also: got %q", got)
	}
	if got := sess.Execute(makeCommand("GET", "bad")); got != "$14\r\nnot-an-integer\r\n" {
		t.Fatalf("GET bad: got %q — INCR must not have changed the key", got)
	}
}

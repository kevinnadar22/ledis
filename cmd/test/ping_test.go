package main

import (
	"testing"
)

func TestPingCommand(t *testing.T) {
	sess := newTestSession(t)
	cmd := makeCommand("PING")
	res, err := sess.Ping(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "+PONG\r\n" {
		t.Errorf("expected +PONG\\r\\n, got %q", res)
	}
}

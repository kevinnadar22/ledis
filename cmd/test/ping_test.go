package main

import (
	"testing"

	"github.com/kevinnadar22/ledis/internal/commands"
)

func TestPingCommand(t *testing.T) {
	cmd := makeCommand("PING")
	res, err := commands.Ping(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "+PONG\r\n" {
		t.Errorf("expected +PONG\\r\\n, got %q", res)
	}
}

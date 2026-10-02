package main

import (
	"testing"
)

func TestEchoCommand(t *testing.T) {
	sess := newTestSession(t)

	t.Run("Valid echo", func(t *testing.T) {
		cmd := makeCommand("ECHO", "hello")
		res, err := sess.Echo(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != "+hello\r\n" {
			t.Errorf("expected +hello\\r\\n, got %q", res)
		}
	})

	t.Run("Missing arguments", func(t *testing.T) {
		cmd := makeCommand("ECHO")
		_, err := sess.Echo(cmd)
		if err == nil {
			t.Error("expected error but got nil")
		}
	})
}

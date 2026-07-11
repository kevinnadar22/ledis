package main

import (
	"testing"
)

func TestEchoCommand(t *testing.T) {
	srv := newTestServer(t)

	t.Run("Valid echo", func(t *testing.T) {
		cmd := makeCommand("ECHO", "hello")
		res, err := srv.Echo(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != "+hello\r\n" {
			t.Errorf("expected +hello\\r\\n, got %q", res)
		}
	})

	t.Run("Missing arguments", func(t *testing.T) {
		cmd := makeCommand("ECHO")
		_, err := srv.Echo(cmd)
		if err == nil {
			t.Error("expected error but got nil")
		}
	})
}

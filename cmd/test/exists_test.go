package main

import (
	"testing"
)

func TestExistsCommand(t *testing.T) {
	srv := newTestServer(t)
	sess := newTestSessionFromServer(srv)

	t.Run("Exists on non-existing key", func(t *testing.T) {
		cmd := makeCommand("EXISTS", "key")
		res, err := sess.Exists(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != ":0\r\n" {
			t.Errorf("expected :0\\r\\n, got %q", res)
		}
	})

	t.Run("Exists on existing key", func(t *testing.T) {
		srv.DB().Set("key", "val")
		cmd := makeCommand("EXISTS", "key")
		res, err := sess.Exists(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != ":1\r\n" {
			t.Errorf("expected :1\\r\\n, got %q", res)
		}
	})

	t.Run("Missing arguments", func(t *testing.T) {
		cmd := makeCommand("EXISTS")
		_, err := sess.Exists(cmd)
		if err == nil {
			t.Error("expected error but got nil")
		}
	})
}

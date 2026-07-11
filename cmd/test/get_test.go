package main

import (
	"testing"
)

func TestGetCommand(t *testing.T) {
	srv := newTestServer(t)

	t.Run("Get non-existing key", func(t *testing.T) {
		cmd := makeCommand("GET", "nonexistent")
		res, err := srv.Get(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != "$-1\r\n" {
			t.Errorf("expected null bulk string $-1\\r\\n, got %q", res)
		}
	})

	t.Run("Get existing key", func(t *testing.T) {
		srv.DB().Set("mykey", "myval")
		cmd := makeCommand("GET", "mykey")
		res, err := srv.Get(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != "$5\r\nmyval\r\n" {
			t.Errorf("expected bulk string for myval, got %q", res)
		}
	})

	t.Run("Missing arguments", func(t *testing.T) {
		cmd := makeCommand("GET")
		_, err := srv.Get(cmd)
		if err == nil {
			t.Error("expected error but got nil")
		}
	})
}

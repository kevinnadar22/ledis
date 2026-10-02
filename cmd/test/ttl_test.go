package main

import (
	"testing"
	"time"
)

func TestTTLCommand(t *testing.T) {
	srv := newTestServer(t)
	sess := newTestSessionFromServer(srv)

	t.Run("TTL on non-existing key", func(t *testing.T) {
		cmd := makeCommand("TTL", "nonexistent")
		res, err := sess.TTL(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != ":-2\r\n" {
			t.Errorf("expected :-2\\r\\n for non-existing key, got %q", res)
		}
	})

	t.Run("TTL on existing key without expiry", func(t *testing.T) {
		srv.DB().Set("key", "val")
		cmd := makeCommand("TTL", "key")
		res, err := sess.TTL(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != ":-1\r\n" {
			t.Errorf("expected :-1\\r\\n for key without expiry, got %q", res)
		}
	})

	t.Run("TTL on key with expiry", func(t *testing.T) {
		srv.DB().Set("key", "val")
		srv.DB().Expire("key", 10) // 10 seconds expiration
		cmd := makeCommand("TTL", "key")
		res, err := sess.TTL(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Expecting positive TTL (around 9 or 10 seconds)
		if res == ":-1\r\n" || res == ":-2\r\n" {
			t.Errorf("expected positive TTL, got %q", res)
		}
	})

	t.Run("TTL on expired key", func(t *testing.T) {
		srv.DB().Set("key", "val")
		srv.DB().Expire("key", 1) // 1 second expiration
		time.Sleep(1100 * time.Millisecond)

		cmd := makeCommand("TTL", "key")
		res, err := sess.TTL(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != ":-2\r\n" {
			t.Errorf("expected :-2\\r\\n for expired key, got %q", res)
		}
	})

	t.Run("Wrong number of arguments", func(t *testing.T) {
		cmd := makeCommand("TTL")
		_, err := sess.TTL(cmd)
		if err == nil {
			t.Error("expected error but got nil")
		}
	})
}

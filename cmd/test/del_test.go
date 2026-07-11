package main

import (
	"testing"
)

func TestDelCommand(t *testing.T) {
	srv := newTestServer(t)
	srv.DB().Set("key1", "val1")
	srv.DB().Set("key2", "val2")

	t.Run("Delete existing and non-existing keys", func(t *testing.T) {
		cmd := makeCommand("DEL", "key1", "key2", "key3")
		res, err := srv.Del(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != ":2\r\n" {
			t.Errorf("expected :2\\r\\n, got %q", res)
		}
		if srv.DB().Exist("key1") || srv.DB().Exist("key2") {
			t.Error("expected key1 and key2 to be deleted")
		}
	})

	t.Run("Missing arguments", func(t *testing.T) {
		cmd := makeCommand("DEL")
		_, err := srv.Del(cmd)
		if err == nil {
			t.Error("expected error but got nil")
		}
	})
}

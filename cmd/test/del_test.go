package main

import (
	"testing"

	"github.com/kevinnadar22/ledis/internal/commands"
	"github.com/kevinnadar22/ledis/internal/store"
)

func TestDelCommand(t *testing.T) {
	store.DB.FlushAll()
	store.DB.Set("key1", "val1")
	store.DB.Set("key2", "val2")

	t.Run("Delete existing and non-existing keys", func(t *testing.T) {
		cmd := makeCommand("DEL", "key1", "key2", "key3")
		res, err := commands.Del(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != ":2\r\n" {
			t.Errorf("expected :2\\r\\n, got %q", res)
		}
		if store.DB.Exist("key1") || store.DB.Exist("key2") {
			t.Error("expected key1 and key2 to be deleted")
		}
	})

	t.Run("Missing arguments", func(t *testing.T) {
		cmd := makeCommand("DEL")
		_, err := commands.Del(cmd)
		if err == nil {
			t.Error("expected error but got nil")
		}
	})
}

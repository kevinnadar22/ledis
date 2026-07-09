package main

import (
	"testing"

	"github.com/kevinnadar22/ledis/internal/commands"
	"github.com/kevinnadar22/ledis/internal/store"
)

func TestExistsCommand(t *testing.T) {
	store.DB.FlushAll()

	t.Run("Exists on non-existing key", func(t *testing.T) {
		cmd := makeCommand("EXISTS", "key")
		res, err := commands.Exists(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != ":0\r\n" {
			t.Errorf("expected :0\\r\\n, got %q", res)
		}
	})

	t.Run("Exists on existing key", func(t *testing.T) {
		store.DB.Set("key", "val")
		cmd := makeCommand("EXISTS", "key")
		res, err := commands.Exists(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != ":1\r\n" {
			t.Errorf("expected :1\\r\\n, got %q", res)
		}
	})

	t.Run("Missing arguments", func(t *testing.T) {
		cmd := makeCommand("EXISTS")
		_, err := commands.Exists(cmd)
		if err == nil {
			t.Error("expected error but got nil")
		}
	})
}

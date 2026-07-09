package main

import (
	"testing"

	"github.com/kevinnadar22/ledis/internal/commands"
	"github.com/kevinnadar22/ledis/internal/store"
)

func TestIncrCommand(t *testing.T) {
	store.DB.FlushAll()

	t.Run("Incr non-existing key", func(t *testing.T) {
		cmd := makeCommand("INCR", "counter")
		res, err := commands.INCR(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != ":1\r\n" {
			t.Errorf("expected :1\\r\\n, got %q", res)
		}
	})

	t.Run("Incr existing integer key", func(t *testing.T) {
		cmd := makeCommand("INCR", "counter")
		res, err := commands.INCR(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != ":2\r\n" {
			t.Errorf("expected :2\\r\\n, got %q", res)
		}
	})

	t.Run("Incr non-integer key", func(t *testing.T) {
		store.DB.Set("notanint", "hello")
		cmd := makeCommand("INCR", "notanint")
		_, err := commands.INCR(cmd)
		if err == nil {
			t.Error("expected error for non-integer increment, got nil")
		}
	})

	t.Run("Missing arguments", func(t *testing.T) {
		cmd := makeCommand("INCR")
		_, err := commands.INCR(cmd)
		if err == nil {
			t.Error("expected error but got nil")
		}
	})
}

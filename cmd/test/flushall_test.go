package main

import (
	"testing"

	"github.com/kevinnadar22/ledis/internal/commands"
	"github.com/kevinnadar22/ledis/internal/store"
)

func TestFlushAllCommand(t *testing.T) {
	store.DB.Set("key1", "val1")
	store.DB.Set("key2", "val2")

	cmd := makeCommand("FLUSHALL")
	res, err := commands.FlushAll(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "+OK\r\n" {
		t.Errorf("expected +OK\\r\\n, got %q", res)
	}
	if store.DB.Count() != 0 {
		t.Errorf("expected store to be empty, got count %d", store.DB.Count())
	}
}

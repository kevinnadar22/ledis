package main

import (
	"testing"
)

func TestFlushAllCommand(t *testing.T) {
	srv := newTestServer(t)
	sess := newTestSessionFromServer(srv)
	srv.DB().Set("key1", "val1")
	srv.DB().Set("key2", "val2")

	cmd := makeCommand("FLUSHALL")
	res, err := sess.FlushAll(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "+OK\r\n" {
		t.Errorf("expected +OK\\r\\n, got %q", res)
	}
	if srv.DB().Count() != 0 {
		t.Errorf("expected store to be empty, got count %d", srv.DB().Count())
	}
}

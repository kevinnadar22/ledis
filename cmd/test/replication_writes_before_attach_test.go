package main

import (
	"fmt"
	"net"
	"testing"
	"time"
)

func TestReplicationWritesBeforeAttach(t *testing.T) {
	master := startTCPLedis(t, true)
	replica := startTCPLedis(t, false)

	masterClient := dialRedis(t, master.Addr)
	defer masterClient.Close()

	const preAttachWrites = 50
	for i := 0; i < preAttachWrites; i++ {
		key := fmt.Sprintf("repl:pre:%03d", i)
		val := fmt.Sprintf("before-%d", i)
		if got := masterClient.Do("SET", key, val); got != "+OK\r\n" {
			t.Fatalf("pre-attach SET %s: got %q", key, got)
		}
	}

	masterOffsetBeforeAttach := master.Srv.ReplOffset()
	if masterOffsetBeforeAttach <= 0 {
		t.Fatalf("master repl offset = %d after %d pre-attach writes, want > 0", masterOffsetBeforeAttach, preAttachWrites)
	}

	host, port, err := net.SplitHostPort(master.Addr)
	if err != nil {
		t.Fatalf("split master addr: %v", err)
	}

	admin := dialRedis(t, replica.Addr)
	defer admin.Close()
	if got := admin.Do("REPLICAOF", host, port); got != "+OK\r\n" {
		t.Fatalf("REPLICAOF: got %q", got)
	}

	waitForStoresEqual(t, master.Srv.DB(), replica.Srv.DB(), 5*time.Second)
	waitForReplOffsetsEqual(t, master, replica, 5*time.Second)

	for i := 0; i < preAttachWrites; i++ {
		key := fmt.Sprintf("repl:pre:%03d", i)
		want := fmt.Sprintf("before-%d", i)
		got, ok := replica.Srv.DB().Get(key)
		if !ok || got != want {
			t.Fatalf("replica key %q = %q (ok=%v), want %q", key, got, ok, want)
		}
	}

	if master.Srv.ReplOffset() != masterOffsetBeforeAttach {
		t.Fatalf("master offset changed during attach: before=%d after=%d",
			masterOffsetBeforeAttach, master.Srv.ReplOffset())
	}
	if replica.Srv.ReplOffset() != masterOffsetBeforeAttach {
		t.Fatalf("replica offset after full sync = %d, want master pre-attach offset %d",
			replica.Srv.ReplOffset(), masterOffsetBeforeAttach)
	}

	const postAttachWrites = 10
	for i := 0; i < postAttachWrites; i++ {
		key := fmt.Sprintf("repl:post:%03d", i)
		if got := masterClient.Do("SET", key, "after"); got != "+OK\r\n" {
			t.Fatalf("post-attach SET %s: got %q", key, got)
		}
	}

	waitForStoresEqual(t, master.Srv.DB(), replica.Srv.DB(), 5*time.Second)
	waitForReplOffsetsEqual(t, master, replica, 5*time.Second)
}

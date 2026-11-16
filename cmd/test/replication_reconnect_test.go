package main

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// Link loss is simulated by closing the upstream socket; reconnect is a second REPLICAOF (no auto-reconnect loop yet).
func TestReplicationManualReconnectPartialSync(t *testing.T) {
	master := startTCPLedis(t, true)
	replica := startTCPLedis(t, false)

	host, port, err := net.SplitHostPort(master.Addr)
	if err != nil {
		t.Fatalf("split master addr: %v", err)
	}

	replicaAdmin := dialRedis(t, replica.Addr)
	defer replicaAdmin.Close()
	if got := replicaAdmin.Do("REPLICAOF", host, port); got != "+OK\r\n" {
		t.Fatalf("REPLICAOF: got %q", got)
	}
	time.Sleep(300 * time.Millisecond)

	masterClient := dialRedis(t, master.Addr)
	defer masterClient.Close()

	const beforeKey = "psync:reconnect:before"
	if got := masterClient.Do("SET", beforeKey, "v1"); got != "+OK\r\n" {
		t.Fatalf("SET before: got %q", got)
	}
	waitForStoresEqual(t, master.Srv.DB(), replica.Srv.DB(), 5*time.Second)

	replica.Srv.CloseReplicationUpstreamLink()
	time.Sleep(300 * time.Millisecond)

	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("psync:reconnect:after:%02d", i)
		val := fmt.Sprintf("v%d", i)
		if got := masterClient.Do("SET", key, val); got != "+OK\r\n" {
			t.Fatalf("SET %s: got %q", key, got)
		}
	}

	if v, ok := replica.Srv.DB().Get("psync:reconnect:after:00"); ok {
		t.Fatalf("replica should be stale while link is down, got after:00=%q", v)
	}

	if got := replicaAdmin.Do("REPLICAOF", host, port); got != "+OK\r\n" {
		t.Fatalf("REPLICAOF reconnect: got %q", got)
	}
	time.Sleep(300 * time.Millisecond)

	waitForStoresEqual(t, master.Srv.DB(), replica.Srv.DB(), 10*time.Second)

	wantReplID := master.Srv.MasterReplID()
	info := replicaAdmin.Do("INFO", "replication")
	if !strings.Contains(info, "master_replid:"+wantReplID) {
		t.Fatalf("replica INFO missing master_replid %q:\n%s", wantReplID, info)
	}
	if !strings.Contains(info, "master_link_status:up") {
		t.Fatalf("replica link not up after reconnect:\n%s", info)
	}
}

func TestReplicationReconnectUsesPSYNCNotFullWhenBacklogCovers(t *testing.T) {
	master := startTCPLedis(t, true)
	replica := startTCPLedis(t, false)

	host, port, err := net.SplitHostPort(master.Addr)
	if err != nil {
		t.Fatalf("split master addr: %v", err)
	}

	replicaAdmin := dialRedis(t, replica.Addr)
	defer replicaAdmin.Close()
	masterClient := dialRedis(t, master.Addr)
	defer masterClient.Close()

	if got := replicaAdmin.Do("REPLICAOF", host, port); got != "+OK\r\n" {
		t.Fatalf("REPLICAOF: got %q", got)
	}
	time.Sleep(300 * time.Millisecond)

	if got := masterClient.Do("SET", "psync:probe", "1"); got != "+OK\r\n" {
		t.Fatalf("SET: got %q", got)
	}
	waitForStoresEqual(t, master.Srv.DB(), replica.Srv.DB(), 5*time.Second)

	offsetBefore := replica.Srv.ReplOffset()
	if offsetBefore <= 0 {
		t.Fatalf("expected positive repl offset after initial sync, got %d", offsetBefore)
	}

	replica.Srv.CloseReplicationUpstreamLink()
	time.Sleep(200 * time.Millisecond)

	if got := masterClient.Do("SET", "psync:probe", "2"); got != "+OK\r\n" {
		t.Fatalf("SET while down: got %q", got)
	}

	if got := replicaAdmin.Do("REPLICAOF", host, port); got != "+OK\r\n" {
		t.Fatalf("REPLICAOF reconnect: got %q", got)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		v, ok := replica.Srv.DB().Get("psync:probe")
		if ok && v == "2" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	v, ok := replica.Srv.DB().Get("psync:probe")
	if !ok || v != "2" {
		t.Fatalf("partial resync did not apply updates, psync:probe=%q ok=%v", v, ok)
	}

	offsetAfter := replica.Srv.ReplOffset()
	if offsetAfter <= offsetBefore {
		t.Fatalf("repl offset did not advance after partial catch-up: before=%d after=%d", offsetBefore, offsetAfter)
	}
	if offsetAfter != master.Srv.ReplOffset() {
		t.Fatalf("replica offset %d != master offset %d after catch-up", offsetAfter, master.Srv.ReplOffset())
	}
}
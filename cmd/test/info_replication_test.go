package main

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func parseInfoSection(body string) map[string]string {
	lines := strings.Split(strings.TrimSpace(body), "\n")
	out := make(map[string]string, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if ok {
			out[key] = val
		}
	}
	return out
}

func readInfoReplicationBody(t *testing.T, bulkReply string) string {
	t.Helper()
	if !strings.HasPrefix(bulkReply, "$") {
		t.Fatalf("expected bulk reply, got %q", bulkReply)
	}
	var n int
	if _, err := fmt.Sscanf(bulkReply, "$%d", &n); err != nil {
		t.Fatalf("parse bulk header: %v", err)
	}
	rest := bulkReply[strings.Index(bulkReply, "\r\n")+2:]
	if len(rest) < n+2 {
		t.Fatalf("truncated bulk body")
	}
	return rest[:n]
}

func TestInfoReplicationMaster(t *testing.T) {
	srv := newTestServer(t)
	srv.MakeMaster()

	sess := newTestSessionFromServer(srv)
	res, err := sess.Info(makeCommand("INFO", "replication"))
	if err != nil {
		t.Fatalf("Info: %v", err)
	}

	fields := parseInfoSection(readInfoReplicationBody(t, res))
	if fields["role"] != "master" {
		t.Fatalf("role = %q, want master", fields["role"])
	}
	if fields["master_replid"] == "" {
		t.Fatal("master_replid missing")
	}
	if fields["master_repl_offset"] != "0" {
		t.Fatalf("master_repl_offset = %q, want 0", fields["master_repl_offset"])
	}
	if fields["repl_backlog_first_byte_offset"] != "0" {
		t.Fatalf("repl_backlog_first_byte_offset = %q, want 0", fields["repl_backlog_first_byte_offset"])
	}
	if fields["connected_slaves"] != "0" {
		t.Fatalf("connected_slaves = %q, want 0", fields["connected_slaves"])
	}
}

func TestInfoReplicationReplicaTCP(t *testing.T) {
	master := startTCPLedis(t, true)
	replica := startTCPLedis(t, false)

	host, port, err := net.SplitHostPort(master.Addr)
	if err != nil {
		t.Fatalf("split master addr: %v", err)
	}

	admin := dialRedis(t, replica.Addr)
	defer admin.Close()
	if got := admin.Do("REPLICAOF", host, port); got != "+OK\r\n" {
		t.Fatalf("REPLICAOF: got %q", got)
	}

	time.Sleep(300 * time.Millisecond)

	masterClient := dialRedis(t, master.Addr)
	defer masterClient.Close()
	if got := masterClient.Do("SET", "info:key", "v"); got != "+OK\r\n" {
		t.Fatalf("SET: got %q", got)
	}
	waitForReplOffsetsEqual(t, master, replica, 5*time.Second)

	replicaInfo := dialRedis(t, replica.Addr)
	defer replicaInfo.Close()
	raw := replicaInfo.Do("INFO", "replication")
	if !strings.HasPrefix(raw, "$") {
		t.Fatalf("INFO replication: got %q", raw)
	}
	fields := parseInfoSection(readInfoReplicationBody(t, raw))
	if fields["role"] != "slave" {
		t.Fatalf("role = %q, want slave", fields["role"])
	}
	if fields["master_host"] != host || fields["master_port"] != port {
		t.Fatalf("master endpoint = %s:%s, want %s:%s",
			fields["master_host"], fields["master_port"], host, port)
	}
	if fields["master_link_status"] != "up" {
		t.Fatalf("master_link_status = %q, want up", fields["master_link_status"])
	}
	if fields["master_replid"] == "" {
		t.Fatal("master_replid missing on replica")
	}
	masterInfo := dialRedis(t, master.Addr)
	defer masterInfo.Close()
	mRaw := masterInfo.Do("INFO", "replication")
	mFields := parseInfoSection(readInfoReplicationBody(t, mRaw))
	if fields["slave_repl_offset"] != mFields["master_repl_offset"] {
		t.Fatalf("slave_repl_offset=%s master_repl_offset=%s",
			fields["slave_repl_offset"], mFields["master_repl_offset"])
	}
	if mFields["connected_slaves"] != "1" {
		t.Fatalf("connected_slaves = %q, want 1", mFields["connected_slaves"])
	}
}
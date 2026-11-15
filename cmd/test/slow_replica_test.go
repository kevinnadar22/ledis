package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kevinnadar22/ledis/internal/resp"
)

// Connects to master, completes replica handshake + full sync, then stops consuming
// the replication stream so the per-replica output queue can fill.
func attachStalledReplicaConn(t *testing.T, masterAddr string) net.Conn {
	t.Helper()

	conn, err := net.Dial("tcp", masterAddr)
	if err != nil {
		t.Fatalf("dial master: %v", err)
	}
	reader := bufio.NewReader(conn)

	write := func(args ...string) {
		if _, err := conn.Write([]byte(resp.EncodeArray(args))); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	readLine := func() string {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read line: %v", err)
		}
		return line
	}

	write("PING")
	if line := readLine(); line != "+PONG\r\n" && line != "+OK\r\n" {
		t.Fatalf("PING: got %q", line)
	}

	write("REPLCONF", "listening-port", "16379")
	if line := readLine(); line != "+OK\r\n" {
		t.Fatalf("REPLCONF listening-port: got %q", line)
	}

	write("REPLCONF", "capa", "psync2")
	if line := readLine(); line != "+OK\r\n" {
		t.Fatalf("REPLCONF capa: got %q", line)
	}

	write("PSYNC", "?", "-1")
	fullSync := readLine()
	if !strings.HasPrefix(fullSync, "+FULLRESYNC ") {
		t.Fatalf("PSYNC: got %q", fullSync)
	}

	lenLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("snapshot length line: %v", err)
	}
	length, err := strconv.Atoi(strings.TrimSpace(lenLine))
	if err != nil || length < 0 {
		t.Fatalf("snapshot length %q: %v", lenLine, err)
	}
	if _, err := io.CopyN(io.Discard, reader, int64(length)); err != nil {
		t.Fatalf("read snapshot: %v", err)
	}

	return conn
}

func TestSlowReplicaDroppedMasterDoesNotStall(t *testing.T) {
	master := startTCPLedis(t, true)

	replConn := attachStalledReplicaConn(t, master.Addr)
	t.Cleanup(func() { replConn.Close() })

	admin := dialRedis(t, master.Addr)
	fields := parseInfoSection(readInfoReplicationBody(t, admin.Do("INFO", "replication")))
	admin.Close()
	if fields["connected_slaves"] != "1" {
		t.Fatalf("connected_slaves = %q, want 1 before flood", fields["connected_slaves"])
	}

	// Large values fill the replica TCP window while the client does not read,
	// so the per-replica output queue backs up and hits the non-blocking send path.
	payload := strings.Repeat("x", 256*1024)
	const writes = 1200 // default replica queue is 1000
	done := make(chan error, 1)
	go func() {
		client := dialRedis(t, master.Addr)
		defer client.Close()
		for i := 0; i < writes; i++ {
			key := fmt.Sprintf("slow-repl:key:%04d", i)
			if got := client.Do("SET", key, payload); got != "+OK\r\n" {
				done <- fmt.Errorf("SET %s: got %q", key, got)
				return
			}
		}
		done <- nil
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("master writes stalled (timeout)")
	}

	admin = dialRedis(t, master.Addr)
	defer admin.Close()
	fields = parseInfoSection(readInfoReplicationBody(t, admin.Do("INFO", "replication")))
	if fields["connected_slaves"] != "0" {
		t.Fatalf("connected_slaves = %q, want 0 after slow replica dropped", fields["connected_slaves"])
	}
}
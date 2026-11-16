package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kevinnadar22/ledis/internal/resp"
)

// attachDrainingReplicaConn completes replica handshake and drains the replication TCP stream.
func attachDrainingReplicaConn(t *testing.T, masterAddr string) (net.Conn, <-chan error) {
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

	drainDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, conn)
		drainDone <- err
	}()
	return conn, drainDone
}

func waitGoroutinesSettle(t *testing.T, baseline, slack int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		runtime.Gosched()
		if runtime.NumGoroutine() <= baseline+slack {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("goroutines did not settle: baseline=%d slack=%d now=%d",
		baseline, slack, runtime.NumGoroutine())
}

func netClosed(err error) bool {
	if err == nil || errors.Is(err, io.EOF) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "closed") ||
		strings.Contains(s, "reset") ||
		strings.Contains(s, "forcibly closed")
}

// ReplicaWriterExitsAfterReplicaDisconnect closes the replica TCP side; the master's
// per-replica writer goroutine should return after Cancel / write error handling.
func TestReplicaWriterExitsAfterReplicaDisconnect(t *testing.T) {
	master := startTCPLedis(t, true)

	replConn, drainDone := attachDrainingReplicaConn(t, master.Addr)
	t.Cleanup(func() { replConn.Close() })

	waitForConnectedSlaves(t, master, 1)

	admin := dialRedis(t, master.Addr)
	if got := admin.Do("SET", "writer-exit", "1"); got != "+OK\r\n" {
		t.Fatalf("SET: got %q", got)
	}
	admin.Close()

	baseline := runtime.NumGoroutine()

	if err := replConn.Close(); err != nil {
		t.Fatalf("close replica conn: %v", err)
	}

	waitForConnectedSlaves(t, master, 0)
	waitGoroutinesSettle(t, baseline, 8)

	select {
	case err := <-drainDone:
		if !netClosed(err) {
			t.Fatalf("replica drain: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("replica drain goroutine did not exit")
	}
}

// ReplicaWriterExitsAfterSlowReplicaDrop ensures the writer stops when the master
// drops a stalled replica (Cancel + TCP close after unlock).
func TestReplicaWriterExitsAfterSlowReplicaDrop(t *testing.T) {
	master := startTCPLedis(t, true)

	replConn := attachStalledReplicaConn(t, master.Addr)
	drainDone := make(chan error, 1)
	go func() {
		var buf [1]byte
		_, err := replConn.Read(buf[:])
		drainDone <- err
	}()

	waitForConnectedSlaves(t, master, 1)

	payload := strings.Repeat("x", 64*1024)
	const writes = 1100
	admin := dialRedis(t, master.Addr)
	for i := 0; i < writes; i++ {
		key := fmt.Sprintf("writer-drop:%04d", i)
		if got := admin.Do("SET", key, payload); got != "+OK\r\n" {
			t.Fatalf("SET %s: got %q", key, got)
		}
	}
	admin.Close()

	waitForConnectedSlaves(t, master, 0)

	baseline := runtime.NumGoroutine()
	waitGoroutinesSettle(t, baseline, 8)

	select {
	case err := <-drainDone:
		if !netClosed(err) {
			t.Fatalf("replica read after master drop: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("replica read goroutine still blocked after master dropped replica")
	}
}

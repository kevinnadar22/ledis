package main

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
)

func waitForConnectedSlaves(t *testing.T, master *tcpLedis, want int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		admin := dialRedis(t, master.Addr)
		fields := parseInfoSection(readInfoReplicationBody(t, admin.Do("INFO", "replication")))
		admin.Close()
		if fields["connected_slaves"] == fmt.Sprint(want) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("connected_slaves did not reach %d in time", want)
}

// ConcurrentBroadcastBacklogMatchesReplica checks that under concurrent writes the
// master backlog and one replica's TCP byte stream stay identical and ordered.
//
//	go test -race ./cmd/test/... -run ConcurrentBroadcastBacklogMatchesReplica
func TestConcurrentBroadcastBacklogMatchesReplica(t *testing.T) {
	master := startTCPLedis(t, true)

	replConn := attachStalledReplicaConn(t, master.Addr)
	t.Cleanup(func() { replConn.Close() })

	waitForConnectedSlaves(t, master, 1)

	streamStart := master.Srv.ReplOffset()

	var recorded bytes.Buffer
	var recordWg sync.WaitGroup
	recordWg.Add(1)
	go func() {
		defer recordWg.Done()
		_, _ = io.Copy(&recorded, replConn)
	}()

	const workers = 16
	const perWorker = 50
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		w := w
		go func() {
			defer wg.Done()
			client := dialRedis(t, master.Addr)
			defer client.Close()
			for i := 0; i < perWorker; i++ {
				key := fmt.Sprintf("ord:%02d:%03d", w, i)
				val := fmt.Sprintf("v-%d-%d", w, i)
				if got := client.Do("SET", key, val); got != "+OK\r\n" {
					t.Errorf("SET %s: got %q", key, got)
					return
				}
			}
		}()
	}
	wg.Wait()

	// Let the replica writer goroutine flush queued commands to the TCP conn.
	time.Sleep(200 * time.Millisecond)
	replConn.Close()
	recordWg.Wait()

	received := bytes.Clone(recorded.Bytes())
	backlog, err := master.Srv.ReadReplicationBacklog(streamStart)
	if err != nil {
		t.Fatalf("ReadReplicationBacklog: %v", err)
	}

	if len(received) == 0 {
		t.Fatal("replica recorded zero replication bytes")
	}
	if !bytes.Equal(backlog, received) {
		n := len(backlog)
		if len(received) < n {
			n = len(received)
		}
		for i := 0; i < n; i++ {
			if backlog[i] != received[i] {
				t.Fatalf("first mismatch at byte %d: backlog %#x replica %#x (backlog=%d replica=%d)",
					i, backlog[i], received[i], len(backlog), len(received))
			}
		}
		t.Fatalf("backlog and replica stream differ: backlog=%d bytes, replica=%d bytes",
			len(backlog), len(received))
	}
}
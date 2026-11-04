package main

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Stress replication offset accounting under the race detector:
//
//	go test -race ./cmd/test/... -run TestReplicationOffsetRace
func TestReplicationOffsetRace(t *testing.T) {
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

	var stop atomic.Bool
	var readers sync.WaitGroup
	readers.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer readers.Done()
			for !stop.Load() {
				_ = master.Srv.ReplOffset()
				_ = replica.Srv.ReplOffset()
			}
		}()
	}

	client := dialRedis(t, master.Addr)
	defer client.Close()

	const writes = 100
	for i := 0; i < writes; i++ {
		key := fmt.Sprintf("repl:offset:%03d", i)
		if got := client.Do("SET", key, fmt.Sprintf("v%d", i)); got != "+OK\r\n" {
			t.Fatalf("SET %s: got %q", key, got)
		}
	}

	stop.Store(true)
	readers.Wait()

	waitForReplOffsetsEqual(t, master, replica, 5*time.Second)
	waitForStoresEqual(t, master.Srv.DB(), replica.Srv.DB(), 5*time.Second)

	mOff := master.Srv.ReplOffset()
	rOff := replica.Srv.ReplOffset()
	if mOff != rOff {
		t.Fatalf("repl offset mismatch after %d writes: master=%d replica=%d", writes, mOff, rOff)
	}
	if mOff <= 0 {
		t.Fatalf("repl offset = %d after %d writes, want > 0", mOff, writes)
	}
}

func waitForReplOffsetsEqual(t *testing.T, master, replica *tcpLedis, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if master.Srv.ReplOffset() == replica.Srv.ReplOffset() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	mOff := master.Srv.ReplOffset()
	rOff := replica.Srv.ReplOffset()
	t.Fatalf("repl offsets not equal within %s: master=%d replica=%d", timeout, mOff, rOff)
}

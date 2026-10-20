package main

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/persistence"
	"github.com/kevinnadar22/ledis/internal/replication"
	"github.com/kevinnadar22/ledis/internal/resp"
	"github.com/kevinnadar22/ledis/internal/store"
)

func replicaOfCommand(host, port string) datatypes.Command {
	return datatypes.Command{
		Cmd:  makeValue("REPLICAOF"),
		Args: []datatypes.Value{makeValue(host), makeValue(port)},
	}
}

func runFakeMaster(t *testing.T, replicaListenPort int, entries []datatypes.RDBEntry, steps *atomic.Int32) (host, port string, cleanup func()) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	host, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)
		readCmd := func() (datatypes.Command, error) {
			cmdLine, err := resp.DecodeBulkStringsArrayFromReader(reader)
			if err != nil {
				return datatypes.Command{}, err
			}
			return resp.Decode(cmdLine)
		}

		ping, err := readCmd()
		if err != nil || ping.Cmd.String() != "PING" {
			t.Errorf("ping: err=%v cmd=%q", err, ping.Cmd.String())
			return
		}
		steps.Add(1)
		if _, err := conn.Write([]byte(resp.EncodeSimpleString("PONG"))); err != nil {
			t.Errorf("handshake write: %v", err)
			return
		}

		replPort, err := readCmd()
		if err != nil || replPort.Cmd.String() != "REPLCONF" || len(replPort.Args) < 2 ||
			replPort.Args[0].String() != "listening-port" || replPort.Args[1].String() != strconv.Itoa(replicaListenPort) {
			t.Errorf("replconf listening-port: err=%v cmd=%+v", err, replPort)
			return
		}
		steps.Add(1)
		if _, err := conn.Write([]byte(resp.EncodeSimpleString("OK"))); err != nil {
			t.Errorf("handshake write: %v", err)
			return
		}

		capa, err := readCmd()
		if err != nil || capa.Cmd.String() != "REPLCONF" || len(capa.Args) < 2 ||
			capa.Args[0].String() != "capa" || capa.Args[1].String() != "psync2" {
			t.Errorf("replconf capa: err=%v cmd=%+v", err, capa)
			return
		}
		steps.Add(1)
		if _, err := conn.Write([]byte(resp.EncodeSimpleString("OK"))); err != nil {
			t.Errorf("handshake write: %v", err)
			return
		}

		psyncCmd, err := readCmd()
		if err != nil || psyncCmd.Cmd.String() != "PSYNC" || len(psyncCmd.Args) != 2 ||
			psyncCmd.Args[0].String() != "?" || psyncCmd.Args[1].String() != "-1" {
			t.Errorf("psync: err=%v cmd=%+v", err, psyncCmd)
			return
		}
		steps.Add(1)

		if _, err := conn.Write([]byte(resp.EncodeSimpleString("FULLRESYNC test-repl-id 0"))); err != nil {
			t.Errorf("write FULLRESYNC: %v", err)
			return
		}
		buf, err := persistence.EncodeSnapshot(entries, nil)
		if err != nil {
			t.Errorf("encode snapshot: %v", err)
			return
		}
		if _, err := fmt.Fprintf(conn, "%d\r\n", buf.Len()); err != nil {
			t.Errorf("write snapshot length: %v", err)
			return
		}
		if _, err := conn.Write(buf.Bytes()); err != nil {
			t.Errorf("write snapshot: %v", err)
			return
		}
	}()

	return host, portStr, func() {
		_ = ln.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("timed out waiting for fake master")
		}
	}
}

func TestReplicationHandshake(t *testing.T) {
	const listenPort = 6379
	var steps atomic.Int32

	host, port, cleanup := runFakeMaster(t, listenPort, nil, &steps)
	defer cleanup()

	replica := replication.New(listenPort, store.NewStore())
	if _, err := replica.ReplicaOf(replicaOfCommand(host, port)); err != nil {
		t.Fatalf("ReplicaOf: %v", err)
	}
	t.Cleanup(func() { replica.Stop() })

	deadline := time.Now().Add(3 * time.Second)
	for steps.Load() < 4 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if steps.Load() != 4 {
		t.Fatalf("handshake steps = %d, want 4", steps.Load())
	}
}

func TestReplicationFullSync(t *testing.T) {
	const listenPort = 6380
	var steps atomic.Int32

	masterDB := store.NewStore()
	masterDB.Set("foo", "bar")
	entries, err := masterDB.SaveSnapshot()
	if err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	host, port, cleanup := runFakeMaster(t, listenPort, entries, &steps)
	defer cleanup()

	replicaDB := store.NewStore()
	replica := replication.New(listenPort, replicaDB)
	if _, err := replica.ReplicaOf(replicaOfCommand(host, port)); err != nil {
		t.Fatalf("ReplicaOf: %v", err)
	}
	t.Cleanup(func() { replica.Stop() })

	deadline := time.Now().Add(3 * time.Second)
	for {
		val, ok := replicaDB.Get("foo")
		if ok && val == "bar" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for full sync to restore foo=bar")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

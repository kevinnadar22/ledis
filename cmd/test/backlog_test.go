package main

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/replication"
	"github.com/kevinnadar22/ledis/internal/store"
)

func newReplBacklogMaster(t *testing.T, capacity int64) *replication.Coordinator {
	t.Helper()
	c := replication.New(0, store.NewStore(), nil, capacity)
	c.MakeMaster()
	return c
}

func recordReplBytes(c *replication.Coordinator, s string) {
	c.HandleBacklog(datatypes.Command{RawContent: s})
}

func TestBacklogReadInsideWindow(t *testing.T) {
	c := newReplBacklogMaster(t, 8)
	recordReplBytes(c, "abcd")

	got, err := c.ReadFromBacklog(0)
	if err != nil {
		t.Fatalf("ReadFromBacklog: %v", err)
	}
	if string(got) != "abcd" {
		t.Fatalf("got %q, want abcd", got)
	}
	if c.ReplOffset() != 4 {
		t.Fatalf("repl_offset = %d, want 4", c.ReplOffset())
	}
}

func TestBacklogRefuseEvictedOffset(t *testing.T) {
	c := newReplBacklogMaster(t, 4)
	recordReplBytes(c, "abcde")

	if c.CanServeFromBacklog(0) {
		t.Fatal("offset 0 should be evicted after wrap")
	}
	_, err := c.ReadFromBacklog(0)
	if err == nil {
		t.Fatal("expected error reading evicted offset")
	}

	got, err := c.ReadFromBacklog(1)
	if err != nil {
		t.Fatalf("ReadFromBacklog(1): %v", err)
	}
	if string(got) != "bcde" {
		t.Fatalf("got %q, want bcde", got)
	}
}

func TestBacklogWrapReadBack(t *testing.T) {
	c := newReplBacklogMaster(t, 4)

	const rounds = 20
	var want strings.Builder
	for i := 0; i < rounds; i++ {
		ch := byte('a' + (i % 26))
		want.WriteByte(ch)
		recordReplBytes(c, string([]byte{ch}))
	}

	oldest := c.OldestReplOffset()
	end := c.ReplOffset()
	if end != int64(rounds) {
		t.Fatalf("repl_offset = %d, want %d", end, rounds)
	}
	if oldest != end-4 {
		t.Fatalf("oldestReplOffset = %d, want %d", oldest, end-4)
	}

	got, err := c.ReadFromBacklog(oldest)
	if err != nil {
		t.Fatalf("ReadFromBacklog(%d): %v", oldest, err)
	}
	wantTail := want.String()[want.Len()-4:]
	if string(got) != wantTail {
		t.Fatalf("got %q, want %q", got, wantTail)
	}
}

// Run: go test -race ./cmd/test/... -run TestBacklogRace
func TestBacklogRace(t *testing.T) {
	c := newReplBacklogMaster(t, 32)

	const readers = 4
	const chunks = 50

	var wg sync.WaitGroup
	wg.Add(readers + 1)

	stop := make(chan struct{})
	var stopOnce sync.Once
	done := func() { stopOnce.Do(func() { close(stop) }) }

	go func() {
		defer wg.Done()
		for i := 0; i < chunks; i++ {
			recordReplBytes(c, "xy")
		}
		done()
	}()

	for w := 0; w < readers; w++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					off := c.ReplOffset()
					if off <= 0 {
						continue
					}
					start := off - 1
					if c.CanServeFromBacklog(start) {
						_, _ = c.ReadFromBacklog(start)
					}
				}
			}
		}()
	}

	wg.Wait()
	if c.ReplOffset() != int64(2*chunks) {
		t.Fatalf("repl_offset = %d, want %d", c.ReplOffset(), 2*chunks)
	}
}

func TestBacklogInfoFieldsTrackState(t *testing.T) {
	c := newReplBacklogMaster(t, 4)
	recordReplBytes(c, "ab")

	info := c.InfoReplication()
	if !strings.Contains(info, "repl_backlog_active:1") {
		t.Fatalf("info missing repl_backlog_active:1:\n%s", info)
	}
	if !strings.Contains(info, "repl_backlog_size:4") {
		t.Fatalf("info missing repl_backlog_size:4:\n%s", info)
	}
	if !strings.Contains(info, "repl_backlog_first_byte_offset:0") {
		t.Fatalf("info missing first_byte_offset 0:\n%s", info)
	}
	if !strings.Contains(info, "repl_backlog_histlen:2") {
		t.Fatalf("info missing histlen 2:\n%s", info)
	}

	recordReplBytes(c, "cdef")
	if !bytes.Contains([]byte(c.InfoReplication()), []byte("repl_backlog_histlen:4")) {
		t.Fatalf("histlen should be 4 after fill+wrap, info:\n%s", c.InfoReplication())
	}
}
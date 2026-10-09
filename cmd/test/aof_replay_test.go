package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kevinnadar22/ledis/internal/commands"
	"github.com/kevinnadar22/ledis/internal/persistence"
	"github.com/kevinnadar22/ledis/internal/resp"
	"github.com/kevinnadar22/ledis/internal/store"
)

func newServerForAOFReplay(t *testing.T, aofPath string) (*commands.Server, *persistence.AOF) {
	t.Helper()
	aof, err := persistence.NewAOF(aofPath, persistence.FsyncNo)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = aof.Close() })
	db := store.NewStore()
	srv := commands.NewServer(db, aof, newTestConfig(""), newTestRDB(), store.NewPubSub())
	return srv, aof
}

func TestAOFReplayAppliesSet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appendonly.aof")
	srv, aof := newServerForAOFReplay(t, path)

	line := "*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"
	if err := aof.Append([]byte(line)); err != nil {
		t.Fatal(err)
	}
	if err := aof.Replay(srv.ReplayHandler()); err != nil {
		t.Fatal(err)
	}
	v, ok := srv.DB().Get("foo")
	if !ok || v != "bar" {
		t.Fatalf("GET foo = %q, ok=%v", v, ok)
	}
}

func TestAOFReplayTruncatesPartialTail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appendonly.aof")

	good := "*3\r\n$3\r\nSET\r\n$1\r\na\r\n$1\r\n1\r\n"
	partial := "*3\r\n$3\r\nSET\r\n$1\r\nb\r\n$2\r\n"
	if err := os.WriteFile(path, []byte(good+partial), 0644); err != nil {
		t.Fatal(err)
	}

	srv, aof := newServerForAOFReplay(t, path)
	if err := aof.Replay(srv.ReplayHandler()); err != nil {
		t.Fatal(err)
	}
	if _, ok := srv.DB().Get("b"); ok {
		t.Fatal("partial SET b must not be applied")
	}
	v, ok := srv.DB().Get("a")
	if !ok || v != "1" {
		t.Fatalf("GET a = %q, ok=%v", v, ok)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != int64(len(good)) {
		t.Fatalf("AOF size = %d, want %d after truncate", info.Size(), len(good))
	}
}

func TestAOFReplayMultiWithoutExecTruncates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appendonly.aof")

	good := "*3\r\n$3\r\nSET\r\n$1\r\nx\r\n$1\r\n1\r\n"
	inTrx := "*3\r\n$3\r\nSET\r\n$1\r\ny\r\n$1\r\n2\r\n"
	body := good + resp.Encode("MULTI") + inTrx
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	srv, aof := newServerForAOFReplay(t, path)
	if err := aof.Replay(srv.ReplayHandler()); err != nil {
		t.Fatal(err)
	}
	if _, ok := srv.DB().Get("y"); ok {
		t.Fatal("SET y inside unfinished MULTI must not apply")
	}
	if v, ok := srv.DB().Get("x"); !ok || v != "1" {
		t.Fatalf("GET x = %q, ok=%v", v, ok)
	}

	info, _ := os.Stat(path)
	if info.Size() != int64(len(good)) {
		t.Fatalf("AOF size = %d, want %d", info.Size(), len(good))
	}
}

func TestAOFReplayMultiExecBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appendonly.aof")

	block := resp.Encode("MULTI") +
		"*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$1\r\nv\r\n" +
		resp.Encode("EXEC")
	if err := os.WriteFile(path, []byte(block), 0644); err != nil {
		t.Fatal(err)
	}

	srv, aof := newServerForAOFReplay(t, path)
	if err := aof.Replay(srv.ReplayHandler()); err != nil {
		t.Fatal(err)
	}
	val, ok := srv.DB().Get("k")
	if !ok || val != "v" {
		t.Fatalf("GET k = %q, ok=%v", val, ok)
	}
}

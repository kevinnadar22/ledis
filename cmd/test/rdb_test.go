package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kevinnadar22/ledis/internal/persistence"
)

func TestSaveCommand(t *testing.T) {
	srv, rdbPath := newTestServerWithRDB(t)
	sess := newTestSessionFromServer(srv)
	srv.DB().Set("foo", "bar")
	srv.DB().Set("hello", "world")

	res, err := sess.Save(makeCommand("SAVE"))
	if err != nil {
		t.Fatalf("Save() unexpected error: %v", err)
	}
	if res != "+OK\r\n" {
		t.Fatalf("Save() result = %q, want %q", res, "+OK\r\n")
	}

	if _, err := os.Stat(rdbPath); err != nil {
		t.Fatalf("expected RDB file at %s: %v", rdbPath, err)
	}

	rdb := persistence.NewRDB()
	entries, err := rdb.Load(rdbPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	got := map[string]string{}
	for _, e := range entries {
		got[e.Key] = e.Value
	}
	if got["foo"] != "bar" || got["hello"] != "world" {
		t.Errorf("loaded entries = %#v, want foo=bar and hello=world", got)
	}
}

func TestSaveCommandEmptyStore(t *testing.T) {
	srv, rdbPath := newTestServerWithRDB(t)
	sess := newTestSessionFromServer(srv)

	res, err := sess.Save(makeCommand("SAVE"))
	if err != nil {
		t.Fatalf("Save() unexpected error: %v", err)
	}
	if res != "+OK\r\n" {
		t.Fatalf("Save() result = %q, want %q", res, "+OK\r\n")
	}

	rdb := persistence.NewRDB()
	entries, err := rdb.Load(rdbPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestSaveCommandRoundTrip(t *testing.T) {
	srv, rdbPath := newTestServerWithRDB(t)
	sess := newTestSessionFromServer(srv)
	srv.DB().Set("a", "1")
	srv.DB().Set("b", "2")

	if _, err := sess.Save(makeCommand("SAVE")); err != nil {
		t.Fatalf("Save() unexpected error: %v", err)
	}

	rdb := persistence.NewRDB()
	entries, err := rdb.Load(rdbPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	restored := newTestServer(t)
	restored.DB().RestoreSnapshot(entries)

	if val, ok := restored.DB().Get("a"); !ok || val != "1" {
		t.Errorf("expected a=1, got %q (ok=%v)", val, ok)
	}
	if val, ok := restored.DB().Get("b"); !ok || val != "2" {
		t.Errorf("expected b=2, got %q (ok=%v)", val, ok)
	}
}

func TestSaveCommandWithExpiration(t *testing.T) {
	srv, rdbPath := newTestServerWithRDB(t)
	sess := newTestSessionFromServer(srv)
	srv.DB().Set("temp", "value")
	srv.DB().Expire("temp", 60)
	srv.DB().Set("perm", "forever")

	if _, err := sess.Save(makeCommand("SAVE")); err != nil {
		t.Fatalf("Save() unexpected error: %v", err)
	}

	rdb := persistence.NewRDB()
	entries, err := rdb.Load(rdbPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	foundTemp, foundPerm := false, false
	for _, e := range entries {
		switch e.Key {
		case "temp":
			foundTemp = true
			if e.Value != "value" {
				t.Errorf("temp value = %q, want value", e.Value)
			}
			if e.Expiration == nil {
				t.Error("expected temp to have expiration")
			}
		case "perm":
			foundPerm = true
			if e.Value != "forever" {
				t.Errorf("perm value = %q, want forever", e.Value)
			}
			if e.Expiration != nil {
				t.Error("expected perm to have no expiration")
			}
		}
	}
	if !foundTemp || !foundPerm {
		t.Errorf("missing keys in snapshot: temp=%v perm=%v", foundTemp, foundPerm)
	}
}

func TestBGSaveCommand(t *testing.T) {
	srv, rdbPath := newTestServerWithRDB(t)
	sess := newTestSessionFromServer(srv)
	srv.DB().Set("bg", "saved")

	res, err := sess.BGSave(makeCommand("BGSAVE"))
	if err != nil {
		t.Fatalf("BGSave() unexpected error: %v", err)
	}
	wantBGSave := "+background saving started\r\n"
	if res != wantBGSave {
		t.Fatalf("BGSave() result = %q, want %q", res, wantBGSave)
	}

	waitForRDBFile(t, rdbPath)

	rdb := persistence.NewRDB()
	entries, err := rdb.Load(rdbPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.Key == "bg" && e.Value == "saved" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected bg=saved in RDB, got %#v", entries)
	}
}

func TestBGSaveCommandEmptyStore(t *testing.T) {
	srv, rdbPath := newTestServerWithRDB(t)
	sess := newTestSessionFromServer(srv)

	res, err := sess.BGSave(makeCommand("BGSAVE"))
	if err != nil {
		t.Fatalf("BGSave() unexpected error: %v", err)
	}
	wantBGSave := "+background saving started\r\n"
	if res != wantBGSave {
		t.Fatalf("BGSave() result = %q, want %q", res, wantBGSave)
	}

	waitForRDBFile(t, rdbPath)

	rdb := persistence.NewRDB()
	entries, err := rdb.Load(rdbPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestRDBCreateAndLoadHeaderOnly(t *testing.T) {
	tempDir := t.TempDir()
	rdbPath := filepath.Join(tempDir, "empty.rdb")
	rdb := persistence.NewRDB()

	if err := rdb.Create(rdbPath); err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}

	entries, err := rdb.Load(rdbPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries from header-only RDB, got %d", len(entries))
	}
}

func waitForRDBFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for RDB file at %s", path)
}

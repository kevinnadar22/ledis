package main

import (
	"path/filepath"
	"testing"

	"github.com/kevinnadar22/ledis/internal/commands"
	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/persistence"
	"github.com/kevinnadar22/ledis/internal/store"
)

func makeValue(s string) datatypes.Value {
	return datatypes.Value{
		Type: datatypes.BulkString,
		Str:  &s,
	}
}

func makeCommand(cmdName string, args ...string) datatypes.Command {
	var valArgs []datatypes.Value
	for _, arg := range args {
		valArgs = append(valArgs, makeValue(arg))
	}
	return datatypes.Command{
		Cmd:  makeValue(cmdName),
		Args: valArgs,
	}
}

func newTestServer(t *testing.T) *commands.Server {
	tempDir := t.TempDir()
	tempAOFPath := filepath.Join(tempDir, "test_appendonly.aof")
	aof, err := persistence.NewAOF(tempAOFPath, persistence.FsyncNo)
	if err != nil {
		t.Fatalf("failed to create test AOF: %v", err)
	}
	t.Cleanup(func() {
		aof.Close()
	})
	db := store.NewStore()
	return commands.NewServer(db, aof)
}

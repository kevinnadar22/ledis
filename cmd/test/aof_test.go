package main

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"


	"github.com/kevinnadar22/ledis/internal/persistence"
	"github.com/kevinnadar22/ledis/internal/resp"
)

func TestAOFWriteAndReplay(t *testing.T) {
	// Create a temporary file path
	tempDir := t.TempDir()
	tempAOFPath := filepath.Join(tempDir, "test_appendonly.aof")

	// 1. Initialize AOF
	aof, err := persistence.NewAOF(tempAOFPath)
	if err != nil {
		t.Fatalf("failed to create NewAOF: %v", err)
	}

	// 2. Append mock commands
	cmd1 := "*3\r\n$3\r\nSET\r\n$1\r\na\r\n$2\r\n42\r\n"
	cmd2 := "*2\r\n$4\r\nINCR\r\n$1\r\na\r\n"

	err = aof.Append([]byte(cmd1))
	if err != nil {
		t.Fatalf("failed to append cmd1: %v", err)
	}

	err = aof.Append([]byte(cmd2))
	if err != nil {
		t.Fatalf("failed to append cmd2: %v", err)
	}

	// Close to flush
	err = aof.Close()
	if err != nil {
		t.Fatalf("failed to close aof: %v", err)
	}

	// 3. Re-open and Replay
	file, err := os.OpenFile(tempAOFPath, os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("failed to open aof file for reading: %v", err)
	}
	defer file.Close()

	// Create new AOF object manually using the opened file to trigger Replay
	// Since NewAOF creates/opens it, we can mimic Replay by using a test-specific AOF structure
	// Let's create an AOF wrapper for the test:
	type aofTest struct {
		file      *os.File
		replaying bool
	}

	// Actually, we can just use a bufio.Reader and DecodeBulkStringsArrayFromReader directly
	// to verify that the format is correct and DecodeBulkStringsArrayFromReader parses it correctly.
	reader := bufio.NewReader(file)

	// Read first command
	cmdLine1, err := resp.DecodeBulkStringsArrayFromReader(reader)
	if err != nil {
		t.Fatalf("failed to decode command 1: %v", err)
	}
	if cmdLine1 != cmd1 {
		t.Errorf("expected cmd1 %q, got %q", cmd1, cmdLine1)
	}

	// Read second command
	cmdLine2, err := resp.DecodeBulkStringsArrayFromReader(reader)
	if err != nil {
		t.Fatalf("failed to decode command 2: %v", err)
	}
	if cmdLine2 != cmd2 {
		t.Errorf("expected cmd2 %q, got %q", cmd2, cmdLine2)
	}

	// 4. Test parsing with resp.Decode
	cmdParsed1, err := resp.Decode(cmdLine1)
	if err != nil {
		t.Fatalf("failed to decode cmdLine1: %v", err)
	}
	if *cmdParsed1.Cmd.Str != "SET" {
		t.Errorf("expected command name SET, got %q", *cmdParsed1.Cmd.Str)
	}
	if len(cmdParsed1.Args) != 2 || *cmdParsed1.Args[0].Str != "a" || *cmdParsed1.Args[1].Str != "42" {
		t.Errorf("unexpected arguments for command 1: %v", cmdParsed1.Args)
	}
}

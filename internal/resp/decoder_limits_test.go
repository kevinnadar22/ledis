package resp

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"bufio"
)

func setBulkLimitForTest(t *testing.T, n int) {
	t.Helper()
	prev := maxBulkStringSize
	SetMaxBulkStringSize(n)
	t.Cleanup(func() { maxBulkStringSize = prev })
}

func buildSetCommand(value string) string {
	return fmt.Sprintf("*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$%d\r\n%s\r\n", len(value), value)
}

func TestDecode_RejectsBulkStringOverLimit(t *testing.T) {
	setBulkLimitForTest(t, 4)
	_, err := Decode(buildSetCommand("hello"))
	if err == nil || !strings.Contains(err.Error(), "bulk string too large") {
		t.Fatalf("Decode() err = %v, want bulk string too large", err)
	}
}

func TestDecode_AllowsBulkStringWithinLimit(t *testing.T) {
	setBulkLimitForTest(t, 4)
	cmd, err := Decode(buildSetCommand("hi"))
	if err != nil {
		t.Fatalf("Decode(): %v", err)
	}
	if cmd.Cmd.Str == nil || *cmd.Cmd.Str != "SET" {
		t.Fatalf("cmd = %+v", cmd)
	}
	if len(cmd.Args) != 2 || cmd.Args[1].Str == nil || *cmd.Args[1].Str != "hi" {
		t.Fatalf("args = %+v", cmd.Args)
	}
}

func TestDecodeBulkStringsArrayFromReader_RejectsOversizedBulk(t *testing.T) {
	setBulkLimitForTest(t, 2)
	raw := buildSetCommand("abc")
	reader := bufio.NewReader(bytes.NewReader([]byte(raw)))
	_, err := DecodeBulkStringsArrayFromReader(reader)
	if err == nil || !strings.Contains(err.Error(), "bulk string too large") {
		t.Fatalf("DecodeBulkStringsArrayFromReader() err = %v, want bulk string too large", err)
	}
}

func TestSetMaxBulkStringSize_FromConfigValue(t *testing.T) {
	// Mirrors cmd/server/main.go wiring after config.Load().
	const fromConfig = 128
	setBulkLimitForTest(t, fromConfig)
	cmd, err := Decode(buildSetCommand(strings.Repeat("x", fromConfig)))
	if err != nil {
		t.Fatalf("Decode at limit: %v", err)
	}
	if cmd.Args[1].Str == nil || len(*cmd.Args[1].Str) != fromConfig {
		t.Fatalf("value length = %d, want %d", len(*cmd.Args[1].Str), fromConfig)
	}
	_, err = Decode(buildSetCommand(strings.Repeat("y", fromConfig+1)))
	if err == nil {
		t.Fatal("expected error over limit")
	}
}

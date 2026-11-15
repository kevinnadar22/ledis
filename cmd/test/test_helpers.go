package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/kevinnadar22/ledis/internal/commands"
	"github.com/kevinnadar22/ledis/internal/config"
	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/persistence"
	"github.com/kevinnadar22/ledis/internal/resp"
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
	srv, _ := newTestServerWithRDB(t)
	return srv
}

func newTestServerWithRDB(t *testing.T) (*commands.Server, string) {
	t.Helper()
	tempDir := t.TempDir()
	tempAOFPath := filepath.Join(tempDir, "test_appendonly.aof")
	tempRDBPath := filepath.Join(tempDir, "dump.rdb")
	aof, err := persistence.NewAOF(tempAOFPath, persistence.FsyncNo)
	if err != nil {
		t.Fatalf("failed to create test AOF: %v", err)
	}
	t.Cleanup(func() {
		aof.Close()
	})
	db := store.NewStore()
	cfg := newTestConfig(tempRDBPath)
	return commands.NewServer(db, aof, cfg, newTestRDB(), store.NewPubSub()), tempRDBPath
}

func newTestSession(t *testing.T) *commands.Session {
	t.Helper()
	return newTestSessionFromServer(newTestServer(t))
}

func newTestSessionFromServer(srv *commands.Server) *commands.Session {
	return commands.NewSession(nil, srv)
}

func newTestConfig(rdbPath string) *config.Config {
	return &config.Config{
		AppendOnly:        true,
		FsyncPolicy:       persistence.FsyncNo,
		RDBFile:           rdbPath,
		MaxCommandSize:    1024 * 1024 * 1024,
		MaxBulkStringSize: 512 * 1024 * 1024,
		ReplBacklogSize:   1024 * 1024,
	}
}

func newTestRDB() *persistence.RDB {
	return persistence.NewRDB()
}

type tcpLedis struct {
	Addr string
	Srv  *commands.Server
}

func handleTestConnection(conn net.Conn, srv *commands.Server, cfg *config.Config) {
	sess := commands.NewSession(conn, srv)
	defer sess.Close()
	defer conn.Close()

	reader := bufio.NewReader(conn)
	for {
		cmdLine, err := resp.DecodeBulkStringsArrayFromReader(reader)
		if err == io.EOF {
			return
		}
		if err != nil {
			return
		}
		if cfg.MaxCommandSize > 0 && len(cmdLine) > cfg.MaxCommandSize {
			conn.Write([]byte(resp.EncodeError(fmt.Sprintf("Command too large: %d > %d", len(cmdLine), cfg.MaxCommandSize))))
			continue
		}
		cmd, err := resp.Decode(cmdLine)
		if err != nil {
			conn.Write([]byte(resp.EncodeError(fmt.Sprintf("Error decoding command: %v", err))))
			continue
		}
		conn.Write([]byte(sess.Execute(cmd)))
	}
}

func startTCPLedis(t *testing.T, asMaster bool) *tcpLedis {
	t.Helper()
	return startTCPLedisWithConfig(t, asMaster, nil)
}

func startTCPLedisWithConfig(t *testing.T, asMaster bool, cfg *config.Config) *tcpLedis {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	tempDir := t.TempDir()
	aofPath := filepath.Join(tempDir, "appendonly.aof")
	aof, err := persistence.NewAOF(aofPath, persistence.FsyncNo)
	if err != nil {
		t.Fatalf("aof: %v", err)
	}

	if cfg == nil {
		cfg = newTestConfig("")
	}
	cfg.Port = port
	db := store.NewStore()
	srv := commands.NewServer(db, aof, cfg, newTestRDB(), store.NewPubSub())
	if asMaster {
		srv.MakeMaster()
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleTestConnection(conn, srv, cfg)
		}
	}()

	t.Cleanup(func() {
		_ = ln.Close()
		aof.Close()
	})

	return &tcpLedis{Addr: ln.Addr().String(), Srv: srv}
}

type redisConn struct {
	conn   net.Conn
	reader *bufio.Reader
	t      *testing.T
}

func dialRedis(t *testing.T, addr string) *redisConn {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	return &redisConn{conn: conn, reader: bufio.NewReader(conn), t: t}
}

func (c *redisConn) Close() {
	c.conn.Close()
}

func (c *redisConn) Do(args ...string) string {
	c.t.Helper()
	if _, err := c.conn.Write([]byte(resp.EncodeArray(args))); err != nil {
		c.t.Fatalf("write: %v", err)
	}
	line, err := c.reader.ReadString('\n')
	if err != nil {
		c.t.Fatalf("read: %v", err)
	}
	switch line[0] {
	case '+', '-', ':':
		return line
	case '$':
		if line == "$-1\r\n" {
			return line
		}
		var n int
		if _, err := fmt.Sscanf(line, "$%d", &n); err != nil {
			c.t.Fatalf("parse bulk header %q: %v", line, err)
		}
		buf := make([]byte, n+2)
		if _, err := io.ReadFull(c.reader, buf); err != nil {
			c.t.Fatalf("read bulk: %v", err)
		}
		return line + string(buf)
	case '*':
		var count int
		if _, err := fmt.Sscanf(line, "*%d", &count); err != nil {
			c.t.Fatalf("parse array header %q: %v", line, err)
		}
		out := line
		for i := 0; i < count; i++ {
			elem, err := c.reader.ReadString('\n')
			if err != nil {
				c.t.Fatalf("read array elem: %v", err)
			}
			out += elem
			if len(elem) > 0 && elem[0] == '$' && elem != "$-1\r\n" {
				var bn int
				if _, err := fmt.Sscanf(elem, "$%d", &bn); err != nil {
					c.t.Fatalf("parse nested bulk: %v", err)
				}
				payload := make([]byte, bn+2)
				if _, err := io.ReadFull(c.reader, payload); err != nil {
					c.t.Fatalf("read nested bulk: %v", err)
				}
				out += string(payload)
			}
		}
		return out
	default:
		c.t.Fatalf("unexpected reply type: %q", line)
		return ""
	}
}

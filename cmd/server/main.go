package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/kevinnadar22/ledis/internal/commands"
	"github.com/kevinnadar22/ledis/internal/config"
	"github.com/kevinnadar22/ledis/internal/persistence"
	"github.com/kevinnadar22/ledis/internal/resp"
	"github.com/kevinnadar22/ledis/internal/store"
)

func main() {
	listener, err := net.Listen("tcp", ":7379")

	if err != nil {
		log.Fatal(err)
		return
	}

	defer listener.Close()

	// create in memory database
	db := store.NewStore()

	// load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Error loading config:", err)
		return
	}
	resp.SetMaxBulkStringSize(cfg.MaxBulkStringSize)

	// create AOF if append only is enabled
	var aof *persistence.AOF

	if cfg.AppendOnly {
		aof, err = persistence.NewAOF("./appendonly.aof", cfg.FsyncPolicy)
		if err != nil {
			log.Fatal("Error creating AOF:", err)
			return
		}
		defer aof.Close()
	}

	// create rdb
	rdb := persistence.NewRDB()

	// create pubsub
	pubsub := store.NewPubSub()

	// create server with db and aof
	srv := commands.NewServer(db, aof, cfg, rdb, pubsub)

	// if rdb file exists, load it
	if cfg.RDBFile != "" {
		err := rdb.Create(cfg.RDBFile)
		if err != nil {
			log.Fatal("Error creating RDB file:", err)
			return
		}

		fmt.Println("Loading RDB from file:", cfg.RDBFile)
		entries, err := rdb.Load(cfg.RDBFile)
		if err != nil {
			log.Fatal("Error loading RDB:", err)
			return
		}
		db.RestoreSnapshot(entries)
	}

	// if aof file exists, replay it
	if cfg.AppendOnly && aof != nil{
		err = aof.Replay(srv.ReplayHandler())
		if err != nil {
			log.Fatal("Error replaying AOF:", err)
		}
	}

	// start accepting connections
	fmt.Println("Listening on port 7379")

	for {
		conn, err := listener.Accept()

		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}

		fmt.Println("New connection:", conn.RemoteAddr())

		go handleConnection(conn, srv, cfg)
	}
}

func handleConnection(conn net.Conn, srv *commands.Server, cfg *config.Config) {
	sess := commands.NewSession(conn, srv)
	defer sess.Close()
	defer conn.Close()

	reader := bufio.NewReader(conn)

	for {
		cmdLine, err := resp.DecodeBulkStringsArrayFromReader(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("Error reading command:", err, string(cmdLine))
			break
		}
		if cfg.MaxCommandSize > 0 && len(cmdLine) > cfg.MaxCommandSize {
			conn.Write([]byte(resp.EncodeError(fmt.Sprintf("Command too large: %d > %d", len(cmdLine), cfg.MaxCommandSize))))
			continue
		}

		cmd, err := resp.Decode(cmdLine)
		if err != nil {
			fmt.Println("Error decoding command:", err)
			conn.Write([]byte(resp.EncodeError(fmt.Sprintf("Error decoding command: %v", err))))
			continue
		}
		conn.Write([]byte(sess.Execute(cmd)))
	}
}

package main

import (
	"fmt"
	"log"
	"net"

	"github.com/kevinnadar22/ledis/internal/commands"
	"github.com/kevinnadar22/ledis/internal/config"
	"github.com/kevinnadar22/ledis/internal/datatypes"
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

	// create server with db and aof
	srv := commands.NewServer(db, aof, cfg, rdb)

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
	if cfg.AppendOnly {
		err = aof.Replay(func(cmd datatypes.Command) error {
			srv.Execute(cmd)
			return nil
		})
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

		go handleConnection(conn, srv)
	}
}

func handleConnection(conn net.Conn, srv *commands.Server) {
	defer conn.Close()

	buffer := make([]byte, 1024)

	for {
		n, err := conn.Read(buffer)
		if err != nil {
			break
		}

		command_data := string(buffer[:n])
		// decode
		cmd, err := resp.Decode(command_data)
		if err != nil {
			fmt.Println("Error decoding command:", err)
			continue
		}
		response := srv.Execute(cmd)
		// write back to client
		conn.Write([]byte(response))
	}
}
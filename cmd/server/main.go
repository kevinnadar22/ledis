package main

import (
	"fmt"

	"log"
	"net"

	"github.com/kevinnadar22/ledis/internal/commands"
	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/persistence"
	"github.com/kevinnadar22/ledis/internal/resp"
)

func main() {
	listener, err := net.Listen("tcp", ":7379")

	if err != nil {
		log.Fatal(err)
		return
	}

	defer listener.Close()
	defer persistence.AOFStore.Close()

	// if aof file exists, replay it
	err = persistence.AOFStore.Replay(func(cmd datatypes.Command) error {
		commands.Execute(cmd)
		return nil
	})
	
	if err != nil {
		log.Fatal("Error replaying AOF:", err)
	}

	fmt.Println("Listening on port 7379")

	for {
		conn, err := listener.Accept()

		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}

		fmt.Println("New connection:", conn.RemoteAddr())

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
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
		response := commands.Execute(cmd)
		// write back to clinet
		conn.Write([]byte(response))
	}
}
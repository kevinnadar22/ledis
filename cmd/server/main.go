package main

import (

	"fmt"

	"log"
	"net"

	"github.com/kevinnadar22/ledis/internal/commands"
	"github.com/kevinnadar22/ledis/internal/resp"
)

func main() {
	listener, err := net.Listen("tcp", ":6379")

	if err != nil {
		log.Fatal(err)
		return
	}

	defer listener.Close()

	fmt.Println("Listening on port 6379")

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
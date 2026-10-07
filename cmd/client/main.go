package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/kevinnadar22/ledis/internal/resp"
)

func main() {
	conn, err := net.Dial("tcp", "localhost:6379")

	if err != nil {
		log.Fatal(err)
	}

	defer conn.Close()

	fmt.Println("Connected to server")

	for {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("> ")
		input, _ := reader.ReadString('\n')

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		cmd := resp.Encode(input)

		_, err = conn.Write([]byte(cmd))
		if err != nil {
			panic(err)
		}

		b := make([]byte, 1024)
		n, err := conn.Read(b)

		if err != nil {
			panic(err)
		}

		_, err = resp.Decode(string(b[:n]))
		if err != nil {
			panic(err)
		}

	}
}

package persistence

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
)

type AOF struct {
	file *os.File
	mu   sync.Mutex
	replaying bool
}

func NewAOF(path string) (*AOF, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	return &AOF{file: file}, nil
}

func (a *AOF) Append(respCmd []byte) error {
	if a.replaying {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	n, err := a.file.Write(respCmd)
	if err != nil {
		return err
	}

	if n != len(respCmd) {
		return io.ErrShortWrite
	}

	// sync
	err = a.file.Sync()
	if err != nil {
		return err
	}

	return nil
}

func (a *AOF) Close() error {
	return a.file.Close()
}


func (a *AOF) Replay(handler func(cmd datatypes.Command) error) error {
	// we need to read the file buffer by buffer, call the decode function and update the bytes consumed, if error, skip that and continue
	// since we are replaying, we don't need to write to AOF 
	a.replaying = true
	defer func() {
		a.replaying = false
	}()

	reader := bufio.NewReader(a.file)


	// while 
	for {
		cmd_line, err := resp.DecodeBulkStringsArrayFromReader(reader)

		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		cmd, err := resp.Decode(cmd_line)

		fmt.Printf("Replaying: %q\n", *cmd.Cmd.Str)

		for _, arg := range cmd.Args {
			fmt.Printf("Arg: %q\n", *arg.Str)
		}
		
		if err != nil {
			fmt.Println("Error decoding command:", err)
			continue
		}
		handler(cmd)
	}

	return nil
}

var AOFStore, _ = NewAOF("./appendonly.aof")
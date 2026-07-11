package persistence

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
)

type FsyncPolicy int

const (
    FsyncAlways FsyncPolicy = iota
    FsyncEverySecond
    FsyncNo
)

type AOF struct {
	file        *os.File
	mu          sync.Mutex
	replaying   bool
	fsyncPolicy FsyncPolicy
	done        chan struct{}
}

func NewAOF(path string, policy FsyncPolicy) (*AOF, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	
	a := &AOF{
		file: file,
		fsyncPolicy: policy,
		done: make(chan struct{}),
	}

	// start background sync goroutine
	if a.fsyncPolicy == FsyncEverySecond {
		a.startSyncer()
	}

	return a, nil
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
	
	if a.fsyncPolicy == FsyncAlways {
		err = a.file.Sync()
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *AOF) Close() error {
    close(a.done)

    a.mu.Lock()
    defer a.mu.Unlock()

    err := a.file.Sync()
    if err != nil {
		return err
	}

	err = a.file.Close()

	if err != nil {
		return err
	}

	return nil
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

func (a *AOF) startSyncer() {
    ticker := time.NewTicker(time.Second)

    go func() {
        defer ticker.Stop()

        for range ticker.C {
            select {
			case <-ticker.C:
                a.mu.Lock()
                _ = a.file.Sync()
                a.mu.Unlock()

            case <-a.done:
                return
			}
        }
    }()
}
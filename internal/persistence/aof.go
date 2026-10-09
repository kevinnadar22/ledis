package persistence

import (
	"bufio"
	"io"
	"os"
	"strings"
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
	// suppress skips Append while EXEC batches one write; assumes one worker (internal/commands/worker.go).
	suppress bool
}

func NewAOF(path string, policy FsyncPolicy) (*AOF, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	a := &AOF{file: file, fsyncPolicy: policy, done: make(chan struct{})}
	if a.fsyncPolicy == FsyncEverySecond {
		a.startSyncer()
	}
	return a, nil
}

func (a *AOF) Lock() {
	if a != nil {
		a.mu.Lock()
	}
}
func (a *AOF) Unlock() {
	if a != nil {
		a.mu.Unlock()
	}
}
func (a *AOF) SetSuppress(v bool) {
	if a != nil {
		a.suppress = v
	}
}

func (a *AOF) WriteLocked(b []byte) error {
	if a == nil {
		return nil
	}
	return a.appendUnlocked(b)
}

func (a *AOF) Append(b []byte) error {
	if a == nil || a.suppress {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.appendUnlocked(b)
}

func (a *AOF) appendUnlocked(b []byte) error {
	if a.replaying {
		return nil
	}
	n, err := a.file.Write(b)
	if err != nil || n != len(b) {
		if err == nil {
			err = io.ErrShortWrite
		}
		return err
	}
	if a.fsyncPolicy == FsyncAlways {
		return a.file.Sync()
	}
	return nil
}

func (a *AOF) Close() error {
	close(a.done)
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.file.Sync(); err != nil {
		return err
	}
	return a.file.Close()
}

func (a *AOF) truncate(off int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	_ = os.Truncate(a.file.Name(), off)
	_, _ = a.file.Seek(off, io.SeekStart)
}

func (a *AOF) Replay(handler func(datatypes.Command) error) error {
	a.replaying = true
	defer func() { a.replaying = false }()
	_, err := a.file.Seek(0, io.SeekStart)

	if err != nil {
		return err
	}

	r := bufio.NewReader(a.file)
	var ok int64
	inMulti := false
	var multiStart int64
	var pending []datatypes.Command

	for {
		line, err := resp.DecodeBulkStringsArrayFromReader(r)
		if err != nil {
			off := ok
			if inMulti {
				off = multiStart
			}
			a.truncate(off)
			return nil
		}
		cmd, err := resp.Decode(line)
		if err != nil {
			return err
		}
		up := strings.ToUpper(cmd.Cmd.String())
		n := int64(len(line))

		if inMulti {
			if up == "EXEC" {
				for _, c := range pending {
					_ = handler(c)
				}
				inMulti = false
				pending = nil
			} else {
				pending = append(pending, cmd)
			}
			ok += n
			continue
		}
		if up == "MULTI" {
			inMulti, multiStart, pending = true, ok, nil
			ok += n
			continue
		}
		if err := handler(cmd); err != nil {
			return err
		}
		ok += n
	}
}

func (a *AOF) startSyncer() {
	ticker := time.NewTicker(time.Second)
	go func() {
		defer ticker.Stop()
		for {
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

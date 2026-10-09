package main

import (
	"sync"
	"testing"
)

// Stress MULTI/EXEC and WATCH from many goroutines; run with -race:
//
//	go test -race ./cmd/test/... -run TestMultiExecRace
func TestMultiExecRace(t *testing.T) {
	srv := newTestServer(t)

	const workers = 16
	const rounds = 50

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		w := w
		go func() {
			defer wg.Done()
			tx := newTestSessionFromServer(srv)
			other := newTestSessionFromServer(srv)
			for i := 0; i < rounds; i++ {
				key := "race-key"
				if w%2 == 0 {
					tx.Execute(makeCommand("WATCH", key))
				}
				tx.Execute(makeCommand("MULTI"))
				tx.Execute(makeCommand("SET", key, "v"))
				other.Execute(makeCommand("SET", key, "other"))
				tx.Execute(makeCommand("EXEC"))
			}
		}()
	}
	wg.Wait()
}

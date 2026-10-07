package store

import (
	"strconv"
	"sync"
	"testing"
)

func TestINCRConcurrentFromMissingKey(t *testing.T) {
	s := NewStore()
	const goroutines = 64
	const perG = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perG; j++ {
				if _, err := s.INCR("counter"); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()

	want := int64(goroutines * perG)
	val, ok := s.Get("counter")
	if !ok {
		t.Fatal("counter missing after concurrent INCR")
	}
	got, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		t.Fatalf("counter value: %q", val)
	}
	if got != want {
		t.Fatalf("counter = %d, want %d", got, want)
	}
}

package store

import (
	"errors"
	"strconv"
	"sync"

	"github.com/kevinnadar22/ledis/internal/utils"
)

type Store struct {
	mu      sync.RWMutex
	data    map[string]string
	expires map[string]int64
}

func NewStore() *Store {
	return &Store{
		data:    make(map[string]string),
		expires: make(map[string]int64),
	}
}

func (s *Store) lookup(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	exp, ok := s.expires[key]
	if ok && utils.IsExpired(exp) {
		delete(s.data, key)
		delete(s.expires, key)
		return "", false
	}

	value, ok := s.data[key]
	return value, ok
}

func (s *Store) Set(key string, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

func (s *Store) Expire(key string, exp int64) {
	// if key is non-existent, do not set exp
	_, ok := s.lookup(key)
	if !ok {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.expires[key] = utils.CalculateExpireAt(exp)
}

func (s *Store) Get(key string) (string, bool) {
	val, ok := s.lookup(key)
	return val, ok
}

func (s *Store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	delete(s.expires, key)
}

func (s *Store) Exist(key string) bool {
	_, ok := s.lookup(key)
	return ok
}

func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

func (s *Store) INCR(key string) (int64, error) {

	val, ok := s.lookup(key)
	if !ok {
		s.data[key] = "1"
		return 1, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	int_val, err := strconv.Atoi(val)
	if err != nil {
		return 0, errors.New("wrong datatype")
	}

	int_val++
	s.data[key] = strconv.Itoa(int_val)
	return int64(int_val), nil
}

func (s *Store) FlushAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string]string)
	s.expires = make(map[string]int64)
}

func (s *Store) TTL(key string) int64 {
	_, ok := s.lookup(key)
	if !ok {
		// non-existing key
		return -2
	}

	exp, ok := s.expires[key]

	if !ok {
		// no exp key
		return -1
	}

	return utils.RemainingTTL(exp)
}

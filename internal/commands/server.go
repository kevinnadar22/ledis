package commands

import (
	"github.com/kevinnadar22/ledis/internal/config"
	"github.com/kevinnadar22/ledis/internal/persistence"
	"github.com/kevinnadar22/ledis/internal/store"
)

type Server struct {
	db        *store.Store
	aof       *persistence.AOF
	config    *config.Config
	rdb       *persistence.RDB
	pubsub    *store.PubSub
	jobs      chan commandJob // channel to send commands to the worker
	watchedKeys map[string]map[*Session]struct{}
}

func NewServer(db *store.Store, aof *persistence.AOF, config *config.Config, rdb *persistence.RDB, pubsub *store.PubSub) *Server {
	s := &Server{
		db:          db,
		aof:         aof,
		config:      config,
		rdb:         rdb,
		pubsub:      pubsub,
		jobs:        make(chan commandJob),
		watchedKeys: make(map[string]map[*Session]struct{}),
	}
	s.startWorker()
	return s
}

func (s *Server) DB() *store.Store {
	return s.db
}

func (s *Server) AOF() *persistence.AOF {
	return s.aof
}

func (s *Server) RDB() *persistence.RDB {
	return s.rdb
}

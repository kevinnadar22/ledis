package commands

import (
	"github.com/kevinnadar22/ledis/internal/persistence"
	"github.com/kevinnadar22/ledis/internal/store"
	"github.com/kevinnadar22/ledis/internal/config"
)

type Server struct {
	db  *store.Store
	aof *persistence.AOF
	config *config.Config
	rdb *persistence.RDB
}

func NewServer(db *store.Store, aof *persistence.AOF, config *config.Config, rdb *persistence.RDB) *Server {
	return &Server{
		db:  db,
		aof: aof,
		config: config,
		rdb: rdb,
	}
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
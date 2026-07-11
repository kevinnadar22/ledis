package commands

import (
	"github.com/kevinnadar22/ledis/internal/persistence"
	"github.com/kevinnadar22/ledis/internal/store"
)

type Server struct {
	db  *store.Store
	aof *persistence.AOF
}

func NewServer(db *store.Store, aof *persistence.AOF) *Server {
	return &Server{
		db:  db,
		aof: aof,
	}
}

func (s *Server) DB() *store.Store {
	return s.db
}

func (s *Server) AOF() *persistence.AOF {
	return s.aof
}

package commands

import (
	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
)

func (s *Server) FlushAll(cmd datatypes.Command) (string, error) {
	s.db.FlushAll()
	_ = s.aof.Append([]byte(cmd.RawContent))
	return resp.EncodeSimpleString("OK"), nil
}

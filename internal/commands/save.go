package commands

import (

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
)

func (s *Server) Save(cmd datatypes.Command) (string, error) {
	entries, err := s.DB().SaveSnapshot()
	if err != nil {
		return resp.EncodeError(err.Error()), nil
	}
	err = s.RDB().Save(s.config.RDBFile, entries)
	if err != nil {
		return resp.EncodeError(err.Error()), nil
	}
	return resp.EncodeSimpleString("OK"), nil
}
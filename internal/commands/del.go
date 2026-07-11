package commands

import (
	"errors"
	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
)

func (s *Server) Del(cmd datatypes.Command) (string, error) {
	if len(cmd.Args) < 1 {
		return "", errors.New("wrong number of arguments for 'del' command")
	}
	count := 0
	for _, arg := range cmd.Args {
		k := *arg.Str
		if s.db.Exist(k) {
			s.db.Delete(k)
			count++
		}
	}

	_ = s.aof.Append([]byte(cmd.RawContent))

	return resp.EncodeInteger(int64(count)), nil
}
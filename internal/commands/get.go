package commands

import (
	"errors"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
)

func (sess *Session) Get(cmd datatypes.Command) (string, error) {
	if len(cmd.Args) < 1 {
		return "", errors.New("wrong number of arguments for 'get' command")
	}
	k := *cmd.Args[0].Str

	v, ok := sess.srv.db.Get(k)

	if ok != true {
		return resp.EncodeBulkString(""), nil
	}
	return resp.EncodeBulkString(v), nil
}

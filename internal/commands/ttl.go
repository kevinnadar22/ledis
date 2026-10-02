package commands

import (
	"errors"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
)

func (sess *Session) TTL(cmd datatypes.Command) (string, error) {
	if len(cmd.Args) != 1 {
		return "", errors.New("wrong number of arguments for 'ttl' command")
	}
	k := *cmd.Args[0].Str
	ttl := sess.srv.db.TTL(k)
	return resp.EncodeInteger(ttl), nil
}
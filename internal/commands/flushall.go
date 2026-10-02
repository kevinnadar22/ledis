package commands

import (
	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
)

func (sess *Session) FlushAll(cmd datatypes.Command) (string, error) {
	sess.srv.db.FlushAll()
	_ = sess.srv.aof.Append([]byte(cmd.RawContent))
	return resp.EncodeSimpleString("OK"), nil
}

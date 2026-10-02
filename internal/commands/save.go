package commands

import (

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
)

func (sess *Session) Save(cmd datatypes.Command) (string, error) {
	entries, err := sess.srv.DB().SaveSnapshot()
	if err != nil {
		return resp.EncodeError(err.Error()), nil
	}
	err = sess.srv.RDB().Save(sess.srv.config.RDBFile, entries)
	if err != nil {
		return resp.EncodeError(err.Error()), nil
	}
	return resp.EncodeSimpleString("OK"), nil
}
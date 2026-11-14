package commands

import (
	"strings"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
)

func (sess *Session) Info(cmd datatypes.Command) (string, error) {
	if len(cmd.Args) > 1 {
		return resp.EncodeError("wrong number of arguments for 'info' command"), nil
	}

	section := "all"
	if len(cmd.Args) == 1 {
		section = strings.ToLower(cmd.Args[0].String())
	}

	switch section {
	case "all", "replication":
		return resp.EncodeBulkString(sess.srv.repl.InfoReplication()), nil
	default:
		return resp.EncodeError("unknown INFO section"), nil
	}
}
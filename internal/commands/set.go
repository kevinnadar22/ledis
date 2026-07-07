package commands

import (
	"errors"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
	"github.com/kevinnadar22/ledis/internal/store"
)

func Set(cmd datatypes.Command) (string, error) {
	if len(cmd.Args) < 2 {
		return "", errors.New("wrong number of arguments for 'set' command")
	}
	k := *cmd.Args[0].Str
	v := *cmd.Args[1].Str

	store.DB.Set(k, v)

	return resp.EncodeSimpleString("OK"), nil
}
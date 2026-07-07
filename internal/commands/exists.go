package commands

import (
	"errors"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
	"github.com/kevinnadar22/ledis/internal/store"
)

func Exists(cmd datatypes.Command) (string, error) {
	if len(cmd.Args) < 1 {
		return "", errors.New("wrong number of arguments for 'exists' command")
	}
	k := *cmd.Args[0].Str
	if store.DB.Exist(k) {
		return resp.EncodeInteger(1), nil
	}
	return resp.EncodeInteger(0), nil
}
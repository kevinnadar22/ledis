package commands

import (
	"errors"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
	"github.com/kevinnadar22/ledis/internal/store"
)

func INCR(cmd datatypes.Command) (string, error) {
	if len(cmd.Args) < 1 {
		return "", errors.New("wrong number of arguments for 'incr' command")
	}
	k := *cmd.Args[0].Str

	val, err := store.DB.INCR(k)

	if err != nil {
		return "", err
	}

	return resp.EncodeInteger(val), nil
}
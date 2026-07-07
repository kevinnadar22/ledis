package commands

import (
	"errors"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
)

func Echo(cmd datatypes.Command) (string, error) {
	if len(cmd.Args) == 0 {
		return "", errors.New("wrong number of arguments for 'echo' command")
	}
	return resp.EncodeSimpleString(*cmd.Args[0].Str), nil
}

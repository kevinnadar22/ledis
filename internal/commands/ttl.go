package commands

import (
	"errors"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
	"github.com/kevinnadar22/ledis/internal/store"
)

func TTL(cmd datatypes.Command) (string, error) {
	if len(cmd.Args) != 1 {
		return "", errors.New("wrong number of arguments for 'ttl' command")
	}
	k := *cmd.Args[0].Str
	ttl := store.DB.TTL(k)
	return resp.EncodeInteger(ttl), nil
}
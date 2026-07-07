package commands

import (
	"errors"
	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
	"github.com/kevinnadar22/ledis/internal/store"
)

func Del(cmd datatypes.Command) (string, error) {
	if len(cmd.Args) < 1 {
		return "", errors.New("wrong number of arguments for 'del' command")
	}
	count := 0
	for _, arg := range cmd.Args {
		k := *arg.Str
		if store.DB.Exist(k) {
			store.DB.Delete(k)
			count++
		}
	}
	return resp.EncodeInteger(int64(count)), nil
}
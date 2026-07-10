package commands

import (
	"errors"
	"log"
	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
	"github.com/kevinnadar22/ledis/internal/store"
	"github.com/kevinnadar22/ledis/internal/persistence"
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

	err = persistence.AOFStore.Append([]byte(cmd.RawContent))
	if err != nil {
		log.Println("Error appending to AOF:", err)
		return "", err
	}

	return resp.EncodeInteger(val), nil
}
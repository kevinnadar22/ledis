package commands

import (
	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
	"github.com/kevinnadar22/ledis/internal/store"
)

func FlushAll(cmd datatypes.Command) (string, error) {
	store.DB.FlushAll()
	return resp.EncodeSimpleString("OK"), nil
}

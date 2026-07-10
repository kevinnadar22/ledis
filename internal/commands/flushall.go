package commands

import (
	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
	"github.com/kevinnadar22/ledis/internal/store"
	"github.com/kevinnadar22/ledis/internal/persistence"
)

func FlushAll(cmd datatypes.Command) (string, error) {
	store.DB.FlushAll()
	persistence.AOFStore.Append([]byte(cmd.RawContent))
	return resp.EncodeSimpleString("OK"), nil
}

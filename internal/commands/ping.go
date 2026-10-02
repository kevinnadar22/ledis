package commands

import (

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
)

func (sess *Session) Ping(cmd datatypes.Command) (string, error) {
	return resp.EncodeSimpleString("PONG"), nil
}
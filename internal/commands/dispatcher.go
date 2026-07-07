package commands

import (
	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
)

func Execute(cmd datatypes.Command) string {
	cmdStr := cmd.Cmd.String()
	var handler func(cmd datatypes.Command) (string, error)

	switch cmdStr {
	case "PING":
		handler = Ping
	case "ECHO":
		handler = Echo
	case "GET":
		handler = Get
	case "SET":
		handler = Set
	case "INCR":
		handler = INCR
	case "DEL":
		handler = Del
	case "EXISTS":
		handler = Exists
	default:
		return resp.EncodeError("unknown command")
	}
	str, err := handler(cmd)
	if err != nil {
		return resp.EncodeError(err.Error())
	}
	return str
}
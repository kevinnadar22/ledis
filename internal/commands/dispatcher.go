package commands

import (
	"strings"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
)

func (s *Server) Execute(cmd datatypes.Command) string {
	cmdStr := strings.ToUpper(cmd.Cmd.String())
	var handler func(cmd datatypes.Command) (string, error)

	switch cmdStr {
	case "PING":
		handler = s.Ping
	case "ECHO":
		handler = s.Echo
	case "GET":
		handler = s.Get
	case "SET":
		handler = s.Set
	case "INCR":
		handler = s.INCR
	case "DEL":
		handler = s.Del
	case "EXISTS":
		handler = s.Exists
	case "FLUSHALL":
		handler = s.FlushAll
	case "TTL":
		handler = s.TTL
	case "SAVE":
		handler = s.Save
	case "BGSAVE":
		handler = s.BGSave
	default:
		return resp.EncodeError("unknown command")
	}
	str, err := handler(cmd)
	if err != nil {
		return resp.EncodeError(err.Error())
	}
	return str
}
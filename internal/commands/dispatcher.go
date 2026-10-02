package commands

import (
	"strings"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
)

func (s *Server) Execute(cmd datatypes.Command) string {
	return (&Session{srv: s}).Execute(cmd)
}

func (sess *Session) Execute(cmd datatypes.Command) string {
	cmdStr := strings.ToUpper(cmd.Cmd.String())
	var handler func(datatypes.Command) (string, error)

	switch cmdStr {
	case "PING":
		handler = sess.Ping
	case "ECHO":
		handler = sess.Echo
	case "GET":
		handler = sess.Get
	case "SET":
		handler = sess.Set
	case "INCR":
		handler = sess.INCR
	case "DEL":
		handler = sess.Del
	case "EXISTS":
		handler = sess.Exists
	case "FLUSHALL":
		handler = sess.FlushAll
	case "TTL":
		handler = sess.TTL
	case "SAVE":
		handler = sess.Save
	case "BGSAVE":
		handler = sess.BGSave
	case "SUBSCRIBE":
		handler = sess.Subscribe
	case "PUBLISH":
		handler = sess.Publish
	case "UNSUBSCRIBE":
		handler = sess.Unsubscribe
	default:
		return resp.EncodeError("unknown command")
	}
	str, err := handler(cmd)
	if err != nil {
		return resp.EncodeError(err.Error())
	}
	return str
}

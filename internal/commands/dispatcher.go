package commands

import (
	"strings"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
	"github.com/kevinnadar22/ledis/internal/utils"
)


// Execute runs a command on the caller goroutine (e.g. AOF replay before live traffic).
func (s *Server) Execute(cmd datatypes.Command) string {
	return NewSession(nil, s).run(cmd)
}

// Execute enqueues the command on the global worker (serialized with all clients).
func (sess *Session) Execute(cmd datatypes.Command) string {
	return sess.enqueue(cmd)
}

func (sess *Session) run(cmd datatypes.Command) string {
	cmdStr := strings.ToUpper(cmd.Cmd.String())
	var handler func(datatypes.Command) (string, error)

	// see if trn is active and if the command is a transaction command
	if sess.trn.active  {
		if !utils.IsCommandAllowedInTransaction(cmdStr) {
			return resp.EncodeError("ERR command not allowed in transaction")
		}
		if !utils.IsTransactionCommand(cmdStr) {
			sess.trn.multiCmds = append(sess.trn.multiCmds, cmd)
			return resp.EncodeSimpleString("QUEUED")
		}
		// here cmds like MULTI, EXEC, DISCARD are allowed, it will flow through the switch case below
	}

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
	case "MULTI":
		handler = sess.Multi
	case "EXEC":
		handler = sess.Exec
	case "DISCARD":
		handler = sess.Discard
	default:
		return resp.EncodeError("unknown command")
	}
	str, err := handler(cmd)
	if err != nil {
		return resp.EncodeError(err.Error())
	}
	return str
}

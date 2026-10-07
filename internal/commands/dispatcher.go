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
	handler = getHandler(sess, cmdStr)

	// MULTI/EXEC/DISCARD
	// see if trn is active and if the command is a transaction command
	if sess.trn.active {
		if handler == nil {
			sess.trn.errorFlag = true
			return resp.EncodeError("unknown command '" + cmdStr + "'")
		}
		if !utils.IsCommandAllowedInTransaction(cmdStr) {
			sess.trn.errorFlag = true
			return resp.EncodeError("command not allowed in transaction")
		}

		if !utils.CheckMinArgs(cmdStr, len(cmd.Args)) {
			sess.trn.errorFlag = true
			return resp.EncodeError("wrong number of arguments for '" + cmdStr + "' command")
		}

		if !utils.IsTransactionCommand(cmdStr) {
			sess.trn.multiCmds = append(sess.trn.multiCmds, cmd)
			return resp.EncodeSimpleString("QUEUED")
		}
		// here cmds like MULTI, EXEC, DISCARD are allowed, it will flow through the switch case below
	} else if handler == nil {
		return resp.EncodeError("unknown command '" + cmdStr + "'")
	}

	// WATCH: mutating commands invalidate watchers of every affected key.
	if utils.IsMutatingCommand(cmdStr) {
		argKeys := make([]string, 0, len(cmd.Args))
		for _, arg := range cmd.Args {
			argKeys = append(argKeys, arg.String())
		}
		keys, all := utils.WatchAffectedKeys(cmdStr, argKeys)
		if all {
			sess.MarkAllWatchesDirty()
		} else {
			for _, k := range keys {
				sess.MarkWatchesDirty(k)
			}
		}
	}

	str, err := handler(cmd)
	if err != nil {
		return resp.EncodeError(err.Error())
	}
	return str
}

func getHandler(sess *Session, cmdStr string) func(datatypes.Command) (string, error) {
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
	case "MULTI":
		handler = sess.Multi
	case "EXEC":
		handler = sess.Exec
	case "DISCARD":
		handler = sess.Discard
	case "WATCH":
		handler = sess.Watch
	case "UNWATCH":
		handler = sess.Unwatch
	default:
		return nil
	}
	return handler
}

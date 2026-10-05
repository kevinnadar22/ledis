package commands

import (
	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
)


func (sess *Session) Multi(cmd datatypes.Command) (string, error) {
	if sess.trn.active {
		return resp.EncodeError("MULTI calls can not be nested"), nil
	}
	sess.trn.active = true
	return resp.EncodeSimpleString("OK"), nil
}

func (sess *Session) Exec(cmd datatypes.Command) (string, error) {
	defer func() {
		// Clear the transaction and watches for the current session
		sess.endTransaction()
		sess.clearWatches()
	}()

	if !sess.trn.active {
		return resp.EncodeError("EXEC without MULTI"), nil
	}

	if sess.trn.errorFlag {
		return resp.EncodeSimpleError("EXECABORT Transaction discarded because of previous errors"), nil
	}

	if sess.trn.dirty {
		return resp.EncodeNil(), nil
	}

	queued := sess.trn.multiCmds
	sess.endTransaction()

	results := []string{}
	for _, cmd := range queued {
		results = append(results, sess.run(cmd))
	}
	return resp.EncodeArrayOfReplies(results), nil
}

func (sess *Session) Discard(cmd datatypes.Command) (string, error) {
	if !sess.trn.active {
		return resp.EncodeError("DISCARD without MULTI"), nil
	}
	sess.endTransaction()
	sess.clearWatches()
	return resp.EncodeSimpleString("OK"), nil
}

func (sess *Session) Watch(cmd datatypes.Command) (string, error) {
	// /check if at least one key is exists inargs
	if len(cmd.Args) < 1 {
		return resp.EncodeError("WATCH requires at least one key"), nil
	}
	for _, key := range cmd.Args {
		keyStr := key.String()
		if _, ok := sess.srv.watchedKeys[keyStr]; !ok {
			sess.srv.watchedKeys[keyStr] = make(map[*Session]struct{})
		}
		sess.srv.watchedKeys[keyStr][sess] = struct{}{}
	}
	return resp.EncodeSimpleString("OK"), nil
}

func (sess *Session) Unwatch(cmd datatypes.Command) (string, error) {
	if len(cmd.Args) == 0 {
		sess.clearWatches()
		return resp.EncodeSimpleString("OK"), nil
	}
	return resp.EncodeError("wrong number of arguments for UNWATCH"), nil
}

// clearWatches removes this session from all server watch lists (Redis: after EXEC/DISCARD/UNWATCH).
func (sess *Session) clearWatches() {
	for key, sessions := range sess.srv.watchedKeys {
		delete(sessions, sess)
		if len(sessions) == 0 {
			delete(sess.srv.watchedKeys, key)
		}
	}
}

func (sess *Session) endTransaction() {
	sess.trn.active = false
	sess.trn.multiCmds = []datatypes.Command{}
	sess.trn.errorFlag = false
	sess.trn.dirty = false
}

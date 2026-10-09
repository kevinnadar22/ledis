package commands

import (
	"bytes"

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

	results := make([]string, 0, len(queued))
	aof := sess.srv.aof

	aof.Lock()
	defer aof.Unlock()
	
	aof.SetSuppress(true)
	defer aof.SetSuppress(false)

	var batch bytes.Buffer
	batch.WriteString(resp.Encode("MULTI"))
	for _, cmd := range queued {
		results = append(results, sess.run(cmd))
		batch.WriteString(cmd.RawContent)
	}
	batch.WriteString(resp.Encode("EXEC"))

	if err := aof.WriteLocked(batch.Bytes()); err != nil {
		return resp.EncodeError(err.Error()), nil
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

func (sess *Session) MarkAllWatchesDirty() {
	for _, sessions := range sess.srv.watchedKeys {
		for session := range sessions {
			session.trn.dirty = true
		}
	}
}

func (sess *Session) MarkWatchesDirty(key string) {
	if _, ok := sess.srv.watchedKeys[key]; ok {
		for session := range sess.srv.watchedKeys[key] {
			session.trn.dirty = true
		}
	}
}

func (sess *Session) endTransaction() {
	sess.trn.active = false
	sess.trn.multiCmds = []datatypes.Command{}
	sess.trn.errorFlag = false
	sess.trn.dirty = false
}

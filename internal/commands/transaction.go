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
	
	if !sess.trn.active {
		return resp.EncodeError("EXEC without MULTI"), nil
	}
	if sess.trn.errorFlag {
		endTransaction(sess.trn)
		return resp.EncodeSimpleError("EXECABORT Transaction discarded because of previous errors"), nil
	}

	queued := sess.trn.multiCmds
	endTransaction(sess.trn)

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
	endTransaction(sess.trn)
	return resp.EncodeSimpleString("OK"), nil
}

func endTransaction(trn *Transaction) {
	trn.active = false
	trn.multiCmds = []datatypes.Command{}
	trn.errorFlag = false
}

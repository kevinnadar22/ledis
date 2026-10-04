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
	sess.trn.active = false
	results := []string{}
	for _, cmd := range sess.trn.multiCmds {

		result := sess.run(cmd)
		results = append(results, result)
	}
	sess.trn.multiCmds = []datatypes.Command{}
	return resp.EncodeArrayOfReplies(results), nil
}

func (sess *Session) Discard(cmd datatypes.Command) (string, error) {
	if !sess.trn.active {
		return resp.EncodeError("DISCARD without MULTI"), nil
	}
	sess.trn.active = false
	sess.trn.multiCmds = []datatypes.Command{}
	return resp.EncodeSimpleString("OK"), nil
}


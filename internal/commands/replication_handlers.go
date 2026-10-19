package commands

import "github.com/kevinnadar22/ledis/internal/datatypes"

func (sess *Session) ReplicaOf(cmd datatypes.Command) (string, error) {
	return sess.srv.repl.ReplicaOf(cmd)
}

func (sess *Session) ReplConf(cmd datatypes.Command) (string, error) {
	return sess.srv.repl.ReplConf(sess.conn, sess.srv.config.Port, cmd)
}

func (sess *Session) ReplicaOfNoOne(cmd datatypes.Command) (string, error) {
	return sess.srv.repl.ReplicaOfNoOne(cmd)
}

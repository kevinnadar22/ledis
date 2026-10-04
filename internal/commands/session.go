package commands

import (
	"errors"
	"net"

	"github.com/kevinnadar22/ledis/internal/store"
	"github.com/kevinnadar22/ledis/internal/datatypes"
)

type Transaction struct {
	active      bool // is in multi mode
	multiCmds    []datatypes.Command
}
type Session struct {
	conn         net.Conn
	srv          *Server
	pubsubClient *store.Client

	// transaction mode
	trn *Transaction
}

func NewSession(conn net.Conn, srv *Server) *Session {
	return &Session{conn: conn, srv: srv, trn: &Transaction{}}
}

func (sess *Session) Close() {
	if sess.pubsubClient == nil {
		return
	}
	sess.srv.pubsub.RemoveClient(sess.pubsubClient)
	close(sess.pubsubClient.Outgoing)
	sess.pubsubClient = nil
	sess.trn = &Transaction{}
}

func (sess *Session) ensurePubsubClient() error {
	if sess.conn == nil {
		return errors.New("command requires an active connection")
	}
	if sess.pubsubClient != nil {
		return nil
	}
	sess.pubsubClient = &store.Client{
		Conn:     sess.conn,
		Outgoing: make(chan string),
	}
	go sess.pubsubClient.WriteLoop()
	return nil
}

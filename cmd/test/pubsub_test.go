package main

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/kevinnadar22/ledis/internal/commands"
	"github.com/kevinnadar22/ledis/internal/resp"
)

func TestSubscribeRequiresConnection(t *testing.T) {
	sess := newTestSession(t)
	got := sess.Execute(makeCommand("SUBSCRIBE", "orders"))
	if got != "-ERR command requires an active connection\r\n" {
		t.Errorf("got %q", got)
	}
}

func TestPublishNoSubscribers(t *testing.T) {
	sess := newTestSession(t)
	got := sess.Execute(makeCommand("PUBLISH", "orders", "hey"))
	if got != ":0\r\n" {
		t.Errorf("got %q, want :0\\r\\n", got)
	}
}

func TestSubscribePublishDeliverMessage(t *testing.T) {
	srv := newTestServer(t)
	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() {
		serverConn.Close()
		clientConn.Close()
	})

	subSess := commands.NewSession(serverConn, srv)
	pubSess := newTestSessionFromServer(srv)

	wantSub := resp.EncodeArray([]string{"subscribe", "orders", "1"})
	if got := subSess.Execute(makeCommand("SUBSCRIBE", "orders")); got != wantSub {
		t.Fatalf("SUBSCRIBE: got %q, want %q", got, wantSub)
	}

	wantMsg := resp.EncodeArray([]string{"message", "orders", "hey"})
	msgCh := make(chan string, 1)
	go func() {
		buf := make([]byte, len(wantMsg))
		_, err := io.ReadFull(clientConn, buf)
		if err != nil {
			return
		}
		msgCh <- string(buf)
	}()

	if got := pubSess.Execute(makeCommand("PUBLISH", "orders", "hey")); got != ":1\r\n" {
		t.Fatalf("PUBLISH: got %q, want :1\\r\\n", got)
	}
	select {
	case got := <-msgCh:
		if got != wantMsg {
			t.Errorf("push message: got %q, want %q", got, wantMsg)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for pub/sub message on subscriber connection")
	}
}

func TestUnsubscribe(t *testing.T) {
	srv := newTestServer(t)
	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() {
		serverConn.Close()
		clientConn.Close()
	})

	sess := commands.NewSession(serverConn, srv)
	sess.Execute(makeCommand("SUBSCRIBE", "orders"))
	sess.Execute(makeCommand("SUBSCRIBE", "news"))

	wantUnsub := resp.EncodeArray([]string{"unsubscribe", "orders", "1"})
	if got := sess.Execute(makeCommand("UNSUBSCRIBE", "orders")); got != wantUnsub {
		t.Fatalf("UNSUBSCRIBE: got %q, want %q", got, wantUnsub)
	}
	if got := sess.Execute(makeCommand("PUBLISH", "orders", "x")); got != ":0\r\n" {
		t.Errorf("PUBLISH after unsub: got %q", got)
	}
	if got := sess.Execute(makeCommand("PUBLISH", "news", "y")); got != ":1\r\n" {
		t.Errorf("PUBLISH news: got %q", got)
	}
	_ = clientConn
}

func TestUnsubscribeWithoutSubscribe(t *testing.T) {
	sess := newTestSession(t)
	got := sess.Execute(makeCommand("UNSUBSCRIBE", "orders"))
	if got != "-ERR not subscribed to any topics\r\n" {
		t.Errorf("got %q", got)
	}
}

package commands

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
	"github.com/kevinnadar22/ledis/internal/utils"
)

func (sess *Session) ReplicaOf(cmd datatypes.Command) (string, error) {
	// check if 2 args are provided
	if len(cmd.Args) != 2 {
		return resp.EncodeError("wrong number of arguments for 'replica of' command"), nil
	}

	// host port
	host := cmd.Args[0]
	port := cmd.Args[1]

	// check if the command is "replica of no one"
	if strings.ToUpper(host.String()) == "NO" && strings.ToUpper(port.String()) == "ONE" {
		return sess.ReplicaOfNoOne(cmd)
	}

	// check if the host and port are valid
	if !utils.ValidateHostPort(host.String(), port.String()) {
		return resp.EncodeError("invalid argument for 'replica of' command"), nil
	}

	// create a new replica client
	ctx, cancel := context.WithCancel(context.Background())
	
	sess.mu.Lock()

	// stop the replication if it is already running
	sess.stopReplicationLocked()

	defer sess.mu.Unlock()
	sess.masterCancel = cancel

	// start the replica client
	go func() {
		sess.replicate(ctx, host.String(), port.String())
	}()

	return resp.EncodeSimpleString("OK"), nil
}


func (sess *Session) ReplicaOfNoOne(cmd datatypes.Command) (string, error) {
	// check if 0 args are provided
	if len(cmd.Args) != 2 {
		return resp.EncodeError("wrong number of arguments for 'replica of no one' command"), nil
	}
	sess.stopReplication()

	return resp.EncodeSimpleString("OK"), nil
}

func (sess *Session) replicate(ctx context.Context, host string, port string) {

	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%s", host, port))
	if err != nil {
		log.Printf("Failed to connect to master: %v", err)
		return
	}
	defer conn.Close()

	for {
		select {
		case <-ctx.Done():
			// context is done, stop the replication
			log.Println("Replication stopped")
			return
		default:
			for {}
			// read the command from the master and execute it
			
		}
	}
}

func (sess *Session) stopReplication() {
    sess.mu.Lock()
    defer sess.mu.Unlock()

    sess.stopReplicationLocked()
}

// Caller must already hold sess.mu.
func (sess *Session) stopReplicationLocked() {
    if sess.masterCancel != nil {
        sess.masterCancel()
        sess.masterCancel = nil
    }
}
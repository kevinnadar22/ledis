package commands

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
	"github.com/kevinnadar22/ledis/internal/utils"
)

type ReplicationClient struct {
	conn          net.Conn
	listeningPort int
	capabilities  map[string]bool
}

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

	sess.srv.mu.Lock()
	defer sess.srv.mu.Unlock()

	// stop the replication if it is already running
	sess.stopReplicationLocked()

	defer sess.srv.mu.Unlock()
	sess.srv.masterCancel = cancel

	// start the replica client
	go func() {
		sess.replicate(ctx, host.String(), port.String())
	}()

	return resp.EncodeSimpleString("OK"), nil
}

func (sess *Session) ReplConf(cmd datatypes.Command) (string, error) {
	if len(cmd.Args) != 2 {
		return resp.EncodeError("wrong number of arguments for 'repl conf' command"), nil
	}

	client, ok := sess.srv.replicas[sess.conn]
	if !ok {
		client, ok = sess.srv.pendingReplicas[sess.conn]
		if !ok {
			client = &ReplicationClient{
				conn: sess.conn,
				listeningPort: sess.srv.config.Port,
				capabilities:  make(map[string]bool),
			}
		}
	}

	option := strings.ToLower(cmd.Args[0].String())
	value := cmd.Args[1].String()

	switch option {
	case "listening-port":
		port, err := strconv.Atoi(value)
		if err != nil || port <= 0 || port > 65535 {
			return resp.EncodeError("invalid value for 'listening-port' option"), nil
		}
		client.listeningPort = port
	case "capa":
		client.capabilities[strings.ToLower(value)] = true
	default:
		return resp.EncodeError("invalid option for 'repl conf' command"), nil
	}

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

	reader := bufio.NewReader(conn)

	conn.Write([]byte(resp.EncodeSimpleString("PING")))
	if _, err := resp.DecodeSimpleStringFromReader(reader); err != nil {
		log.Printf("Failed to read response from master: %v", err)
		return
	}

	// REPLCONF listening-port
	conn.Write([]byte(resp.EncodeSimpleString("REPLCONF listening-port " + strconv.Itoa(sess.srv.config.Port))))
	if _, err := resp.DecodeSimpleStringFromReader(reader); err != nil {
		log.Printf("Failed to read response from master: %v", err)
		return
	}

	// this replica can handle psync2 protocol
	conn.Write([]byte(resp.EncodeSimpleString("REPLCONF capa psync2")))
	if _, err := resp.DecodeSimpleStringFromReader(reader); err != nil {
		log.Printf("Failed to read response from master: %v", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			// context is done, stop the replication
			log.Println("Replication stopped")
			return
		default:
			for {
			}
			// read the command from the master and execute it

		}
	}
}

func (sess *Session) stopReplication() {
	sess.srv.mu.Lock()
	defer sess.srv.mu.Unlock()

	sess.stopReplicationLocked()
}

// Caller must already hold sess.mu.
func (sess *Session) stopReplicationLocked() {
	if sess.srv.masterCancel != nil {
		sess.srv.masterCancel()
		sess.srv.masterCancel = nil
	}
}

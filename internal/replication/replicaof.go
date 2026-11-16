package replication

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
	"github.com/kevinnadar22/ledis/internal/utils"
)

func (c *Coordinator) ReplicaOf(cmd datatypes.Command) (string, error) {
	if len(cmd.Args) != 2 {
		return resp.EncodeError("wrong number of arguments for 'replica of' command"), nil
	}

	host, port := cmd.Args[0], cmd.Args[1]
	if strings.ToUpper(host.String()) == "NO" && strings.ToUpper(port.String()) == "ONE" {
		return c.ReplicaOfNoOne(cmd)
	}
	if !utils.ValidateHostPort(host.String(), port.String()) {
		return resp.EncodeError("invalid argument for 'replica of' command"), nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	masterHost, masterPort := host.String(), port.String()
	c.setUpstream(masterHost, masterPort, cancel)

	go c.StartReplicaLoop(ctx, masterHost, masterPort)
	return resp.EncodeSimpleString("OK"), nil
}

func (c *Coordinator) ReplicaOfNoOne(cmd datatypes.Command) (string, error) {
	if len(cmd.Args) != 2 {
		return resp.EncodeError("wrong number of arguments for 'replica of no one' command"), nil
	}
	c.Stop()
	return resp.EncodeSimpleString("OK"), nil
}

func (c *Coordinator) StartReplicaLoop(ctx context.Context, host, port string) {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%s", host, port))
	if err != nil {
		log.Printf("Failed to connect to master: %v", err)
		return
	}
	fmt.Println("connected to master")
	defer conn.Close()

	// lock the master connection
	c.mu.Lock()
	c.upstreamConn = conn
	c.mu.Unlock()

	reader := bufio.NewReader(conn)
	for _, msg := range [][]string{
		{"PING"},
		{"REPLCONF", "listening-port", strconv.Itoa(c.listenPort)},
		{"REPLCONF", "capa", "psync2"},
	} {
		conn.Write([]byte(resp.EncodeArray(msg)))
		if msg, err := resp.DecodeSimpleStringFromReader(reader); err != nil {
			log.Printf("replication handshake error on sending message: %v %s", err, msg)
			return
		} else {
			fmt.Println("message sent by master:", msg)
		}
	}

	err = c.PerformPartialSync(conn, reader)
	if err != nil {
		log.Printf("replication handshake error on sending psync request: %v", err)
		return
	}

	// replication handshake complete
	fmt.Println("replication handshake complete, loaded snapshot from master")

	for {
		select {
		case <-ctx.Done():
			log.Println("Replication stopped")
			return
		default:
			if err := c.readApplyReplicationCommand(reader); err != nil {
				if err == io.EOF {
					return
				}
				log.Printf("replication stream: %v", err)
				return
			}
		}
	}
}

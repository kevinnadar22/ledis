package replication

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
	c.mu.Lock()
	c.stopLocked()
	c.masterCancel = cancel
	c.mu.Unlock()

	go c.run(ctx, host.String(), port.String())
	return resp.EncodeSimpleString("OK"), nil
}

func (c *Coordinator) ReplicaOfNoOne(cmd datatypes.Command) (string, error) {
	if len(cmd.Args) != 2 {
		return resp.EncodeError("wrong number of arguments for 'replica of no one' command"), nil
	}
	c.Stop()
	return resp.EncodeSimpleString("OK"), nil
}

func (c *Coordinator) run(ctx context.Context, host, port string) {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%s", host, port))
	if err != nil {
		log.Printf("Failed to connect to master: %v", err)
		return
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	for _, msg := range []string{
		"PING",
		"REPLCONF listening-port " + strconv.Itoa(c.listenPort),
		"REPLCONF capa psync2",
	} {
		conn.Write([]byte(resp.EncodeSimpleString(msg)))
		if _, err := resp.DecodeSimpleStringFromReader(reader); err != nil {
			log.Printf("replication handshake: %v", err)
			return
		}
	}

	conn.Write([]byte(resp.EncodeSimpleString("PSYNC ? -1")))

	fullSyncResponse, err := resp.DecodeSimpleStringFromReader(reader)
	if err != nil {
		log.Printf("replication handshake: %v", err)
		return
	}

	parts := strings.Fields(fullSyncResponse)
	if len(parts) != 3 || parts[0] != "FULLRESYNC" {
		fmt.Errorf("invalid FULLRESYNC response")
		return
	}
	
	replID := parts[1]
	
	replOffset, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		fmt.Errorf("invalid FULLRESYNC response")
		return 
	}

	fmt.Printf("Full sync completed: ReplID: %s, ReplOffset: %d\n", replID, replOffset)



	for {
		select {
		case <-ctx.Done():
			log.Println("Replication stopped")
			return
		default:
			for {
			}
		}
	}
}

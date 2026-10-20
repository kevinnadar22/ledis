package replication

import (
	"fmt"
	"net"
	"strconv"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
)

func (c *Coordinator) Psync(conn net.Conn, cmd datatypes.Command, entries []datatypes.RDBEntry) (string, error) {
	// check if the command is valid
	if len(cmd.Args) != 2 {
		return "", fmt.Errorf("invalid command")
	}

	replID := cmd.Args[0].String()
	replOffset, err := strconv.ParseInt(cmd.Args[1].String(), 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid offset")
	}

	// lock the master connection
	c.mu.RLock()
	masterReplID := c.replID
	masterReplOffset := c.replStartOffset
	c.mu.RUnlock()

	// if repldId is ? then we need to send the full sync
	if replID == "?" || replOffset < masterReplOffset || replID != masterReplID {
		c.writeFullSyncResponse(conn)
		return "", c.SendFullSyncResponse(conn, entries)
	}
	
	c.writeFullSyncResponse(conn)
	return "", c.SendFullSyncResponse(conn, entries)
}

func (c *Coordinator) writeFullSyncResponse(conn net.Conn) error {
	r := resp.EncodeSimpleString("FULLRESYNC " + c.replID + " " + strconv.FormatInt(c.replStartOffset, 10))
	conn.Write([]byte(r))
	return nil
}
// psync_master.go — inbound PSYNC on a replica TCP connection (master role only).

package replication

import (
	"fmt"
	"net"
	"strconv"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
)

// Psync handles PSYNC from a connecting replica: full resync or +CONTINUE partial path.
func (c *Coordinator) Psync(conn net.Conn, cmd datatypes.Command, entries []datatypes.RDBEntry) (string, error) {
	// PSYNC <replid> <offset>
	if c.Role != datatypes.RoleMaster {
		return "", fmt.Errorf("psync: not a master")
	}
	if len(cmd.Args) != 2 {
		return "", fmt.Errorf("invalid command")
	}

	replID := cmd.Args[0].String()
	replOffset, err := strconv.ParseInt(cmd.Args[1].String(), 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid offset")
	}

	if !c.CanServeFromBacklog(replID, replOffset) {
		if err := c.sendFullResyncLine(conn); err != nil {
			return "", err
		}
		return "", c.SendFullSyncResponse(conn, entries)
	}

	if err := sendContinue(conn); err != nil {
		return "", err
	}

	// now send the partial sync response
	// read it from ReadFromBacklog and send to the wire
	backlog, err := c.ReadFromBacklog(replID, replOffset)
	if err != nil {
		return "", err
	}
	_, err = conn.Write(backlog)
	if err != nil {
		return "", err
	}

	// promote the pending to the replica
	c.PromotePendingToReplica(conn)
	return "", nil
}

func (c *Coordinator) sendFullResyncLine(conn net.Conn) error {
	if conn == nil {
		return fmt.Errorf("psync: nil connection")
	}
	c.mu.RLock()
	line := "FULLRESYNC " + c.replID + " " + strconv.FormatInt(c.repl_offset, 10)
	c.mu.RUnlock()
	_, err := conn.Write([]byte(resp.EncodeSimpleString(line)))
	return err
}

func sendContinue(conn net.Conn) error {
	if conn == nil {
		return fmt.Errorf("psync: nil connection")
	}
	_, err := conn.Write([]byte(resp.EncodeSimpleString("CONTINUE")))
	return err
}
// psync_replica.go — outbound PSYNC on the upstream master connection (replica role).

package replication

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/kevinnadar22/ledis/internal/resp"
)

// PerformPartialSync sends PSYNC with the current upstream id/offset and handles +CONTINUE.
// Backlog commands after CONTINUE are applied via readApplyReplicationCommand on the same reader.
func (c *Coordinator) PerformPartialSync(conn net.Conn, reader *bufio.Reader) error {
	c.mu.RLock()
	replID := c.upstream.ReplID
	offset := c.repl_offset
	c.mu.RUnlock()

	if replID == "" {
		return fmt.Errorf("partial sync: no master repl id")
	}
	if err := requestPSYNC(conn, replID, offset); err != nil {
		return err
	}

	reply, err := readPSYNCReply(reader)
	if err != nil {
		return err
	}
	if strings.HasPrefix(reply, "FULLRESYNC") {
		masterReplID, masterReplOffset, err := c.parseFullResyncReply(reply)
		if err != nil {
			return err
		}
		c.setUpstreamMaster(masterReplID, masterReplOffset)
		return c.ReadFullSyncResponse(reader, c.db)
	}
	
	if reply != "CONTINUE" {
		return fmt.Errorf("unexpected PSYNC reply: %s", reply)
	}
	for reader.Buffered() > 0 {
		if err := c.readApplyReplicationCommand(reader); err != nil {
			return err
		}
	}
	return nil
}

func requestPSYNC(conn net.Conn, replID string, offset int64) error {
	_, err := conn.Write([]byte(resp.EncodeArray([]string{
		"PSYNC", replID, strconv.FormatInt(offset, 10),
	})))
	return err
}

// readPSYNCReply returns the simple-string payload after PSYNC (e.g. FULLRESYNC … or CONTINUE).
func readPSYNCReply(reader *bufio.Reader) (string, error) {
	return resp.DecodeSimpleStringFromReader(reader)
}

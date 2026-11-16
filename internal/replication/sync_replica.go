// sync_replica.go — full resync after PSYNC FULLRESYNC (replica).

package replication

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/kevinnadar22/ledis/internal/persistence"
	"github.com/kevinnadar22/ledis/internal/store"
)

// PerformFullSync sends PSYNC ? -1, parses FULLRESYNC, and loads the RDB snapshot.
func (c *Coordinator) PerformFullSync(conn net.Conn, reader *bufio.Reader) error {
	if err := requestPSYNC(conn, "?", -1); err != nil {
		return err
	}

	fullSyncResponse, err := readPSYNCReply(reader)
	if err != nil {
		log.Printf("replication handshake error on reading full sync response: %v", err)
		return err
	}

	masterReplID, masterReplOffset, err := c.parseFullResyncReply(fullSyncResponse)
	if err != nil {
		return err
	}
	c.setUpstreamMaster(masterReplID, masterReplOffset)

	if err := c.ReadFullSyncResponse(reader, c.db); err != nil {
		log.Printf("replication handshake error on reading full sync snapshot: %v", err)
		return err
	}
	return nil
}

// ReadFullSyncResponse reads the snapshot length line and RDB bytes from the master.
func (c *Coordinator) ReadFullSyncResponse(reader *bufio.Reader, db *store.Store) error {
	header, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	header = strings.TrimSpace(header)

	length, err := strconv.Atoi(header)
	if err != nil {
		return fmt.Errorf("invalid snapshot length header %q: %w", header, err)
	}
	if length < 0 {
		return fmt.Errorf("invalid snapshot length %d", length)
	}

	body := make([]byte, length)
	if _, err = io.ReadFull(reader, body); err != nil {
		return err
	}
	entries, err := persistence.DecodeSnapshot(body)
	if err != nil {
		return err
	}
	db.RestoreSnapshot(entries)
	return nil
}

func (c *Coordinator) parseFullResyncReply(reply string) (string, int64, error) {
	parts := strings.Fields(reply)
	if len(parts) != 3 || parts[0] != "FULLRESYNC" {
		return "", 0, fmt.Errorf("invalid FULLRESYNC response: %s", reply)
	}
	offset, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid FULLRESYNC offset in %q: %w", reply, err)
	}
	return parts[1], offset, nil
}
// sync_master.go — RDB snapshot bulk send after PSYNC FULLRESYNC (master).

package replication

import (
	"fmt"
	"net"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/persistence"
)

// SendFullSyncResponse writes the snapshot length line and RDB bytes.
func (c *Coordinator) SendFullSyncResponse(conn net.Conn, entries []datatypes.RDBEntry) error {
	buf, err := persistence.EncodeSnapshot(entries, nil)
	if err != nil {
		return err
	}
	header := fmt.Sprintf("%d\r\n", buf.Len())
	if _, err = conn.Write([]byte(header)); err != nil {
		return err
	}
	if _, err = conn.Write(buf.Bytes()); err != nil {
		return err
	}
	fmt.Println("full sync response sent to replica")
	c.PromotePendingToReplica(conn)
	return nil
}
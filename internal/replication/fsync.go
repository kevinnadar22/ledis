package replication

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/persistence"
	"github.com/kevinnadar22/ledis/internal/store"
)

func (c *Coordinator) SendFullSyncResponse(conn net.Conn, entries []datatypes.RDBEntry) error {
	buf, err := persistence.EncodeSnapshot(entries, nil)
	if err != nil {
		return err
	}
	// Plain decimal length line (not RESP *array); then raw RDB bytes.
	header := fmt.Sprintf("%d\r\n", buf.Len())
	if _, err = conn.Write([]byte(header)); err != nil {
		return err
	}
	if _, err = conn.Write(buf.Bytes()); err != nil {
		return err
	}
	fmt.Println("full sync response sent to replica")
	return nil
}

func (c *Coordinator) ReadFullSyncResponse(reader *bufio.Reader, db *store.Store) error {
	// read the header
	header, err := reader.ReadString('\n')

	if err != nil {
		return err
	}
	// trim \r\n

	header = strings.TrimSpace(header)

	length, err := strconv.Atoi(header)
	if err != nil {
		return fmt.Errorf("invalid snapshot length header %q: %w", header, err)
	}
	if length < 0 {
		return fmt.Errorf("invalid snapshot length %d", length)
	}

	// read the response
	body := make([]byte, length)
	_, err = io.ReadFull(reader, body)
	if err != nil {
		return err
	}
	entries, err := persistence.DecodeSnapshot(body)
	if err != nil {
		return err
	}
	db.RestoreSnapshot(entries)
	return nil
}

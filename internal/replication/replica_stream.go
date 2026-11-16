package replication

import (
	"bufio"
	"fmt"

	"github.com/kevinnadar22/ledis/internal/resp"
)

func (c *Coordinator) applyReplicationCommandLine(cmdLine string) error {
	cmd, err := resp.Decode(cmdLine)
	if err != nil {
		return fmt.Errorf("replication stream decode: %w", err)
	}
	if c.apply != nil {
		c.apply(cmd)
	}
	c.IncrementReplOffset(cmdLine)
	return nil
}

func (c *Coordinator) readApplyReplicationCommand(reader *bufio.Reader) error {
	cmdLine, err := resp.DecodeBulkStringsArrayFromReader(reader)
	if err != nil {
		return err
	}
	return c.applyReplicationCommandLine(cmdLine)
}
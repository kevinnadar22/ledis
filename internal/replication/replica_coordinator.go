package replication

import (
	"github.com/kevinnadar22/ledis/internal/datatypes"
)

func (c *Coordinator) stopUpstreamLocked() {
	if c.upstreamCancel != nil {
		c.upstreamCancel()
		c.upstreamCancel = nil
	}
}

func (c *Coordinator) setUpstreamMaster(masterReplID string, masterReplOffset int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.upstream.ReplID = masterReplID
	c.upstream.ReplOffset = masterReplOffset
	c.Role = datatypes.RoleReplica
	c.repl_offset = masterReplOffset
}

func (c *Coordinator) IncrementReplOffset(cmdLine string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.repl_offset += int64(len(cmdLine))
}
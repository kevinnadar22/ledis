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

func (c *Coordinator) setUpstreamMaster(replID string, replStartOffset int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.upstream.ReplID = replID
	c.upstream.ReplOffset = replStartOffset
	c.Role = datatypes.RoleReplica
}

func (c *Coordinator) IncrementReplOffset(cmdLine string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.repl_offset += int64(len(cmdLine))
}
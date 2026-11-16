package replication

import (
	"context"

	"github.com/kevinnadar22/ledis/internal/datatypes"
)

func (c *Coordinator) setUpstream(host, port string, cancel context.CancelFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopUpstreamLocked()
	c.upstream.Host = host
	c.upstream.Port = port
	c.upstreamCancel = cancel
}

func (c *Coordinator) stopUpstreamLocked() {
	if c.upstreamCancel != nil {
		c.upstreamCancel()
		c.upstreamCancel = nil
	}
	if c.upstreamConn != nil {
		_ = c.upstreamConn.Close()
		c.upstreamConn = nil
	}
}

// CloseUpstreamLink closes the upstream TCP connection (e.g. simulated network drop).
func (c *Coordinator) CloseUpstreamLink() {
	c.mu.Lock()
	conn := c.upstreamConn
	c.upstreamConn = nil
	c.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
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

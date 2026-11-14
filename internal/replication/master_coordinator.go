package replication

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net"
	"time"

	"github.com/kevinnadar22/ledis/internal/datatypes"
)

func (c *Coordinator) MakeMaster() {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.replID = hex.EncodeToString(b)
	c.repl_offset = 0
	c.Role = datatypes.RoleMaster
}

func (c *Coordinator) PromotePendingToReplica(conn net.Conn) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.replicas[conn]; ok {
		return
	}
	pending, ok := c.pending[conn]
	if !ok {
		return
	}
	delete(c.pending, conn)
	c.replicas[conn] = pending
}

func (c *Coordinator) newReplicationClient(conn net.Conn, defaultPort int) *datatypes.ReplicationClient {
	r := &datatypes.ReplicationClient{
		ListeningPort: defaultPort,
		Capabilities:  make(map[string]bool),
		Queue:         make(chan datatypes.Command, 1000),
	}
	go func() {
		writer := bufio.NewWriterSize(conn, 32*1024) // 32KB buffer
		ticker := time.NewTicker(10 * time.Millisecond)

		defer ticker.Stop()

		for {
			select {
			case cmd := <-r.Queue:
				_, err := writer.Write([]byte(cmd.RawContent))
				if err != nil {
					log.Println("error writing to writer:", err)
				}
			case <-ticker.C:
				if writer.Buffered() > 0 {
					err := writer.Flush()
					if err != nil {
						log.Println("error flushing writer:", err)
					}
				}
			}
		}
	}()
	return r
}

func (c *Coordinator) getClientLocked(conn net.Conn, defaultPort int) *datatypes.ReplicationClient {
	if client, ok := c.replicas[conn]; ok {
		return client
	}
	if client, ok := c.pending[conn]; ok {
		return client
	}
	client := c.newReplicationClient(conn, defaultPort)
	c.pending[conn] = client
	return client
}

func (c *Coordinator) HandleBacklog(cmd datatypes.Command) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cmdBytes := cmd.ToBytes()
	backlog := &c.repl_backlog

	for _, b := range cmdBytes {
		backlog.buf[backlog.ringWriteIndex] = b
		backlog.ringWriteIndex = (backlog.ringWriteIndex + 1) % backlog.capacity
		c.repl_offset++
	}
	if c.repl_offset >= backlog.capacity {
		backlog.oldestReplOffset = c.repl_offset - backlog.capacity
	}
}

func (c *Coordinator) canServeFromBacklogLocked(offset int64) bool {
	return offset >= 0 &&
		offset >= c.repl_backlog.oldestReplOffset &&
		offset < c.repl_offset
}

func (c *Coordinator) CanServeFromBacklog(offset int64) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.canServeFromBacklogLocked(offset)
}

func (c *Coordinator) ReadFromBacklog(offset int64) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.canServeFromBacklogLocked(offset) {
		return nil, errors.New("offset is not in backlog")
	}

	if c.repl_offset < c.repl_backlog.capacity {
		// Ring not wrapped: stream offset equals physical index for data so far.
		return c.repl_backlog.buf[offset:c.repl_offset], nil
	}

	response := make([]byte, 0, c.repl_offset-offset)
	for streamOff := offset; streamOff < c.repl_offset; streamOff++ {
		phys := c.repl_backlog.physicalIndex(streamOff)
		response = append(response, c.repl_backlog.buf[phys])
	}
	return response, nil
}

func (c *Coordinator) BroadcastCommandToReplicas(cmd datatypes.Command) {
	c.mu.Lock()
	clients := make([]*datatypes.ReplicationClient, 0, len(c.replicas))
	for _, client := range c.replicas {
		clients = append(clients, client)
	}
	c.mu.Unlock()
	for _, client := range clients {
		client.Queue <- cmd
	}
	c.HandleBacklog(cmd)
}

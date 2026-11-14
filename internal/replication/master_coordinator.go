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
	c.appendToBacklogLocked(cmd)
}

func (c *Coordinator) appendToBacklogLocked(cmd datatypes.Command) {
	cmdBytes := cmd.ToBytes()
	backlog := &c.repl_backlog

	for _, b := range cmdBytes {
		backlog.buf[backlog.ringWriteIndex] = b // physical write
		backlog.ringWriteIndex = (backlog.ringWriteIndex + 1) % backlog.capacity
		c.repl_offset++ // advance replication stream
	}

	if c.repl_offset >= backlog.capacity {
		backlog.oldestReplOffset = c.repl_offset - backlog.capacity
	}
}

// canServeFromBacklogLocked checks a replication stream offset (PSYNC offset), not a buf index.
// offset == repl_offset means caught up (CONTINUE, zero backlog bytes).
func (c *Coordinator) canServeFromBacklogLocked(streamOffset int64) bool {
	return streamOffset >= c.repl_backlog.oldestReplOffset &&
		streamOffset <= c.repl_offset
}

func (c *Coordinator) CanServeFromBacklog(offset int64) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.canServeFromBacklogLocked(offset)
}

// ReadFromBacklog reads stream bytes [startOffset, repl_offset). startOffset is a stream offset.
func (c *Coordinator) ReadFromBacklog(startOffset int64) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.canServeFromBacklogLocked(startOffset) {
		return nil, errors.New("offset is not in backlog")
	}
	response := make([]byte, 0, c.repl_offset-startOffset)
	for streamOff := startOffset; streamOff < c.repl_offset; streamOff++ {
		bufIndex := c.repl_backlog.physicalIndex(streamOff)
		response = append(response, c.repl_backlog.buf[bufIndex])
	}
	return response, nil
}

func (c *Coordinator) disconnectReplicaLocked(conn net.Conn) {
	if conn == nil {
		return
	}
	addr := conn.RemoteAddr()
	delete(c.replicas, conn)
	delete(c.pending, conn)
	if err := conn.Close(); err != nil {
		log.Printf("replication: close replica %v: %v", addr, err)
		return
	}
	log.Printf("replication: disconnected slow replica %s (output buffer full)", addr)
}

func (c *Coordinator) BroadcastCommandToReplicas(cmd datatypes.Command) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.appendToBacklogLocked(cmd)
	for conn, client := range c.replicas {
		select {
		case client.Queue <- cmd:
		default:
			c.disconnectReplicaLocked(conn)
		}
	}
}

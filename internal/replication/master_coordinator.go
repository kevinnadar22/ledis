package replication

import (
	"bufio"
	"context"
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
	ctx, cancel := context.WithCancel(context.Background())
	r := &datatypes.ReplicationClient{
		ListeningPort: defaultPort,
		Capabilities:  make(map[string]bool),
		Queue:         make(chan datatypes.Command, 1000),
		Ctx:           ctx,
		Cancel:        cancel,
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
					go c.DisconnectReplica(conn)
					return
				}
			case <-r.Ctx.Done():
				return
			case <-ticker.C:
				if writer.Buffered() > 0 {
					err := writer.Flush()
					if err != nil {
						log.Println("error flushing writer:", err)
						go c.DisconnectReplica(conn)
						return
					}
				}
			}
		}
	}()
	return r
}

func (c *Coordinator) GetClient(conn net.Conn, defaultPort int) *datatypes.ReplicationClient {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.getClientLocked(conn, defaultPort)
}

// getClientLocked returns or creates a replication client; caller must hold c.mu.
func (c *Coordinator) getClientLocked(conn net.Conn, defaultPort int) *datatypes.ReplicationClient {
	if client := c.lookupClientLocked(conn); client != nil {
		return client
	}
	client := c.newReplicationClient(conn, defaultPort)
	c.pending[conn] = client
	return client
}

// lookupClientLocked returns a client if registered; caller must hold c.mu.
func (c *Coordinator) lookupClientLocked(conn net.Conn) *datatypes.ReplicationClient {
	if client, ok := c.replicas[conn]; ok {
		return client
	}
	if client, ok := c.pending[conn]; ok {
		return client
	}
	return nil
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

// disconnectReplicaLocked drops replica state; caller must hold c.mu. Does not close TCP.
func (c *Coordinator) disconnectReplicaLocked(conn net.Conn, client *datatypes.ReplicationClient) bool {
	if conn == nil {
		return false
	}
	if client == nil {
		client = c.lookupClientLocked(conn)
	}
	if client == nil {
		return false
	}
	addr := conn.RemoteAddr()
	client.Cancel()
	delete(c.replicas, conn)
	delete(c.pending, conn)
	log.Printf("replication: disconnected slow replica %s (output buffer full)", addr)
	return true
}

func (c *Coordinator) closeReplicaTCP(conn net.Conn) {
	if conn == nil {
		return
	}
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetLinger(0)
	}
	addr := conn.RemoteAddr()
	if err := conn.Close(); err != nil {
		log.Printf("replication: close replica %v: %v", addr, err)
	}
}

// DisconnectReplica acquires c.mu, drops state, unlocks, then closes TCP (do not close under c.mu).
func (c *Coordinator) DisconnectReplica(conn net.Conn) {
	c.mu.Lock()
	removed := c.disconnectReplicaLocked(conn, nil)
	c.mu.Unlock()
	if removed {
		c.closeReplicaTCP(conn)
	}
}

func (c *Coordinator) BroadcastCommandToReplicas(cmd datatypes.Command) {
	c.mu.Lock()
	c.appendToBacklogLocked(cmd)

	var toClose []net.Conn
	for conn, client := range c.replicas {
		select {
		case client.Queue <- cmd:
		default:
			if c.disconnectReplicaLocked(conn, client) {
				toClose = append(toClose, conn)
			}
		}
	}
	c.mu.Unlock()

	for _, conn := range toClose {
		c.closeReplicaTCP(conn)
	}
}
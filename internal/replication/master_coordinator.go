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
					go c.DisconnectReplica(conn, true, nil)
					return
				}
			case <-r.Ctx.Done():
				return
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

// GetClient returns the replication client for conn. lock acquires c.mu when true.
// create registers a new client in pending when absent; if false, returns nil when absent.
func (c *Coordinator) GetClient(conn net.Conn, defaultPort int, lock, create bool) *datatypes.ReplicationClient {
	if lock {
		c.mu.Lock()
		defer c.mu.Unlock()
	}
	if client, ok := c.replicas[conn]; ok {
		return client
	}
	if client, ok := c.pending[conn]; ok {
		return client
	}
	if !create {
		return nil
	}
	client := c.newReplicationClient(conn, defaultPort)
	c.pending[conn] = client
	return client
}

func (c *Coordinator) HandleBacklog(cmd datatypes.Command) {
	c.AppendToBacklog(cmd, true)
}

func (c *Coordinator) AppendToBacklog(cmd datatypes.Command, lock bool) {
	if lock {
		c.mu.Lock()
		defer c.mu.Unlock()
	}
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

// CanServeFromBacklog checks a replication stream offset (PSYNC offset), not a buf index.
// offset == repl_offset means caught up (CONTINUE, zero backlog bytes).
func (c *Coordinator) CanServeFromBacklog(offset int64, lock bool) bool {
	if lock {
		c.mu.RLock()
		defer c.mu.RUnlock()
	}
	return offset >= c.repl_backlog.oldestReplOffset &&
		offset <= c.repl_offset
}

// ReadFromBacklog reads stream bytes [startOffset, repl_offset). startOffset is a stream offset.
func (c *Coordinator) ReadFromBacklog(startOffset int64) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.CanServeFromBacklog(startOffset, false) {
		return nil, errors.New("offset is not in backlog")
	}
	response := make([]byte, 0, c.repl_offset-startOffset)
	for streamOff := startOffset; streamOff < c.repl_offset; streamOff++ {
		bufIndex := c.repl_backlog.physicalIndex(streamOff)
		response = append(response, c.repl_backlog.buf[bufIndex])
	}
	return response, nil
}

// DisconnectReplica removes a replica from master state. client may be nil (lookup only, no create).
// lock==false: caller holds c.mu; TCP close is the caller's job after unlock.
// lock==true: acquires c.mu, updates state, unlocks, then closes TCP (SetLinger(0)).
// Returns whether the conn was registered and removed.
func (c *Coordinator) DisconnectReplica(conn net.Conn, lock bool, client *datatypes.ReplicationClient) bool {
	if lock {
		c.mu.Lock()
	}
	if conn == nil {
		if lock {
			c.mu.Unlock()
		}
		return false
	}
	if client == nil {
		client = c.GetClient(conn, 0, false, false)
	}
	if client == nil {
		if lock {
			c.mu.Unlock()
		}
		return false
	}
	addr := conn.RemoteAddr()
	client.Cancel()
	delete(c.replicas, conn)
	delete(c.pending, conn)
	log.Printf("replication: disconnected slow replica %s (output buffer full)", addr)

	if lock {
		c.mu.Unlock()
		if tc, ok := conn.(*net.TCPConn); ok {
			_ = tc.SetLinger(0)
		}
		if err := conn.Close(); err != nil {
			log.Printf("replication: close replica %v: %v", addr, err)
		}
	}
	return true
}

func (c *Coordinator) BroadcastCommandToReplicas(cmd datatypes.Command) {
	c.mu.Lock()
	c.AppendToBacklog(cmd, false)

	var toClose []net.Conn
	for conn, client := range c.replicas {
		select {
		case client.Queue <- cmd:
		default:
			if c.DisconnectReplica(conn, false, client) {
				toClose = append(toClose, conn)
			}
		}
	}
	c.mu.Unlock()

	for _, conn := range toClose {
		if tc, ok := conn.(*net.TCPConn); ok {
			_ = tc.SetLinger(0)
		}
		addr := conn.RemoteAddr()
		if err := conn.Close(); err != nil {
			log.Printf("replication: close replica %v: %v", addr, err)
		}
	}
}
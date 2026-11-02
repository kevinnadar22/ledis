package replication

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
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
	c.replStartOffset = 0
	c.replOffset = 0
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
				writer.Write([]byte(cmd.RawContent))
			case <-ticker.C:
				if writer.Buffered() > 0 {
					writer.Flush()
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

func (c *Coordinator) BroadcastCommandToReplicas(cmd datatypes.Command) {
	c.mu.RLock()
	clients := make([]*datatypes.ReplicationClient, 0, len(c.replicas))
	for _, client := range c.replicas {
		clients = append(clients, client)
	}
	c.mu.RUnlock()
	for _, client := range clients {
		client.Queue <- cmd
	}
}
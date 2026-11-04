package replication

import (
	"context"
	"net"
	"sync"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/store"
)

type CommandApplier func(cmd datatypes.Command)

// Coordinator holds master-side and replica-side replication state for one server.
type Coordinator struct {
	mu sync.RWMutex

	Role datatypes.Role

	// Master: identity + downstream replicas.
	replID                         string
	replicas                       map[net.Conn]*datatypes.ReplicationClient
	pending                        map[net.Conn]*datatypes.ReplicationClient
	repl_backlog_first_byte_offset int64

	// Replica: upstream link.
	upstreamConn   net.Conn
	upstreamCancel context.CancelFunc
	upstream       datatypes.UpstreamMaster

	// offsets maintained by all servers, upstream and downstream
	repl_offset int64

	listenPort int
	db         *store.Store
	apply      CommandApplier
}

func New(listenPort int, db *store.Store, apply CommandApplier) *Coordinator {
	return &Coordinator{
		replicas:   make(map[net.Conn]*datatypes.ReplicationClient),
		pending:    make(map[net.Conn]*datatypes.ReplicationClient),
		listenPort: listenPort,
		db:         db,
		apply:      apply,
	}
}

func (c *Coordinator) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopUpstreamLocked()
}

func (c *Coordinator) ReplOffset() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.repl_offset
}

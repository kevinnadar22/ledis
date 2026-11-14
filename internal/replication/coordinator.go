package replication

import (
	"context"
	"net"
	"sync"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/store"
)

type CommandApplier func(cmd datatypes.Command)

// ReplBacklog is a fixed-size ring of raw replication bytes plus stream bounds.
type ReplBacklog struct {
	// Replication stream (logical byte positions on the master).
	oldestReplOffset int64 // oldest stream offset still present in the ring

	// Physical ring buffer (indices 0 .. capacity-1).
	ringWriteIndex int64 // next buf slot to write
	capacity       int64
	buf            []byte
}

func (b *ReplBacklog) physicalIndex(streamOffset int64) int64 {
	return streamOffset % b.capacity
}

// Coordinator holds master-side and replica-side replication state for one server.
type Coordinator struct {
	mu sync.RWMutex

	Role datatypes.Role

	// Master: identity + downstream replicas.
	replID   string
	replicas map[net.Conn]*datatypes.ReplicationClient
	pending  map[net.Conn]*datatypes.ReplicationClient

	// Master: backlog
	repl_backlog ReplBacklog

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

func New(listenPort int, db *store.Store, apply CommandApplier, replBacklogSize int64) *Coordinator {
	return &Coordinator{
		replicas:     make(map[net.Conn]*datatypes.ReplicationClient),
		pending:      make(map[net.Conn]*datatypes.ReplicationClient),
		listenPort:   listenPort,
		db:           db,
		apply:        apply,
		repl_backlog: ReplBacklog{
			buf:              make([]byte, replBacklogSize),
			capacity:         replBacklogSize,
			ringWriteIndex:   0,
			oldestReplOffset: 0,
		},
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

func (c *Coordinator) OldestReplOffset() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.repl_backlog.oldestReplOffset
}

// replBacklogHistLen is bytes retained in the ring [oldestReplOffset, repl_offset).
func (c *Coordinator) replBacklogHistLenLocked() int64 {
	if c.repl_offset <= c.repl_backlog.oldestReplOffset {
		return 0
	}
	return c.repl_offset - c.repl_backlog.oldestReplOffset
}

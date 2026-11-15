package replication

import (
	"context"
	"net"
	"sync"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/store"
)

type CommandApplier func(cmd datatypes.Command)

// ReplBacklog stores replication bytes in a ring. Two coordinate systems:
//   - Stream offsets: monotonic positions in the master's byte stream (0, 1, 2, …).
//   - Physical indices: slots in buf, always in [0, capacity).
//
// Before the ring wraps, stream offset often equals buf index; after wrap use streamOffset % capacity.
type ReplBacklog struct {
	oldestReplOffset int64 // stream: oldest byte still kept in the ring
	ringWriteIndex   int64 // physical: next buf slot to write
	capacity         int64 // physical: len(buf)
	buf              []byte
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

	// repl_offset is a replication stream offset (not a buf index): next byte position
	// the master will assign; also the replica's processed offset when caught up.
	repl_offset int64

	listenPort int
	db         *store.Store
	apply      CommandApplier
}

// physicalIndex maps a replication stream offset to a ring buffer slot.
func (b *ReplBacklog) physicalIndex(streamOffset int64) int64 {
	return streamOffset % b.capacity
}

func New(listenPort int, db *store.Store, apply CommandApplier, replBacklogSize int64) *Coordinator {
	return &Coordinator{
		replicas:   make(map[net.Conn]*datatypes.ReplicationClient),
		pending:    make(map[net.Conn]*datatypes.ReplicationClient),
		listenPort: listenPort,
		db:         db,
		apply:      apply,
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
	c.StopUpstream(false)
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

// ReplBacklogHistLen is stream bytes retained: repl_offset - oldestReplOffset.
func (c *Coordinator) ReplBacklogHistLen(lock bool) int64 {
	if lock {
		c.mu.RLock()
		defer c.mu.RUnlock()
	}
	if c.repl_offset <= c.repl_backlog.oldestReplOffset {
		return 0
	}
	return c.repl_offset - c.repl_backlog.oldestReplOffset
}

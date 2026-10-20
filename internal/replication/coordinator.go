package replication

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"sync"

	"github.com/kevinnadar22/ledis/internal/store"
)

type Client struct {
	Conn          net.Conn
	ListeningPort int
	Capabilities  map[string]bool
}

type Master struct {
	Conn   net.Conn
	Cancel context.CancelFunc
	// save replication offset and id
	replID     string
	replOffset int64
}

type Coordinator struct {
	mu sync.Mutex

	// This server's own replication identity and history.
	replID          string
	replOffset      int64
	replStartOffset int64

	// master holds a list of replicas
	replicas map[net.Conn]*Client
	pending  map[net.Conn]*Client

	// Upstream master, only when this server is a replica.
	master *Master

	listenPort int
	db         *store.Store
}

func New(listenPort int, db *store.Store) *Coordinator {
	return &Coordinator{
		replicas:   make(map[net.Conn]*Client),
		pending:    make(map[net.Conn]*Client),
		listenPort: listenPort,
		master:     &Master{},
		db:         db,
	}
}

func (c *Coordinator) stopLocked() {
	if c.master.Cancel != nil {
		c.master.Cancel()
		c.master.Cancel = nil
	}
}

func (c *Coordinator) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopLocked()
}

func (c *Coordinator) MakeMaster() {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	replID := hex.EncodeToString(b)
	replStartOffset := int64(0)
	replOffset := int64(0)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.replID = replID
	c.replStartOffset = replStartOffset
	c.replOffset = replOffset
}

package replication

import (
	"context"
	"net"
	"sync"
)

type Client struct {
	Conn          net.Conn
	ListeningPort int
	Capabilities  map[string]bool
}

type Coordinator struct {
	mu           sync.Mutex
	replicas     map[net.Conn]*Client
	pending      map[net.Conn]*Client
	masterCancel context.CancelFunc
	listenPort   int
}

func New(listenPort int) *Coordinator {
	return &Coordinator{
		replicas:   make(map[net.Conn]*Client),
		pending:    make(map[net.Conn]*Client),
		listenPort: listenPort,
	}
}

func (c *Coordinator) stopLocked() {
	if c.masterCancel != nil {
		c.masterCancel()
		c.masterCancel = nil
	}
}

func (c *Coordinator) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopLocked()
}

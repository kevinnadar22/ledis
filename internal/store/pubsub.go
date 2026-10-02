package store

import (
	"net"
	"sync"
    "github.com/kevinnadar22/ledis/internal/resp"
)

type Client struct {
    Conn     net.Conn
    Outgoing chan string
}

type PubSub struct {
    mu sync.RWMutex

    subscriptions map[string]map[*Client]struct{}
}

func NewPubSub() *PubSub {
    return &PubSub{
        subscriptions: make(map[string]map[*Client]struct{}),
    }
}

func (ps *PubSub) Subscribe(client *Client, topic string) {
    ps.mu.Lock()
    defer ps.mu.Unlock()
    if _, ok := ps.subscriptions[topic]; !ok {
        ps.subscriptions[topic] = make(map[*Client]struct{})
    }
    ps.subscriptions[topic][client] = struct{}{}
}

func (ps *PubSub) Unsubscribe(client *Client, topic string) {
    ps.mu.Lock()
    defer ps.mu.Unlock()
    delete(ps.subscriptions[topic], client)
    if len(ps.subscriptions[topic]) == 0 {
        delete(ps.subscriptions, topic)
    }
}

func (ps *PubSub) Publish(topic string, message string) {
    ps.mu.RLock()
    defer ps.mu.RUnlock()
    messageArray := []string{"message", topic, message}
    for client := range ps.subscriptions[topic] {
        client.Outgoing <- resp.EncodeArray(messageArray)
    }
}


func (c *Client) WriteLoop() {
	for message := range c.Outgoing {
        c.Conn.Write([]byte(message))
    }
}

func (ps *PubSub) RemoveClient(client *Client) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for topic, clients := range ps.subscriptions {
		delete(clients, client)
		if len(clients) == 0 {
			delete(ps.subscriptions, topic)
		}
	}
}

func (ps *PubSub) GetSubscriptionCountbyClient(clientToCheck *Client) int {
    ps.mu.RLock()
    defer ps.mu.RUnlock()
    count := 0
    for _, clients := range ps.subscriptions {
        if _, exists := clients[clientToCheck]; exists {
            count++
        }
    }
    return count
}

func (ps *PubSub) GetSubscriptionCountbyTopic(topic string) int {
    ps.mu.RLock()
    defer ps.mu.RUnlock()
    return len(ps.subscriptions[topic])
}
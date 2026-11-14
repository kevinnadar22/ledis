package replication

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/kevinnadar22/ledis/internal/datatypes"
)

// InfoReplication returns the Redis-style "# Replication" section using only state Ledis tracks today.
func (c *Coordinator) InfoReplication() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var b strings.Builder
	b.WriteString("# Replication\r\n")

	if c.Role == datatypes.RoleReplica {
		b.WriteString("role:slave\r\n")
		if c.upstream.Host != "" {
			b.WriteString("master_host:")
			b.WriteString(c.upstream.Host)
			b.WriteString("\r\n")
		}
		if c.upstream.Port != "" {
			b.WriteString("master_port:")
			b.WriteString(c.upstream.Port)
			b.WriteString("\r\n")
		}
		link := "down"
		if c.upstreamConn != nil {
			link = "up"
		}
		b.WriteString("master_link_status:")
		b.WriteString(link)
		b.WriteString("\r\n")
		if c.upstream.ReplID != "" {
			b.WriteString("master_replid:")
			b.WriteString(c.upstream.ReplID)
			b.WriteString("\r\n")
		}
		b.WriteString("slave_repl_offset:")
		b.WriteString(strconv.FormatInt(c.repl_offset, 10))
		b.WriteString("\r\n")
		return b.String()
	}

	b.WriteString("role:master\r\n")
	if c.replID != "" {
		b.WriteString("master_replid:")
		b.WriteString(c.replID)
		b.WriteString("\r\n")
	}
	b.WriteString("master_repl_offset:")
	b.WriteString(strconv.FormatInt(c.repl_offset, 10))
	b.WriteString("\r\n")
	b.WriteString("repl_backlog_first_byte_offset:")
	b.WriteString(strconv.FormatInt(c.repl_backlog_first_byte_offset, 10))
	b.WriteString("\r\n")
	b.WriteString("connected_slaves:")
	b.WriteString(strconv.Itoa(len(c.replicas)))
	b.WriteString("\r\n")

	i := 0
	for conn, client := range c.replicas {
		host, _, err := net.SplitHostPort(conn.RemoteAddr().String())
		if err != nil {
			host = conn.RemoteAddr().String()
		}
		port := client.ListeningPort
		if port == 0 {
			if _, portStr, err := net.SplitHostPort(conn.RemoteAddr().String()); err == nil {
				port, _ = strconv.Atoi(portStr)
			}
		}
		fmt.Fprintf(&b, "slave%d:ip=%s,port=%d\r\n", i, host, port)
		i++
	}

	return b.String()
}
package replication

import (
	"net"
	"strconv"
	"strings"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/resp"
)

func (c *Coordinator) ReplConf(conn net.Conn, defaultPort int, cmd datatypes.Command) (string, error) {
	if len(cmd.Args) != 2 {
		return resp.EncodeError("wrong number of arguments for 'repl conf' command"), nil
	}
	if conn == nil {
		return resp.EncodeError("replconf requires a connection"), nil
	}

	option := strings.ToLower(cmd.Args[0].String())
	value := cmd.Args[1].String()

	c.mu.Lock()
	defer c.mu.Unlock()

	client, ok := c.replicas[conn]
	if !ok {
		client, ok = c.pending[conn]
		if !ok {
			client = &Client{
				Conn:          conn,
				ListeningPort: defaultPort,
				Capabilities:  make(map[string]bool),
			}
			c.pending[conn] = client
		}
	}

	switch option {
	case "listening-port":
		port, err := strconv.Atoi(value)
		if err != nil || port <= 0 || port > 65535 {
			return resp.EncodeError("invalid value for 'listening-port' option"), nil
		}
		client.ListeningPort = port
	case "capa":
		client.Capabilities[strings.ToLower(value)] = true
	default:
		return resp.EncodeError("invalid option for 'repl conf' command"), nil
	}

	return resp.EncodeSimpleString("OK"), nil
}

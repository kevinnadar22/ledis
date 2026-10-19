package replication

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"strconv"

	"github.com/kevinnadar22/ledis/internal/resp"
)

func (c *Coordinator) Psync(conn net.Conn) (string, error) {
	// generate 40 random bytes
	randomBytes := make([]byte, 40)
	rand.Read(randomBytes)
	replID := hex.EncodeToString(randomBytes)
	return resp.EncodeSimpleString("PSYNC " + replID + " " + strconv.FormatInt(0, 10)), nil
}
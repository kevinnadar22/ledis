package main

import (
	"fmt"
	"math"
	"math/rand"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/store"
)

func waitForStoresEqual(t *testing.T, master, replica *store.Store, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := compareStores(master, replica); err == nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err := compareStores(master, replica); err != nil {
		t.Fatal(err)
	}
}

func compareStores(master, replica *store.Store) error {
	mEntries, err := master.SaveSnapshot()
	if err != nil {
		return err
	}
	rEntries, err := replica.SaveSnapshot()
	if err != nil {
		return err
	}

	m := make(map[string]datatypes.RDBEntry, len(mEntries))
	for _, e := range mEntries {
		m[e.Key] = e
	}
	r := make(map[string]datatypes.RDBEntry, len(rEntries))
	for _, e := range rEntries {
		r[e.Key] = e
	}

	if len(m) != len(r) {
		return fmt.Errorf("key count: master=%d replica=%d", len(m), len(r))
	}

	const expSlackMs = int64(10_000) // expires are UnixMilli; allow small replication skew
	for key, me := range m {
		re, ok := r[key]
		if !ok {
			return fmt.Errorf("replica missing key %q", key)
		}
		if me.Value != re.Value {
			return fmt.Errorf("key %q value master=%q replica=%q", key, me.Value, re.Value)
		}
		switch {
		case me.Expiration == nil && re.Expiration == nil:
		case me.Expiration != nil && re.Expiration != nil:
			if math.Abs(float64(*me.Expiration-*re.Expiration)) > float64(expSlackMs) {
				return fmt.Errorf("key %q expiry master=%d replica=%d", key, *me.Expiration, *re.Expiration)
			}
		default:
			return fmt.Errorf("key %q expiry presence mismatch", key)
		}
	}
	return nil
}

func TestReplicationMasterReplicaMixedCommands(t *testing.T) {
	master := startTCPLedis(t, true)
	replica := startTCPLedis(t, false)

	host, port, err := net.SplitHostPort(master.Addr)
	if err != nil {
		t.Fatalf("split master addr: %v", err)
	}

	admin := dialRedis(t, replica.Addr)
	defer admin.Close()
	if got := admin.Do("REPLICAOF", host, port); got != "+OK\r\n" {
		t.Fatalf("REPLICAOF: got %q", got)
	}

	time.Sleep(300 * time.Millisecond)

	client := dialRedis(t, master.Addr)
	defer client.Close()

	rng := rand.New(rand.NewSource(42))
	const ops = 100
	counterKey := "repl:counter"

	for i := 0; i < ops; i++ {
		key := fmt.Sprintf("repl:key:%03d", i%50)

		switch i % 10 {
		case 0, 1, 2:
			val := fmt.Sprintf("v%d-%d", i, rng.Intn(1000))
			if got := client.Do("SET", key, val); got != "+OK\r\n" {
				t.Fatalf("SET %s: got %q", key, got)
			}
		case 3:
			val := fmt.Sprintf("ex-%d", i)
			if got := client.Do("SET", key, val, "EX", "3600"); got != "+OK\r\n" {
				t.Fatalf("SET EX %s: got %q", key, got)
			}
		case 4:
			if got := client.Do("DEL", key); !strings.HasPrefix(got, ":") {
				t.Fatalf("DEL %s: got %q", key, got)
			}
		case 5:
			if got := client.Do("INCR", counterKey); !strings.HasPrefix(got, ":") {
				t.Fatalf("INCR %s: got %q", counterKey, got)
			}
		case 6:
			if got := client.Do("SET", key+"-nx", "only-if-new", "NX"); got != "+OK\r\n" && got != "$-1\r\n" {
				t.Fatalf("SET NX: got %q", got)
			}
		case 7:
			_ = client.Do("SET", key+"-xx-base", "base")
			if got := client.Do("SET", key+"-xx-base", "updated", "XX"); got != "+OK\r\n" {
				t.Fatalf("SET XX: got %q", got)
			}
		case 8:
			txKey := fmt.Sprintf("repl:tx:%03d", i)
			if got := client.Do("MULTI"); got != "+OK\r\n" {
				t.Fatalf("MULTI: got %q", got)
			}
			if got := client.Do("SET", txKey, "a"); got != "+QUEUED\r\n" {
				t.Fatalf("SET in MULTI: got %q", got)
			}
			if got := client.Do("INCR", counterKey); got != "+QUEUED\r\n" {
				t.Fatalf("INCR in MULTI: got %q", got)
			}
			if got := client.Do("DEL", key); got != "+QUEUED\r\n" {
				t.Fatalf("DEL in MULTI: got %q", got)
			}
			if got := client.Do("EXEC"); !strings.HasPrefix(got, "*") {
				t.Fatalf("EXEC: got %q", got)
			}
		case 9:
			if got := client.Do("DEL", key, key+"-nx", key+"-xx-base"); !strings.HasPrefix(got, ":") {
				t.Fatalf("DEL multi-key: got %q", got)
			}
		}
	}

	waitForStoresEqual(t, master.Srv.DB(), replica.Srv.DB(), 5*time.Second)

	if v, ok := master.Srv.DB().Get(counterKey); !ok {
		t.Fatalf("master missing %s", counterKey)
	} else if rv, ok := replica.Srv.DB().Get(counterKey); !ok || rv != v {
		t.Fatalf("counter mismatch master=%q replica=%q", v, rv)
	}

	mTTL := master.Srv.DB().TTL("repl:key:003")
	rTTL := replica.Srv.DB().TTL("repl:key:003")
	if mTTL >= 0 && rTTL >= 0 && abs64(mTTL-rTTL) > 3 {
		t.Fatalf("TTL drift key repl:key:003 master=%d replica=%d", mTTL, rTTL)
	}
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

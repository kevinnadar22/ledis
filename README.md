# Ledis

**Ledis** is a from-scratch, Redis-inspired key–value server written in Go. It is structured as a **10-level roadmap**: each level adds real production concepts (wire protocol, durability, data structures, distribution) instead of jumping straight to a full clone.

Clients talk to Ledis over **TCP** using **[RESP](https://redis.io/docs/reference/protocol-spec/)** (Redis Serialization Protocol)—the same framing `redis-cli` uses—so you can build and debug with familiar tools while owning every layer of the stack.


**Current progress:** ✅ **Level 5** complete (pub/sub + transactions with `WATCH`). **Level 4** and **6–10** are next.

See the roadmap table for the full picture.

## Roadmap

| Level | Topic | Status |
|------:|-------|--------|
| 1 | TCP server, RESP parser, `PING`, `ECHO` | ✅ Done |
| 2 | `SET`, `GET`, `DEL`, `EXISTS` | ✅ Done |
| 3 | Expiration, `TTL`, persistence (AOF) | ✅ Done |
| 4 | Lists, hashes, sets | ⬜ Not started |
| 5 | Pub/Sub, transactions | ✅ Done |
| 6 | Replication | ⬜ Not started |
| 7 | Streams | ⬜ Not started |
| 8 | Memory optimization | ⬜ Not started |
| 9 | Cluster, sharding | ⬜ Not started |
| 10 | Leader election, consensus | ⬜ Not started |

### Implemented commands (so far)

| Command | Notes |
|---------|--------|
| `PING`, `ECHO` | Level 1 |
| `GET`, `SET`, `DEL`, `EXISTS`, `INCR`, `FLUSHALL` | Level 2 (+ extras) |
| `TTL`, AOF replay | Level 3 |
| `SAVE`, `BGSAVE` | RDB snapshots |
| `SUBSCRIBE`, `UNSUBSCRIBE`, `PUBLISH` | Level 5 (pub/sub) |
| `MULTI`, `EXEC`, `DISCARD`, `WATCH`, `UNWATCH` | Level 5 (transactions) |

## Requirements

- [Go](https://go.dev/dl/) 1.25+ (see `go.mod`)

## Build

From the repository root:

```bash
go build -o ledis ./cmd/server
```

Or build without writing a binary:

```bash
go build ./...
```

## Run

Start the server (default listen address **`:7379`**):

```bash
go run ./cmd/server
```

With flags (parsed in `internal/config`):

```bash
go run ./cmd/server -appendonly=true -appendfsync=everysec -rdb=./dump.rdb
```

| Flag | Values | Default |
|------|--------|---------|
| `-appendonly` | `true` / `false` | `true` |
| `-appendfsync` | `no`, `everysec`, `always` | `everysec` |
| `-rdb` | path to RDB file | `""` (optional) |

When AOF is enabled, commands are appended to `./appendonly.aof` in the server working directory.

### Try it with redis-cli

```bash
redis-cli -h 127.0.0.1 -p 7379 PING
redis-cli -h 127.0.0.1 -p 7379 SET foo bar
redis-cli -h 127.0.0.1 -p 7379 GET foo
```

Pub/Sub (use two terminals):

```bash
# Terminal 1
redis-cli -h 127.0.0.1 -p 7379 SUBSCRIBE orders

# Terminal 2
redis-cli -h 127.0.0.1 -p 7379 PUBLISH orders hello
```

### Live reload (optional)

If you use [Air](https://github.com/air-verse/air):

```bash
air
```

Configuration is in `.air.toml` (builds `./cmd/server`).

## Test

Integration-style command tests live under `cmd/test`:

```bash
go test ./cmd/test/...
```

Transaction concurrency (race detector):

```bash
go test -race ./cmd/test/... -run TestMultiExecRace
```

Run all packages:

```bash
go test ./...
```

## Benchmarking vs Redis

> **Not a production benchmark.** One localhost run (`-n 1000000 -c 50`, `ping_mbulk,set,get,incr`). Ledis had **`-appendonly=false`**; Redis used default **6379**. Results vary by machine and load—see **[docs/benchmarking.md](docs/benchmarking.md)** for commands, caveats, and interview notes.

| Test (`redis-benchmark`) | Redis `:6379` (req/s) | Ledis `:7379` (req/s) |
|--------------------------|----------------------:|----------------------:|
| `ping_mbulk` (RESP PING) | ~59,700 | ~77,400 |
| `set` | ~61,500 | ~56,300 |
| `get` | ~63,700 | ~77,200 |
| `incr` | ~63,300 | ~79,200 |

Ledis uses a **single command worker** (global serialization); Redis is a full production server—treat this as a smoke test, not a winner/loser scoreboard.

```bash
go run ./cmd/server -appendonly=false
redis-benchmark -h 127.0.0.1 -p 6379 -t ping_mbulk,set,get,incr -n 1000000 -c 50
redis-benchmark -h 127.0.0.1 -p 7379 -t ping_mbulk,set,get,incr -n 1000000 -c 50
```

## Layout

```
cmd/server/     TCP listener and connection loop
cmd/test/       Command tests
internal/commands/  Session-based command handlers
internal/resp/      RESP encode/decode
internal/store/     In-memory KV + pub/sub
internal/persistence/  AOF and RDB
docs/           Notes (e.g. pub/sub, sockets)
```

## License

[MIT](LICENSE)

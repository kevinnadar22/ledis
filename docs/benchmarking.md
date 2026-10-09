# Benchmarking Ledis vs Redis

> **Disclaimer:** These numbers are **informal, local-machine smoke tests**—not a production benchmark report. They compare a learning server (Ledis) to [Redis](https://redis.io/) under a narrow workload. Use them to talk about **architecture tradeoffs**, not to claim Ledis is “faster than Redis.”

## Run configuration

| Setting | Redis | Ledis |
|---------|--------|--------|
| Host / port | `127.0.0.1:6379` | `127.0.0.1:7379` |
| Server start | `redis-server` (default) | `go run ./cmd/server -appendonly=false` |
| Persistence during test | Install default (often RDB/AOF on Redis) | **AOF off** (no `appendonly.aof` writes) |
| Tool | `redis-benchmark` | same |
| Requests | `-n 1000000` | same |
| Parallel clients | `-c 50` | same |
| Tests | `-t ping_mbulk,set,get,incr` | same |
| Keep-alive | default (on) | same |

> **Ledis default is `-appendonly=true`.** These numbers intentionally disable AOF so the comparison stresses **RESP + command handling**, not append/fsync. Re-run with AOF on for durability-realistic throughput.

**PING:** use **`ping_mbulk`** only. Ledis speaks RESP arrays; `ping_inline` can close the connection.

## Detailed results (one localhost run)

Throughput and p50 come from the same `redis-benchmark` invocation as above. Figures are **rounded**; repeat the commands on your machine to reproduce.

| Label | `-t` test | What it hits in Ledis | Redis req/s | Ledis req/s | Redis p50 | Ledis p50 | Notes |
|-------|-----------|------------------------|------------:|------------:|----------:|----------:|-------|
| **PING (RESP bulk)** | `ping_mbulk` | `bufio` read → `DecodeBulkStringsArrayFromReader` → `PING` handler | ~59,700 | ~77,400 | ~0.59 ms | ~0.32 ms | No store access; pure protocol + reply |
| **SET** | `set` | `SET` → in-memory store (no AOF in this run) | ~61,500 | ~56,300 | ~0.65 ms | ~0.35 ms | Ledis: single worker + mutex on store |
| **GET** | `get` | `GET` → store read | ~63,700 | ~77,200 | ~0.57 ms | ~0.32 ms | Read-only; no persistence in this run |
| **INCR** | `incr` | `INCR` → parse int, write store | ~63,300 | ~79,200 | ~0.42 ms | ~0.32 ms | RMW on one key; benchmark uses its own key |

### How to read the columns

| Column | Meaning |
|--------|---------|
| **req/s** | `throughput summary` from `redis-benchmark` (higher = more commands completed per second). |
| **p50** | Median latency per command for that test (lower = snappier at typical case). |
| **What it hits in Ledis** | Rough path; live traffic also goes through the **global command worker** (`internal/commands/worker.go`). |

### Architecture (why req/s is not the whole story)

| Topic | Redis (typical) | Ledis (current) |
|-------|-----------------|-----------------|
| Command execution | Highly optimized, often multi-threaded I/O / optional threads | **One goroutine** drains a job queue; commands run **serially** |
| 50 benchmark clients | Can be served concurrently | 50 senders; **one executor**—work piles in a channel |
| Feature set | Full Redis | Subset of commands, learning codebase |
| Fair comparison? | Reference production server | Smoke test that the stack is sane on localhost |

## Commands

**Terminal 1 — Ledis (no AOF):**

```bash
go run ./cmd/server -appendonly=false
```

Listens on **`:7379`**.

**Terminal 2 — Redis** (if not already running):

```bash
redis-server
```

Default **`:6379`**.

**Terminal 3 — benchmarks** (same flags on both; only the port changes):

```bash
# Redis
redis-benchmark -h 127.0.0.1 -p 6379 -t ping_mbulk,set,get,incr -n 1000000 -c 50

# Ledis (ignore CONFIG warning)
redis-benchmark -h 127.0.0.1 -p 7379 -t ping_mbulk,set,get,incr -n 1000000 -c 50
```

Quick sanity check (single client):

```bash
redis-benchmark -h 127.0.0.1 -p 7379 -t ping_mbulk,set,get,incr -n 1000 -c 1
```

## Automated tests (correctness, not RPS)

Integration tests live under `cmd/test` only:

```bash
go test ./cmd/test/...
go test -race ./cmd/test/... -run TestMultiExecRace
```

See also [aof-replay.md](./aof-replay.md) for durability/replay behavior when AOF **is** enabled.

# Benchmarking Ledis vs Redis

> **Disclaimer:** These numbers are **informal, local-machine smoke tests**—not a production benchmark report. They compare a learning server (Ledis) to [Redis](https://redis.io/) under a narrow workload. Use them to talk about **architecture and durability tradeoffs**, not to claim Ledis is “faster than Redis.”

## Why persistence has to match

If Redis writes snapshots or AOF to disk while Ledis does not (or the opposite), **req/s mostly measures I/O policy**, not comparable in-memory command paths. Anyone reading the README carefully—including interviewers—will ask about that.

This document uses **persistence off on both** for the main comparison, then a **third column for Ledis with AOF enabled** so you can quote the durability cost in plain language (for example: “~47k ping/s in memory, ~47k with `appendfsync everysec` on this box—everysec batches fsync, so the hit shows up more on `always` or heavy write mixes”).

## Run configuration

| Setting | Redis | Ledis (in-memory column) | Ledis (AOF column) |
|---------|--------|---------------------------|---------------------|
| Host / port | `127.0.0.1:6379` | `127.0.0.1:7379` | same |
| Persistence | **`--save "" --appendonly no`** (no RDB, no AOF) | **`-appendonly=false`** | **default** (`-appendonly=true`, **`-appendfsync=everysec`**) |
| Tool | `redis-benchmark` | same | same |
| Requests | `-n 1000000` | same | same |
| Parallel clients | `-c 50` | same | same |
| Tests | `-t ping_mbulk,set,get,incr` | same | same |
| Repetitions | **3 runs per server; table uses median req/s** | same | same |

> **PING:** use **`ping_mbulk`** only. Ledis speaks RESP arrays; `ping_inline` can close the connection.

> **Ledis default is `-appendonly=true`.** The in-memory column deliberately turns AOF off so Redis and Ledis both avoid append/fsync during that comparison.

## Results (median of 3 runs, this machine)

Throughput only (`redis-benchmark -q`, middle value of three runs). Re-run the commands below on your hardware; absolute req/s will differ.

| Test (`-t`) | Redis `:6379` (req/s) | Ledis `:7379` no AOF (req/s) | Ledis `:7379` AOF everysec (req/s) |
|-------------|----------------------:|-----------------------------:|-----------------------------------:|
| `ping_mbulk` | ~55,600 | ~46,700 | ~47,100 |
| `set` | ~48,300 | ~40,900 | ~40,700 |
| `get` | ~56,000 | ~40,800 | ~41,700 |
| `incr` | ~61,400 | ~41,200 | ~41,300 |

On this run, **AOF with `everysec` barely moved the needle** versus no AOF—fsync is amortized, and the benchmark mix is not dominated by fsync-bound writes. That is still useful in interviews: name the policy (`no` / `everysec` / `always`), say what you measured, and note that **`always` or write-heavy workloads** are where you expect a larger gap.

### How to read the columns

| Column | Meaning |
|--------|---------|
| **Redis (no persistence)** | Baseline in-memory Redis with RDB saves and AOF disabled for the test window. |
| **Ledis no AOF** | Same idea: no `appendonly.aof` writes. |
| **Ledis AOF everysec** | Each mutating command appended; fsync at most once per second (`internal/persistence` + `-appendfsync=everysec`). |
| **Median of 3** | Single runs swing with CPU scheduling, background apps, and cache warmth; median of three is easier to defend than one lucky or unlucky run. |

### Architecture (why req/s is not the whole story)

| Topic | Redis (typical) | Ledis (current) |
|-------|-----------------|-----------------|
| Command execution | Highly optimized, often multi-threaded I/O / optional threads | **One goroutine** drains a job queue; commands run **serially** |
| 50 benchmark clients | Can be served concurrently | 50 senders; **one executor**—work piles in a channel |
| Feature set | Full Redis | Subset of commands, learning codebase |
| Fair comparison? | Reference production server | Smoke test that the stack is sane on localhost |

## Commands

**Terminal 1 — Redis (persistence off):**

```bash
redis-server --save "" --appendonly no --port 6379
```

If Redis is already running, you can disable saves/AOF for the session (until restart):

```bash
redis-cli CONFIG SET save ""
redis-cli CONFIG SET appendonly no
```

**Terminal 2 — Ledis (pick one):**

```bash
# In-memory column (matches Redis persistence-off run)
go run ./cmd/server -appendonly=false

# AOF column (default fsync policy: everysec)
go run ./cmd/server
```

Listens on **`:7379`**.

**Terminal 3 — benchmark (run three times; record median req/s per test):**

```bash
redis-benchmark -h 127.0.0.1 -p 6379 -t ping_mbulk,set,get,incr -n 1000000 -c 50 -q
redis-benchmark -h 127.0.0.1 -p 7379 -t ping_mbulk,set,get,incr -n 1000000 -c 50 -q
```

Repeat each line **three times** per server configuration. Use the **middle** req/s for each test name (`PING_MBULK`, `SET`, `GET`, `INCR`).

Quick sanity check (single client):

```bash
redis-benchmark -h 127.0.0.1 -p 7379 -t ping_mbulk,set,get,incr -n 1000 -c 1 -q
```

## Interview talking points

- **Apples to apples:** “We turned off Redis RDB/AOF and Ledis AOF for the main table, then added Ledis with AOF so durability isn’t invisible in the story.”
- **Single worker:** Ledis serializes all commands through one worker; Redis is a different class of engine—compare **shapes** (protocol, persistence cost), not winner/loser marketing.
- **Durability vs speed:** AOF trades disk work for crash recovery; `everysec` vs `always` is the usual latency/fsync trade-off. EXEC batches one AOF record for the whole transaction block (see [aof-replay.md](./aof-replay.md)).

## Automated tests (correctness, not RPS)

Integration tests live under `cmd/test` only:

```bash
go test ./cmd/test/...
go test -race ./cmd/test/... -run TestMultiExecRace
```

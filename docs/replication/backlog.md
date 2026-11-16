# Ledis replication backlog — notes

Human-oriented summary of how the master backlog works in code (`internal/replication/master_coordinator.go`, `coordinator.go`), what we fixed, and how stream offsets differ from the ring buffer.

See also [offsets.md](./offsets.md) for offset vocabulary in general.

---

## What the backlog is for

The master appends every **raw byte** of every replicated command (`Command.RawContent`) into a **fixed-size ring buffer**. That window lets a reconnecting replica do **partial resync** (read missing bytes from the master) instead of a full snapshot when its offset still falls inside the window.

Live replicas still get commands on their **per-replica queues**; the backlog is a **separate**, shared copy of the same byte stream for catch-up.

---

## Two coordinate systems (this is the main confusion)

Imagine one endless tape of replication bytes:

```text
…  byte#2  byte#3  byte#4  byte#5  …
     ↑       ↑       ↑
   offset  offset  offset
     2       3       4
```

That position on the tape is a **stream offset** (logical, never wraps).

In memory we only keep the last **N** bytes in a circular array `buf` with indices `0 … N-1`. Those indices are **physical** (ring) slots.

| Idea | Field / name in code | Notes |
|------|----------------------|--------|
| Next byte the master will write | `repl_offset` on `Coordinator` | Stream space. After 4 bytes exist, value is **4** (not “the 4th byte” — that is offset **3**). |
| Oldest stream byte still in the ring | `repl_backlog.oldestReplOffset` | Shown in INFO as `repl_backlog_first_byte_offset`. |
| Next physical slot to write | `repl_backlog.ringWriteIndex` | **Not** a stream offset. |
| Ring size | `repl_backlog.capacity` | INFO: `repl_backlog_size`. |
| Map stream → slot | `physicalIndex(off) = off % capacity` | Before the ring fills, stream offset often equals `buf` index; after wrap, they diverge. |

**Rule:** PSYNC, `CanServeFromBacklog`, and `ReadFromBacklog` always use **stream offsets**, never raw `buf` indices.

### Tiny example (capacity 4)

After writing `"abcd"`:

- Stream bytes at offsets **0,1,2,3**; `repl_offset = 4`.
- Physical: same bytes in `buf[0..3]`.

After writing a 5th byte `"e"`:

- Stream: oldest kept offset **1** (`oldestReplOffset = repl_offset - capacity = 1`).
- Offset **0** is gone from the ring (evicted).
- The byte at stream offset **4** sits in **`buf[0]`** (`4 % 4`).

---

## What `repl_offset` means

- Counting **positions**, not “human byte number”:
  - 2 bytes written → offsets **0** and **1** exist → **`repl_offset = 2`**.
  - The **3rd** byte (when it arrives) will be stream offset **2**.
- Replica caught up with master when both have the **same** `repl_offset`.
- Valid stream data currently in the backlog is **`[oldestReplOffset, repl_offset)`** — upper bound is **exclusive** (no byte exists *at* `repl_offset` yet).

When the ring has not yet wrapped, `oldestReplOffset` stays **0**. After wrap:

```text
oldestReplOffset = repl_offset - capacity
```

(We fixed an earlier off-by-one that used `capacity - 1`.)

---

## `CanServeFromBacklog` and caught-up replicas

Question: should a fully caught-up replica (`offset == repl_offset`) be “in the window”?

**Yes.** We use:

```text
oldestReplOffset <= streamOffset <= repl_offset
```

- **`offset == repl_offset`**: servable → partial path can return **zero** backlog bytes and **`+CONTINUE`** (not full resync).
- **`offset > repl_offset`**: not servable (replica ahead of master — invalid).
- **`offset < oldestReplOffset`**: not servable (too old — evicted from ring → full resync).

`offset >= 0` is redundant while `oldestReplOffset` is always ≥ 0.

`ReadFromBacklog(startOffset)` returns bytes at stream positions **`[startOffset, repl_offset)`**. If `startOffset == repl_offset`, the slice is empty.

---

## Write path: backlog and replicas stay in order

**Bug we fixed:** `BroadcastCommandToReplicas` used to enqueue on replica channels **after** releasing the lock, then append to the backlog. Two concurrent writes could land in **different orders** on the backlog vs replica queues → wrong partial resync bytes.

**Fix:** Under **one** `mu.Lock`:

1. `appendToBacklogLocked(cmd)`
2. Enqueue the same command to each replica with a **non-blocking** send:

```go
select {
case client.Queue <- cmd:
default:
    disconnectReplicaLocked(conn, client) // then close TCP after unlock
}
```

Concurrent broadcasts serialize on that lock, so stream order matches live replication order. A slow replica with a full output queue is **closed and removed** instead of stalling every write.

---

## Concurrency / locks

- Backlog append uses **`Lock`** (writers).
- `ReadFromBacklog` / `CanServeFromBacklog` use **`RLock`** (readers).
- Many goroutines can hold **`RLock`** at once; **`RLock` is not re-entrant** on the same goroutine. `ReadFromBacklog` uses `canServeFromBacklogLocked` while already holding `RLock`.

---

## INFO replication fields (master)

From backlog state:

| INFO field | Source |
|------------|--------|
| `repl_backlog_active` | `1` if `capacity > 0` |
| `repl_backlog_size` | `capacity` |
| `repl_backlog_first_byte_offset` | `oldestReplOffset` |
| `repl_backlog_histlen` | `repl_offset - oldestReplOffset` |

---

## Tests

In `cmd/test/backlog_test.go` (not `internal/`):

| Test | What it checks |
|------|----------------|
| `TestBacklogReadInsideWindow` | Read from stream start returns exact bytes |
| `TestBacklogRefuseEvictedOffset` | Offset below `oldestReplOffset` rejected after wrap |
| `TestBacklogWrapReadBack` | After many writes, read from oldest matches tail of ring |
| `TestBacklogCaughtUpOffsetCanServeEmptyRead` | `offset == repl_offset` → can serve, zero bytes |
| `TestBacklogRace` | Concurrent append + read under `-race` |
| `TestBacklogInfoFieldsTrackState` | INFO backlog fields track writes |

```bash
go test -race ./cmd/test/... -run TestBacklog
```

Slow replica drop (non-blocking enqueue): `cmd/test/slow_replica_test.go` — `TestSlowReplicaDroppedMasterDoesNotStall` uses a stalled replica TCP client and large `SET`s so the output queue (default 1000) fills and the master drops the replica without stalling writes (`connected_slaves` → 0).

Tests call `HandleBacklog` directly on a coordinator; production path is `BroadcastCommandToReplicas` → `appendToBacklogLocked`.

---

## Not wired yet

- **PSYNC partial resync** still returns `+CONTINUE` when offsets match; it does not yet stream `ReadFromBacklog` bytes on the connection.
- Full resync paths in `psync.go` still use simple offset / replid checks; backlog serve logic should drive partial vs full when that is implemented.

---

## Mental checklist

1. **Stream offset** = position on the logical replication tape.
2. **Physical index** = `buf[ streamOffset % capacity ]` after lookup.
3. **`repl_offset`** = next stream position (one past the last written byte).
4. **Backlog window** = stream offsets from **`oldestReplOffset`** up to and including **`repl_offset`** for *eligibility*; bytes returned are only **`[start, repl_offset)`**.
5. **Caught up** = same `repl_offset` on master and replica; serve check passes with **empty** read.
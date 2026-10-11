![Cover: MULTI/EXEC batched AOF write and crash-safe replay](./assets/ledis-aof-multi-exec-cover.jpg)

# My first `EXEC` hung forever (AOF lock + suppress)

I was wiring Redis-style transactions into [Ledis, my Redis-compatible server in Go](https://github.com/kevinnadar22/ledis). `MULTI` / `SET` / `EXEC` worked in memory. Then I turned the append-only file (AOF) back on, ran `EXEC` again, and the server **stopped responding**. No panic—just a goroutine stuck forever.

The deadlock was boring in hindsight:

1. **`Exec`** takes `aof.Lock()` so nothing else can interleave bytes while it writes the transaction block.
2. **`Exec`** runs each queued command; **`SET`** still calls **`aof.Append()`**, which tries to take **`aof.Lock()`** again on the same goroutine.

Same mutex, nested acquire, hang. The fix is not “smarter locking magic”—it is **hold the lock once**, **tell `Append` to stand down**, **write the whole block in one shot**.

## Lock, suppress, one `WriteLocked`

During `EXEC`:

```go
aof.Lock()
defer aof.Unlock()

aof.SetSuppress(true)
defer aof.SetSuppress(false)

var batch bytes.Buffer
batch.WriteString(resp.Encode("MULTI"))
for _, cmd := range queued {
    results = append(results, sess.run(cmd)) // SET's Append is a no-op while suppress is true
    batch.WriteString(cmd.RawContent)
}
batch.WriteString(resp.Encode("EXEC"))

aof.WriteLocked(batch.Bytes())
```

- **`SetSuppress(true)`** means “mutating commands run, but do not append.” That avoids re-entering the mutex through `Append`.
- **`WriteLocked`** writes without locking again—you already hold `mu`.
- Suppress is only safe because Ledis still uses a **single command worker**; parallel executors would need per-session skip flags instead of one global bit.

While `MULTI` is active, nothing hits the AOF anyway. On commit, the file should look like one Redis transaction chunk: `MULTI`, each queued command as raw RESP, then `EXEC`.

## Proof on disk: 56 bytes for `MULTI` / `SET k v` / `EXEC`

After `EXEC`, `appendonly.aof` is not three separate appends—it is **one contiguous block** (56 bytes for this example):

```text
*1\r\n$5\r\nMULTI\r\n
*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$1\r\nv\r\n
*1\r\n$4\r\nEXEC\r\n
```

Same bytes as a `%q` dump from Go:

```text
"*1\r\n$5\r\nMULTI\r\n*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$1\r\nv\r\n*1\r\n$4\r\nEXEC\r\n"
```

Hex (pipe through `xxd -r -p` to recreate the file):

```text
2a310d0a24350d0a4d554c54490d0a2a330d0a24330d0a5345540d0a24310d0a6b0d0a24310d0a760d0a2a310d0a24340d0a455845430d0a
```

We lock this in with an end-to-end test—**[`TestAOFMultiSetExecEndToEnd`](https://github.com/kevinnadar22/ledis/blob/main/cmd/test/aof_test.go)** in `cmd/test/aof_test.go`. It spins up a real server with a temp AOF, runs `MULTI` → `SET k v` → `EXEC`, reads the file from disk, and asserts the bytes match `Encode("MULTI") + setWire + Encode("EXEC")` in order. No mocking the write path.

```bash
go test ./cmd/test/... -run TestAOFMultiSetExecEndToEnd -count=1
```

## Crash-safe replay (when the tail is torn)

Batching fixes the deadlock; **replay** fixes “what if we died mid-write?”

On startup, `Replay` reads RESP commands from offset 0. After each full command, if the reader is at EOF, we are done—if we are still inside an unclosed `MULTI` block, we **truncate back to `multiStart`** so a half-logged transaction never replays. A partial non-transaction command at the end truncates to the last good **`ok`** offset instead.

Inside a logged `MULTI` … `EXEC` block, replay buffers inner commands and runs them only when it sees `EXEC`—same semantics as live `EXEC`, without writing the AOF again during recovery.

## Takeaways

| Problem | What we did |
|---------|-------------|
| `EXEC` + `Append` deadlock | `Lock` + `SetSuppress` + single `WriteLocked` batch |
| On-disk shape | One `MULTI` + raw queued RESP + `EXEC` per commit |
| Torn AOF tail | Truncate to `ok` or `multiStart`; explicit EOF “done” path |
| Regression | `TestAOFMultiSetExecEndToEnd` checks the file bytes |

## Related reading

- [AOF replay notes](https://github.com/kevinnadar22/ledis/blob/main/docs/aof-replay.md)
- [Transactions](https://github.com/kevinnadar22/ledis/blob/main/docs/transactions.md)
- Code: [`transaction.go`](https://github.com/kevinnadar22/ledis/blob/main/internal/commands/transaction.go), [`aof.go`](https://github.com/kevinnadar22/ledis/blob/main/internal/persistence/aof.go)

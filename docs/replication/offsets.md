# Redis / Ledis Replication – Offsets Summary

## 1. What is a replication offset?

A replication offset tracks how many bytes of replication data a node has processed or generated.

It is measured in bytes, not commands.

Example:

```
SET name kev  → 24 bytes
SET age 21    → 18 bytes
INCR counter  → 15 bytes
```

| Command      | Replication offset |
| ------------ | ------------------ |
| Initial      | 0                  |
| SET name kev | 24                 |
| SET age 21   | 42                 |
| INCR counter | 57                 |

Why bytes instead of commands? Because commands have different lengths, and replication needs to track an exact position in the byte stream.

## 2. Master vs Replica offsets

Master – `master_repl_offset`

Tracks the latest byte offset generated in the master's replication stream.

Replica – `repl_offset`

Tracks the latest byte offset the replica has processed from its master.

Example:

```
Master offset: 1000
Replica offset: 800
```

The replica is 200 bytes behind.

Every node maintains its own replication progress. A replica that also serves downstream replicas can generate its own replication stream.

## 3. What is `repl_backlog_first_byte_offset`?

The master maintains a circular buffer called the replication backlog.

It stores recently generated replication data so disconnected replicas can catch up without downloading the entire dataset again.

`repl_backlog_first_byte_offset` represents the replication offset of the oldest byte currently available in that buffer.

Important distinction:

- Physical buffer index: Where the byte sits in memory.
- Replication offset: Its position in the logical replication stream.

They are not the same.

## 4. How is the first byte offset calculated?

\\[ FirstByteOffset = MasterOffset - BacklogHistLen + 1 \\]

Where:

- `MasterOffset`: Latest replication stream offset.
- `BacklogHistLen`: Number of bytes currently retained.
- `FirstByteOffset`: Offset of the oldest retained byte.

Example:

```
masterReplOffset := int64(1499)
backlogHistLen := int64(500)

firstByteOffset := masterReplOffset - backlogHistLen + 1

// 1000
```

The backlog contains offsets `1000–1499`.

For an empty backlog, the next available offset is `MasterOffset + 1`.

## 5. Partial vs Full Resynchronization

This is the primary reason the master maintains a replication backlog.

Imagine:

```
Master backlog: [1000 ........ 1499]
```

Scenario A: Replica offset = 1200

The replica needs bytes `1201–1499`.

Those bytes are available.

Partial resynchronization

Scenario B: Replica offset = 800

The replica needs bytes `801–1499`.

The master no longer has bytes `801–999`.

Full resynchronization

Redis also checks the replication ID to ensure the replica belongs to a compatible replication history.

## 6. What is FULLRESYNC?

When partial synchronization isn't possible, the master sends:

```
FULLRESYNC <replication_id> <master_repl_offset>
```

Example:

```
FULLRESYNC abc123 5000
```

Meaning:

- `FULLRESYNC`: Replica must receive a complete dataset.
- `abc123`: Master's replication ID.
- `5000`: Replication offset establishing the new baseline.

Your Ledis function:

```
func (c *Coordinator) writeFullSyncResponse(conn net.Conn) error {
    if conn == nil {
        return fmt.Errorf("psync: nil connection")
    }

    c.mu.RLock()
    line := "FULLRESYNC " + c.replID + " " +
        strconv.FormatInt(c.master_repl_offset, 10)
    c.mu.RUnlock()

    _, err := conn.Write([]byte(resp.EncodeSimpleString(line)))
    return err
}
```

This only announces full synchronization. Snapshot transfer happens separately.

## 7. Real-world distributed systems applications

These concepts are not Redis-specific.

| System         | Related concept                    |
| -------------- | ---------------------------------- |
| Redis          | Replication offsets and backlog    |
| Kafka          | Consumer offsets and retained logs |
| PostgreSQL     | WAL positions and retention        |
| MySQL          | Binary log positions               |
| Event sourcing | Event sequence positions           |

The underlying distributed systems concepts are:

1. Progress tracking: How far has a node processed?
2. History retention: How much historical data is available?
3. Failure recovery: Can a disconnected node catch up?
4. Consistency: Have nodes processed the same history?


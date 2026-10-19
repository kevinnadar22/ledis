# Redis Replication — My Notes (for docs/replication.md)

## Roles
- One master (primary), one or more replicas.
- Why: availability/uptime + spread read load. Replicas mostly serve reads.
- Failover: a replica is promoted to master and things continue.
- 3rd reason: replicas instead of disk persistence (copies of data without disks, fewer moving parts to fail).
- Replicas are read-only by default. Writable replicas (config option): writes stay local to that replica and get overwritten when the master later sends the same key.
- Chained replication: a replica can be the master of other replicas. E.g. 2 DCs: master sends once to one replica in the remote DC, which feeds the rest there (saves cross-DC bandwidth).
- Replication never blocks the master and mostly doesn't block the replica, but there are windows during sync where a replica refuses reads ("still syncing with master").

## Start
- `REPLICAOF <ip> <port>` makes a server a replica of that master.
- `REPLICAOF NO ONE` turns a replica back into a master (promotion).

## Write path on master
1. Client writes; master applies it to the in-memory DB.
2. Appends the same command to the backlog and advances its offset.
3. Writes the command into each replica's client output buffer; the I/O layer flushes that buffer to the replica's socket.

## Client output buffer (one per replica)
- Why it exists: Redis is single-threaded; reading memory is cheap, writing to the network is slow. So it drops output into a per-client buffer and moves on. To the master, a replica is just another client.
- Hard limit: buffer hits it, master closes the connection (replica not consuming fast enough). Default 256 MB (confirmed by the talk, not 128 MB).
- Soft limit: 64 MB sustained for 60 s also closes the connection.
- Config: `client-output-buffer-limit replica 256mb 64mb 60` (old name `slave`). Setting it to 0 = no limit, memory grows without bound.
- Log line when it overflows: "Client scheduled to be closed ASAP". Raising the limit is a trade-off; no buffer is endless.
- After disconnect for overflow, the replica often can't partial-sync ("lack of backlog") and falls back to full sync.

## Backlog buffer
- Fixed-size circular buffer of the latest commands; oldest data gets overwritten (e.g. 512 MB; default 1 MB).
- Used for partial sync.
- One backlog shared by all replicas (vs one output buffer per replica). It keeps data even after a replica's output buffer drained, for replicas that come back later.
- Config: `repl-backlog-size` (default 1 MB).

### Offsets
- Every byte written gets an offset that only increases.
- `master_repl_offset` = latest byte. `repl_backlog_first_byte_offset` starts at 1 and stays there until the circular buffer fills, then rises forever.
- Valid backlog data = first-byte offset .. master_repl_offset. Replica asks for an offset below the first byte: full sync.
- Replica asks for the NEXT byte it doesn't have (its offset + 1).

## Partial vs full sync
- Partial: replica slightly behind sends replication ID + offset; master sends only the missing commands.
- Replication ID must match, not just the offset.
- Replication ID: a master gets a new unique ID every time it starts/restarts. An offset only means something together with its ID.
- What makes a replica a replica: it shares the master's replication ID. Its offset can differ (catching up), and its backlog size/first-byte offset can differ too.
- Second replication ID (`master_replid2` + `second_repl_offset`, PSYNC2, Redis 4.0+): when a replica is promoted it keeps the old master's ID and offset as its second ID, so the other replicas can partial-sync with it instead of all doing a full sync. (Before this, the promoted replica took a new ID and every replica had to full sync.) second_repl_offset = old offset + 1, then stays frozen while master_repl_offset grows.
- When full sync happens: replica starts empty, OR a partial sync fails (ID mismatch / offset outside backlog).
- If the replica's offset is older than the oldest data still in the backlog: full sync.

## Full sync
- Master sends an RDB snapshot of the whole dataset.
- Disk-based: master saves the RDB to disk, then transfers it.
- Diskless mode: master streams the RDB straight over the network, no disk.
- Replica saves the received RDB to a file.
- `repl-timeout` (default 60 s): the transfer must finish within it or the connection is cut. Big dataset or bad network: raise repl-timeout or shrink the dataset.
- Fork can fail with "can't allocate memory" (common with several Redis instances on one host). Fix: `vm.overcommit_memory = 1` (default 0 can refuse the fork; Redis warns at startup). Spare memory needed depends on write pattern: same page rewritten = little, writes spread over many pages = much more.
- Note (talk is 2019): Redis 7 uses one shared replication buffer, and replicas can now load a full sync without disk (`repl-diskless-load`).

## Open question: why does the replica use disk?
- The RDB is the whole dataset, not a command stream.
- Replica keeps serving old data until the snapshot fully arrives, then swaps it in at once; disk is the safe place to hold it meanwhile.
- `repl-diskless-load` loads directly from the socket but risks losing the old data if the transfer fails.
- During transfer, new writes queue in that replica's output buffer and are sent right after the RDB.

## Still to learn
- (Data consistency done, see below.)

## Data consistency: best effort
- Redis replication is best-effort consistent, not 100%. Writes to replicas are asynchronous, so you don't know when a write reaches a replica.
- Why Redis doesn't use quorums/Raft (like the MIT lecture: quorums, read-after-write, leader election): it's mainly used as a cache, where losing a few writes is fine and microsecond latency is the point. Waiting on a quorum per write would make it slow. Keep it simple. For strong consistency use a Raft-based system (e.g. etcd).

### Option 1: read from master
- Avoids stale reads from replicas that haven't got the write yet.
- Not 100% safe either: the master can die, and the promoted replica may lack the last writes. Also doesn't work when there are too many reads for one instance.

### Option 2 (server config): min-replicas-to-write + min-replicas-max-lag
- `min-replicas-to-write N`: minimum number of replicas that must be "good".
- `min-replicas-max-lag S`: lag is in SECONDS, NOT offset lag. Lag = seconds since the master last heard a ping/ACK from that replica.
- Example: `min-replicas-to-write 3`, `min-replicas-max-lag 10` means master accepts a write only if at least 3 replicas reported in within the last 10 s; otherwise the write is rejected before it is written.
- It's a liveness check, not a data check: it does NOT look at how far behind a replica's offset is. A replica that's alive but far behind still counts, and the master keeps sending it writes.
- It doesn't guarantee your write reached the replicas; it stops a master that's cut off from its replicas from accepting writes.

### Option 3 (client side): WAIT numreplicas timeout
- `WAIT <numreplicas> <timeout-ms>`: block until that many replicas acknowledge the write, or the timeout hits.
- Example: 7 replicas, wait for 4 (majority) with a timeout of a few seconds. Acts like a quorum indirectly.
- NOT a real quorum: say 4 of 7 replicas got the write and 3 didn't, then the master dies. Failover (Sentinel) usually prefers the most up-to-date replica but doesn't guarantee it, so a stale replica can be promoted and the acknowledged write is lost.
- Raft fixes this: a stale node can't win the election vote, so the new leader always has every committed write (Ledis Level 10).
- WAIT must run on the SAME connection as the write. With connection pooling it can land on another connection and give the wrong answer.

### Option 4: cooperative loader client
- For bulk loads: the loader checks master and replica offsets every few ms and sends the next batch only when they're close. Loads fast without letting replicas fall far behind.

## Replica ACKs (every second)
- Each replica sends `REPLCONF ACK <offset>` to the master about once per second.
- Why:
  1. Master knows the replica is alive; this is what min-replicas-max-lag uses.
  2. WAIT uses it to know which replicas have reached the write's offset.
  3. `INFO replication` shows how far behind each replica is.
- Build this in Ledis.

## Monitoring: ROLE and INFO replication
- `ROLE` on master: its offset + each replica's ip, port, offset. Replica offset trailing master's = behind.
- `ROLE` on replica: its master, link state, its own offset.
- `INFO replication`: each replica's ip, port, state, offset, lag (normally 0-1 s; climbs when a replica dies or network breaks).
- Also shows `master_replid`, `master_replid2`, `master_repl_offset`, `second_repl_offset`, `repl_backlog_first_byte_offset`, `repl_backlog_size`, `repl_backlog_histlen` (grows to backlog size, then stays).

## Corrections to what I first thought
- I thought min-replicas-max-lag was offset lag; it's seconds since the last ACK.
- I didn't know replicas send an ACK with their offset every second.

## Memory deep dive (Oct 19)

### Fork + copy-on-write (RDB save / full sync)
- Master forks; the child is a temporary helper (not a replica) that writes the RDB, then exits. Parent keeps serving clients.
- At fork, parent and child share the same memory pages; nothing is copied.
- Parent writes a page: the OS copies that page for the parent first, and the child keeps the original, so the snapshot stays frozen.
- Cost = pages the parent touched during the snapshot: few writes means tiny extra memory, heavy writes mean up to ~2x.
- Child exits: extra copies freed, memory returns to normal (lasts seconds to minutes).
- Writes during the snapshot queue in the replica's output buffer, sent after the RDB.
- Far-behind replica: no merge; it drops all old data and loads the new RDB. Only partial sync adds onto existing data.

### What counts toward maxmemory
- Counted: user data, replication backlog, normal client output buffers.
- The backlog eats into usable memory: maxmemory 10 MB + backlog 5 MB = only 5 MB for keys. Backlog = maxmemory: not even one key can be written (talk demo, noeviction).
- Replica output buffers sit outside maxmemory: up to 256 MB each, so 10 replicas can add a lot of RAM.
- Normal client buffers: the talk is vague, but the current Redis source (evict.c) counts them toward maxmemory.
- Not counted: replica output buffers, AOF buffer (also the cluster migrate buffer).
- Real RAM can exceed maxmemory, so set it lower when replicas are attached.

### Why replica + AOF buffers are excluded
- They're internal replication/persistence machinery, not user data; counting them would let replication or persistence pressure evict user keys.
- Eviction loop: evict key, DEL goes into replica buffers, memory goes up, evict more, more DELs, and so on until the DB is empty. Excluding them breaks the loop.

### maxmemory-policy
- `noeviction` (default): nothing deleted; writes return an error.
- `allkeys-lru` / `volatile-lru`: evict least recently used.
- `allkeys-lfu` / `volatile-lfu`: evict least frequently used.
- `volatile-ttl`: evict the key with the shortest remaining TTL.
- `allkeys-random` / `volatile-random`: evict a random key (no sampling).
- `allkeys-*` considers every key; `volatile-*` only keys with an expiry set.

### Eviction sampling
- Redis never scans all keys.
- Takes a random sample of 5 keys (default; change with `maxmemory-samples`).
- Picks the worst in the sample per the policy (LRU: least recently used, LFU: least frequently used, TTL: shortest remaining TTL), evicts it.
- Repeats sample-and-evict until memory is back under maxmemory.
- Since 3.0 a small pool of good candidates from earlier samples makes it close to true LRU; 10 samples gets closer, at a bit more CPU.

### Why approximate LRU, not true LRU
- True LRU = hash map + doubly linked list, move-to-front on every access.
- Memory: 2 pointers = 16 bytes per key (~1.6 GB on 100M keys). Redis uses a 24-bit clock in the existing key header: 0 extra.
- CPU: true LRU updates the list on every GET; sampling works only when memory is full.
- A cache needs an old key, not the exact oldest; 10 samples ≈ true LRU. Trades exactness for memory and CPU.
- Build both in Ledis L8 and measure.

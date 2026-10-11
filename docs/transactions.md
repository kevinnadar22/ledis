Redis Transactions

A Redis transaction starts with MULTI. After that, every command a client sends is not run right away; it is queued, and the server replies QUEUED. When the client sends EXEC, Redis runs every queued command in order and sends back one reply per command. DISCARD throws the queue away instead. While EXEC is running, no other client's commands can run in between, so the transaction is isolated.

Atomic in Redis does not mean all or nothing. If a command is wrong at queue time, like an unknown command or the wrong number of arguments, EXEC refuses to run anything and replies EXECABORT. But if a command fails while it is running, like INCR on a string, only that one command returns an error and the rest still run. There is no rollback. Redis keeps it this way for simplicity and speed, since runtime errors are almost always bugs in the client's code.

WATCH gives optimistic locking. A client calls WATCH on one or more keys before MULTI. If any watched key changes before EXEC, whether another client wrote to it, it was deleted, its expiry changed, it expired on its own, or even this same client changed it, then EXEC returns a null reply and nothing runs. The client is expected to retry. EXEC and DISCARD both clear all watches automatically, so UNWATCH is only needed when a client gives up before getting to either one.

For Ledis, each connection keeps a flag for whether it is inside MULTI, a queue of commands, an error flag for queue-time errors, and the version of each watched key. Each key in the store gets a version number that goes up on every write, delete, expiry change, actual expiry, and flush. On EXEC, Ledis aborts if the error flag is set, returns null if any watched version changed, and otherwise runs the whole queue under the store lock. The whole block is written to the AOF in one go, and then the queue and watches are cleared.


In Redis, if a command is invalid or the wrong number of arguments is given while queuing commands after MULTI, Redis returns an error immediately and does not queue that command. Later, when EXEC is called, if any command in the transaction failed to queue (a static error), EXEC will reply with EXECABORT and abort the entire transaction; none of the queued commands will be executed. However, if a command encounters a runtime error during execution (like running INCR on a string), only that command returns an error, and the rest continue as normal; there is no rollback. This ensures that static errors abort the whole transaction, while runtime errors only affect the specific command.

WATCH/UNWATCH Command - Optmistic Locking means that the client will try to execute the transaction and if the data has changed, the transaction will be aborted. Watch command will watch the key and if the key is modified, the transaction will be aborted. Unwatch command will unwatch the key and the transaction will be executed.
SO if a multi returned null, the transaction will be aborted. The client will need to retry the transaction till it succeeds.

## redis-cli session

Real output from stock `redis-cli` against Ledis on port 7379.

### Basic MULTI/EXEC

```
127.0.0.1:7379> flushall
OK
127.0.0.1:7379> set counter 10
OK
127.0.0.1:7379> multi
OK
127.0.0.1:7379(TX)> set name kevin
QUEUED
127.0.0.1:7379(TX)> incr counter
QUEUED
127.0.0.1:7379(TX)> get name
QUEUED
127.0.0.1:7379(TX)> exec
1) OK
2) (integer) 11
3) "kevin"
```

### DISCARD and EXECABORT

```
127.0.0.1:7379> multi
OK
127.0.0.1:7379(TX)> set temp 1
QUEUED
127.0.0.1:7379(TX)> discard
OK
127.0.0.1:7379> get temp
(nil)
127.0.0.1:7379> multi
OK
127.0.0.1:7379(TX)> set a
(error) ERR wrong number of arguments for 'SET' command
127.0.0.1:7379(TX)> exec
(error) EXECABORT Transaction discarded because of previous errors
127.0.0.1:7379> get a
(nil)
```

### WATCH abort

Another client changes the watched key before EXEC, so the transaction is aborted.

```
# terminal 1
127.0.0.1:7379> watch counter
OK
127.0.0.1:7379> multi
OK
127.0.0.1:7379(TX)> incr counter
QUEUED

# terminal 2
127.0.0.1:7379> set counter 100
OK

# terminal 1
127.0.0.1:7379(TX)> exec
(nil)
127.0.0.1:7379> get counter
"100"
```

fix aof everysec fsync double tick
replace single 1024-byte conn.Read with buffered reading for large or split commands
fix incr locking/race when key does not exist yet

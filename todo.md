move dispatcher helper functions to utils package
add test: MULTI, SET a (missing value), EXEC should return EXECABORT
incr on non-integer should return err value is not an integer or out of range like redis
fix aof everysec fsync double tick
replace single 1024-byte conn.Read with buffered reading for large or split commands
fix incr locking/race when key does not exist yet
add go test -race isolation test for multi/exec with concurrent clients
run gofmt (e.g. double space in IsCommandAllowedInTransaction in dispatcher.go)

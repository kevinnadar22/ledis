package utils


// have a static map of transaction commands like MULTI, EXEC, DISCARD, just for lookup
var transactionCommands = map[string]bool{
	"MULTI": true,
	"EXEC": true,
	"DISCARD": true,
}

var notAllowedCommandsInTransaction = map[string]bool{
	"SUBSCRIBE": true,
	"UNSUBSCRIBE": true,
	"WATCH": true,
	"UNWATCH": true,
}

var MutatingCommands = map[string]bool{
	"SET": true,
	"DEL": true,
	"INCR": true,
	"FLUSHALL": true,
}


func IsTransactionCommand(cmdStr string) bool {
	return transactionCommands[cmdStr]
}

func IsCommandAllowedInTransaction(cmdStr string) bool {
	return !notAllowedCommandsInTransaction[cmdStr]
}

func IsMutatingCommand(cmdStr string) bool {
	return MutatingCommands[cmdStr]
}
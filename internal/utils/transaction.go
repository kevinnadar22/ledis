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
}

func IsTransactionCommand(cmdStr string) bool {
	return transactionCommands[cmdStr]
}

func IsCommandAllowedInTransaction(cmdStr string) bool {
	return !notAllowedCommandsInTransaction[cmdStr]
}
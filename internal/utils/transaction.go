package utils

var transactionCommands = map[string]bool{
	"MULTI":   true,
	"EXEC":    true,
	"DISCARD": true,
}

var notAllowedCommandsInTransaction = map[string]bool{
	"SUBSCRIBE":   true,
	"UNSUBSCRIBE": true,
	"WATCH":       true,
	"UNWATCH":     true,
}

type watchKeyPolicy int

const (
	watchKeysAllWatches watchKeyPolicy = iota
	watchKeysFirstArg
	watchKeysAllArgs
	watchKeysArgPairs // future MSET k v k v ...
)

var mutatingWatchPolicy = map[string]watchKeyPolicy{
	"SET":      watchKeysFirstArg,
	"INCR":     watchKeysFirstArg,
	"DEL":      watchKeysAllArgs,
	"FLUSHALL": watchKeysAllWatches,
}

func IsTransactionCommand(cmdStr string) bool {
	return transactionCommands[cmdStr]
}

func IsCommandAllowedInTransaction(cmdStr string) bool {
	return !notAllowedCommandsInTransaction[cmdStr]
}

func IsMutatingCommand(cmdStr string) bool {
	_, ok := mutatingWatchPolicy[cmdStr]
	return ok
}

func WatchAffectedKeys(cmdStr string, argKeys []string) (keys []string, invalidateAll bool) {
	policy, ok := mutatingWatchPolicy[cmdStr]
	if !ok {
		if len(argKeys) > 0 {
			return []string{argKeys[0]}, false
		}
		return nil, false
	}

	switch policy {
	case watchKeysAllWatches:
		return nil, true
	case watchKeysFirstArg:
		if len(argKeys) > 0 {
			return []string{argKeys[0]}, false
		}
	case watchKeysAllArgs:
		return argKeys, false
	case watchKeysArgPairs:
		for i := 0; i+1 < len(argKeys); i += 2 {
			keys = append(keys, argKeys[i])
		}
		return keys, false
	}
	return nil, false
}

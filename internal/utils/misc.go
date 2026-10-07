package utils

// have a table of command and its min arg and max arg

var MinArgsCommandTable = map[string]int{
	"GET":  1,
	"SET":  2,
	"DEL":  1,
	"INCR": 1,
}

func CheckMinArgs(cmdStr string, args int) bool {
	if _, ok := MinArgsCommandTable[cmdStr]; !ok {
		return true
	}
	if args < MinArgsCommandTable[cmdStr] {
		return false
	}
	return true
}

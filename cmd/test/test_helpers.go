package main

import "github.com/kevinnadar22/ledis/internal/datatypes"

func makeValue(s string) datatypes.Value {
	return datatypes.Value{
		Type: datatypes.BulkString,
		Str:  &s,
	}
}

func makeCommand(cmdName string, args ...string) datatypes.Command {
	var valArgs []datatypes.Value
	for _, arg := range args {
		valArgs = append(valArgs, makeValue(arg))
	}
	return datatypes.Command{
		Cmd:  makeValue(cmdName),
		Args: valArgs,
	}
}

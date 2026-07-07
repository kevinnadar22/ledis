package datatypes

import (
	"fmt"
	"strconv"
	"strings"
)

type ByteConsumed int

type ResType int

const (
	SimpleString ResType = iota
	SimpleError
	BulkString
	Integer
	Array
)

type Value struct {
	Type ResType
	Integer 	int64
	Str 		*string
	Array 		[]Value
}

func (v Value) String() string {
	switch v.Type {
	case SimpleString:
		if v.Str != nil {
			return *v.Str
		}
		return ""
	case SimpleError:
		if v.Str != nil {
			return "ERR: " + *v.Str
		}
		return "ERR"
	case BulkString:
		if v.Str != nil {
			return *v.Str
		}
		return "(nil)"
	case Integer:
		return strconv.FormatInt(v.Integer, 10)
	case Array:
		var parts []string
		for _, val := range v.Array {
			parts = append(parts, val.String())
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		type temp Value
		return fmt.Sprintf("%v", temp(v))
	}
}

type Command struct {
	Cmd Value
	Args []Value
	Content Value
}
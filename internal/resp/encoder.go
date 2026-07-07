// normal text to resp
package resp

import (
	"fmt"
	"strconv"
	"strings"
)



func Encode(input string) string {
    var b strings.Builder
    args := strings.Split(input, " ")

    fmt.Fprintf(&b, "*%d\r\n", len(args))

    for _, arg := range args {
        fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(arg), arg)
    }

    return b.String()
}

func EncodeSimpleString(input string) (string) {
    str := "+" + input + "\r\n"
    return str
}

func EncodeError(input string) (string) {
    str := "-ERR " + input + "\r\n"
    return str
}

func EncodeInteger(input int64) (string) {
    return ":" + strconv.FormatInt(input,10) + "\r\n"
}
// normal text to resp
package resp

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)


func Splitter(input string) ([]string, error) {
    var args []string

	var t string
	var quote_state bool

	if strings.TrimSpace(input) == "" {
		return []string{}, nil
	}

    for i, v := range input {
		var prev_char byte
		if i != 0 {
			prev_char = input[i-1]
		}

		// if the current char is a space and we are not in a quote state, add the current string to args
		if v == ' ' && quote_state != true{
			// skip if the previous char is also a space
			if prev_char != byte(' ') {
				args = append(args, t)
				t = ""
			}
			continue
		}

		if v == '"' {

			if quote_state && t == "" {
				// empty string ""
				quote_state = false
				args = append(args, "")
				continue
			}

			if quote_state {
				// if already in quote state, and check prev char
				// if prev char is not \\, then it is a closing quote
				if prev_char != byte('\\') {
					quote_state = false
					continue
				} else {
					// remove last char if escaped and add the quote
					if prev_char == byte('\\') {
						t = t[:len(t)-1] + string(v)
					}
					continue
				}
			}

			// starting quote statement
			quote_state = true
			continue // don't add the quote itself
		}

        t += string(v)
    }

	if t != "" {
		args = append(args, t)
	}

	if quote_state == true {
		return nil, errors.New("unclosed quote")
	}

	return args, nil
}


func Encode(input string) string {
    var b strings.Builder
    args, err := Splitter(input)

    if err != nil {
        panic(err)
    }

    fmt.Fprintf(&b, "*%d\r\n", len(args))

    for _, arg := range args {
        fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(arg), arg)
    }

    fmt.Println(args)

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

func EncodeBulkString(input string) (string) {
	if input == "" {
		return "$-1\r\n"
	}
	str := "$" + strconv.Itoa(len(input)) + "\r\n" + input + "\r\n"
	return str
}
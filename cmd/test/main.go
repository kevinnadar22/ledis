package main

import (
	"errors"
	"fmt"
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
		if v == ' ' && quote_state != true {
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

func main() {
	input := `SET msg "hello \"world\""`
	encoded_input, _ := Splitter(input)
	fmt.Println(encoded_input)
	// print len
	fmt.Println(len(encoded_input))
}

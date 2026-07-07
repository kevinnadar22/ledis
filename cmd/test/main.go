package main

import (
	"fmt"
	"github.com/kevinnadar22/ledis/internal/resp"
)

func main() {
	input := "set name kevin 1"
	encoded_input := resp.Encode(input)
	val, _, _ := resp.DecodeArray(encoded_input, 0)
	fmt.Println(val.String())
}
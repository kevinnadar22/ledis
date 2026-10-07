// decode from client, resp to go dt
package resp

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/kevinnadar22/ledis/internal/datatypes"
)

func getDecoderFunc(cmd_type byte) (func(string, int) (datatypes.Value, datatypes.ByteConsumed, error), error) {

	switch cmd_type {
	case '+':
		return DecodeSimpleString, nil
	case '-':
		return DecodeSimpleError, nil
	case '$':
		return DecodeBulkStrings, nil
	case ':':
		return DecodeInteger, nil
	case '*':
		return DecodeArray, nil
	default:
		return nil, errors.New("unknown command type")
	}
}

func DecodeSimpleString(cmd string, start_byte int) (datatypes.Value, datatypes.ByteConsumed, error) {
	// Example : +ok\r\n
	end := start_byte + strings.Index(cmd[start_byte:], "\r\n")

	if end == -1 {
		return datatypes.Value{}, 0, nil
	}

	str := cmd[start_byte+1 : end] // skipping the +

	// bytes consumed would be end-start+2 (include the \r\n)
	return datatypes.Value{
		Type: datatypes.SimpleString,
		Str:  &str,
	}, datatypes.ByteConsumed(end - start_byte + 2), nil
}

func DecodeSimpleError(cmd string, start_byte int) (datatypes.Value, datatypes.ByteConsumed, error) {
	// Example : -err\r\n
	end := start_byte + strings.Index(cmd[start_byte:], "\r\n")

	if end == -1 {
		return datatypes.Value{}, 0, nil
	}

	str := cmd[start_byte+1 : end] // skipping the -

	// bytes consumed would be end-start+2 (include the \r\n)
	return datatypes.Value{
		Type: datatypes.SimpleError,
		Str:  &str,
	}, datatypes.ByteConsumed(end - start_byte + 2), nil
}

func DecodeBulkStrings(cmd string, start_byte int) (datatypes.Value, datatypes.ByteConsumed, error) {
	// example $4\r\nhell\r\n or $-1\r\n
	end := start_byte + strings.Index(cmd[start_byte:], "\r\n")

	if end == -1 {
		return datatypes.Value{}, 0, nil
	}

	if cmd[start_byte+1:end] == "-1" {
		return datatypes.Value{}, 4, nil
	}

	string_length, err := strconv.Atoi(cmd[start_byte+1 : end])

	if err != nil {
		return datatypes.Value{}, 0, err
	}

	start := end + 2 // start from hell in $4\r\nhell\r\n
	end = start + string_length

	bulk_str := cmd[start:end]

	bytes_consumed := (start + string_length + 2) - start_byte

	return datatypes.Value{
		Type: datatypes.BulkString,
		Str:  &bulk_str,
	}, datatypes.ByteConsumed(bytes_consumed), nil
}

func DecodeInteger(cmd string, start_byte int) (datatypes.Value, datatypes.ByteConsumed, error) {
	// example :1000\r\n
	end := start_byte + strings.Index(cmd[start_byte:], "\r\n")

	// check if CRLF exists
	if end == -1 {
		return datatypes.Value{}, 0, nil
	}

	integer, err := strconv.Atoi(cmd[start_byte+1 : end])

	if err != nil {
		return datatypes.Value{}, 0, err
	}

	return datatypes.Value{
		Type:    datatypes.Integer,
		Integer: int64(integer),
	}, datatypes.ByteConsumed(end - start_byte + 2), nil
}

func DecodeArray(cmd string, start_byte int) (datatypes.Value, datatypes.ByteConsumed, error) {
	// *<number of elements>
	// <element1>
	// <element2>
	// ...
	// <elementN>

	// example *2\r\n$4\r\nping\r\n$4\r\npong\r\n
	// returns [datatypes.Value{Type: datatypes.BulkString, Str: &"ping"}, datatypes.Value{Type: datatypes.BulkString, Str: &"pong"}]

	end := start_byte + strings.Index(cmd[start_byte:], "\r\n")

	if end == -1 {
		return datatypes.Value{}, 0, errors.New("invalid array")
	}

	array_length, err := strconv.Atoi(cmd[start_byte+1 : end])

	if err != nil {
		return datatypes.Value{}, 0, errors.New("invalid array length")
	}

	value_array := make([]datatypes.Value, array_length)

	array_offset := end + 2

	for i := 0; i < array_length; i++ {
		cmd_type := cmd[array_offset]

		decoderFunc, err := getDecoderFunc(cmd_type)
		if err != nil {
			return datatypes.Value{}, 0, err
		}

		value, byte_consumed, err := decoderFunc(cmd, array_offset)

		array_offset += int(byte_consumed)

		value_array[i] = value

	}

	return datatypes.Value{
		Type:  datatypes.Array,
		Array: value_array,
	}, datatypes.ByteConsumed(array_offset - start_byte + 2), nil
}

func Decode(cmd string) (datatypes.Command, error) {
	cmd_type := cmd[0]

	decoderFunc, err := getDecoderFunc(cmd_type)
	if err != nil {
		return datatypes.Command{}, err
	}
	v, _, err := decoderFunc(cmd, 0)
	if err != nil {
		return datatypes.Command{}, err
	}

	if v.Type != datatypes.Array {
		return datatypes.Command{
			RawContent: cmd,
		}, nil
	}

	cmd_val := v.Array[0]
	args := v.Array[1:]

	return datatypes.Command{Cmd: cmd_val, Args: args, RawContent: cmd}, nil
}

func DecodeBulkStringsArrayFromReader(reader *bufio.Reader) (string, error) {
	// decodes a cmd and returns a string version like
	// *3\r\n$4\r\nPING\r\n$4\r\nPONG\r\n
	first_byte, err := reader.ReadByte()
	if err != nil {
		return "", err
	}

	if first_byte != '*' {
		return "", errors.New("invalid bulk string array")
	}

	var builder strings.Builder
	builder.WriteByte('*')

	lenLine, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	builder.WriteString(lenLine)

	if !strings.HasSuffix(lenLine, "\r\n") {
		return "", errors.New("invalid array line terminator")
	}

	lenStr := lenLine[:len(lenLine)-2]
	arrayLength, err := strconv.Atoi(lenStr)
	if err != nil {
		return "", err
	}

	if arrayLength < 0 {
		return "", errors.New("invalid array length")
	}

	for i := 0; i < arrayLength; i++ {
		firstByte, err := reader.ReadByte()
		if err != nil {
			return "", err
		}
		if firstByte != '$' {
			return "", errors.New("invalid bulk string prefix")
		}
		builder.WriteByte('$')

		lenLine, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		builder.WriteString(lenLine)

		if !strings.HasSuffix(lenLine, "\r\n") {
			return "", errors.New("invalid bulk string length line terminator")
		}

		bulkLenStr := lenLine[:len(lenLine)-2]
		bulkLen, err := strconv.Atoi(bulkLenStr)
		if err != nil {
			return "", err
		}

		if bulkLen == -1 {
			continue
		}
		if bulkLen < 0 {
			return "", errors.New("invalid bulk string length")
		}

		dataBuf := make([]byte, bulkLen+2)
		_, err = io.ReadFull(reader, dataBuf)
		if err != nil {
			return "", err
		}
		builder.Write(dataBuf)

		if dataBuf[bulkLen] != '\r' || dataBuf[bulkLen+1] != '\n' {
			return "", errors.New("invalid bulk string data line terminator")
		}
	}

	return builder.String(), nil
}

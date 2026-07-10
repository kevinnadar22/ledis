package commands

import (
	"errors"
	"log"
	"strconv"
	"strings"

	"github.com/kevinnadar22/ledis/internal/datatypes"
	"github.com/kevinnadar22/ledis/internal/persistence"
	"github.com/kevinnadar22/ledis/internal/resp"
	"github.com/kevinnadar22/ledis/internal/store"
)

type SetOptions struct {
    EX *int64
	NX *bool
	XX *bool
}

func ParseSetOptions(cmd datatypes.Command) (SetOptions, error) {
	var options SetOptions
	for i := 2; i < len(cmd.Args); i++ {
		arg := cmd.Args[i]
		switch strings.ToUpper(*arg.Str){
		case "EX":
			if options.EX != nil {
				return options, errors.New("duplicate option")
			}

			if i+1 >= len(cmd.Args) {
				return options, errors.New("missing argument for 'EX' option")
			}

			val, err := strconv.ParseInt(*cmd.Args[i+1].Str, 10, 64)
			if err != nil {
				return options, errors.New("invalid argument for 'EX' option")
			}
			
			if val <= 0 {
				return options, errors.New("invalid argument for 'EX' option")
			}
			
			options.EX = &val
			i++
		case "NX":
			if options.NX != nil || options.XX != nil {
				return options, errors.New("duplicate option")
			}
			b := true
			options.NX = &b
		case "XX":
			if options.NX != nil || options.XX != nil {
				return options, errors.New("duplicate option")
			}
			b := true
			options.XX = &b
		default:
			return options, errors.New("unknown option")
		}
	}
	return options, nil
}

func Set(cmd datatypes.Command) (string, error) {
	if len(cmd.Args) < 2 {
		return "", errors.New("wrong number of arguments for 'set' command")
	}

	var (
		options SetOptions
		err error
	)

	if len(cmd.Args) > 2 {
		options, err = ParseSetOptions(cmd)
		if err != nil {
			return "", err
		}
	}

	k := *cmd.Args[0].Str
	v := *cmd.Args[1].Str

	isExistent := store.DB.Exist(k)

	if (options.NX != nil && isExistent) || (options.XX != nil && !isExistent) {
		return resp.EncodeBulkString(""), nil
	}

	store.DB.Set(k, v)

	if options.EX != nil {
		store.DB.Expire(k,  *options.EX)
	}


	err = persistence.AOFStore.Append([]byte(cmd.RawContent))
	if err != nil {
		log.Println("Error appending to AOF:", err)
		return "", err
	}

	return resp.EncodeSimpleString("OK"), nil
}
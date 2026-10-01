package config

import (
	"flag"
	"fmt"
	"github.com/kevinnadar22/ledis/internal/persistence"
)

type Config struct {
	AppendOnly bool
	FsyncPolicy persistence.FsyncPolicy
	RDBFile string
}

func GetConfig() *Config {
	return &Config{
		AppendOnly: true,
		FsyncPolicy: persistence.FsyncEverySecond,
		RDBFile: "./dump.rdb",
	}
}

func Load() (*Config, error) {
	appendOnly := flag.Bool("appendonly", true, "enable AOF")
	appendFsync := flag.String("appendfsync", "everysec", "no|everysec|always")
	rdbFile := flag.String("rdb", "", "path to rdb file")

	flag.Parse()

	var policy persistence.FsyncPolicy

	switch *appendFsync {
	case "no":
		policy = persistence.FsyncNo
	case "everysec":
		policy = persistence.FsyncEverySecond
	case "always":
		policy = persistence.FsyncAlways
	default:
		return nil, fmt.Errorf("invalid appendfsync: %s", *appendFsync)
	}


	// log config
	fmt.Printf("Config: %+v\n", &Config{
		AppendOnly: *appendOnly,
		FsyncPolicy: policy,
	})

	return &Config{
		AppendOnly: *appendOnly,
		FsyncPolicy: policy,
		RDBFile: *rdbFile,
	}, nil
}
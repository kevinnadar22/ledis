package config

import (
	"flag"
	"fmt"
	"github.com/kevinnadar22/ledis/internal/persistence"
)

type Config struct {
	AppendOnly  bool
	FsyncPolicy persistence.FsyncPolicy
	RDBFile     string
	MaxCommandSize int
	MaxBulkStringSize int
}



func Load() (*Config, error) {
	appendOnly := flag.Bool("appendonly", true, "enable AOF")
	appendFsync := flag.String("appendfsync", "everysec", "no|everysec|always")
	rdbFile := flag.String("rdb", "", "path to rdb file")
	maxCommandSize := flag.Int("max-command-size", 1024*1024*1024, "max command size in bytes")

	// 512mb bulk string size
	maxBulkStringSize := flag.Int("max-bulk-string-size", 512*1024*1024, "max bulk string size in bytes")

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

	return &Config{
		AppendOnly:  *appendOnly,
		FsyncPolicy: policy,
		RDBFile:     *rdbFile,
		MaxCommandSize: *maxCommandSize,
		MaxBulkStringSize: *maxBulkStringSize,
	}, nil
}

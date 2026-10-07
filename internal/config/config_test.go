package config

import (
	"flag"
	"os"
	"testing"
)

func loadConfig(t *testing.T, args ...string) *Config {
	t.Helper()
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	argv := append([]string{"ledis-test"}, args...)
	os.Args = argv
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	return cfg
}

func TestLoad_MaxCommandSizeDefault(t *testing.T) {
	cfg := loadConfig(t)
	if cfg.MaxCommandSize != 1024*1024*1024 {
		t.Errorf("MaxCommandSize = %d, want 1GiB", cfg.MaxCommandSize)
	}
}

func TestLoad_MaxBulkStringSizeDefault(t *testing.T) {
	cfg := loadConfig(t)
	if cfg.MaxBulkStringSize != 512*1024*1024 {
		t.Errorf("MaxBulkStringSize = %d, want 512MiB", cfg.MaxBulkStringSize)
	}
}

func TestLoad_MaxCommandSizeFlag(t *testing.T) {
	cfg := loadConfig(t, "-max-command-size=8192")
	if cfg.MaxCommandSize != 8192 {
		t.Errorf("MaxCommandSize = %d, want 8192", cfg.MaxCommandSize)
	}
}

func TestLoad_MaxBulkStringSizeFlag(t *testing.T) {
	cfg := loadConfig(t, "-max-bulk-string-size=4096")
	if cfg.MaxBulkStringSize != 4096 {
		t.Errorf("MaxBulkStringSize = %d, want 4096", cfg.MaxBulkStringSize)
	}
}

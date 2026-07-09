package main

import (
	"testing"
	"time"

	"github.com/kevinnadar22/ledis/internal/commands"

	"github.com/kevinnadar22/ledis/internal/store"
)

func TestSetCommand(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		setupStore  func()
		verifyStore func(t *testing.T)
		wantRes     string
		wantErr     bool
	}{
		{
			name: "Basic SET key value",
			args: []string{"mykey", "myval"},
			setupStore: func() {
				store.DB.FlushAll()
			},
			verifyStore: func(t *testing.T) {
				val, ok := store.DB.Get("mykey")
				if !ok || val != "myval" {
					t.Errorf("expected mykey to be 'myval', got '%s'", val)
				}
			},
			wantRes: "+OK\r\n",
			wantErr: false,
		},
		{
			name: "SET with missing arguments",
			args: []string{"mykey"},
			setupStore: func() {
				store.DB.FlushAll()
			},
			verifyStore: func(t *testing.T) {},
			wantRes:     "",
			wantErr:     true,
		},
		{
			name: "SET NX when key does not exist",
			args: []string{"mykey", "myval", "NX"},
			setupStore: func() {
				store.DB.FlushAll()
			},
			verifyStore: func(t *testing.T) {
				val, ok := store.DB.Get("mykey")
				if !ok || val != "myval" {
					t.Errorf("expected mykey to be set to 'myval', got '%s'", val)
				}
			},
			wantRes: "+OK\r\n",
			wantErr: false,
		},
		{
			name: "SET NX when key already exists",
			args: []string{"mykey", "newval", "NX"},
			setupStore: func() {
				store.DB.FlushAll()
				store.DB.Set("mykey", "oldval")
			},
			verifyStore: func(t *testing.T) {
				val, ok := store.DB.Get("mykey")
				if !ok || val != "oldval" {
					t.Errorf("expected mykey to remain 'oldval', got '%s'", val)
				}
			},
			wantRes: "$-1\r\n",
			wantErr: false,
		},
		{
			name: "SET XX when key does not exist",
			args: []string{"mykey", "myval", "XX"},
			setupStore: func() {
				store.DB.FlushAll()
			},
			verifyStore: func(t *testing.T) {
				if store.DB.Exist("mykey") {
					t.Error("expected mykey to not exist")
				}
			},
			wantRes: "$-1\r\n",
			wantErr: false,
		},
		{
			name: "SET XX when key already exists",
			args: []string{"mykey", "newval", "XX"},
			setupStore: func() {
				store.DB.FlushAll()
				store.DB.Set("mykey", "oldval")
			},
			verifyStore: func(t *testing.T) {
				val, ok := store.DB.Get("mykey")
				if !ok || val != "newval" {
					t.Errorf("expected mykey to be updated to 'newval', got '%s'", val)
				}
			},
			wantRes: "+OK\r\n",
			wantErr: false,
		},
		{
			name: "SET EX option sets expiration",
			args: []string{"mykey", "myval", "EX", "1"},
			setupStore: func() {
				store.DB.FlushAll()
			},
			verifyStore: func(t *testing.T) {
				val, ok := store.DB.Get("mykey")
				if !ok || val != "myval" {
					t.Errorf("expected mykey to be set, got '%s'", val)
				}
				ttl := store.DB.TTL("mykey")
				if ttl < 0 {
					t.Errorf("expected positive TTL, got %d", ttl)
				}
				time.Sleep(1100 * time.Millisecond)
				if store.DB.Exist("mykey") {
					t.Error("expected key to have expired")
				}
			},
			wantRes: "+OK\r\n",
			wantErr: false,
		},
		{
			name: "SET with duplicate EX option",
			args: []string{"mykey", "myval", "EX", "10", "EX", "20"},
			setupStore: func() {
				store.DB.FlushAll()
			},
			verifyStore: func(t *testing.T) {},
			wantRes:     "",
			wantErr:     true,
		},
		{
			name: "SET with missing EX argument",
			args: []string{"mykey", "myval", "EX"},
			setupStore: func() {
				store.DB.FlushAll()
			},
			verifyStore: func(t *testing.T) {},
			wantRes:     "",
			wantErr:     true,
		},
		{
			name: "SET with invalid EX value",
			args: []string{"mykey", "myval", "EX", "not-a-number"},
			setupStore: func() {
				store.DB.FlushAll()
			},
			verifyStore: func(t *testing.T) {},
			wantRes:     "",
			wantErr:     true,
		},
		{
			name: "SET with negative EX value",
			args: []string{"mykey", "myval", "EX", "-5"},
			setupStore: func() {
				store.DB.FlushAll()
			},
			verifyStore: func(t *testing.T) {},
			wantRes:     "",
			wantErr:     true,
		},
		{
			name: "SET with unknown option",
			args: []string{"mykey", "myval", "INVALID"},
			setupStore: func() {
				store.DB.FlushAll()
			},
			verifyStore: func(t *testing.T) {},
			wantRes:     "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupStore()
			cmd := makeCommand("SET", tt.args...)
			res, err := commands.Set(cmd)

			if (err != nil) != tt.wantErr {
				t.Errorf("Set() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if res != tt.wantRes {
				t.Errorf("Set() result = %q, want %q", res, tt.wantRes)
			}

			tt.verifyStore(t)
		})
	}
}

package main

import (
	"reflect"
	"testing"
)

func TestSplitter(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{"Basic command", "SET name maria", []string{"SET", "name", "maria"}, false},
		{"Extra spaces", "SET   name    maria", []string{"SET", "name", "maria"}, false},
		{"Quoted spaces", `SET name "maria kevin"`, []string{"SET", "name", "maria kevin"}, false},
		{"Empty string", "", []string{}, false},
		{"Just spaces", "   ", []string{}, false},
		{"Empty quotes", `SET name ""`, []string{"SET", "name", ""}, false},
		{"Escaped quotes", `SET msg "hello \"world\""`, []string{"SET", "msg", `hello "world"`}, false},
		{"Unclosed quote", `SET name "maria`, []string{"SET", "name", "maria"}, true}, // Change 'want' depending on your error logic
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Splitter(tt.input)
			
			// If we expected an error and didn't get one, or vice-versa
			if (err != nil) != tt.wantErr {
				t.Errorf("Splitter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			// Check if the slices match
			if !reflect.DeepEqual(got, tt.want) && !tt.wantErr {
				t.Errorf("Splitter() = %v, want %v", got, tt.want)
			}
		})
	}
}
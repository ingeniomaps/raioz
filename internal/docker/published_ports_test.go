package docker

import (
	"context"
	"reflect"
	"testing"
)

func TestParsePublishedHostPorts(t *testing.T) {
	tests := []struct {
		name   string
		column string
		want   []int
	}{
		{"published on both families", "0.0.0.0:36379->6379/tcp, [::]:36379->6379/tcp", []int{36379}},
		{"exposed only", "6379/tcp", []int{}},
		{"mixed", "80/tcp, 0.0.0.0:8443->443/tcp, 2019/tcp, :::8080->80/tcp", []int{8080, 8443}},
		{"bound to one address", "127.0.0.1:5432->5432/tcp", []int{5432}},
		{"folded range", "0.0.0.0:8000-8002->8000-8002/tcp", []int{8000, 8001, 8002}},
		{
			"range next to a single port",
			"0.0.0.0:5671-5672->5671-5672/tcp, [::]:5671-5672->5671-5672/tcp, 0.0.0.0:15672->15672/tcp",
			[]int{5671, 5672, 15672},
		},
		{"absurd range keeps its first port", "0.0.0.0:1000-60000->1000-60000/tcp", []int{1000}},
		{"backwards range keeps its first port", "0.0.0.0:9000-8000->9000-8000/tcp", []int{9000}},
		{"empty", "", []int{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := parsePublishedHostPorts(tc.column); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// The prune helpers never run unscoped: with no scope given they are
// limited to what raioz created, not let loose on the whole daemon.
func TestCleanFilterArgs(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want []string
	}{
		{"no scope falls back to raioz-managed", context.Background(),
			[]string{"--filter", "label=com.raioz.managed=true"}},
		{"project scope", WithCleanScope(context.Background(), map[string]string{
			"com.raioz.managed": "true", "com.raioz.project": "bencha",
		}), []string{"--filter", "label=com.raioz.managed=true", "--filter", "label=com.raioz.project=bencha"}},
		{"empty scope is not no scope", WithCleanScope(context.Background(), map[string]string{}),
			[]string{"--filter", "label=com.raioz.managed=true"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cleanFilterArgs(tc.ctx); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

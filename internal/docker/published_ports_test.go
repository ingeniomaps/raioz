package docker

import (
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
		{"range", "0.0.0.0:8000-8002->8000-8002/tcp", []int{8000}},
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

package app

import (
	"reflect"
	"testing"

	"raioz/internal/docker"
)

func TestUniqueConflictingProjects(t *testing.T) {
	cases := []struct {
		name      string
		conflicts []docker.PortConflict
		current   string
		want      []string
	}{
		{
			name: "deduplicates and sorts",
			conflicts: []docker.PortConflict{
				{Port: "9001:8080", Project: "hypixo-keycloak", Service: "keycloak"},
				{Port: "5540:5540", Project: "hypixo-keycloak", Service: "redisinsight"},
				{Port: "8025:8025", Project: "alpha-mail", Service: "mailhog"},
			},
			current: "gouduet-keycloak",
			want:    []string{"alpha-mail", "hypixo-keycloak"},
		},
		{
			name: "skips own project",
			conflicts: []docker.PortConflict{
				{Port: "9001:8080", Project: "self", Service: "x"},
				{Port: "5432:5432", Project: "other", Service: "y"},
			},
			current: "self",
			want:    []string{"other"},
		},
		{
			name: "skips conflicts without project label",
			conflicts: []docker.PortConflict{
				{Port: "80:80", Project: "", Service: "?"},
				{Port: "443:443", Project: "real", Service: "z"},
			},
			current: "self",
			want:    []string{"real"},
		},
		{
			name:      "empty input → empty output",
			conflicts: nil,
			current:   "self",
			want:      []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := uniqueConflictingProjects(tc.conflicts, tc.current)
			if got == nil {
				got = []string{}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("uniqueConflictingProjects() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFilterOtherActiveProjects(t *testing.T) {
	cases := []struct {
		name    string
		active  []string
		current string
		want    []string
	}{
		{
			name:    "removes self and dedupes",
			active:  []string{"a", "self", "b", "a"},
			current: "self",
			want:    []string{"a", "b"},
		},
		{
			name:    "empty current keeps everything",
			active:  []string{"a", "b"},
			current: "",
			want:    []string{"a", "b"},
		},
		{
			name:    "skips empty entries",
			active:  []string{"", "x", ""},
			current: "self",
			want:    []string{"x"},
		},
		{
			name:    "all filtered → empty",
			active:  []string{"self", "self"},
			current: "self",
			want:    []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := filterOtherActiveProjects(tc.active, tc.current)
			if got == nil {
				got = []string{}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("filterOtherActiveProjects() = %v, want %v", got, tc.want)
			}
		})
	}
}

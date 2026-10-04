package models

import "testing"

func TestResourcesValidate(t *testing.T) {
	tests := []struct {
		name    string
		res     *Resources
		wantErr bool
	}{
		{"nil", nil, false},
		{"empty", &Resources{}, false},
		{"megabytes", &Resources{Memory: "256m"}, false},
		{"gigabytes with b", &Resources{Memory: "1gb"}, false},
		{"plain bytes", &Resources{Memory: "268435456"}, false},
		{"fractional", &Resources{Memory: "1.5g", CPUs: 0.5}, false},
		{"unit only", &Resources{Memory: "m"}, true},
		{"unknown unit", &Resources{Memory: "256x"}, true},
		{"spaces", &Resources{Memory: "256 m"}, true},
		{"negative cpus", &Resources{CPUs: -1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.res.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResourcesOrDefault(t *testing.T) {
	fallback := &Resources{Memory: "128m"}
	own := &Resources{CPUs: 1}

	if got := (*Resources)(nil).OrDefault(fallback); got != fallback {
		t.Error("an undeclared block takes the default")
	}
	if got := (&Resources{}).OrDefault(fallback); got != fallback {
		t.Error("an empty block takes the default")
	}
	if got := own.OrDefault(fallback); got != own {
		t.Error("a declared block is not merged with the default")
	}
	if got := (&Resources{CPUs: 0.5}).CPUsString(); got != "0.5" {
		t.Errorf("CPUsString = %q", got)
	}
}

func TestResourcesMemoryBytes(t *testing.T) {
	tests := []struct {
		memory string
		want   int64
		ok     bool
	}{
		{"", 0, false},
		{"lots", 0, false},
		{"0", 0, false},
		{"1024", 1024, true},
		{"512b", 512, true},
		{"64k", 64 << 10, true},
		{"256m", 256 << 20, true},
		{"256MB", 256 << 20, true},
		{"1g", 1 << 30, true},
		{"1.5G", 3 << 29, true},
	}
	for _, tt := range tests {
		t.Run(tt.memory, func(t *testing.T) {
			got, ok := (&Resources{Memory: tt.memory}).MemoryBytes()
			if got != tt.want || ok != tt.ok {
				t.Errorf("MemoryBytes(%q) = %d, %v; want %d, %v", tt.memory, got, ok, tt.want, tt.ok)
			}
		})
	}
	if _, ok := (*Resources)(nil).MemoryBytes(); ok {
		t.Error("nil declares no memory cap")
	}
}

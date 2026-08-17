package models

import (
	"encoding/json"
	"testing"
)

// `env:` is polymorphic in the user's config: a list of files or a map of
// variables. The discriminator is IsObject, and every consumer branches on
// it, so the parse has to set it right for each shape.
func TestEnvValueUnmarshalJSON(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantErr   bool
		wantObj   bool
		wantFiles []string
		wantVars  map[string]string
	}{
		{
			name:      "array of files",
			input:     `["local-deps", "services/shared"]`,
			wantFiles: []string{"local-deps", "services/shared"},
		},
		{
			name:  "empty array is still file-shaped",
			input: `[]`,
		},
		{
			name:     "object of variables",
			input:    `{"DATABASE_URL": "postgres://x", "API_KEY": "k"}`,
			wantObj:  true,
			wantVars: map[string]string{"DATABASE_URL": "postgres://x", "API_KEY": "k"},
		},
		{
			name:    "object with a non-string value",
			input:   `{"PORT": 8080}`,
			wantErr: true,
		},
		{
			name:    "scalar is neither shape",
			input:   `42`,
			wantErr: true,
		},
		{
			// null parses as a nil []string, so it lands on the file
			// branch rather than erroring. Pinned because a consumer
			// reading GetFilePaths() gets nil, not a failure.
			name:  "null yields an empty file list",
			input: `null`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got EnvValue
			err := json.Unmarshal([]byte(tc.input), &got)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got.IsObject != tc.wantObj {
				t.Errorf("IsObject = %v, want %v", got.IsObject, tc.wantObj)
			}
			if len(got.Files) != len(tc.wantFiles) {
				t.Errorf("Files = %v, want %v", got.Files, tc.wantFiles)
			}
			for i, f := range tc.wantFiles {
				if got.Files[i] != f {
					t.Errorf("Files[%d] = %q, want %q", i, got.Files[i], f)
				}
			}
			for k, v := range tc.wantVars {
				if got.Variables[k] != v {
					t.Errorf("Variables[%q] = %q, want %q", k, got.Variables[k], v)
				}
			}
		})
	}
}

func TestEnvValueMarshalJSON(t *testing.T) {
	cases := []struct {
		name string
		in   EnvValue
		want string
	}{
		{name: "files", in: EnvValue{Files: []string{"a", "b"}}, want: `["a","b"]`},
		{
			name: "variables",
			in:   EnvValue{IsObject: true, Variables: map[string]string{"K": "v"}},
			want: `{"K":"v"}`,
		},
		{
			// IsObject without Variables can only be hand-built; it falls
			// through to the file branch and emits null.
			name: "object flag without variables",
			in:   EnvValue{IsObject: true},
			want: `null`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

// Both shapes must survive a marshal/unmarshal cycle, or a rewritten
// config would change meaning.
func TestEnvValueRoundTrip(t *testing.T) {
	for _, input := range []string{`["a","b"]`, `{"K":"v"}`} {
		var parsed EnvValue
		if err := json.Unmarshal([]byte(input), &parsed); err != nil {
			t.Fatalf("unmarshal %s: %v", input, err)
		}
		out, err := json.Marshal(parsed)
		if err != nil {
			t.Fatalf("marshal %s: %v", input, err)
		}
		if string(out) != input {
			t.Errorf("round trip of %s produced %s", input, out)
		}
	}
}

func TestEnvValueAccessors(t *testing.T) {
	files := EnvValue{Files: []string{"a"}}
	if got := files.GetFilePaths(); len(got) != 1 || got[0] != "a" {
		t.Errorf("GetFilePaths() = %v, want [a]", got)
	}
	if got := files.GetVariables(); got != nil {
		t.Errorf("file-shaped env must expose no variables, got %v", got)
	}

	vars := EnvValue{IsObject: true, Variables: map[string]string{"K": "v"}}
	if got := vars.GetFilePaths(); got != nil {
		t.Errorf("object-shaped env must expose no files, got %v", got)
	}
	if got := vars.GetVariables(); got["K"] != "v" {
		t.Errorf("GetVariables() = %v, want K=v", got)
	}
}

// `network:` is the same polymorphism: a bare name or an object that pins
// the subnet. A subnet is what makes container IPs deterministic, so
// losing it on parse would silently break proxy.publish:false setups.
func TestNetworkConfigUnmarshalJSON(t *testing.T) {
	cases := []struct {
		name       string
		input      string
		wantErr    bool
		wantName   string
		wantSubnet string
		wantObject bool
	}{
		{name: "string name", input: `"acme-net"`, wantName: "acme-net"},
		{
			name:       "object with subnet",
			input:      `{"name": "acme-net", "subnet": "172.28.0.0/16"}`,
			wantName:   "acme-net",
			wantSubnet: "172.28.0.0/16",
			wantObject: true,
		},
		{name: "object without name", input: `{"subnet": "172.28.0.0/16"}`, wantErr: true},
		{name: "number is neither shape", input: `42`, wantErr: true},
		{
			// null parses as an empty string, so the network ends up
			// nameless instead of rejected.
			name:  "null yields a nameless network",
			input: `null`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got NetworkConfig
			err := json.Unmarshal([]byte(tc.input), &got)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got.GetName() != tc.wantName {
				t.Errorf("GetName() = %q, want %q", got.GetName(), tc.wantName)
			}
			if got.GetSubnet() != tc.wantSubnet {
				t.Errorf("GetSubnet() = %q, want %q", got.GetSubnet(), tc.wantSubnet)
			}
			if got.HasSubnet() != (tc.wantSubnet != "") {
				t.Errorf("HasSubnet() = %v for subnet %q", got.HasSubnet(), tc.wantSubnet)
			}
			if got.IsObject != tc.wantObject {
				t.Errorf("IsObject = %v, want %v", got.IsObject, tc.wantObject)
			}
		})
	}
}

func TestNetworkConfigMarshalJSON(t *testing.T) {
	cases := []struct {
		name string
		in   NetworkConfig
		want string
	}{
		{name: "plain name", in: NetworkConfig{Name: "n"}, want: `"n"`},
		{
			name: "object with subnet",
			in:   NetworkConfig{Name: "n", Subnet: "10.0.0.0/16", IsObject: true},
			want: `{"name":"n","subnet":"10.0.0.0/16"}`,
		},
		{
			// Object form with no subnet carries nothing the string form
			// doesn't, so it normalizes back down to a name.
			name: "object without subnet collapses to the name",
			in:   NetworkConfig{Name: "n", IsObject: true},
			want: `"n"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

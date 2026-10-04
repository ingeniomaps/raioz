package host

import (
	"os"
	"reflect"
	"testing"
)

func TestParseCommand(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty", "", nil},
		{"whitespace only", "   ", nil},
		{"single word", "npm", []string{"npm"}},
		{"with args", "npm run dev", []string{"npm", "run", "dev"}},
		{"multiple spaces", "go  run  main.go", []string{"go", "run", "main.go"}},
		{"tabs and spaces", "make\tlaunch", []string{"make", "launch"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCommand(tc.input)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseCommand(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestReadLogTail(t *testing.T) {
	t.Run("missing file returns empty", func(t *testing.T) {
		got := ReadLogTail("/no/such/path/xyz123", 5)
		if got != "" {
			t.Errorf("ReadLogTail(missing) = %q, want empty", got)
		}
	})

	t.Run("empty file returns empty", func(t *testing.T) {
		f := t.TempDir() + "/empty.log"
		if err := os.WriteFile(f, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		if got := ReadLogTail(f, 5); got != "" {
			t.Errorf("ReadLogTail(empty) = %q, want empty", got)
		}
	})

	t.Run("fewer lines than requested returns all", func(t *testing.T) {
		f := t.TempDir() + "/short.log"
		if err := os.WriteFile(f, []byte("a\nb\nc\n"), 0644); err != nil {
			t.Fatal(err)
		}
		got := ReadLogTail(f, 5)
		if got != "a\nb\nc" {
			t.Errorf("ReadLogTail(short, 5) = %q, want %q", got, "a\nb\nc")
		}
	})

	t.Run("more lines than requested returns tail", func(t *testing.T) {
		f := t.TempDir() + "/long.log"
		if err := os.WriteFile(f, []byte("1\n2\n3\n4\n5\n6\n7\n"), 0644); err != nil {
			t.Fatal(err)
		}
		got := ReadLogTail(f, 3)
		if got != "5\n6\n7" {
			t.Errorf("ReadLogTail(long, 3) = %q, want %q", got, "5\n6\n7")
		}
	})
}

func TestFormatEarlyExitError(t *testing.T) {
	t.Run("without stderr tail", func(t *testing.T) {
		dir := t.TempDir()
		err := formatEarlyExitError("api", 500_000_000, errStub("exit 1"), dir+"/missing")
		msg := err.Error()
		if !contains(msg, `service "api" exited within`) {
			t.Errorf("missing prefix: %q", msg)
		}
		if contains(msg, "stderr tail") {
			t.Errorf("should not include tail block when file missing: %q", msg)
		}
	})

	t.Run("with stderr tail", func(t *testing.T) {
		dir := t.TempDir()
		path := dir + "/stderr.log"
		if e := os.WriteFile(path, []byte("listen :8080: in use\n"), 0644); e != nil {
			t.Fatal(e)
		}
		err := formatEarlyExitError("api", 500_000_000, errStub("exit 1"), path)
		msg := err.Error()
		if !contains(msg, "listen :8080: in use") {
			t.Errorf("tail line missing: %q", msg)
		}
		if !contains(msg, "stderr tail") {
			t.Errorf("tail header missing: %q", msg)
		}
	})
}

type errStub string

func (e errStub) Error() string { return string(e) }

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

package host

import (
	"reflect"
	"testing"
)

func TestSplitCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    []string
	}{
		{"plain words", "npm run dev", []string{"npm", "run", "dev"}},
		{"extra whitespace", "  go   run\t. ", []string{"go", "run", "."}},
		{"double quotes keep one argument", `sh -c "npm ci && npm run dev"`, []string{"sh", "-c", "npm ci && npm run dev"}},
		{"single quotes keep one argument", `./run --name 'my app'`, []string{"./run", "--name", "my app"}},
		{"quotes inside a word", `--label=a" "b`, []string{"--label=a b"}},
		{"double quotes inside single", `sh -c 'echo "hi there"'`, []string{"sh", "-c", `echo "hi there"`}},
		{"escaped quote inside double", `echo "say \"hi\""`, []string{"echo", `say "hi"`}},
		{"escaped space", `cat my\ file`, []string{"cat", "my file"}},
		{"backslash is literal in single quotes", `echo 'a\b'`, []string{"echo", `a\b`}},
		{"empty quoted argument survives", `run "" x`, []string{"run", "", "x"}},
		{"unterminated quote takes the rest", `sh -c "exit 3`, []string{"sh", "-c", "exit 3"}},
		{"empty", "", nil},
		{"only spaces", "   ", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SplitCommand(tt.command); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitCommand(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}

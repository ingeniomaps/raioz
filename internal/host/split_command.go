package host

import "strings"

// SplitCommand turns a `command:` / `stop:` string from raioz.yaml into the
// argv raioz executes. Words are separated by whitespace; single quotes,
// double quotes and a backslash keep whitespace inside a word, the way a
// shell reads them:
//
//	sh -c "npm ci && npm run dev"  ->  ["sh", "-c", "npm ci && npm run dev"]
//	./run --name 'my app'          ->  ["./run", "--name", "my app"]
//
// Nothing else is shell: no variable expansion, no pipes, no `&&`. A
// command that needs them says so with `sh -c "..."`.
//
// An unterminated quote is not an error here — the rest of the string
// becomes the last word, and the process reports what it could not run.
func SplitCommand(command string) []string {
	var (
		words   []string
		word    strings.Builder
		inWord  bool
		quote   rune // 0, '\'' or '"'
		escaped bool
	)
	flush := func() {
		if inWord {
			words = append(words, word.String())
			word.Reset()
			inWord = false
		}
	}

	for _, r := range command {
		switch {
		case escaped:
			word.WriteRune(r)
			escaped = false
		case quote == '\'':
			if r == '\'' {
				quote = 0
			} else {
				word.WriteRune(r)
			}
		case r == '\\':
			escaped = true
			inWord = true
		case quote == '"':
			if r == '"' {
				quote = 0
			} else {
				word.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			inWord = true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			flush()
		default:
			word.WriteRune(r)
			inWord = true
		}
	}
	if escaped {
		word.WriteRune('\\')
	}
	flush()
	return words
}

package config

import (
	"bytes"
	"regexp"

	"raioz/internal/errors"
	"raioz/internal/i18n"
)

// secretPattern is a known credential format whose presence anywhere
// in raioz.yaml is treated as an incident. ADR-036 (yaml hygiene policy
// + "secrets never in yaml") owns the rationale: a token committed to
// the yaml is a credential-rotation event waiting to happen, and
// scanning the raw bytes before yaml.Unmarshal catches it even when it
// lands in a typo'd or unknown field that the schema would otherwise
// silently discard.
type secretPattern struct {
	name string
	re   *regexp.Regexp
}

// secretPatterns is compiled once at package init. Patterns are
// intentionally specific (prefix + minimum length) so a colliding
// non-secret string is virtually impossible. Add new entries as new
// credential formats appear in the wild.
var secretPatterns = []secretPattern{
	{name: "GitHub personal access token", re: regexp.MustCompile(`ghp_[A-Za-z0-9]{36,}`)},
	{name: "GitHub OAuth token", re: regexp.MustCompile(`gho_[A-Za-z0-9]{36,}`)},
	{name: "GitHub user-to-server token", re: regexp.MustCompile(`ghu_[A-Za-z0-9]{36,}`)},
	{name: "GitHub server-to-server token", re: regexp.MustCompile(`ghs_[A-Za-z0-9]{36,}`)},
	{name: "GitHub refresh token", re: regexp.MustCompile(`ghr_[A-Za-z0-9]{36,}`)},
	{name: "GitLab personal access token", re: regexp.MustCompile(`glpat-[A-Za-z0-9_\-]{20,}`)},
	{name: "Slack token", re: regexp.MustCompile(`xox[boap]-[A-Za-z0-9-]+`)},
	{name: "AWS access key ID", re: regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{name: "PEM private key", re: regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
}

// ScanForSecrets reports the first credential-like pattern found in
// yamlBytes. Returns nil when nothing matches.
//
// The returned error NEVER includes the matched bytes — only the
// pattern name and approximate line number reach the message and
// context. This is load-bearing: the whole point of the scan is to
// keep the secret out of subsequent logs, error tracking, and CI
// output.
func ScanForSecrets(yamlBytes []byte) error {
	for _, p := range secretPatterns {
		loc := p.re.FindIndex(yamlBytes)
		if loc == nil {
			continue
		}
		line := lineOfByte(yamlBytes, loc[0])
		return errors.New(
			errors.ErrCodeSecretInYAML,
			i18n.T("error.secret_in_yaml", p.name, line),
		).WithSuggestion(
			i18n.T("error.secret_in_yaml_suggestion"),
		).WithContext("pattern", p.name).WithContext("line", line)
	}
	return nil
}

// lineOfByte returns the 1-indexed line containing byte offset off.
// Offsets past the end clamp to the last line.
func lineOfByte(data []byte, off int) int {
	if off > len(data) {
		off = len(data)
	}
	return bytes.Count(data[:off], []byte{'\n'}) + 1
}

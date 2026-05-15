package auth

import "fmt"

// ProviderFor returns the auth provider keyed by name, which is the
// raw value of `services.<n>.auth` from raioz.yaml. The empty
// string maps to the strict / default provider — the same
// public-only hardening raioz has applied since v0.1.
//
// Unknown names return an error rather than a silent fallback to
// strict. A typo'd `auth:` field should surface at preflight, not
// silently downgrade auth.
//
// Cases beyond "" are added in later commits of issue 067 as the
// inherit / gh / ssh providers land.
func ProviderFor(name string) (Provider, error) {
	switch name {
	case "":
		return &strictProvider{}, nil
	default:
		return nil, fmt.Errorf(
			"unknown auth provider %q (valid: omit for default; "+
				"inherit/gh/ssh land in later commits of issue 067)",
			name,
		)
	}
}

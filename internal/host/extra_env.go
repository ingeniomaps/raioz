package host

import "context"

type extraEnvKey struct{}

// WithExtraEnv attaches vars StartService adds on top of the service's own
// `env:`. Restart uses it to hand over what up computed at launch time
// (discovery, PORT), which the service config does not carry.
func WithExtraEnv(ctx context.Context, vars map[string]string) context.Context {
	if len(vars) == 0 {
		return ctx
	}
	return context.WithValue(ctx, extraEnvKey{}, vars)
}

// ExtraEnv returns the vars attached with WithExtraEnv, nil when none.
func ExtraEnv(ctx context.Context) map[string]string {
	vars, _ := ctx.Value(extraEnvKey{}).(map[string]string)
	return vars
}

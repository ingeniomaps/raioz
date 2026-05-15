package interfaces

import "errors"

// ErrDaemonUnreachable is returned by ContainerManager methods when
// the Docker daemon is down, socket missing, or the network probe to
// it fails. App-layer callers branch on it with errors.Is to offer
// offline-cleanup escape hatches (see `raioz down --force-state-cleanup`).
// The substring matching that maps docker CLI prose to this sentinel
// is the adapter's concern, not the app layer's.
var ErrDaemonUnreachable = errors.New("docker daemon unreachable")

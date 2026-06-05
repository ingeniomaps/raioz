// Package refcount tracks which raioz projects reference each shared
// dependency, so `raioz down` can tear a shared dep down only when its
// last consumer leaves. It replaces the old "scan live containers and
// guess if anyone is home" heuristic — which was blind to a sibling that
// consumes only shared deps (those carry no project label) — with an
// explicit, persisted count that the down path trusts directly.
//
// State lives in a single JSON file under naming.RaiozStateDir() (ADR-022),
// keyed by workspace then dep name. Writes are atomic (fsutil) and the
// read-modify-write cycle is serialized by a workspace-shared advisory
// file lock plus a per-process mutex — the same belt-and-suspenders the
// proxy uses (ADR-010), because sibling projects in one workspace mutate
// this file from separate processes.
package refcount

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"raioz/internal/fsutil"
	"raioz/internal/naming"
)

const (
	stateFileName = "shared-deps.json"
	lockFileName  = ".shared-deps.lock"
)

// processMu closes the per-process gap left by advisory file locks: two
// goroutines in the same process can both grab LOCK_EX on the same file
// without blocking. Cross-process serialization comes from the flock.
var processMu sync.Mutex

// state is the on-disk shape: workspace -> dep -> sorted set of project
// names that currently reference the dep. The empty-string workspace key
// holds name-override deps declared outside any workspace (single-owner
// in practice, but tracked the same way for uniformity).
type state struct {
	Workspaces map[string]map[string][]string `json:"workspaces"`
}

func newState() *state {
	return &state{Workspaces: map[string]map[string][]string{}}
}

func statePath() string { return filepath.Join(naming.RaiozStateDir(), stateFileName) }
func lockPath() string  { return filepath.Join(naming.RaiozStateDir(), lockFileName) }

// withLock runs fn while holding the cross-process advisory lock and the
// per-process mutex. The lock file is created under RaiozStateDir().
func withLock(fn func() error) (err error) {
	dir := naming.RaiozStateDir()
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		return fmt.Errorf("create refcount state dir %q: %w", dir, mkErr)
	}

	processMu.Lock()
	defer processMu.Unlock()

	f, openErr := os.OpenFile(lockPath(), os.O_CREATE|os.O_RDWR, 0o644)
	if openErr != nil {
		return fmt.Errorf("open refcount lock %q: %w", lockPath(), openErr)
	}
	if lockErr := fsutil.FileLockExclusive(f); lockErr != nil {
		_ = f.Close()
		return fmt.Errorf("acquire refcount lock %q: %w", lockPath(), lockErr)
	}
	defer func() {
		_ = fsutil.FileUnlock(f)
		_ = f.Close()
	}()

	return fn()
}

// load reads the state file. A missing file is an empty state, not an
// error — the first `up` of any workspace starts from nothing.
func load() (*state, error) {
	data, err := os.ReadFile(statePath())
	if err != nil {
		if os.IsNotExist(err) {
			return newState(), nil
		}
		return nil, fmt.Errorf("read refcount state %q: %w", statePath(), err)
	}
	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("unmarshal refcount state %q: %w", statePath(), err)
	}
	if s.Workspaces == nil {
		s.Workspaces = map[string]map[string][]string{}
	}
	return &s, nil
}

// save writes the state atomically, or deletes the file when the state is
// empty so a clean teardown leaves no stale artifact (ADR-023: state
// mirrors reality).
func save(s *state) error {
	for ws, deps := range s.Workspaces {
		if len(deps) == 0 {
			delete(s.Workspaces, ws)
		}
	}
	if len(s.Workspaces) == 0 {
		if err := os.Remove(statePath()); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove empty refcount state %q: %w", statePath(), err)
		}
		return nil
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal refcount state: %w", err)
	}
	if err := fsutil.WriteFileAtomic(statePath(), data, 0o644); err != nil {
		return fmt.Errorf("write refcount state %q: %w", statePath(), err)
	}
	return nil
}

// AddRef records that project references dep within workspace. Idempotent:
// a second `up` of the same project does not duplicate the entry.
func AddRef(workspace, dep, project string) error {
	return withLock(func() error {
		s, err := load()
		if err != nil {
			return err
		}
		deps := s.Workspaces[workspace]
		if deps == nil {
			deps = map[string][]string{}
			s.Workspaces[workspace] = deps
		}
		refs := deps[dep]
		if !slices.Contains(refs, project) {
			refs = append(refs, project)
			slices.Sort(refs)
			deps[dep] = refs
		}
		return save(s)
	})
}

// DropRef removes project's reference to dep and returns the projects that
// still reference it. An empty slice means project was the last consumer
// and the caller should tear the dep down.
func DropRef(workspace, dep, project string) ([]string, error) {
	var remaining []string
	err := withLock(func() error {
		s, err := load()
		if err != nil {
			return err
		}
		deps := s.Workspaces[workspace]
		if deps == nil {
			return save(s) // nothing referenced; remaining stays empty
		}
		refs := slices.DeleteFunc(deps[dep], func(p string) bool { return p == project })
		if len(refs) == 0 {
			delete(deps, dep)
		} else {
			deps[dep] = refs
		}
		remaining = append([]string(nil), refs...)
		return save(s)
	})
	return remaining, err
}

// Refs returns the projects currently referencing dep in workspace.
func Refs(workspace, dep string) ([]string, error) {
	var out []string
	err := withLock(func() error {
		s, err := load()
		if err != nil {
			return err
		}
		if deps := s.Workspaces[workspace]; deps != nil {
			out = append([]string(nil), deps[dep]...)
		}
		return nil
	})
	return out, err
}

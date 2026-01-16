package mocks

import "raioz/internal/workspace"

type WorkspaceResolver struct {
	ResolveFunc func(project string) (*workspace.Workspace, error)
}

func (m *WorkspaceResolver) Resolve(project string) (*workspace.Workspace, error) {
	if m.ResolveFunc != nil {
		return m.ResolveFunc(project)
	}
	return workspace.Resolve(project)
}

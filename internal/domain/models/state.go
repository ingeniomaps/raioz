package models

import "raioz/internal/state"

// State domain types — aliased from internal/state
type (
	GlobalState                = state.GlobalState
	ProjectState               = state.ProjectState
	ServiceState               = state.ServiceState
	ConfigChange               = state.ConfigChange
	AlignmentIssue             = state.AlignmentIssue
	ServicePreference          = state.ServicePreference
	WorkspaceProjectPreference = state.WorkspaceProjectPreference
	ServiceInfo                = state.ServiceInfo
)

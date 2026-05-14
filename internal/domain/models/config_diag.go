package models

// MissingDependency represents a dependency that is required but not found.
type MissingDependency struct {
	ServiceName string   // Service that requires the dependency
	RequiredBy  string   // Service that requires it (or "root" if from root config)
	Dependency  string   // Name of the missing dependency
	FoundConfig *Service // Config found in service's .raioz.json (if any)
	FoundPath   string   // Path where config was found (if any)
}

// DependencyConflict represents a conflict between root and service dependencies.
type DependencyConflict struct {
	ServiceName   string   // Service name
	RootConfig    *Service // Config in root .raioz.json
	ServiceConfig *Service // Config in service's .raioz.json
	Differences   []string // List of differences
}

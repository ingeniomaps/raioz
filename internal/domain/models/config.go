// Package models provides domain model type aliases.
//
// These aliases allow the domain/interfaces layer to reference domain types
// without importing infrastructure packages directly. The actual struct
// definitions remain in their original packages for backward compatibility.
//
// To add a new language or domain type, add a type alias here and update
// the corresponding interface file in domain/interfaces/.
package models

import "raioz/internal/config"

// Config domain types — aliased from internal/config
type (
	Deps                = config.Deps
	Service             = config.Service
	SourceConfig        = config.SourceConfig
	DockerConfig        = config.DockerConfig
	Project             = config.Project
	ProjectCommands     = config.ProjectCommands
	EnvironmentCommands = config.EnvironmentCommands
	ServiceCommands     = config.ServiceCommands
	EnvConfig           = config.EnvConfig
	EnvValue            = config.EnvValue
	NetworkConfig       = config.NetworkConfig
	HealthcheckConfig   = config.HealthcheckConfig
	Infra               = config.Infra
	InfraEntry          = config.InfraEntry
	MissingDependency   = config.MissingDependency
	DependencyConflict  = config.DependencyConflict
)

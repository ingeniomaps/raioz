package config

import "raioz/internal/domain/models"

// These types live canonically in internal/domain/models. The aliases below
// keep `config.Foo` callers working while we finish the migration described
// in ADR-009. New code should reference `models.Foo` directly.
type (
	EnvValue             = models.EnvValue
	NetworkConfig        = models.NetworkConfig
	HealthcheckConfig    = models.HealthcheckConfig
	Infra                = models.Infra
	ServiceProxyOverride = models.ServiceProxyOverride
	PublishSpec          = models.PublishSpec
	InfraEntry           = models.InfraEntry
)

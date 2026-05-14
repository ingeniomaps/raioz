package config

import "raioz/internal/domain/models"

// RoutingConfig and YAMLWatch live canonically in internal/domain/models;
// the aliases here keep `models.RoutingConfig` / `models.YAMLWatch` callers
// compiling (see ADR-009).
type (
	RoutingConfig = models.RoutingConfig
	YAMLWatch     = models.YAMLWatch
)

// YAMLIntSlice accepts either a single int (`expose: 5432`) or a list
// (`expose: [5432, 9090]`) so the common single-port case stays tidy.
type YAMLIntSlice []int

// UnmarshalYAML implements yaml.Unmarshaler for YAMLIntSlice.
func (s *YAMLIntSlice) UnmarshalYAML(unmarshal func(any) error) error {
	var single int
	if err := unmarshal(&single); err == nil {
		*s = []int{single}
		return nil
	}
	var slice []int
	if err := unmarshal(&slice); err != nil {
		return err
	}
	*s = slice
	return nil
}

// YAMLPublish is the polymorphic shape of `publish:`. Zero value means unset
// (internal only). Auto=true means the user asked for automatic allocation
// (bool form). Ports lists specific host ports requested explicitly.
//
// Mutually exclusive: Auto and Ports cannot both be set. The unmarshaller
// enforces this; later code can assume at most one is populated.
type YAMLPublish struct {
	// Set is true when the field was present in YAML (even as `publish: false`).
	// Distinguishes "user said internal-only" from "user didn't say anything".
	// Mostly useful for future semantics; today both mean internal-only.
	Set bool
	// Auto is true when the user wrote `publish: true` — raioz picks host ports.
	Auto bool
	// Ports is the list of explicit host ports the user requested.
	Ports []int
}

// UnmarshalYAML implements yaml.Unmarshaler for YAMLPublish, accepting bool,
// int, or []int so the YAML stays readable in the common cases.
func (p *YAMLPublish) UnmarshalYAML(unmarshal func(any) error) error {
	p.Set = true

	// bool form: publish: true / publish: false
	var b bool
	if err := unmarshal(&b); err == nil {
		p.Auto = b
		return nil
	}

	// single int form: publish: 5432
	var single int
	if err := unmarshal(&single); err == nil {
		p.Ports = []int{single}
		return nil
	}

	// list form: publish: [5432, 9090]
	var slice []int
	if err := unmarshal(&slice); err == nil {
		p.Ports = slice
		return nil
	}

	return nil
}

// YAMLDevConfig allows a dependency to specify a local path for development override.
type YAMLDevConfig struct {
	Path string `yaml:"path"`
}

// YAMLStringSlice is a helper type that allows a YAML field to be either a single string or a list.
type YAMLStringSlice []string

// UnmarshalYAML implements yaml.Unmarshaler for YAMLStringSlice.
func (s *YAMLStringSlice) UnmarshalYAML(unmarshal func(any) error) error {
	var single string
	if err := unmarshal(&single); err == nil {
		*s = []string{single}
		return nil
	}

	var slice []string
	if err := unmarshal(&slice); err != nil {
		return err
	}
	*s = slice
	return nil
}

// YAMLStringOrSlice is a helper that allows pre/post hooks to be a single string or a list of commands.
type YAMLStringOrSlice []string

// UnmarshalYAML implements yaml.Unmarshaler for YAMLStringOrSlice.
func (s *YAMLStringOrSlice) UnmarshalYAML(unmarshal func(any) error) error {
	var single string
	if err := unmarshal(&single); err == nil {
		*s = []string{single}
		return nil
	}

	var slice []string
	if err := unmarshal(&slice); err != nil {
		return err
	}
	*s = slice
	return nil
}

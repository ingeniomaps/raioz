package config

import (
	"encoding/json"
	"fmt"
)

// DeprecatedFields tracks deprecated fields that should show warnings
// This struct is not actually used - it's just for documentation
// We parse JSON manually to detect deprecated fields
type DeprecatedFields struct {
	NamespaceInProject bool   `json:"namespace,omitempty"`
	TypeInService      string `json:"type,omitempty"`
	TypeInInfra        string `json:"type_infra,omitempty"`
}

// CheckDeprecatedFields checks for deprecated fields in the JSON and returns warnings
func CheckDeprecatedFields(jsonData []byte) ([]string, error) {
	var raw map[string]any
	if err := json.Unmarshal(jsonData, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	var warnings []string

	// Check for namespace in project
	if project, ok := raw["project"].(map[string]any); ok {
		if _, hasNamespace := project["namespace"]; hasNamespace {
			warnings = append(warnings,
				"Field 'namespace' in project is deprecated. "+
					"It is redundant with 'name' and will be ignored. "+
					"Please remove it from your .raioz.json.",
			)
		}
	}

	// Check for type in services
	if services, ok := raw["services"].(map[string]any); ok {
		for svcName, svcData := range services {
			if svc, ok := svcData.(map[string]any); ok {
				if _, hasType := svc["type"]; hasType {
					warnings = append(warnings,
						fmt.Sprintf(
							"Field 'type' in service '%s' is deprecated. "+
								"It was not used for validation and will be ignored. "+
								"Please remove it from your .raioz.json.",
							svcName,
						),
					)
				}
			}
		}
	}

	// Check for type in infra
	if infra, ok := raw["infra"].(map[string]any); ok {
		for infraName, infraData := range infra {
			if inf, ok := infraData.(map[string]any); ok {
				if _, hasType := inf["type"]; hasType {
					warnings = append(warnings,
						fmt.Sprintf(
							"Field 'type' in infra '%s' is deprecated. "+
								"It was not used for validation and will be ignored. "+
								"Please remove it from your .raioz.json.",
							infraName,
						),
					)
				}
			}
		}
	}

	return warnings, nil
}

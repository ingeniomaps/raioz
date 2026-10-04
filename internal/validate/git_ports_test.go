package validate

import (
	"testing"

	"raioz/internal/domain/models"
)

// In raioz.yaml a git service's docker block only carries what the user
// wrote next to it (`ports:`); how the service runs comes from the
// checkout. The legacy rule that a git service must name a Dockerfile or a
// command there still holds for .raioz.json.
func TestValidateServices_GitServiceWithPorts(t *testing.T) {
	deps := func(format models.SourceFormat) *models.Deps {
		return &models.Deps{
			SourceFormat: format,
			Services: map[string]models.Service{
				"web": {
					Source: models.SourceConfig{Kind: "git", Repo: "https://example.com/web.git", Branch: "main", Path: "./web"},
					Docker: &models.DockerConfig{Ports: []string{"8080:3000"}},
				},
			},
		}
	}
	if err := validateServices(deps(models.SourceFormatYAML)); err != nil {
		t.Errorf("raioz.yaml: ports on a git service must be accepted, got %v", err)
	}
	if err := validateServices(deps(models.SourceFormatLegacyJSON)); err == nil {
		t.Error("legacy json: a git service without dockerfile or command must still be rejected")
	}
}

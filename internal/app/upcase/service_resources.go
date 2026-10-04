package upcase

import "raioz/internal/domain/models"

// serviceResources is the cap a service starts with. Its own `resources:`
// block always applies. The root default is a default for the containers
// raioz creates, so it reaches a Dockerfile service only: applied to a host
// process or to someone's compose stack it would cap things the user never
// asked to cap.
func serviceResources(svc models.Service, det models.DetectResult) *models.Resources {
	if svc.ResourcesInherited && det.Runtime != models.RuntimeDockerfile {
		return nil
	}
	return svc.Resources
}

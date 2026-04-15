package app

import (
	"fmt"
	"os"
	"time"
)

// CIResult represents the result of a CI run
type CIResult struct {
	Success     bool               `json:"success"`
	StartTime   string             `json:"startTime"`
	EndTime     string             `json:"endTime,omitempty"`
	Duration    float64            `json:"duration,omitempty"`
	Message     string             `json:"message,omitempty"`
	Workspace   string             `json:"workspace,omitempty"`
	ComposeFile string             `json:"composeFile,omitempty"`
	StateFile   string             `json:"stateFile,omitempty"`
	Services    []string           `json:"services,omitempty"`
	Infra       []string           `json:"infra,omitempty"`
	Validations []ValidationResult `json:"validations"`
	Errors      []string           `json:"errors,omitempty"`
	Warnings    []string           `json:"warnings,omitempty"`
}

// ValidationResult represents the result of a single validation check
type ValidationResult struct {
	Check   string `json:"check"`
	Status  string `json:"status"` // passed, failed, skipped
	Message string `json:"message,omitempty"`
}

// CIOptions contains options for the CI use case
type CIOptions struct {
	ConfigPath   string
	Keep         bool
	Ephemeral    bool
	JobID        string
	SkipBuild    bool
	SkipPull     bool
	OnlyValidate bool
	ForceReclone bool
}

// CIUseCase handles the "ci" use case
type CIUseCase struct {
	deps *Dependencies
}

// NewCIUseCase creates a new CIUseCase with injected dependencies
func NewCIUseCase(deps *Dependencies) *CIUseCase {
	return &CIUseCase{deps: deps}
}

// Execute runs the CI command and returns the result.
func (uc *CIUseCase) Execute(opts CIOptions) (*CIResult, error) {
	startTime := time.Now()

	result := &CIResult{
		Success:     false,
		StartTime:   startTime.Format(time.RFC3339),
		Validations: []ValidationResult{},
		Errors:      []string{},
		Warnings:    []string{},
	}

	defer func() {
		result.EndTime = time.Now().Format(time.RFC3339)
		result.Duration = time.Since(startTime).Seconds()
	}()

	// Fast preflight checks
	if err := uc.validateFastPreflight(); err != nil {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "preflight",
			Status:  "failed",
			Message: err.Error(),
		})
		result.Errors = append(
			result.Errors,
			fmt.Sprintf("Preflight check failed: %v", err),
		)
		return result, nil
	}

	result.Validations = append(result.Validations, ValidationResult{
		Check:  "preflight",
		Status: "passed",
	})

	// Try YAML mode first
	if proj := ResolveYAMLProject(uc.deps, opts.ConfigPath); proj != nil {
		return uc.executeYAML(proj, opts, result)
	}

	// Legacy JSON mode
	return uc.executeLegacy(opts, result)
}

// executeYAML runs CI validations for a YAML-mode project.
func (uc *CIUseCase) executeYAML(
	proj *YAMLProject,
	opts CIOptions,
	result *CIResult,
) (*CIResult, error) {
	result.Validations = append(result.Validations, ValidationResult{
		Check:  "load_config",
		Status: "passed",
	})

	// Validate service paths and dependency images
	for name, svc := range proj.Deps.Services {
		if svc.Source.Path != "" {
			if _, err := os.Stat(svc.Source.Path); os.IsNotExist(err) {
				result.Validations = append(result.Validations, ValidationResult{
					Check:  "service_paths",
					Status: "failed",
					Message: fmt.Sprintf(
						"%s: path not found: %s", name, svc.Source.Path,
					),
				})
				result.Errors = append(
					result.Errors,
					fmt.Sprintf(
						"Service %s path not found: %s", name, svc.Source.Path,
					),
				)
				return result, nil
			}
		}
	}

	for name, entry := range proj.Deps.Infra {
		if entry.Inline != nil && entry.Inline.Image == "" {
			result.Validations = append(result.Validations, ValidationResult{
				Check:   "dependency_images",
				Status:  "failed",
				Message: fmt.Sprintf("%s: no image specified", name),
			})
			result.Errors = append(
				result.Errors,
				fmt.Sprintf("Dependency %s has no image", name),
			)
			return result, nil
		}
	}

	result.Validations = append(result.Validations, ValidationResult{
		Check:  "validation",
		Status: "passed",
	})

	// Collect service and infra names
	for name := range proj.Deps.Services {
		result.Services = append(result.Services, name)
	}
	for name := range proj.Deps.Infra {
		result.Infra = append(result.Infra, name)
	}

	if opts.OnlyValidate {
		result.Success = true
		result.Message = "All validations passed"
		return result, nil
	}

	// Pull dependency images
	if !opts.SkipPull {
		err := uc.deps.DockerRunner.ValidateAllImages(proj.Deps)
		if err != nil {
			result.Validations = append(result.Validations, ValidationResult{
				Check:   "images",
				Status:  "failed",
				Message: err.Error(),
			})
			result.Errors = append(
				result.Errors,
				fmt.Sprintf("Image validation failed: %v", err),
			)
			return result, nil
		}
		result.Validations = append(result.Validations, ValidationResult{
			Check:  "images",
			Status: "passed",
		})
	}

	result.Success = true
	result.Message = "CI run completed successfully"
	return result, nil
}

package cmd

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

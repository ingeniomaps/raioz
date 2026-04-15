package production

// ProductionConfig represents a production configuration loaded from Docker Compose
type ProductionConfig struct {
	Services map[string]ProductionService `yaml:"services"`
	Networks map[string]interface{}       `yaml:"networks,omitempty"`
	Volumes  map[string]interface{}       `yaml:"volumes,omitempty"`
}

// ProductionService represents a service in production configuration
type ProductionService struct {
	Image       string            `yaml:"image"`
	Ports       []string          `yaml:"ports,omitempty"`
	Volumes     []string          `yaml:"volumes,omitempty"`
	DependsOn   interface{}       `yaml:"depends_on,omitempty"` // Can be []string or map[string]map[string]string
	Environment []string          `yaml:"environment,omitempty"`
	EnvFile     []string          `yaml:"env_file,omitempty"`
	Networks    []string          `yaml:"networks,omitempty"`
	Command     interface{}       `yaml:"command,omitempty"` // Can be string or []string
	Build       interface{}       `yaml:"build,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Restart     string            `yaml:"restart,omitempty"`
}

// ComparisonResult represents the result of comparing local and production configs
type ComparisonResult struct {
	ServiceDifferences []ServiceDifference `json:"serviceDifferences"`
	InfraDifferences   []InfraDifference   `json:"infraDifferences,omitempty"`
	Warnings           []string            `json:"warnings"`
	Errors             []string            `json:"errors"`
}

// ServiceDifference represents differences found for a specific service
type ServiceDifference struct {
	ServiceName      string           `json:"serviceName"`
	InLocalOnly      bool             `json:"inLocalOnly"`
	InProductionOnly bool             `json:"inProductionOnly"`
	ImageMismatch    *ImageMismatch   `json:"imageMismatch,omitempty"`
	PortMismatch     *PortMismatch    `json:"portMismatch,omitempty"`
	VolumeMismatch   *VolumeMismatch  `json:"volumeMismatch,omitempty"`
	DependsMismatch  *DependsMismatch `json:"dependsMismatch,omitempty"`
	EnvMismatch      *EnvMismatch     `json:"envMismatch,omitempty"`
	Severity         string           `json:"severity"` // info, warning, error
}

// ImageMismatch represents image/tag differences
type ImageMismatch struct {
	Local      string `json:"local"`
	Production string `json:"production"`
	LocalTag   string `json:"localTag,omitempty"`
	ProdTag    string `json:"prodTag,omitempty"`
}

// PortMismatch represents port differences
type PortMismatch struct {
	Local      []string `json:"local"`
	Production []string `json:"production"`
}

// VolumeMismatch represents volume differences
type VolumeMismatch struct {
	Local      []string `json:"local"`
	Production []string `json:"production"`
}

// DependsMismatch represents dependency differences
type DependsMismatch struct {
	Local      []string `json:"local"`
	Production []string `json:"production"`
}

// EnvMismatch represents environment variable differences
type EnvMismatch struct {
	LocalOnly      []string `json:"localOnly,omitempty"`
	ProductionOnly []string `json:"productionOnly,omitempty"`
}

// InfraDifference represents differences in infrastructure services
type InfraDifference struct {
	InfraName        string         `json:"infraName"`
	InLocalOnly      bool           `json:"inLocalOnly"`
	InProductionOnly bool           `json:"inProductionOnly"`
	ImageMismatch    *ImageMismatch `json:"imageMismatch,omitempty"`
	PortMismatch     *PortMismatch  `json:"portMismatch,omitempty"`
	Severity         string         `json:"severity"`
}

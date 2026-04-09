package detect

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// InferredDep represents a dependency inferred from environment files or config.
type InferredDep struct {
	Name    string // e.g., "postgres", "redis", "rabbitmq"
	Image   string // e.g., "postgres:16", "redis:7"
	Port    string // e.g., "5432"
	Source  string // where it was inferred from (e.g., "api/.env:DATABASE_URL")
}

// InferredLink represents a dependsOn relationship between services.
type InferredLink struct {
	From   string // service name
	To     string // dependency name
	Source string // where it was inferred from
}

// knownServices maps URL patterns/env var names to infrastructure dependencies.
var knownServices = []struct {
	patterns []string // regex patterns for env var values
	envNames []string // env var name patterns
	name     string
	image    string
	port     string
}{
	{
		patterns: []string{`postgres://`, `postgresql://`, `:5432`},
		envNames: []string{"DATABASE_URL", "DB_URL", "POSTGRES_", "PG_"},
		name:     "postgres",
		image:    "postgres:16",
		port:     "5432",
	},
	{
		patterns: []string{`redis://`, `:6379`},
		envNames: []string{"REDIS_URL", "REDIS_HOST", "REDIS_"},
		name:     "redis",
		image:    "redis:7",
		port:     "6379",
	},
	{
		patterns: []string{`mysql://`, `:3306`},
		envNames: []string{"MYSQL_"},
		name:     "mysql",
		image:    "mysql:8",
		port:     "3306",
	},
	{
		patterns: []string{`mongodb://`, `mongo://`, `:27017`},
		envNames: []string{"MONGO_URL", "MONGODB_URI", "MONGO_"},
		name:     "mongodb",
		image:    "mongo:7",
		port:     "27017",
	},
	{
		patterns: []string{`amqp://`, `rabbitmq://`, `:5672`},
		envNames: []string{"RABBITMQ_URL", "AMQP_URL", "RABBIT_"},
		name:     "rabbitmq",
		image:    "rabbitmq:3-management",
		port:     "5672",
	},
	{
		patterns: []string{`nats://`, `:4222`},
		envNames: []string{"NATS_URL", "NATS_"},
		name:     "nats",
		image:    "nats:latest",
		port:     "4222",
	},
	{
		patterns: []string{`:9200`, `elasticsearch://`},
		envNames: []string{"ELASTIC_", "ELASTICSEARCH_"},
		name:     "elasticsearch",
		image:    "elasticsearch:8",
		port:     "9200",
	},
	{
		patterns: []string{`:9092`},
		envNames: []string{"KAFKA_", "KAFKA_BOOTSTRAP"},
		name:     "kafka",
		image:    "confluentinc/cp-kafka:latest",
		port:     "9092",
	},
}

// InferDepsFromEnv scans .env files in a directory and its subdirectories
// to detect infrastructure dependencies.
func InferDepsFromEnv(rootDir string) ([]InferredDep, []InferredLink) {
	var deps []InferredDep
	var links []InferredLink
	seen := make(map[string]bool)

	// Scan root .env
	rootEnvFiles := findEnvFiles(rootDir)
	for _, envFile := range rootEnvFiles {
		found := scanEnvFile(envFile, "")
		for _, dep := range found {
			if !seen[dep.Name] {
				seen[dep.Name] = true
				dep.Source = relPath(rootDir, envFile) + ":" + dep.Source
				deps = append(deps, dep)
			}
		}
	}

	// Scan each subdirectory's .env files
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return deps, links
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		subDir := filepath.Join(rootDir, entry.Name())
		envFiles := findEnvFiles(subDir)
		serviceName := entry.Name()

		for _, envFile := range envFiles {
			found := scanEnvFile(envFile, serviceName)
			for _, dep := range found {
				if !seen[dep.Name] {
					seen[dep.Name] = true
					dep.Source = serviceName + "/" + filepath.Base(envFile) + ":" + dep.Source
					deps = append(deps, dep)
				}
				// Create a link: this service depends on this dep
				links = append(links, InferredLink{
					From:   serviceName,
					To:     dep.Name,
					Source:  serviceName + "/" + filepath.Base(envFile),
				})
			}
		}
	}

	return deps, links
}

// InferDepsFromCompose reads a docker-compose.yml and extracts infrastructure services.
func InferDepsFromCompose(composePath string) []InferredDep {
	data, err := os.ReadFile(composePath)
	if err != nil {
		return nil
	}

	var deps []InferredDep
	content := string(data)

	// Simple pattern matching for image: lines in compose
	imageRegex := regexp.MustCompile(`(?m)^\s+image:\s*["']?([^\s"']+)["']?`)
	serviceRegex := regexp.MustCompile(`(?m)^  (\w[\w-]*):\s*$`)

	services := serviceRegex.FindAllStringSubmatch(content, -1)
	images := imageRegex.FindAllStringSubmatch(content, -1)

	for i, svc := range services {
		name := svc[1]
		if name == "services" || name == "networks" || name == "volumes" {
			continue
		}

		image := ""
		if i < len(images) {
			image = images[i][1]
		}

		// Only include infra-looking services (known databases, caches, etc.)
		if isInfraImage(image) || isInfraName(name) {
			dep := InferredDep{
				Name:   name,
				Image:  image,
				Source: filepath.Base(composePath),
			}
			if port := inferPortFromImage(image); port != "" {
				dep.Port = port
			}
			deps = append(deps, dep)
		}
	}

	return deps
}

func findEnvFiles(dir string) []string {
	candidates := []string{".env", ".env.local", ".env.development", ".env.dev"}
	var found []string
	for _, name := range candidates {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			found = append(found, path)
		}
	}
	return found
}

func scanEnvFile(path, _ string) []InferredDep {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var deps []InferredDep
	seen := make(map[string]bool)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		envName := strings.TrimSpace(parts[0])
		envValue := strings.TrimSpace(parts[1])

		for _, known := range knownServices {
			if seen[known.name] {
				continue
			}

			matched := false

			// Check value patterns
			for _, pattern := range known.patterns {
				if strings.Contains(strings.ToLower(envValue), strings.ToLower(pattern)) {
					matched = true
					break
				}
			}

			// Check env var name patterns
			if !matched {
				for _, namePattern := range known.envNames {
					if strings.HasPrefix(strings.ToUpper(envName), namePattern) {
						matched = true
						break
					}
				}
			}

			if matched {
				seen[known.name] = true
				deps = append(deps, InferredDep{
					Name:   known.name,
					Image:  known.image,
					Port:   known.port,
					Source: envName,
				})
			}
		}
	}

	return deps
}

func isInfraImage(image string) bool {
	infraPrefixes := []string{
		"postgres", "mysql", "mariadb", "mongo", "redis",
		"rabbitmq", "nats", "kafka", "zookeeper", "elasticsearch",
		"memcached", "minio", "localstack", "mailhog", "mailpit",
	}
	lower := strings.ToLower(image)
	for _, prefix := range infraPrefixes {
		if strings.HasPrefix(lower, prefix) || strings.Contains(lower, "/"+prefix) {
			return true
		}
	}
	return false
}

func isInfraName(name string) bool {
	infraNames := []string{
		"db", "database", "postgres", "mysql", "mongo", "redis",
		"cache", "queue", "rabbitmq", "nats", "kafka", "minio",
		"elasticsearch", "elastic", "mail", "smtp",
	}
	lower := strings.ToLower(name)
	for _, n := range infraNames {
		if lower == n || strings.HasPrefix(lower, n+"-") || strings.HasSuffix(lower, "-"+n) {
			return true
		}
	}
	return false
}

func inferPortFromImage(image string) string {
	lower := strings.ToLower(image)
	for _, known := range knownServices {
		if strings.HasPrefix(lower, known.name) || strings.Contains(lower, "/"+known.name) {
			return known.port
		}
	}
	return ""
}

func relPath(base, path string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return filepath.Base(path)
	}
	return rel
}

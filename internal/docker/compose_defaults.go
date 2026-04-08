package docker

// addDefaultInfraEnv adds default environment variables for common infra services
func addDefaultInfraEnv(name, image string) map[string]string {
	envVars := make(map[string]string)

	// PostgreSQL defaults
	if image == "postgres" || name == "database" || name == "postgres" || name == "postgresql" {
		// Only add default if not already set via env_file
		envVars["POSTGRES_PASSWORD"] = "postgres"
		envVars["POSTGRES_USER"] = "postgres"
		envVars["POSTGRES_DB"] = "postgres"
	}

	return envVars
}

// addDefaultInfraHealthcheck adds default healthcheck configuration for common infra services
func addDefaultInfraHealthcheck(name, image string) map[string]any {
	// PostgreSQL healthcheck
	if image == "postgres" || name == "database" || name == "postgres" || name == "postgresql" {
		return map[string]any{
			"test": []string{
				"CMD-SHELL",
				"pg_isready -U ${POSTGRES_USER:-postgres} -d ${POSTGRES_DB:-postgres}",
			},
			"interval":     "5s",
			"timeout":      "5s",
			"retries":      10,
			"start_period": "10s",
		}
	}

	// PgAdmin healthcheck
	if image == "dpage/pgadmin4" || name == "pgadmin" {
		return map[string]any{
			"test": []string{
				"CMD-SHELL",
				"curl -f http://localhost/misc/ping 2>/dev/null || wget --no-verbose --tries=1 --spider http://localhost/misc/ping 2>/dev/null || exit 1",
			},
			"interval":     "30s",
			"timeout":      "10s",
			"retries":      5,
			"start_period": "40s",
		}
	}

	// Redis healthcheck
	if image == "redis" || name == "redis" {
		return map[string]any{
			"test": []string{
				"CMD-SHELL",
				"redis-cli ping | grep PONG",
			},
			"interval":     "10s",
			"timeout":      "5s",
			"retries":      5,
			"start_period": "10s",
		}
	}

	// MongoDB healthcheck
	if image == "mongo" || name == "mongo" || name == "mongodb" {
		return map[string]any{
			"test": []string{
				"CMD-SHELL",
				"mongosh --eval 'db.adminCommand(\"ping\")' | grep -q 'ok.*1'",
			},
			"interval":     "10s",
			"timeout":      "5s",
			"retries":      5,
			"start_period": "10s",
		}
	}

	// MySQL/MariaDB healthcheck
	if image == "mysql" || image == "mariadb" || name == "mysql" || name == "mariadb" {
		return map[string]any{
			"test": []string{
				"CMD-SHELL",
				"mysqladmin ping -h localhost || exit 1",
			},
			"interval":     "10s",
			"timeout":      "5s",
			"retries":      5,
			"start_period": "10s",
		}
	}

	return nil
}

package config

const SchemaJSON = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["schemaVersion", "project", "services", "env"],
  "properties": {
    "schemaVersion": {
      "type": "string",
      "pattern": "^1\\.0$"
    },
    "workspace": {
      "type": "string",
      "minLength": 1,
      "pattern": "^[a-z0-9-]+$",
      "description": "Workspace name (optional). If not specified, uses project.name as workspace. Multiple projects with the same workspace share the same workspace directory."
    },
    "profiles": {
      "type": "array",
      "items": {
        "type": "string",
        "minLength": 1,
        "pattern": "^[a-z0-9-]+$"
      },
      "description": "Default profiles when running raioz up without --profile. If missing or empty, all services and infra are started. Example: frontend, backend, load-balancer, nginx."
    },
    "network": {
      "oneOf": [
        {
          "type": "string",
          "minLength": 1,
          "pattern": "^[a-z0-9-]+$",
          "description": "Network name (simple format)"
        },
        {
          "type": "object",
          "required": ["name"],
          "properties": {
            "name": {
              "type": "string",
              "minLength": 1,
              "pattern": "^[a-z0-9-]+$",
              "description": "Network name"
            },
            "subnet": {
              "type": "string",
              "pattern": "^[0-9]+\\.[0-9]+\\.[0-9]+\\.[0-9]+/[0-9]+$",
              "description": "Network subnet in CIDR notation (e.g., '150.150.0.0/16')"
            }
          },
          "additionalProperties": false,
          "description": "Network configuration with optional subnet"
        }
      ],
      "description": "Network configuration (shared by workspace). Optional at root level - if not specified, will use project.network for backward compatibility. Can be a string (name only) or an object with name and optional subnet"
    },
    "project": {
      "type": "object",
      "required": ["name"],
      "properties": {
        "name": {
          "type": "string",
          "minLength": 1,
          "pattern": "^[a-z0-9-]+$",
          "description": "Project name (used for identification and as workspace name if workspace is not specified)"
        },
        "commands": {
          "type": "object",
          "properties": {
            "up": {
              "type": "string",
              "description": "Global command to start services when no docker or source.command is specified"
            },
            "down": {
              "type": "string",
              "description": "Global command to stop services when no docker or source.command is specified"
            },
            "health": {
              "type": "string",
              "description": "Command to check if the project is running. Should return exit code 0 if healthy, non-zero if not."
            },
            "dev": {
              "type": "object",
              "properties": {
                "up": {
                  "type": "string",
                  "description": "Command to start services in dev mode"
                },
                "down": {
                  "type": "string",
                  "description": "Command to stop services in dev mode"
                },
                "health": {
                  "type": "string",
                  "description": "Command to check health in dev mode"
                }
              },
              "additionalProperties": false
            },
            "prod": {
              "type": "object",
              "properties": {
                "up": {
                  "type": "string",
                  "description": "Command to start services in prod mode"
                },
                "down": {
                  "type": "string",
                  "description": "Command to stop services in prod mode"
                },
                "health": {
                  "type": "string",
                  "description": "Command to check health in prod mode"
                }
              },
              "additionalProperties": false
            }
          },
          "additionalProperties": false
        },
        "env": {
          "oneOf": [
            {
              "type": "array",
              "items": {
                "type": "string"
              },
              "description": "Array of file paths. Special case: [\".\"] means use .env in project directory as primary (read-only if exists)"
            },
            {
              "type": "object",
              "additionalProperties": {
                "type": "string"
              },
              "description": "Object with direct variables (e.g., {\"DATABASE_URL\": \"postgres://...\"})"
            }
          ],
          "description": "Project-level environment variables. If array with [\".\"], uses .env in project directory as primary (read-only if exists). If object, variables will be written to project.env"
        }
      },
      "additionalProperties": false
    },
    "services": {
      "type": "object",
      "minProperties": 0,
      "additionalProperties": {
        "type": "object",
        "required": ["source"],
        "properties": {
          "source": {
            "type": "object",
            "required": ["kind"],
            "properties": {
              "kind": {
                "type": "string",
                "enum": ["git", "image", "local"]
              },
              "repo": {
                "type": "string"
              },
              "branch": {
                "type": "string"
              },
              "path": {
                "type": "string"
              },
              "image": {
                "type": "string"
              },
              "tag": {
                "type": "string"
              },
              "access": {
                "type": "string",
                "enum": ["readonly", "editable"],
                "description": "Access mode for git repositories. 'readonly' prevents automatic checkout/pull. Only applies when kind == 'git'."
              },
              "command": {
                "type": "string",
                "description": "Command to run directly on the host (without Docker). If specified, the service will run on the host instead of in a container."
              },
              "runtime": {
                "type": "string",
                "enum": ["node", "go", "python", "java", "rust"],
                "description": "Runtime type for host execution (optional, for documentation purposes)"
              }
            },
            "additionalProperties": false,
            "allOf": [
              {
                "if": {
                  "properties": { "kind": { "const": "git" } }
                },
                "then": {
                  "required": ["repo", "branch", "path"]
                }
              },
              {
                "if": {
                  "properties": { "kind": { "const": "image" } }
                },
                "then": {
                  "required": ["image", "tag"]
                }
              },
              {
                "if": {
                  "properties": { "kind": { "const": "local" } }
                },
                "then": {
                  "required": ["path"]
                }
              }
            ]
          },
          "dependsOn": {
            "type": ["array", "null"],
            "items": {
              "type": "string"
            },
            "description": "Service/infra dependencies at service level (for local/host services or to combine with docker.dependsOn). Can be used together with docker.dependsOn."
          },
          "docker": {
            "type": ["object", "null"],
            "properties": {
              "mode": {
                "type": "string",
                "enum": ["dev", "prod"],
                "description": "Docker mode (required if docker section is present and source.command is not set)"
              },
              "ports": {
                "type": ["array", "null"],
                "items": {
                  "type": "string",
                  "pattern": "^[0-9]+:[0-9]+$"
                }
              },
              "volumes": {
                "type": ["array", "null"],
                "items": {
                  "type": "string"
                }
              },
              "dependsOn": {
                "type": ["array", "null"],
                "items": {
                  "type": "string"
                }
              },
              "dockerfile": {
                "type": "string"
              },
              "command": {
                "type": "string",
                "description": "Command to run inside the Docker container (for wrapper mode). This is different from source.command which runs on the host."
              },
              "runtime": {
                "type": "string",
                "enum": ["node", "go", "python", "java", "rust"],
                "description": "Runtime type for Docker wrapper mode (optional, for documentation purposes)"
              },
              "ip": {
                "oneOf": [
                  { "type": "string", "pattern": "^[0-9]+\\.[0-9]+\\.[0-9]+\\.[0-9]+$" },
                  { "type": "string", "pattern": "^\\$\\{[A-Za-z_][A-Za-z0-9_]*\\}$" }
                ],
                "description": "Static IP address in the network (e.g., '150.150.0.10') or env var (e.g., '${NGINX_IP}'). Only works if network has a subnet configured."
              },
              "envVolume": {
                "type": "string",
                "description": "Optional: mount the generated .env file as a volume at this path inside the container (e.g., '/app/.env'). The .env file will be available both via env_file (for environment variables) and as a mounted file at this path."
              },
              "healthcheck": {
                "type": "object",
                "description": "Docker healthcheck (same format as docker-compose). test: string or array (e.g. [\"CMD\", \"curl\", \"-f\", \"http://localhost/health\"]), interval, timeout, retries, start_period, start_interval (duration strings), disable (bool).",
                "properties": {
                  "test": {
                    "oneOf": [
                      { "type": "string" },
                      { "type": "array", "items": { "type": "string" } }
                    ]
                  },
                  "interval": { "type": "string" },
                  "timeout": { "type": "string" },
                  "retries": { "type": "integer" },
                  "start_period": { "type": "string" },
                  "start_interval": { "type": "string" },
                  "disable": { "type": "boolean" }
                }
              }
            },
            "additionalProperties": false
          },
          "env": {
            "oneOf": [
              {
                "type": "array",
                "items": {
                  "type": "string"
                },
                "description": "Array of file paths (e.g., [\"local-deps\", \"services/shared\"])"
              },
              {
                "type": "object",
                "additionalProperties": {
                  "type": "string"
                },
                "description": "Object with direct variables (e.g., {\"DATABASE_URL\": \"postgres://...\", \"API_KEY\": \"...\"})"
              }
            ],
            "description": "Can be either an array of file paths or an object with variables. If object, variables will be written to projects/{project}/services/{service}.env"
          },
          "profiles": {
            "type": "array",
            "items": {
              "type": "string",
              "minLength": 1,
              "pattern": "^[a-z0-9-]+$"
            },
            "description": "Include this service only when using raioz up --profile <name> or when profile is in root profiles. Example: frontend, backend, load-balancer."
          },
          "enabled": {
            "type": ["boolean", "null"],
            "description": "Explicitly enable or disable the service. Defaults to true if not specified. Disabled services are not cloned, built, or started."
          },
          "mock": {
            "type": "object",
            "properties": {
              "enabled": {
                "type": "boolean"
              },
              "image": {
                "type": "string"
              },
              "tag": {
                "type": "string"
              },
              "ports": {
                "type": "array",
                "items": {
                  "type": "string",
                  "pattern": "^[0-9]+:[0-9]+$"
                }
              },
              "env": {
                "type": "array",
                "items": {
                  "type": "string"
                }
              }
            },
            "additionalProperties": false
          },
          "featureFlag": {
            "type": "object",
            "properties": {
              "enabled": {
                "type": "boolean"
              },
              "disabled": {
                "type": "boolean"
              },
              "envVar": {
                "type": "string"
              },
              "envValue": {
                "type": "string"
              },
              "profiles": {
                "type": "array",
                "items": {
                  "type": "string",
                  "minLength": 1,
                  "pattern": "^[a-z0-9-]+$"
                }
              }
            },
            "additionalProperties": false
          },
          "commands": {
            "type": "object",
            "properties": {
              "up": {
                "type": "string",
                "description": "Command to start this service when no docker or source.command is specified"
              },
              "down": {
                "type": "string",
                "description": "Command to stop this service when no docker or source.command is specified"
              },
              "health": {
                "type": "string",
                "description": "Command to check if this service is running. Should return exit code 0 if healthy, non-zero if not."
              },
              "dev": {
                "type": "object",
                "properties": {
                  "up": {
                    "type": "string",
                    "description": "Command to start this service in dev mode"
                  },
                  "down": {
                    "type": "string",
                    "description": "Command to stop this service in dev mode"
                  },
                  "health": {
                    "type": "string",
                    "description": "Command to check health in dev mode"
                  }
                },
                "additionalProperties": false
              },
              "prod": {
                "type": "object",
                "properties": {
                  "up": {
                    "type": "string",
                    "description": "Command to start this service in prod mode"
                  },
                  "down": {
                    "type": "string",
                    "description": "Command to stop this service in prod mode"
                  },
                  "health": {
                    "type": "string",
                    "description": "Command to check health in prod mode"
                  }
                },
                "additionalProperties": false
              }
            },
            "additionalProperties": false
          },
          "volumes": {
            "type": "array",
            "items": {
              "type": "string",
              "pattern": "^.+:.+$",
              "description": "Symlink mapping in format 'SRC:DEST' where SRC is relative to project directory (or absolute) and DEST is relative to service directory"
            },
            "description": "For host services: create symlinks from project paths to service directory. Format: 'SRC:DEST' (e.g., './certs:certs' creates symlink from project/certs to service/certs)"
          }
        },
        "additionalProperties": false
      }
    },
    "infra": {
      "type": "object",
      "additionalProperties": {
        "oneOf": [
          {
            "type": "string",
            "minLength": 1,
            "description": "Path to a Docker Compose YAML fragment (relative to config file or absolute). Merged into the generated compose."
          },
          {
            "type": "object",
            "required": ["image"],
        "properties": {
          "image": {
            "type": "string",
            "minLength": 1
          },
          "tag": {
            "type": "string"
          },
          "ports": {
            "type": ["array", "null"],
            "items": {
              "type": "string",
              "pattern": "^[0-9]+:[0-9]+$"
            },
            "description": "Array of port mappings (e.g., [\"5432:5432\"]) or null if no ports"
          },
          "volumes": {
            "type": ["array", "null"],
            "items": {
              "type": "string"
            },
            "description": "Array of volume mappings or null if no volumes"
          },
          "ip": {
            "oneOf": [
              { "type": "string", "pattern": "^[0-9]+\\.[0-9]+\\.[0-9]+\\.[0-9]+$" },
              { "type": "string", "pattern": "^\\$\\{[A-Za-z_][A-Za-z0-9_]*\\}$" }
            ],
            "description": "Static IP address in the network (e.g., '150.150.0.10') or env var (e.g., '${POSTGRES_IP}'). Only works if project.network has a subnet configured."
          },
          "healthcheck": {
            "type": "object",
            "description": "Docker healthcheck (same format as docker-compose). test: string or array, interval, timeout, retries, start_period, start_interval (duration strings), disable (bool).",
            "properties": {
              "test": {
                "oneOf": [
                  { "type": "string" },
                  { "type": "array", "items": { "type": "string" } }
                ]
              },
              "interval": { "type": "string" },
              "timeout": { "type": "string" },
              "retries": { "type": "integer" },
              "start_period": { "type": "string" },
              "start_interval": { "type": "string" },
              "disable": { "type": "boolean" }
            }
          },
          "env": {
            "oneOf": [
              {
                "type": "array",
                "items": {
                  "type": "string"
                },
                "description": "Array of file paths (e.g., [\"local-deps\", \"services/shared\"])"
              },
              {
                "type": "object",
                "additionalProperties": {
                  "type": "string"
                },
                "description": "Object with direct variables (e.g., {\"DATABASE_URL\": \"postgres://...\", \"API_KEY\": \"...\"})"
              }
            ],
            "description": "Can be either an array of file paths or an object with variables. If object, variables will be written to projects/{project}/services/{service}.env"
          },
          "profiles": {
            "type": "array",
            "items": {
              "type": "string",
              "minLength": 1,
              "pattern": "^[a-z0-9-]+$"
            },
            "description": "If set, this infra is only included when using raioz up --profile <name> or when profile is in root profiles."
          },
          "seed": {
            "type": "array",
            "items": {
              "type": "string"
            },
            "description": "Files or directories to mount in /docker-entrypoint-initdb.d/ for automatic database initialization (supports postgres, mysql, mongo)"
          }
        },
            "additionalProperties": false
          }
        ]
      }
    },
    "env": {
      "type": "object",
      "required": ["useGlobal", "files"],
      "properties": {
        "useGlobal": {
          "type": "boolean"
        },
        "files": {
          "type": "array",
          "items": {
            "type": "string"
          }
        },
        "variables": {
          "type": "object",
          "additionalProperties": {
            "type": "string"
          },
          "description": "Direct variables to write/update in global.env. These variables will be available to all services."
        }
      },
      "additionalProperties": false
    }
  },
  "additionalProperties": false
}`

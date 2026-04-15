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
      "description": "Workspace name (optional). If not specified, uses project.name as workspace."
    },
    "profiles": {
      "type": "array",
      "items": {
        "type": "string",
        "minLength": 1,
        "pattern": "^[a-z0-9-]+$"
      },
      "description": "Default profiles when running raioz up without --profile."
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
      "description": "Network config (shared by workspace). String or object."
    },
    "project": {
      "type": "object",
      "required": ["name"],
      "properties": {
        "name": {
          "type": "string",
          "minLength": 1,
          "pattern": "^[a-z0-9-]+$",
          "description": "Project name (used for identification)"
        },
        "commands": {
          "type": "object",
          "properties": {
            "up": {
              "type": "string",
              "description": "Global command to start services"
            },
            "down": {
              "type": "string",
              "description": "Global command to stop services"
            },
            "health": {
              "type": "string",
              "description": "Health check command. Exit 0 if healthy."
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
              "description": "Array of file paths. ['.'] uses .env in project dir."
            },
            {
              "type": "object",
              "additionalProperties": {
                "type": "string"
              },
              "description": "Object with direct variables"
            }
          ],
          "description": "Project-level environment variables."
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
                "description": "Access mode for git repos. Only for kind == 'git'."
              },
              "command": {
                "type": "string",
                "description": "Command to run on the host (without Docker)."
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
            "description": "Service/infra dependencies at service level."
          },
          "docker": {
            "type": ["object", "null"],
            "properties": {
              "mode": {
                "type": "string",
                "enum": ["dev", "prod"],
                "description": "Docker mode (required if docker section is present)"
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
                "description": "Command to run inside the Docker container."
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
                "description": "Static IP or env var. Requires subnet."
              },
              "envVolume": {
                "type": "string",
                "description": "Mount .env file at this container path."
              },
              "healthcheck": {
                "type": "object",
                "description": "Docker healthcheck (same as docker-compose).",
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
                "description": "Array of file paths"
              },
              {
                "type": "object",
                "additionalProperties": {
                  "type": "string"
                },
                "description": "Object with direct variables"
              }
            ],
            "description": "Array of file paths or object with variables."
          },
          "profiles": {
            "type": "array",
            "items": {
              "type": "string",
              "minLength": 1,
              "pattern": "^[a-z0-9-]+$"
            },
            "description": "Include only when using --profile <name>."
          },
          "enabled": {
            "type": ["boolean", "null"],
            "description": "Enable or disable the service. Default: true."
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
                "description": "Command to start this service"
              },
              "down": {
                "type": "string",
                "description": "Command to stop this service"
              },
              "health": {
                "type": "string",
                "description": "Health check command. Exit 0 if healthy."
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
              "description": "Symlink mapping 'SRC:DEST'"
            },
            "description": "Host services: create symlinks. Format: 'SRC:DEST'."
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
            "description": "Path to a Docker Compose YAML fragment."
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
            "description": "Static IP or env var. Requires subnet."
          },
          "healthcheck": {
            "type": "object",
            "description": "Docker healthcheck (same as docker-compose).",
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
                "description": "Object with direct variables"
              }
            ],
            "description": "Array of file paths or object with variables."
          },
          "profiles": {
            "type": "array",
            "items": {
              "type": "string",
              "minLength": 1,
              "pattern": "^[a-z0-9-]+$"
            },
            "description": "Include only when using --profile <name>."
          },
          "seed": {
            "type": "array",
            "items": {
              "type": "string"
            },
            "description": "Files/dirs for /docker-entrypoint-initdb.d/"
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
          "description": "Direct variables for global.env."
        }
      },
      "additionalProperties": false
    }
  },
  "additionalProperties": false
}`

package upcase

import (
	"reflect"
	"testing"

	"raioz/internal/config"
	"raioz/internal/detect"
)

// --- parseHealthCommandOutput --------------------------------------------------

func TestParseHealthCommandOutput(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"on lowercase", "on", true},
		{"on uppercase", "ON", true},
		{"off lowercase", "off", false},
		{"off mixed", "Off", false},
		{"json active", `{"status":"active"}`, true},
		{"json running", `{"status":"running"}`, true},
		{"json healthy", `{"status":"healthy"}`, true},
		{"json up", `{"status":"up"}`, true},
		{"json on", `{"status":"on"}`, true},
		{"json inactive", `{"status":"inactive"}`, false},
		{"json stopped", `{"status":"stopped"}`, false},
		{"json unhealthy", `{"status":"unhealthy"}`, false},
		{"json down", `{"status":"down"}`, false},
		{"json off", `{"status":"off"}`, false},
		{"json no status", `{"foo":"bar"}`, true},
		{"json unknown status", `{"status":"weird"}`, true},
		{"non-json", "some output", true},
		{"empty", "", true},
		{"whitespace only", "   ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseHealthCommandOutput(tt.in)
			if got != tt.want {
				t.Errorf("parseHealthCommandOutput(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// --- getServiceHealthCommand ---------------------------------------------------

func TestGetServiceHealthCommand(t *testing.T) {
	tests := []struct {
		name string
		svc  config.Service
		mode string
		want string
	}{
		{
			name: "no commands",
			svc:  config.Service{},
			mode: "dev",
			want: "",
		},
		{
			name: "root health only",
			svc: config.Service{
				Commands: &config.ServiceCommands{Health: "check.sh"},
			},
			mode: "dev",
			want: "check.sh",
		},
		{
			name: "dev takes precedence over root",
			svc: config.Service{
				Commands: &config.ServiceCommands{
					Health: "root.sh",
					Dev:    &config.EnvironmentCommands{Health: "dev.sh"},
				},
			},
			mode: "dev",
			want: "dev.sh",
		},
		{
			name: "prod mode uses prod command",
			svc: config.Service{
				Commands: &config.ServiceCommands{
					Dev:  &config.EnvironmentCommands{Health: "dev.sh"},
					Prod: &config.EnvironmentCommands{Health: "prod.sh"},
				},
			},
			mode: "prod",
			want: "prod.sh",
		},
		{
			name: "empty mode defaults to dev",
			svc: config.Service{
				Commands: &config.ServiceCommands{
					Dev: &config.EnvironmentCommands{Health: "dev.sh"},
				},
			},
			mode: "",
			want: "dev.sh",
		},
		{
			name: "prod mode falls back to root when no prod",
			svc: config.Service{
				Commands: &config.ServiceCommands{Health: "root.sh"},
			},
			mode: "prod",
			want: "root.sh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getServiceHealthCommand(tt.svc, tt.mode)
			if got != tt.want {
				t.Errorf("getServiceHealthCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- getLocalProjectCommand ----------------------------------------------------

func TestGetLocalProjectCommand(t *testing.T) {
	tests := []struct {
		name    string
		deps    *config.Deps
		cmdType string
		mode    string
		want    string
	}{
		{
			name:    "nil commands",
			deps:    &config.Deps{Project: config.Project{Name: "p"}},
			cmdType: "up",
			mode:    "dev",
			want:    "",
		},
		{
			name: "up with dev command",
			deps: &config.Deps{Project: config.Project{
				Commands: &config.ProjectCommands{
					Up:  "make",
					Dev: &config.EnvironmentCommands{Up: "make dev"},
				},
			}},
			cmdType: "up",
			mode:    "dev",
			want:    "make dev",
		},
		{
			name: "up default mode is dev",
			deps: &config.Deps{Project: config.Project{
				Commands: &config.ProjectCommands{
					Dev: &config.EnvironmentCommands{Up: "make dev"},
				},
			}},
			cmdType: "up",
			mode:    "",
			want:    "make dev",
		},
		{
			name: "up prod",
			deps: &config.Deps{Project: config.Project{
				Commands: &config.ProjectCommands{
					Prod: &config.EnvironmentCommands{Up: "make prod"},
				},
			}},
			cmdType: "up",
			mode:    "prod",
			want:    "make prod",
		},
		{
			name: "up falls back to root",
			deps: &config.Deps{Project: config.Project{
				Commands: &config.ProjectCommands{Up: "make"},
			}},
			cmdType: "up",
			mode:    "dev",
			want:    "make",
		},
		{
			name: "down dev",
			deps: &config.Deps{Project: config.Project{
				Commands: &config.ProjectCommands{
					Dev: &config.EnvironmentCommands{Down: "make stop"},
				},
			}},
			cmdType: "down",
			mode:    "dev",
			want:    "make stop",
		},
		{
			name: "down prod",
			deps: &config.Deps{Project: config.Project{
				Commands: &config.ProjectCommands{
					Prod: &config.EnvironmentCommands{Down: "make stop prod"},
				},
			}},
			cmdType: "down",
			mode:    "prod",
			want:    "make stop prod",
		},
		{
			name: "down root fallback",
			deps: &config.Deps{Project: config.Project{
				Commands: &config.ProjectCommands{Down: "make down"},
			}},
			cmdType: "down",
			mode:    "dev",
			want:    "make down",
		},
		{
			name: "health dev",
			deps: &config.Deps{Project: config.Project{
				Commands: &config.ProjectCommands{
					Dev: &config.EnvironmentCommands{Health: "check.sh"},
				},
			}},
			cmdType: "health",
			mode:    "dev",
			want:    "check.sh",
		},
		{
			name: "health prod",
			deps: &config.Deps{Project: config.Project{
				Commands: &config.ProjectCommands{
					Prod: &config.EnvironmentCommands{Health: "prod-check.sh"},
				},
			}},
			cmdType: "health",
			mode:    "prod",
			want:    "prod-check.sh",
		},
		{
			name: "health root fallback",
			deps: &config.Deps{Project: config.Project{
				Commands: &config.ProjectCommands{Health: "check"},
			}},
			cmdType: "health",
			mode:    "dev",
			want:    "check",
		},
		{
			name: "unknown command type",
			deps: &config.Deps{Project: config.Project{
				Commands: &config.ProjectCommands{Up: "make"},
			}},
			cmdType: "restart",
			mode:    "dev",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getLocalProjectCommand(tt.deps, tt.cmdType, tt.mode)
			if got != tt.want {
				t.Errorf("getLocalProjectCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- isYAMLMode ----------------------------------------------------------------

func TestIsYAMLMode(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{"yaml v2", "2.0", true},
		{"json v1", "1.0", false},
		{"empty", "", false},
		{"unknown", "3.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &config.Deps{SchemaVersion: tt.version}
			got := isYAMLMode(d)
			if got != tt.want {
				t.Errorf("isYAMLMode(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

// --- buildServiceContext -------------------------------------------------------

func TestBuildServiceContext(t *testing.T) {
	det := detect.DetectResult{Runtime: detect.RuntimeGo, StartCommand: "go run ."}
	envVars := map[string]string{"FOO": "bar"}
	ports := []string{"8080:8080"}
	deps := []string{"postgres"}

	ctx := buildServiceContext(
		"api", det, "acme-net", envVars, ports, deps,
		"acme-api", "/path/to/api", "acme",
	)

	if ctx.Name != "api" {
		t.Errorf("Name = %q, want api", ctx.Name)
	}
	if ctx.NetworkName != "acme-net" {
		t.Errorf("NetworkName = %q, want acme-net", ctx.NetworkName)
	}
	if ctx.ProjectName != "acme" {
		t.Errorf("ProjectName = %q, want acme", ctx.ProjectName)
	}
	if ctx.ContainerName != "acme-api" {
		t.Errorf("ContainerName = %q, want acme-api", ctx.ContainerName)
	}
	if ctx.Path != "/path/to/api" {
		t.Errorf("Path = %q, want /path/to/api", ctx.Path)
	}
	if len(ctx.Ports) != 1 || ctx.Ports[0] != "8080:8080" {
		t.Errorf("Ports = %v, want [8080:8080]", ctx.Ports)
	}
	if len(ctx.DependsOn) != 1 || ctx.DependsOn[0] != "postgres" {
		t.Errorf("DependsOn = %v, want [postgres]", ctx.DependsOn)
	}
	if ctx.EnvVars["FOO"] != "bar" {
		t.Errorf("EnvVars[FOO] = %q, want bar", ctx.EnvVars["FOO"])
	}
	if ctx.Detection.Runtime != detect.RuntimeGo {
		t.Errorf("Detection.Runtime = %q, want go", ctx.Detection.Runtime)
	}
}

// --- infraPorts / servicePorts -------------------------------------------------

func TestInfraPorts(t *testing.T) {
	t.Run("inline with ports", func(t *testing.T) {
		entry := config.InfraEntry{Inline: &config.Infra{Ports: []string{"5432"}}}
		got := infraPorts(entry)
		if len(got) != 1 || got[0] != "5432" {
			t.Errorf("got %v, want [5432]", got)
		}
	})
	t.Run("nil inline", func(t *testing.T) {
		entry := config.InfraEntry{}
		got := infraPorts(entry)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
}

func TestServicePorts(t *testing.T) {
	t.Run("docker ports", func(t *testing.T) {
		svc := config.Service{Docker: &config.DockerConfig{Ports: []string{"3000:3000"}}}
		got := servicePorts(svc)
		if len(got) != 1 || got[0] != "3000:3000" {
			t.Errorf("got %v, want [3000:3000]", got)
		}
	})
	t.Run("no docker", func(t *testing.T) {
		svc := config.Service{}
		got := servicePorts(svc)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
}

// --- orderedServiceNames -------------------------------------------------------

func TestOrderedServiceNames(t *testing.T) {
	t.Run("no dependencies", func(t *testing.T) {
		deps := &config.Deps{
			Services: map[string]config.Service{
				"a": {},
				"b": {},
			},
		}
		got := orderedServiceNames(deps)
		if len(got) != 2 {
			t.Errorf("expected 2 services, got %d", len(got))
		}
	})
	t.Run("linear chain", func(t *testing.T) {
		deps := &config.Deps{
			Services: map[string]config.Service{
				"web": {DependsOn: []string{"api"}},
				"api": {DependsOn: []string{"db"}},
				"db":  {},
			},
		}
		got := orderedServiceNames(deps)
		if len(got) != 3 {
			t.Fatalf("expected 3, got %d", len(got))
		}
		// db must come before api, api before web
		idx := map[string]int{}
		for i, n := range got {
			idx[n] = i
		}
		if idx["db"] > idx["api"] {
			t.Errorf("db should come before api: %v", got)
		}
		if idx["api"] > idx["web"] {
			t.Errorf("api should come before web: %v", got)
		}
	})
	t.Run("ignores infra deps", func(t *testing.T) {
		deps := &config.Deps{
			Services: map[string]config.Service{
				"api": {DependsOn: []string{"postgres"}},
			},
			Infra: map[string]config.InfraEntry{
				"postgres": {},
			},
		}
		got := orderedServiceNames(deps)
		if len(got) != 1 || got[0] != "api" {
			t.Errorf("expected [api], got %v", got)
		}
	})
}

// --- mergeSliceUnique ----------------------------------------------------------

func TestMergeSliceUnique(t *testing.T) {
	tests := []struct {
		name string
		a, b []string
		want []string
	}{
		{"both empty", nil, nil, nil},
		{"only a", []string{"x", "y"}, nil, []string{"x", "y"}},
		{"only b", nil, []string{"x", "y"}, []string{"x", "y"}},
		{"disjoint", []string{"a"}, []string{"b"}, []string{"a", "b"}},
		{"overlap", []string{"a", "b"}, []string{"b", "c"}, []string{"a", "b", "c"}},
		{"a dups", []string{"a", "a", "b"}, nil, []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeSliceUnique(tt.a, tt.b)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d (got=%v)", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// --- mergeVariables ------------------------------------------------------------

func TestMergeVariables(t *testing.T) {
	t.Run("both nil", func(t *testing.T) {
		got := mergeVariables(nil, nil)
		if got == nil {
			t.Error("expected non-nil map")
		}
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})
	t.Run("new overrides old", func(t *testing.T) {
		old := map[string]string{"FOO": "1", "BAR": "old"}
		new := map[string]string{"BAR": "new", "BAZ": "3"}
		got := mergeVariables(old, new)
		if got["FOO"] != "1" {
			t.Errorf("FOO = %q, want 1", got["FOO"])
		}
		if got["BAR"] != "new" {
			t.Errorf("BAR = %q, want new", got["BAR"])
		}
		if got["BAZ"] != "3" {
			t.Errorf("BAZ = %q, want 3", got["BAZ"])
		}
	})
	t.Run("nil new", func(t *testing.T) {
		old := map[string]string{"K": "v"}
		got := mergeVariables(old, nil)
		if got["K"] != "v" {
			t.Errorf("K = %q, want v", got["K"])
		}
	})
}

// --- volumeContainerPath / mergeVolumesOnlyNew --------------------------------

func TestVolumeContainerPath(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"/host:/container", "/container"},
		{"/host:/container:ro", "/container:ro"},
		{"justone", "justone"},
		{"named:vol", "vol"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := volumeContainerPath(tt.in)
			if got != tt.want {
				t.Errorf("volumeContainerPath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestMergeVolumesOnlyNew(t *testing.T) {
	t.Run("disjoint", func(t *testing.T) {
		base := []string{"/h1:/c1"}
		add := []string{"/h2:/c2"}
		got := mergeVolumesOnlyNew(base, add)
		if len(got) != 2 {
			t.Errorf("expected 2, got %v", got)
		}
	})
	t.Run("duplicate container path filtered", func(t *testing.T) {
		base := []string{"/h1:/container"}
		add := []string{"/h2:/container"} // same container path
		got := mergeVolumesOnlyNew(base, add)
		if len(got) != 1 {
			t.Errorf("expected dedup to 1, got %v", got)
		}
		if got[0] != "/h1:/container" {
			t.Errorf("expected base kept, got %v", got)
		}
	})
	t.Run("base dedupes itself", func(t *testing.T) {
		base := []string{"/a:/c", "/b:/c"}
		got := mergeVolumesOnlyNew(base, nil)
		if len(got) != 1 {
			t.Errorf("expected 1, got %v", got)
		}
	})
}

// --- cloneService --------------------------------------------------------------

func TestCloneService(t *testing.T) {
	enabled := true
	orig := config.Service{
		Source:    config.SourceConfig{Kind: "git", Repo: "r"},
		DependsOn: []string{"a", "b"},
		Volumes:   []string{"/x:/y"},
		Profiles:  []string{"dev"},
		Enabled:   &enabled,
		Hostname:  "api.local",
		Docker: &config.DockerConfig{
			Mode:      "dev",
			Ports:     []string{"3000"},
			Volumes:   []string{"/a:/b"},
			DependsOn: []string{"db"},
			IP:        "10.0.0.1",
		},
	}

	cloned := cloneService(orig)

	if cloned.Source.Kind != "git" {
		t.Errorf("Source.Kind = %q, want git", cloned.Source.Kind)
	}
	if len(cloned.DependsOn) != 2 {
		t.Errorf("DependsOn len = %d", len(cloned.DependsOn))
	}
	if cloned.Hostname != "api.local" {
		t.Errorf("Hostname = %q", cloned.Hostname)
	}
	if cloned.Docker == nil {
		t.Fatal("Docker should not be nil")
	}
	if cloned.Docker.Mode != "dev" {
		t.Errorf("Docker.Mode = %q", cloned.Docker.Mode)
	}
	if cloned.Docker.IP != "10.0.0.1" {
		t.Errorf("Docker.IP = %q", cloned.Docker.IP)
	}

	// Verify deep copy: mutating clone should not affect original
	cloned.DependsOn[0] = "mutated"
	if orig.DependsOn[0] == "mutated" {
		t.Error("cloneService did not deep-copy DependsOn")
	}
	cloned.Docker.Ports[0] = "mutated"
	if orig.Docker.Ports[0] == "mutated" {
		t.Error("cloneService did not deep-copy Docker.Ports")
	}
}

func TestCloneServiceNilDocker(t *testing.T) {
	orig := config.Service{Source: config.SourceConfig{Kind: "local"}}
	cloned := cloneService(orig)
	if cloned.Docker != nil {
		t.Errorf("Docker should stay nil, got %+v", cloned.Docker)
	}
}

// TestCloneService_PreservesAllFields_Issue006: generative guard. Populates
// every exported field of config.Service via reflection, clones it, then
// walks the clone and fails for any field that came back zero. Prevents
// the recurring "someone added a field but forgot to copy it in
// cloneService" bug — already happened twice (ProxyOverride in the past,
// HostnameAliases now). If this test fails on a new field X, adding
// `X: s.X` (or deep-copy equivalent) to cloneService is the fix.
func TestCloneService_PreservesAllFields_Issue006(t *testing.T) {
	var orig config.Service
	setAllFieldsNonZero(reflect.ValueOf(&orig).Elem())

	cloned := cloneService(orig)

	v := reflect.ValueOf(cloned)
	typ := v.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		if v.Field(i).IsZero() {
			t.Errorf("cloneService dropped field %s — add it to cloneService "+
				"in workspace_project_conflict.go", field.Name)
		}
	}
}

// TestCloneInfraEntry_PreservesAllFields_Issue006: same guard for
// cloneInfraEntry. Infra has a bigger footprint (Seed, Expose, Publish…)
// and the same historical pattern of silent drops on merge.
func TestCloneInfraEntry_PreservesAllFields_Issue006(t *testing.T) {
	var orig config.Infra
	setAllFieldsNonZero(reflect.ValueOf(&orig).Elem())
	entry := config.InfraEntry{Path: "/p", Inline: &orig}

	cloned := cloneInfraEntry(entry)
	if cloned.Inline == nil {
		t.Fatal("cloned.Inline is nil")
	}

	v := reflect.ValueOf(*cloned.Inline)
	typ := v.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		if v.Field(i).IsZero() {
			t.Errorf("cloneInfraEntry dropped field %s — add it to "+
				"cloneInfraEntry in workspace_project_conflict.go", field.Name)
		}
	}
}

// setAllFieldsNonZero recursively walks a struct via reflection and sets
// every settable field to a non-zero value. Used by the generative
// preservation tests above — the IsZero check on the clone only
// distinguishes "dropped" from "kept" if the original was non-zero to
// begin with.
func setAllFieldsNonZero(v reflect.Value) {
	switch v.Kind() {
	case reflect.String:
		v.SetString("x")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1)
	case reflect.Slice:
		// Non-nil len-1 slice; the element stays zero but IsZero on the
		// slice header returns false, which is what the preservation
		// check looks at.
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
	case reflect.Map:
		v.Set(reflect.MakeMapWithSize(v.Type(), 0))
	case reflect.Ptr:
		// Non-nil pointer to a fresh element. Recurse so nested structs
		// also come back non-zero (a pointer to an all-zero struct would
		// report zero for its fields but the POINTER itself is non-zero,
		// which is all the clone check cares about).
		v.Set(reflect.New(v.Type().Elem()))
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			f := v.Field(i)
			if f.CanSet() {
				setAllFieldsNonZero(f)
			}
		}
	}
}

// TestCloneService_HostnameAliases_Issue006: on workspace merge, the old
// project's HostnameAliases must survive the clone or the proxy silently
// drops the extra hostnames the user declared — same class of bug as the
// previously-fixed ProxyOverride drop that already lives in cloneService.
func TestCloneService_HostnameAliases_Issue006(t *testing.T) {
	orig := config.Service{
		Hostname:        "sso",
		HostnameAliases: []string{"accounts", "login"},
	}
	cloned := cloneService(orig)
	if cloned.Hostname != "sso" {
		t.Errorf("Hostname = %q, want sso", cloned.Hostname)
	}
	if len(cloned.HostnameAliases) != 2 ||
		cloned.HostnameAliases[0] != "accounts" ||
		cloned.HostnameAliases[1] != "login" {
		t.Errorf("HostnameAliases = %v, want [accounts login]", cloned.HostnameAliases)
	}
	cloned.HostnameAliases[0] = "mut"
	if orig.HostnameAliases[0] == "mut" {
		t.Error("cloneService did not deep-copy HostnameAliases")
	}
}

// --- cloneInfraEntry -----------------------------------------------------------

func TestCloneInfraEntry(t *testing.T) {
	t.Run("with inline", func(t *testing.T) {
		orig := config.InfraEntry{
			Path: "/some/path",
			Inline: &config.Infra{
				Image:   "postgres",
				Tag:     "16",
				Ports:   []string{"5432"},
				Volumes: []string{"data:/var"},
				IP:      "10.0.0.2",
			},
		}
		cloned := cloneInfraEntry(orig)
		if cloned.Path != "/some/path" {
			t.Errorf("Path = %q", cloned.Path)
		}
		if cloned.Inline == nil {
			t.Fatal("Inline should not be nil")
		}
		if cloned.Inline.Image != "postgres" {
			t.Errorf("Image = %q", cloned.Inline.Image)
		}
		// Mutate clone, ensure independent
		cloned.Inline.Ports[0] = "mut"
		if orig.Inline.Ports[0] == "mut" {
			t.Error("cloneInfraEntry did not deep-copy Inline.Ports")
		}
	})
	t.Run("nil inline", func(t *testing.T) {
		orig := config.InfraEntry{Path: "/p"}
		cloned := cloneInfraEntry(orig)
		if cloned.Inline != nil {
			t.Error("Inline should stay nil")
		}
	})
	// Issue #006 sibling: cloneInfraEntry was dropping Hostname and
	// HostnameAliases — latent bug that would bite the moment someone
	// reached a dep under aliases (e.g. `pg.example.dev` + `db.example.dev`
	// for the same postgres) across a workspace merge.
	t.Run("hostname and aliases round-trip", func(t *testing.T) {
		orig := config.InfraEntry{
			Inline: &config.Infra{
				Image:           "mailhog/mailhog",
				Hostname:        "mail",
				HostnameAliases: []string{"smtp", "inbox"},
			},
		}
		cloned := cloneInfraEntry(orig)
		if cloned.Inline == nil {
			t.Fatal("Inline should not be nil")
		}
		if cloned.Inline.Hostname != "mail" {
			t.Errorf("Hostname = %q, want mail", cloned.Inline.Hostname)
		}
		if len(cloned.Inline.HostnameAliases) != 2 {
			t.Errorf("HostnameAliases len = %d, want 2", len(cloned.Inline.HostnameAliases))
		}
		cloned.Inline.HostnameAliases[0] = "mut"
		if orig.Inline.HostnameAliases[0] == "mut" {
			t.Error("cloneInfraEntry did not deep-copy HostnameAliases")
		}
	})
}

// --- inferServicePort additional cases ----------------------------------------

func TestInferServicePortConfigPriority(t *testing.T) {
	svc := config.Service{
		Docker: &config.DockerConfig{Ports: []string{"4242:80"}},
	}
	det := detect.DetectResult{Runtime: detect.RuntimeGo}
	got := inferServicePort(svc, det)
	if got != 4242 {
		t.Errorf("config port should win, got %d", got)
	}
}

func TestInferServicePortUnknownRuntime(t *testing.T) {
	svc := config.Service{}
	det := detect.DetectResult{Runtime: detect.Runtime("weird")}
	got := inferServicePort(svc, det)
	if got != 0 {
		t.Errorf("expected 0 for unknown runtime, got %d", got)
	}
}

// --- isProcessAlive / isProcessRunning ----------------------------------------

func TestIsProcessAliveCurrent(t *testing.T) {
	// Current process must be alive
	if !isProcessAlive(1) && !isProcessAlive(2) {
		// Skip if we can't inspect low PIDs (e.g., sandbox)
		t.Skip("cannot inspect low PIDs in this environment")
	}
}

func TestIsProcessAliveInvalid(t *testing.T) {
	if isProcessAlive(-1) {
		t.Error("negative PID should not be alive")
	}
}

func TestIsProcessRunningInvalid(t *testing.T) {
	if isProcessRunning(-5) {
		t.Error("negative PID should not be running")
	}
}

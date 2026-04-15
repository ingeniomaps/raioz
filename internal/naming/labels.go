package naming

// Docker label keys that raioz stamps on every container it creates.
// Filtering by these labels (instead of by name prefix) is the only reliable
// way to distinguish containers managed by raioz from containers that belong
// to other projects sharing the same Docker daemon.
const (
	LabelManaged   = "com.raioz.managed"   // always "true" on raioz-managed containers
	LabelWorkspace = "com.raioz.workspace" // workspace name, "" when not set
	LabelProject   = "com.raioz.project"   // raioz.yaml project name
	LabelService   = "com.raioz.service"   // service/dep/proxy name inside the project
	LabelKind      = "com.raioz.kind"      // "service" | "dependency" | "proxy"
)

// Kind values for LabelKind.
const (
	KindService    = "service"
	KindDependency = "dependency"
	KindProxy      = "proxy"
)

// WorkspaceName returns the currently configured workspace name, or "" when
// no workspace is set (i.e. the active prefix is still DefaultPrefix). Use
// this when stamping LabelWorkspace so that projects without a workspace
// don't end up labeled with the generic "raioz" string.
func WorkspaceName() string {
	if prefix == DefaultPrefix {
		return ""
	}
	return prefix
}

// Labels returns the standard label set to stamp on a raioz-managed container.
// workspace and project may both be empty. An empty project is the signal
// that the container is NOT owned by any single project — typical for
// workspace-shared dependencies that outlive individual project downs.
// kind must be one of the Kind* constants.
func Labels(workspace, project, service, kind string) map[string]string {
	labels := map[string]string{
		LabelManaged: "true",
		LabelKind:    kind,
	}
	if project != "" {
		labels[LabelProject] = project
	}
	if workspace != "" {
		labels[LabelWorkspace] = workspace
	}
	if service != "" {
		labels[LabelService] = service
	}
	return labels
}

// IsSharedDep reports whether a dependency's container is meant to be shared
// across every project in the workspace. A shared dep either explicitly
// overrides its container name (user said "use this exact name") or runs
// inside a workspace (deps are workspace-scoped by convention). Shared deps
// must survive individual project downs until the last consumer leaves.
func IsSharedDep(nameOverride string) bool {
	return nameOverride != "" || WorkspaceName() != ""
}

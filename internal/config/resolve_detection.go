package config

import "raioz/internal/detect"

// ResolveServiceDetection returns a DetectResult for a service, honoring
// explicit overrides declared in raioz.yaml before falling back to directory
// auto-detection.
//
// Precedence:
//  1. svc.Source.ComposeFiles (yaml: `compose: [...]`) -> RuntimeCompose with
//     exactly those files, in the order given.
//  2. svc.Source.Command      (yaml: `command: ...`)  -> RuntimeMake (generic
//     host exec) with StartCommand/DevCommand set to the user command.
//  3. Fallback: detect.Detect(path) — scan the directory for compose/
//     Dockerfile/etc.
//
// Shared between `raioz up`, `raioz check`, `raioz status`, and `raioz down`
// so the runtime classification is consistent across commands.
func ResolveServiceDetection(svc Service, path string) detect.DetectResult {
	// Compose override wins first — user is asking for a specific docker compose setup.
	if len(svc.Source.ComposeFiles) > 0 {
		files := make([]string, len(svc.Source.ComposeFiles))
		copy(files, svc.Source.ComposeFiles)
		return detect.DetectResult{
			Runtime:      detect.RuntimeCompose,
			ComposeFile:  files[0],
			ComposeFiles: files,
			StartCommand: "docker compose up -d",
			DevCommand:   "docker compose up",
		}
	}

	// Custom command override — route through HostRunner (RuntimeMake is the
	// generic "invoke something on the host" bucket used by the dispatcher).
	if svc.Source.Command != "" {
		return detect.DetectResult{
			Runtime:      detect.RuntimeMake,
			StartCommand: svc.Source.Command,
			DevCommand:   svc.Source.Command,
		}
	}

	if path == "" {
		return detect.DetectResult{Runtime: detect.RuntimeUnknown}
	}
	return detect.Detect(path)
}

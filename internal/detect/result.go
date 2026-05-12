package detect

import "raioz/internal/domain/models"

// Runtime and DetectResult are type aliases for the canonical definitions
// in internal/domain/models. Detect remains the package that produces these
// values from disk scans; the domain owns the types so port interfaces can
// reference them without importing infrastructure. See ADR-009.
type (
	Runtime      = models.Runtime
	DetectResult = models.DetectResult
)

// Runtime constants re-exported from internal/domain/models so the existing
// `detect.RuntimeXxx` call sites keep compiling. New code may use either
// form; they refer to the same value.
const (
	RuntimeCompose    = models.RuntimeCompose
	RuntimeDockerfile = models.RuntimeDockerfile
	RuntimeNPM        = models.RuntimeNPM
	RuntimeGo         = models.RuntimeGo
	RuntimeMake       = models.RuntimeMake
	RuntimePython     = models.RuntimePython
	RuntimeRust       = models.RuntimeRust
	RuntimeJust       = models.RuntimeJust
	RuntimeTask       = models.RuntimeTask
	RuntimePHP        = models.RuntimePHP
	RuntimeJava       = models.RuntimeJava
	RuntimeDotnet     = models.RuntimeDotnet
	RuntimeRuby       = models.RuntimeRuby
	RuntimeElixir     = models.RuntimeElixir
	RuntimeDart       = models.RuntimeDart
	RuntimeSwift      = models.RuntimeSwift
	RuntimeScala      = models.RuntimeScala
	RuntimeClojure    = models.RuntimeClojure
	RuntimeZig        = models.RuntimeZig
	RuntimeGleam      = models.RuntimeGleam
	RuntimeHaskell    = models.RuntimeHaskell
	RuntimeDeno       = models.RuntimeDeno
	RuntimeBun        = models.RuntimeBun
	RuntimeImage      = models.RuntimeImage
	RuntimeUnknown    = models.RuntimeUnknown
)

# ADR-040: Sibling mode A trust is transitive and unaudited

- **Status:** Accepted — 2026-05-15
- **Date:** 2026-05-15
- **Drives:** issue 076

## Context

ADR-008 introduced sibling raioz projects as dependencies with
two modes:

- **Mode A** — `dependencies.<n>.project: ../sibling`. The
  consumer's `raioz up` spawns a recursive `raioz up` in the
  sibling directory. The sibling's `pre:` / `preUp:` / `post:`
  hooks run, its services come up, and only then does the
  consumer's own pipeline proceed.
- **Mode B** — `dependencies.<n>.siblingProject: ../sibling` +
  an `image:`. Defaults to pulling the image; only delegates
  to the sibling raioz when the developer brought it up
  locally first.

ADR-036 published the yaml hygiene policy with three preflight
gates that run before `yaml.Unmarshal` on the loaded yaml:

- **H1** — secret scanner rejects committed credential patterns.
- **H2** — path safety enforces containment in the project dir
  (sibling project paths are explicitly **exempt**).
- **H3** — image pinning warns on `:latest` or untagged deps.

Question that ADR-008 and ADR-036 left implicit: when a consumer
yaml declares `project: ../sibling`, do H1/H2/H3 run against the
**sibling** yaml before raioz spawns it?

Today: no. The parent loads its own yaml, the gates fire, and
then `spawnSibling` (`internal/app/upcase/sibling_spawn.go`)
runs `raioz up` in the sibling dir. The child raioz applies
H1/H2/H3 to its own yaml on the next process — but at that
point the spawn already happened. There is no parent-side
"audit the sibling yaml before spawning" pass.

This creates an unwritten transitive-trust model: by declaring
`project: ../sibling`, the consumer commits to running whatever
hooks the sibling defines, on whatever schedule. Equivalent in
effect to `make -C ../sibling`, but a reviewer reading the
consumer yaml might not realize that.

## Decision

Mode A trust is transitive and unaudited by design. raioz does
**not** preflight a sibling yaml before spawning it. The
sibling is code-equivalent to the local yaml, under the same
threat model as ADR-036.

Three things change:

1. **Policy is documented.** This ADR states the trust model
   explicitly so reviewers reading a consumer yaml know that
   `project:` is a code-execution dependency, not a config
   reference.
2. **`docs/SECURITY.md` gains a "Transitive trust via sibling
   projects" subsection** under "What raioz does NOT protect
   against." Cross-references this ADR and recommends mode B
   when the developer does not fully trust the sibling.
3. **Mode B is positioned as the lower-trust option.** Mode B
   only spawns the sibling raioz when the developer themselves
   brought it up; otherwise it pulls the image. Choose mode B
   when transitivity is a concern.

### What raioz deliberately does NOT do

The "won't do" list mirrors ADR-036's:

- **No sandbox** (cgroups / seccomp / namespaces) on the
  sibling spawn. Inherits ADR-036's rationale: raioz is a
  development tool, not a sandbox runtime. Containers already
  provide the sandboxing layer for service code; the spawn
  itself is a normal `exec` of the same trusted binary.
- **No script classifier.** raioz never inspects the sibling's
  `pre:` / `preUp:` / `post:` to "decide" if they are
  dangerous. Heuristics over shell scripts are brittle.
- **No yaml signing or pinning.** No `raioz.yaml.sig`, no
  hash-of-trusted-state. The consumer-yaml-as-source-of-truth
  model from ADR-036 extends to siblings.
- **No interactive first-run prompt** ("you are about to spawn
  sibling X with hook Y — confirm?"). ADR-036 rejected this
  for the local yaml; same reasoning applies. Friction kills
  the orchestrator value proposition; reviewers and CI gates
  catch the cases where it matters.

### Optional escape hatch (not in v0.7)

A future opt-in flag `raioz up --audit-siblings` could run
H1/H2/H3 against every sibling yaml before spawning. ~50 LoC,
reuses the existing scanners against `sib.Dir + "/raioz.yaml"`.
**Not implemented in v0.7** — current scope is documentation
only. Reconsider when a concrete user need surfaces (most
likely a CI-only flag, off by default).

## Consequences

### Positive

- Reviewers reading a consumer yaml know `project:` carries
  the same gravity as a `make -C` line. The policy is
  unambiguous.
- Teams worried about transitivity have a documented
  alternative (mode B) and a future opt-in path
  (`--audit-siblings`).
- SECURITY.md now lists the assumption explicitly under "What
  raioz does NOT protect against," consistent with the rest of
  the threat model.

### Negative

- The policy may sound stark to first-time readers ("raioz
  runs code from another directory!"). Mitigation: framing in
  terms of `make` / `npm` / `cargo` parallels. None of those
  tools audit invoked subprojects either; raioz is no more
  permissive.
- Until `--audit-siblings` lands, teams that *want* a preflight
  on siblings must implement it externally (CI step that runs
  the H1/H2/H3 logic against expected sibling paths). Listed
  as deferred work in this ADR's "won't do" section.

### Neutral

- Mode B's role as "lower-trust default" is now written. Some
  README + CLAUDE.md tweaks may follow to surface this; not
  scoped to this ADR.

## Alternatives considered

- **Implement `--audit-siblings` in this ADR.** Rejected as
  scope creep: the doc change alone is the load-bearing part.
  A flag with no published user need is speculative.
- **Run H1/H2/H3 against siblings by default (not opt-in).**
  Rejected: same arguments as ADR-036 against silent
  inspection of "code-equivalent" inputs. A sibling yaml could
  legitimately contain bash one-liners that look like
  `curl|bash` but are vetted; failing those at parent-time
  would break legit workflows for no benefit.
- **Move the spawn behind interactive confirmation on first
  run.** Rejected: ADR-036 rationale on friction and trust
  flapping applies equally here.

## References

- Code:
  `internal/app/upcase/sibling_spawn.go::spawnSibling`,
  `internal/config/path_safety.go` (H2 sibling exemption),
  `internal/config/secret_scan.go` (H1).
- Docs:
  `docs/SECURITY.md#transitive-trust-via-sibling-projects`.
- Issues: 076 (this ADR), 072 (sibling spawn timeout — a
  related bound on transitive trust, time-wise).
- Related: ADR-008 (sibling projects), ADR-036 (yaml trust
  model — same threat model for the local yaml).

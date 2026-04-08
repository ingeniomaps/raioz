# Skill: commit-conventions

## Description

Rules for writing git commit messages in this project.

## Template

```
type(scope): short description

Optional body explaining why the change was needed.
Wrap lines at 72 characters.
```

Subject = **what** (the outcome). Body = **why**.
The diff shows **how**.

Subjects should describe the outcome, not the
implementation detail.

```
Bad:  refactor(auth): extract validateEmail into helper
Good: refactor(auth): simplify email validation logic
```

Body is optional. Include it when:

- The reason is not obvious from the subject
- The change affects behavior
- There is architectural context

Do not describe implementation details already visible
in the diff. Focus on reasoning and context.

## Types

All types are lowercase.

| Type       | When                                    |
| ---------- | --------------------------------------- |
| `feat`     | New feature or capability               |
| `fix`      | Bug fix                                 |
| `refactor` | Code restructuring (no behavior change) |
| `docs`     | Documentation only                      |
| `chore`    | Maintenance (deps, config, cleanup)     |
| `build`    | Build system, Docker, CI                |
| `perf`     | Performance improvement                 |
| `test`     | Adding or fixing tests                  |
| `style`    | Formatting (no code change)             |

### Type selection guide

- Adding new capability → `feat`
- Correcting incorrect behavior → `fix`
- Improving code without behavior change → `refactor`
- Only formatting → `style`
- Documentation only → `docs`
- Dependencies or tooling → `chore`
- Build/CI/Docker changes → `build`
- Performance improvement → `perf`
- Adding/fixing tests → `test`

## Scope

Use scope when it adds clarity. Omit it when obvious.
Scope must be lowercase, short (1-2 words), and
represent a module or subsystem.
Avoid broad scopes like "system".

Common scopes: `auth`, `api`, `realm`, `docker`,
`infra`, `ui`, `db`, `frontend`, `partners`, `users`.

```
feat(realm): add partner tracking   # clear scope
fix(auth): handle null email        # clear scope
chore: remove dead files            # no scope needed
```

## Breaking changes

Mark with `!` after the type:

```
refactor(infra)!: use NETWORK_NAME from env

All compose files now read the network name from
env vars. Update NETWORK_NAME in .env.
```

Add `BREAKING CHANGE:` footer when extra detail helps:

```
build(docker)!: require KC_VERSION build arg

BREAKING CHANGE: docker build without --build-arg
KC_VERSION will fail. make build reads it from .env.
```

Use `!` when the change:

- Requires action from users or dependent projects
- Changes API, config keys, or directory structure
- Maps to a SemVer MAJOR version bump

## Commit decision flow

1. Identify the primary intent of the change
2. Choose the most specific type
3. Write the subject describing WHAT changed (outcome)
4. If breaking, add `!` after type
5. Add body explaining WHY if needed
6. Ensure subject fits within 50 chars

## Rules

1. **Subject**: max 50 chars (soft), 72 chars (hard)
2. **Body**: wrap at 72 chars, optional
3. **Language**: English (types, subject, body)
4. **Mood**: Imperative ("Add" not "Added")
5. **No period** at end of subject
6. **Capitalize** first letter after the colon
7. **One logical change** per commit
8. **Blank line** between subject and body
9. **No AI attribution** in commit messages
10. **Prefer small commits** over one large commit
11. **No temporary commits** (WIP, temp, quick patch)
12. **One type per commit** — never combine (feat+fix)
13. **Be specific** — avoid "improve", "update", "adjust"

## Body anti-patterns

Do not:

- **Repeat the subject** in the body with more words
- **List files changed** — the diff shows that
- **Use bullet points describing steps** — that is how
- **Write paragraphs** — keep body to 2-4 lines max
- **Add emoji** anywhere in the commit message
- **Use filename as scope** — scope is a module, not a file

```
Bad body (repeats subject):
  feat(auth): add OAuth login
  Add OAuth login support to the auth module.

Bad body (lists files):
  Updated compose.yaml, Makefile, and .env.example.

Bad body (bullets of how):
  - Added Prometheus
  - Added Grafana
  - Configured dashboards

Bad scope (filename):
  fix(setup-local.sh): correct env path

Good scope (module):
  fix(setup): correct env path
```

## Multiple files, one commit?

One commit per **logical change**, not per file.
If 10 files change for the same reason (e.g. rename),
that is one commit. If a file has two unrelated fixes,
those are two commits.

## Checklist

Before committing, ask:

1. Can someone understand this just reading the message?
2. Is this a single logical change?
3. Would a reviewer understand this in 6 months?

## Bad examples

```
update stuff                        # vague
fix bug                             # what bug?
changes                             # meaningless
misc fixes                          # multiple changes
WIP                                 # temporary
temp fix                            # temporary
refactor(auth): extract validate    # describes how
feat(auth-system-module): add login # verbose scope
feat+fix(auth): improve login       # multiple types
feat(auth): improve login           # vague subject
refactor(api): update service       # what changed?
```

## Good examples

```
feat(realm): add partner tracking

Protocol mappers needed for analytics — track where
each user registered from (partner, platform, direct).
```

```
fix(auth): handle null email
```

```
fix(infra): read network name from env

Multiple projects on the same host need isolated
networks. Hardcoded name caused collisions.
```

```
refactor(docker): use compose.yaml naming

Docker Compose v2 recommends compose.yaml over
docker-compose.yml. Avoids deprecation warnings.
```

```
chore: remove dead config directory

Only infinispan-cluster.xml was used. Realm import,
themes, and single-node config were never loaded.
```

```
build(docker)!: require KC_VERSION build arg

Prevents silently building with outdated version.
make build reads it from .env automatically.

BREAKING CHANGE: docker build without --build-arg
KC_VERSION will fail.
```

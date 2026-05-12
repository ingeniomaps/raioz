# ADR-007: `IsNonHTTPImage` matches by bare image name, not substring

- **Status:** Accepted
- **Date:** 2026-05-12 (retroactively documented)

## Context

raioz auto-routes dependencies through Caddy when they speak
HTTP. Databases, brokers, and similar non-HTTP services should
not get an HTTPS route — there's nothing to proxy. The
heuristic lives in `internal/proxy/filter.go` as
`IsNonHTTPImage(image)`.

The first implementation matched by substring: if the image
string contained `"redis"`, classify as non-HTTP. This broke
when a user declared a dep with image `redis/redisinsight` —
RedisInsight is a web UI for Redis, very much HTTP-speaking,
but the substring match tagged it as Redis and skipped its
route. The user got 502.

## Decision

`IsNonHTTPImage` matches by **bare image name**: the last path
segment of the image string, with tag and digest stripped.

```text
postgres:16              → bare "postgres"
docker.io/library/redis  → bare "redis"
redis/redisinsight:2.0   → bare "redisinsight"
bitnami/postgresql@sha256:... → bare "postgresql"
```

The bare name is compared against an explicit allowlist of known
non-HTTP image names in `nonHTTPImageNames`. New entries land in
that slice; substring matching is banned.

## Consequences

### Positive

- `redis/redisinsight` and other "X by vendor Y" images route
  correctly.
- The blocklist is greppable. Adding a new non-HTTP image is a
  single PR with a single-line diff plus a test case.

### Negative

- A user declaring a custom image with a name we haven't seen
  (e.g. private registry `internal/pg-fork`) bypasses the
  blocklist and gets an HTTPS route they may not want.
  Workaround: `dependencies.<n>.routing: {}` opt-in / opt-out
  in raioz.yaml.

### Neutral

- The blocklist is hand-curated. Periodic review of `compose`
  fixtures in tests is the closest thing to coverage.

## Alternatives considered

- **Substring match** — what we had; the bug above.
- **Probe with TCP/HTTP check at startup** — slow, flaky on
  cold start, false negatives when service is still booting.
- **Require explicit opt-in for every dep** — too much user
  ceremony for the common case (postgres/redis "just works").

## References

- Code: `internal/proxy/filter.go` (`IsNonHTTPImage`,
  `nonHTTPImageNames`), `internal/proxy/filter_test.go`

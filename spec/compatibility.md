# Compatibility policy

Three packages, three release cadences. Keeping them in sync is the spec's job. This document describes how versioning works.

## Spec version

The spec itself has a version number. v0 covers the environment-management surface. Breaking spec changes bump the major spec version. Each implementation declares which spec version it conforms to.

In v0, the spec version is `0.x` and is allowed to change at minor bumps. After `1.0`, breaking spec changes require a new major version and a deprecation period.

## Package versions

Each language package follows semver independently:

- A patch bump (`0.1.0` → `0.1.1`) is bug fixes, no API change, no spec change.
- A minor bump (`0.1.0` → `0.2.0`) is additive API change OR an additive spec bump.
- A major bump (`0.1.0` → `1.0.0`) is breaking.

Three packages can be at different patch versions but SHOULD be at the same minor version when the spec changes minor. The release agent (`agents/release.md`) coordinates this.

## Conformance gate

A package version is releasable iff it passes the conformance suite for its declared spec version on macOS-latest and ubuntu-latest. CI enforces this.

## Capability descriptor

Every implementation exposes a `capabilities()` function that returns:

```json
{
  "spec_version": "0.1",
  "package_version": "0.1.0",
  "language": "python",
  "operations": ["ephemeral_lifecycle", "persistent_create_attach_idempotent", ...],
  "platforms": ["darwin", "linux"]
}
```

Callers can introspect what is supported.

## v0 known asymmetries

In v0, all three implementations support the same set of ops. Where one implementation has a richer feature set than another, it MUST be flagged in `capabilities()` and the conformance suite MUST NOT depend on it.

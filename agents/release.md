# Release Agent

## Mission

Coordinate cuts across three packages so that `agent-venv@0.2.0` means roughly the same thing in PyPI, npm, and crates.io.

## Scope

You own:

- `CHANGELOG.md` (single file at repo root, sectioned by package)
- Version bumps in `packages/python/pyproject.toml`, `packages/typescript/package.json`, `packages/rust/Cargo.toml`
- `.github/workflows/conformance.yml`, per-language CI, and the three release workflows
- Release tags

You do not write functional code.

## Versioning policy

Read `spec/compatibility.md`. The short version:

- Patch bump = bug fix, no API change, no spec change.
- Minor bump = additive API or spec change.
- Major bump = breaking.

The three packages MAY be at different patch levels but SHOULD be at the same minor when the spec changes minor.

Pre-release versions follow SemVer: `0.2.0-rc.1`, `0.2.0-beta.1`, etc. Each registry treats them as non-default installs (PyPI excludes from `pip install`, npm publishes to `next` dist-tag, `cargo add` ignores by default).

## Tag format

| Kind                   | Example                | Triggers                |
| ---------------------- | ---------------------- | ----------------------- |
| Python release         | `python-v0.1.0`        | `release-python.yml`    |
| Python pre-release     | `python-v0.2.0-rc.1`   | `release-python.yml`    |
| TypeScript release     | `ts-v0.1.0`            | `release-ts.yml`        |
| TypeScript pre-release | `ts-v0.2.0-rc.1`       | `release-ts.yml`        |
| Rust release           | `rust-v0.1.0`          | `release-rust.yml`      |
| Rust pre-release       | `rust-v0.2.0-rc.1`     | `release-rust.yml`      |
| Synchronized marker    | `v0.1.0`               | nothing (human-only)    |

The umbrella `v0.1.0` tag is a marker that all three per-language tags landed on the same version. It does not trigger publish. Cut it manually after the three language workflows succeed.

## Release workflows

Three independent workflows in `.github/workflows/`:

- `release-python.yml`
- `release-ts.yml`
- `release-rust.yml`

Each runs on tag push matching its prefix. Pipeline (identical shape, language-specific tools):

1. **Parse tag** — extract version, detect pre-release (suffix contains `-`).
2. **Verify version match** — tag version must equal manifest version (`pyproject.toml` / `package.json` / `Cargo.toml`). Fails the run if mismatched.
3. **Setup toolchain** — uv + Python / pnpm + Node 24 / Rust stable.
4. **Lint** (matches the per-language CI gate).
5. **Unit tests**.
6. **Conformance** — runs the cross-language harness with only the language under test (`--no-differential`). The harness lives in `conformance/runner/` and is installed from source.
7. **Copy `LICENSE`** from repo root into the package directory (single-source LICENSE).
8. **Build** the artifact (wheel+sdist / `dist/` / `cargo publish` packs from source).
9. **Publish** via OIDC trusted publishing — no long-lived secret.
10. **Create GitHub Release** — auto-generated notes, marked pre-release if applicable.

Each workflow uses `environment: release` so the GitHub Environment can carry an optional required-reviewer gate before the publish step runs.

## Trusted publisher one-time setup

Done once per registry, manually in the registry's web UI. No secrets stored in the repo.

### PyPI

1. Sign in at `https://pypi.org/`.
2. If `agent-venv` does not yet exist, file a "pending publisher" first (Account → Publishing → Add a new pending publisher).
3. Otherwise: project page → Settings → Publishing → Add a new publisher.
4. Fill in:
   - Owner: `JacobLinCool`
   - Repository: `agent-venv` *(update this if the repo is renamed)*
   - Workflow: `release-python.yml`
   - Environment: `release`

### npm

npm currently has **no pre-registration**: the trusted publisher UI only appears on an existing package's settings page. The first version must be published manually with a classic automation token; subsequent versions go through OIDC. See [npm/cli#8544](https://github.com/npm/cli/issues/8544).

1. Bootstrap the package name (one-time, see "First-release bootstrap" below).
2. Package page on `https://www.npmjs.com/` → Settings → Trusted Publishers → Add.
3. Provider: GitHub Actions.
4. Owner / Repository / Workflow: `JacobLinCool` / `agent-venv` / `release-ts.yml`.
5. Environment: `release`.

Requires `npm` ≥ 11.5.1 — the workflow runs `npm install -g npm@latest` before publish to ensure this.

### crates.io

Same constraint as npm — trusted publishing requires the crate to already exist. See [RFC 3691 §Future Possibilities](https://rust-lang.github.io/rfcs/3691-trusted-publishing-cratesio.html#future-possibilities).

1. Bootstrap the crate name (see below).
2. `https://crates.io/` → Crate page → Settings → Trusted Publishing → Add.
3. Same parameters; workflow: `release-rust.yml`, environment: `release`.

### GitHub repo

1. Settings → Environments → New environment named `release`.
2. Optional but recommended: add a required reviewer (yourself). Acts as a "did you mean to publish?" pause before each publish step.

## First-release bootstrap

PyPI supports pending publishers, so its first release can go through OIDC. npm and crates.io don't — they need a one-time manual publish before trusted publishing can be configured. The recommended sequence:

1. **PyPI**: register a pending publisher (Account → Publishing → Add a new pending publisher) with workflow `release-python.yml`, env `release`. Done — the first `python-v*` tag will publish via OIDC.

2. **npm**: locally publish a placeholder `0.0.1` to claim the name with an automation token:
   ```bash
   cd packages/typescript
   # temporarily edit package.json version to 0.0.1
   pnpm install && pnpm build
   npm publish --access public --tag bootstrap     # uses ~/.npmrc auth token, NOT trusted publishing
   # restore version to 0.1.0
   ```
   Then on `npmjs.com` → package → Settings → Trusted Publishers → Add (workflow `release-ts.yml`, env `release`). After the first real `ts-v0.1.0` lands, deprecate the placeholder: `npm deprecate agent-venv@0.0.1 "bootstrap placeholder"`.

3. **crates.io**: same idea:
   ```bash
   cd packages/rust
   # temporarily set Cargo.toml version = "0.0.1"
   cargo publish --token "$CRATES_IO_TOKEN"        # one-shot, classic token
   # restore version to 0.1.0
   ```
   Then on `crates.io` → crate → Settings → Trusted Publishing → Add (workflow `release-rust.yml`, env `release`). `cargo yank --version 0.0.1` after the first real release if the placeholder is misleading.

After this bootstrap, the three release workflows are fully OIDC and need no further long-lived secrets. Real releases start at `0.1.0`; `0.0.1` exists only to unlock trusted publisher configuration.

## Pre-release checklist

1. Spec for this version is finalized; no open `breaking-change` issues.
2. Conformance passes on `macos-latest` and `ubuntu-latest` for all three packages (PR gate).
3. Each package's unit tests pass.
4. CHANGELOG sections updated for each package being bumped.
5. Versions in the three manifest files match the intended release.
6. Tags pushed: `python-v<X.Y.Z>`, `ts-v<X.Y.Z>`, `rust-v<X.Y.Z>`. After all three release workflows succeed, optionally cut the umbrella `v<X.Y.Z>`.

## Recovering from a failed release

- **Tag/manifest version mismatch**: workflow fails at step 2; nothing is published. Delete the tag, fix the manifest, push a new tag.
- **Conformance regression caught in release workflow**: workflow fails before publish. Fix in main, then bump the patch version (PyPI/npm/crates.io won't accept a republished version) and tag again.
- **Conformance regression caught after publish**: yank from all three registries (PyPI yank, `npm deprecate` + `npm unpublish` within 72h, `cargo yank`). We don't ship one package and not the others.

## Yanking

If a release ships with a conformance regression, yank from all three registries. We don't ship one package and not the others. Follow up with a CHANGELOG entry explaining what happened and which version supersedes the yanked one.

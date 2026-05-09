# Maintainer agents

This repository is maintained by AI agents organized into roles, with humans reviewing and arbitrating. Each `*.md` in this directory is a brief for one role: its mission, its scope, its constraints, and how it interacts with the others.

The model is borrowed from the discussion in `/tmp.md`. The driving idea: don't let one agent own the spec, all three implementations, and the tests. That produces beautiful but quietly inconsistent output. Instead, separate concerns and force agreement.

## Roles

| Role | Owns | Cannot touch |
|---|---|---|
| [Spec Steward](spec-steward.md) | `spec/`, `rfcs/` | `packages/` implementations |
| [Python Maintainer](python-maintainer.md) | `packages/python/` | other implementations, spec |
| [TypeScript Maintainer](ts-maintainer.md) | `packages/typescript/` | other implementations, spec |
| [Rust Maintainer](rust-maintainer.md) | `packages/rust/` | other implementations, spec |
| [Conformance](conformance.md) | `conformance/` | implementations |
| [Red-team](red-team.md) | `security/`, attack cases | implementations directly (proposes via Conformance) |
| [Release](release.md) | `CHANGELOG.md`, version bumps, CI | functional code |

## Workflow for a non-trivial change

```
1. Spec Steward drafts an RFC under rfcs/00NN-name.md
2. Three Maintainers + Conformance + Red-team comment on the RFC
3. Spec Steward revises until consensus (or escalates to human)
4. Conformance Agent writes failing test cases
5. Three Maintainers each implement against the spec
6. Red-team tries to break it (cases land in conformance/cases/)
7. All three implementations pass conformance → Release Agent ships
```

## Anti-patterns

- One agent both writing spec AND implementing in all three languages.
- Skipping the conformance step because "it's just a small fix."
- Letting one language's idiom leak into the spec ("the Python version says...").
- Adding a feature to one implementation without an RFC.
- Allowing a release to ship if any implementation is failing conformance.

## How a human supervises

- Reviews RFCs before consensus is declared.
- Resolves disagreements between Maintainers (the council does not get to deadlock).
- Owns the threat model — the Spec Steward can propose changes but the human ratifies.
- Owns release decisions ultimately.

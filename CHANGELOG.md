# Changelog

## Unreleased

### Changed

- Adopt the checksum-verified `go-library-tools` v1.4.0 CLI and immutable W14
  reusable workflow, with matching configuration, inventory, cohesion,
  repository, online specification, workflow, and implementation gates,
  without changing the sorting API or runtime behavior.

- Adopt the checksum-verified `go-library-tools` v1.3.0 CLI, schema-v2
  cohesion metadata, repository-local cohesion gate, and immutable reusable
  workflow without changing the sorting API or runtime behavior.

- Replace the repository-local verification implementation with the pinned
  `go-library-tools` v1.0.6 CLI and reusable workflow while preserving package
  policy and content-addressed verification evidence.

- Remove the copied verification scripts, package-manager lockfiles, and
  repository-local tool pins; spelling policy remains in `cspell.json` while
  the shared toolchain is owned by `go-library-tools`.

### Documentation

- Add direct support navigation and document that the module exposes only the
  root `externalsort` package, with no subpackages or optional adapter modules.

- Add the canonical module installation command and link the quick start and
  catalog metadata to the compiler-checked runnable package example.

- Link ecosystem and Integration and data movement family guidance to the
  immutable v1.4.0 documentation release.

- Publish the module's family, capabilities, selection, ownership, lifecycle,
  support, and delivery boundaries and link to the immutable v1.3.0 ecosystem
  index and family guidance.

- Use task-oriented README headings instead of internal planning terminology.

- Replace the archived monorepo link with package-owned documentation.

## 1.0.0 - 2026-08-25

### Changed

- Exclude intentional nested modules from root local-proxy archives so local,
  bootstrap, CI, and public module checksums describe the same source
  boundary.

- Track the pinned documentation-tool lockfile so clean CI checkouts install
  the exact validated cspell dependency.

- Reconcile standalone dependency checksums against deterministic current
  module archives so CI, local verification, and release consumers resolve
  identical content.

- Harden standalone documentation validation with deterministic spelling and
  link checks, package-specific documentation gates, and repository-local
  contributor guidance.

### Changed

- Publish the module from its standalone `github.com/faustbrian/go-external-sort` identity while preserving its documented API and behavior.

### Documentation

- Link the package README to package-owned documentation.

### Fixed

- Bind all temporary create, open, and removal operations to a revalidated
  rooted directory handle so parent-path replacement cannot redirect cleanup
  outside the configured storage root.
- Bind encrypted records to a random per-store identity so ciphertext from a
  different store is rejected even when callers reuse a key and the chunk,
  ordinal, and record-size metadata match.
- Allocate authenticated, process-unique nonce domains across concurrent stores
  and return an owned cleanup handle when construction residue cannot be
  removed immediately.
- Reject overlapping and callback-reentrant lifecycle calls without racing,
  deleting active iteration state, or weakening idempotent close semantics.
- Reject whole-record truncation and trailing encrypted bytes instead of
  accepting a shortened or extended chunk as complete, while reporting
  underlying read failures as typed storage errors and sealing a store when
  failed temporary-file cleanup would otherwise permit artifact accumulation.

### Added

- Bounded lexicographic external sorting for caller-defined fixed-size records.
- Per-record AES-256-GCM temporary-storage encryption with authenticated chunk
  and ordinal binding.
- Explicit record, in-memory chunk, byte, and 64-file merge limits.
- Owner-only temporary storage, typed fail-closed errors, duplicate
  preservation, deterministic cleanup, fuzzing, benchmarks, and exact
  statement-coverage evidence.
- Exact boundary and mutation coverage for resource ceilings, merge
  termination, authenticated framing, cross-chunk substitution, nonce
  consumption, and ordering helpers.
- Hostile-filesystem, cross-store corruption, concurrent lifecycle, process
  termination, bounded-resource, and merge-history fuzz campaigns, plus a safe
  caller-owned Kubernetes cleanup runbook.

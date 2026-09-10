# external-sort

[![CI](https://github.com/faustbrian/go-external-sort/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/faustbrian/go-external-sort/actions/workflows/ci.yml)
[![CodeQL](https://img.shields.io/badge/CodeQL-required-blue)](https://github.com/faustbrian/go-external-sort/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/badge/coverage-100%25_required-blue)](CONTRIBUTING.md#verification)
[![Mutation](https://img.shields.io/badge/mutation-100%25_required-blue)](CONTRIBUTING.md#verification)
[![Documentation](https://img.shields.io/badge/docs-checked_in_CI-blue)](docs/)
[![Go Reference](https://pkg.go.dev/badge/github.com/faustbrian/go-external-sort.svg)](https://pkg.go.dev/github.com/faustbrian/go-external-sort)
[![Release](https://img.shields.io/github/v/release/faustbrian/go-external-sort?sort=semver)](https://github.com/faustbrian/go-external-sort/releases)
[![Go](https://img.shields.io/badge/go-1.27.0-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

`external-sort` performs bounded external sorting of fixed-size opaque records
while encrypting every temporary record with AES-256-GCM. It is intended for
large reconciliation and migration datasets that cannot safely be retained in
memory or written to plaintext temporary files.

## Installation

```sh
go get github.com/faustbrian/go-external-sort@v1
```

## Quick start

See the [compiler-checked package example](example_test.go) for a complete
runnable setup that creates a store, adds fixed-width records, consumes them in
sorted order, and closes all owned resources.

The parent directory must already exist, must not be a symlink, and must have
no group or other permission bits. Existing ancestor links are resolved when
the factory is created; each store binds the resolved directory to a rooted
handle and rejects later identity or permission changes. The key must contain
exactly 32 bytes.

## Module and package map

This repository contains one Go module,
`github.com/faustbrian/go-external-sort`, with the sole public root package
`externalsort`. It has no subpackages or optional adapter modules.

## Guarantees

- fixed record size and explicit total-record limit;
- bounded contiguous in-memory chunks;
- at most 64 files in one merge;
- lexicographic byte ordering with duplicates preserved;
- a random-seeded process-unique nonce domain, a retry-safe record counter, and
  AES-256-GCM authentication for every temporary record;
- authentication of store identity, format version, chunk, ordinal, and record
  size;
- exact owner-only temporary modes (`0700` directories and `0600` files),
  independent of process umask;
- descriptor-relative storage and cleanup that cannot be redirected by a
  renamed or replaced parent pathname ancestor; and
- complete temporary-directory removal after a successful `Close`.

Stores permit one active lifecycle operation. Overlapping or reentrant calls
fail with `ErrConcurrentUse`; callers can retry after the active operation
returns. A record passed to the iteration callback is valid only until that
callback returns. Copy it when retention is required.

## When to use this package

Use this module when the data is fixed-width, sorting must be bounded, and
plaintext spill files are unacceptable. Prefer an in-memory sort for small,
public datasets. This implementation deliberately rejects configurations that
need more than 64 chunks instead of hiding an unbounded or multi-pass merge.
Increase the chunk size within the declared byte ceiling or partition the
dataset at a higher semantic layer.

`Close` removes process-owned artifacts, but no process can guarantee cleanup
after abrupt termination or host loss. Operators should place the parent on an
encrypted ephemeral filesystem and apply the descriptor-relative ownership and
age checks in the [operations guide](docs/operations.md) before removing stale
directories.

## Documentation

For shared package selection, ownership, and lifecycle guidance, see the
versioned [Golib ecosystem index](https://github.com/faustbrian/go-library-tools/blob/v1.4.0/docs/ecosystem/README.md)
and its [Integration and data movement family](https://github.com/faustbrian/go-library-tools/blob/v1.4.0/docs/ecosystem/design-language.md#package-families-and-selection).

- [API and lifecycle](docs/api.md)
- [Architecture and file format](docs/architecture.md)
- [Adoption and migration](docs/adoption.md)
- [Compatibility](docs/compatibility.md)
- [Threat model](docs/threat-model.md)
- [Performance](docs/performance.md)
- [Operations and Kubernetes](docs/operations.md)
- [FAQ](docs/faq.md)
- [Support](SUPPORT.md)
- [Security policy](SECURITY.md)
- [Release notes](CHANGELOG.md)

## License

MIT. See [LICENSE](LICENSE).

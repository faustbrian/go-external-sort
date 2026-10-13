# Package map

[Documentation index](README.md) · [Repository entry](../README.md)

This map follows the current source modules, not a synchronized release.
Use the repository README for published installation versions; an unpublished
source major is not installable merely because its package appears here.
Internal packages and non-releasable fixtures are intentionally excluded.

## github.com/faustbrian/go-external-sort

`external-sort` performs bounded external sorting of fixed-size opaque records while encrypting every temporary record with AES-256-GCM. It is intended for large reconciliation and migration datasets that cannot safely be retained in memory or written to plaintext temporary files.

| Import path | Responsibility |
| --- | --- |
| [`github.com/faustbrian/go-external-sort`](../root_supported.go) | Use when fixed-width data exceeds a bounded in-memory sort and plaintext temporary files are unacceptable. |

Choose the next guide in the [documentation index](README.md)
or return to the [repository entry](../README.md) for installation and ownership.

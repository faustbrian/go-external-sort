# API and lifecycle

[Documentation index](README.md) · [Repository entry](../README.md)

`NewFactory(Config)` validates the complete storage policy. `Config` declares:

- `ParentDirectory`: existing absolute non-symlink owner-only directory;
- `RecordBytes`: exact byte length of every record;
- `ChunkRecords`: maximum records held in the contiguous sort buffer; and
- `MaximumRecords`: hard limit for the complete store.

The ceiling constants bound record size, chunk count, chunk bytes, and merge
fan-in. A configuration whose ceiling would create more than 64 files is
rejected before `Open`.

`Factory.Open(ctx, key)` requires exactly 32 key bytes, consumes a random
32-byte store identity, binds the validated parent identity to a rooted
directory handle, and creates a new owner-only work directory relative to that
handle. Parent replacement or permission changes make `Open` fail closed. The
factory is safe for concurrent use. Each returned `Store` permits one active
lifecycle operation. While the store is open, overlapping or callback-reentrant
calls return `ErrConcurrentUse` without changing the active operation. Once
closing begins, `Add` and `ForEachSorted` return `ErrClosed`; an overlapping
`Close` still returns `ErrConcurrentUse`.

`Open` normally returns either a usable Store or a nil Store with an error. If
construction creates a directory but its immediate cleanup also fails, `Open`
returns both `ErrStorage` and a non-nil cleanup-only Store. The caller must call
`Close` whenever the returned Store is non-nil, even when `err` is also non-nil.
The cleanup-only Store rejects `Add` and `ForEachSorted` with `ErrClosed` and
keeps retryable ownership of the rooted residue.

`Store.Add` copies one exact-size record. A failed call does not retain its
input. `Store.ForEachSorted` finalizes input and invokes the callback in
lexicographic byte order. Duplicate records remain distinct. Callback slices
are cleared after each callback and must be copied before retention. Output is
not atomic: a later failure may follow records already yielded. Callback errors
pass through unchanged; callback panics propagate after plaintext clearing and
reader-close attempts. Cancellation is cooperative and cannot interrupt a
blocked callback, synchronous IO, entropy read or in-memory sort.

`ForEachSorted` can be called once after pending input is successfully spilled.
It remains finalized after callback, authentication, merge-storage, or merge
cancellation failure. A pre-finalization spill or cancellation failure may be
retried. `Close` is idempotent and must be called on every path. A directory
removal failure leaves only `Close` retryable; other operations reject the
closing store. A rooted-handle close failure returns `ErrStorage` after the
directory and sensitive state have already been finalized, so later `Close`
calls are idempotent. After an overlapping call returns `ErrConcurrentUse`,
retry only after the active operation has returned.

Errors are compared with `errors.Is`. Context cancellation is returned
directly. Owned sentinel errors distinguish invalid configuration, unsafe
parents, invalid keys and records, limit exhaustion, concurrent or finalized
lifecycle misuse, entropy, storage, and authenticated corruption without
exposing sensitive values.

Choose the next guide in the [documentation index](README.md)
or return to the [repository entry](../README.md) for installation and ownership.

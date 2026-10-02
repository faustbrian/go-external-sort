# Threat model: go-external-sort

**Model version:** 1.0 (2026-10-02).

**Source baseline:** `8d834b603072ef382a85977648258ffbcc0c2cdb` on `main`.

**Scope:** the single root module, including factory validation, encrypted
spill files, merge iteration, cleanup, and supported/unsupported rooted
filesystem variants. There are no nested modules, public storage adapters, or
external module dependencies. This is a source assessment, not a deployment
or release certification. See the [security policy](../SECURITY.md) for private
reporting and the [operations guide](operations.md) for cleanup ownership.

## Assets, attacker control, and consumers

Assets are record confidentiality and integrity in temporary storage, bounded
memory and filesystem use, correct duplicate-preserving sort output, and
cleanup confined to the selected parent. Keys, plaintext records, temporary
paths, and retained ciphertext can be sensitive.

An attacker can supply records through the application and alter ciphertext
if it can reach storage. Trusted caller policy selects the existing absolute
parent directory, AES-256 key, record/chunk/population limits, context, and
output callback. The parent and process must be protected from hostile writers;
owner-only permission bits do not authenticate an application tenant.

Owned consumers are the executable example and package tests. The only direct
sibling source consumer found is the blank import in
`go-library-tools/release/compatibility-consumer/consumer_test.go`, pinned to
v1.0.0. No runtime reverse consumer was found in that inventory. External
applications own their admission, key management, publication, and cleanup
policies.

## Boundary and hostile-input matrix

| Boundary | Controls and named risks | Existing behavioral evidence |
| --- | --- | --- |
| Configuration to allocation/storage | Positive fixed record and chunk sizes, division-based 256 MiB chunk bound, at most 1 MiB per record, one million records per chunk, and rounded fan-in of at most 64 files. Invalid policies fail before storage opens. | `TestFactoryRejectsUnsafeStorageAndInvalidResourceBounds`, `TestConfigurationBoundariesAreExact`, `TestStoreAllocatesExactlyTheDeclaredRecordBuffer` |
| Parent path to temporary work | Final symlink and group/other permissions are rejected. `Open` rechecks the original directory identity through `os.OpenRoot`; subsequent file and cleanup operations are relative to that handle. Work directories use 0700, files use exclusive creation and 0600, and random-name retries stop after 128 attempts. Unsupported JS/Plan 9 platforms fail closed. | `TestFactoryRejectsAReplacedCanonicalParentAtOpen`, `TestFactoryPinsTheResolvedParentAgainstAncestorLinkReplacement`, `TestStoreNeverDeletesThroughAReplacedParentAncestor`, `TestFactoryForcesOwnerOnlyWorkDirectoryPermissions` |
| Record admission to spill | `Add` copies exact-size records and enforces the configured total. Sorting and encryption use only the bounded chunk. A failed spill removes the newly admitted record; failed temporary cleanup seals the store for `Close` only. | `TestStoreRejectsInvalidRequestsLimitsAndReuse`, `TestStoreFailsClosedForChunkWriteSyncCloseAndCancellation`, `TestStoreNeverReusesANonceAfterAPartialWriteFailure` |
| Encryption and stored framing | Standard-library AES-256-GCM authenticates format, store identity, nonce domain, chunk, ordinal, and record size. A synchronized random-seeded process nonce domain and monotonic per-store counter avoid within-process nonce reuse; exhaustion fails closed. Fixed-size reads reject truncation, extra bytes, corruption, and substitution. | `TestFactoryAllocatesDistinctNonceDomainsWhenConcurrentEntropyRepeats`, `TestStoreFailsClosedBeforeTheNonceCounterCanWrap`, `TestStoreAuthenticatesChunkPositionAndRejectsCorruption`, `TestStoreRejectsCrossStoreSubstitutionWithAReusedKey` |
| Merge to callback/output | The heap retains at most one plaintext record per chunk. Files close on normal return, read failure, callback error, and callback panic. Yielded record storage is cleared after the callback; callers must copy if retaining it. Failure can follow already emitted records: output is not an atomic transaction. | `TestStoreBoundsTemporaryBytesDescriptorsAndCleanupWork`, `TestStoreClosesOpenedReadersWhenALaterOpenFails`, `TestStoreClosesChunkReadersWhenCallbackPanics` |
| Lifecycle and cancellation | One active operation per store; overlap and reentry fail without holding a lock across callbacks or storage. Context is checked at admission, directory retries, spill records, and merge output. No background goroutines are started. | `TestStoreRejectsCloseWhileIterationOwnsTheStore`, `TestStoreRejectsCallbackReentry`, `TestStoreFinalizesEmptyInputAndPropagatesCallbackAndCancellation` |
| Errors and cleanup | Owned failures use payload-free sentinel errors; Factory/Store formatting, slog, and JSON redact their state. `Close` clears plaintext and retries failed directory removal; failed construction can return a non-nil cleanup handle with its error. | `TestFactoryAndStoreRepresentationsRedactStorageAndRecords`, `TestStoreReportsMissingChunksWithoutExposingPathsOrRecords`, `TestFactoryAndStoreReportOwnedStorageFailures` |

These test names identify existing coverage, not newly executed evidence.
Existing framing, configuration, sorting, and merge fuzz targets are separate
from application admission controls. No new runtime checks were run for this
documentation-only assessment; exact hosted results belong in the ecosystem
ledger.

The configured population bounds temporary ciphertext to
`MaximumRecords * (RecordBytes + 12 + 16)` bytes, excluding filesystem metadata
and application output. The chunk buffer is not a process-wide memory budget:
merge plaintext, cryptographic scratch, concurrent stores, and caller copies
need additional headroom. Retries of a failed spill consume nonce counters;
counter exhaustion remains an error rather than permission to reuse a nonce.

## Protected

- plaintext fixed-size records on temporary storage;
- undetected modification, truncation, duplication, position reordering,
  cross-linking, and cross-store or cross-chunk substitution;
- accidental group or world access to newly created work directories and
  chunks;
- redirection of work-directory, chunk, and cleanup operations through a
  replaced parent pathname ancestor;
- unbounded caller-controlled records, chunk memory, or merge file fan-in; and
- sensitive values appearing in public errors.

## Caller responsibilities

- derive and isolate a 32-byte key for each sensitive dataset;
- protect process memory and the parent directory;
- use an encrypted ephemeral filesystem;
- call `Close` and monitor cleanup failures;
- apply descriptor-relative crash-recovery cleanup without following links or
  deleting live work; and
- select bounds appropriate for the source population.

## Outside scope

Privileged host compromise, memory inspection, malicious bind mounts or device
files introduced inside the trusted root, whole-filesystem rollback, entropy
subsystem compromise, traffic analysis from file sizes, and availability
attacks within approved bounds are outside scope.

## Owned residual risks

These are conditional limitations, not acceptance of a deployment with
unbounded attacker work or a claim that the entire security goal has passed.

| Residual risk | Owner and rationale | Mitigation and review condition |
| --- | --- | --- |
| Accepted bounds can still exceed memory, disk, inode, or descriptor budgets; many stores multiply the budget. | Application admission/availability owner; per-store limits do not control process-wide concurrency or storage quota. | Bound ingress and concurrent stores, select smaller policies, and reserve memory/storage headroom. Review on record size, population, concurrency, or capacity changes. |
| Cancellation cannot interrupt a synchronous filesystem/entropy call, one sort or AES operation, or a blocked callback; `Close` has no context. | Application storage and callback owners; the implementation uses ordinary local synchronous APIs, not cancellable adapters. | Use trusted local storage with measured latency, bounded callback work, cooperative deadlines, and shutdown grace. Review on backend, callback, platform, or latency changes. |
| A trusted parent writer, malicious device, privileged host, or compromised process can defeat filesystem assumptions or observe memory. | Deployment/storage owner; owner-only modes and rooted paths do not isolate code sharing the same OS authority. | Isolate the process and dedicated parent, prohibit hostile mounts/devices/writers, and use encrypted ephemeral storage. Review on identity, mounts, privilege, or tenancy changes. |
| Reusing a key across processes or restarted datasets loses the process-local nonce uniqueness guarantee; random domains are not durable cross-process coordination. | Application key-management owner; the library does not derive keys or persist nonce state. | Use a unique key for each independent sensitive dataset, protect key lifetime, and do not treat repeat-entropy tests as cross-process guarantees. Review on key derivation, reuse, process restart, or entropy changes. |
| Callback errors and context errors can contain sensitive information; callbacks can retain copies or publish partial output before a later error. | Application output/observability owner; caller errors pass through unchanged and publication is outside the sort store. | Redact caller diagnostics, copy only when necessary, and stage output until successful completion when atomic publication matters. Review on callbacks, logging, retention, or output transactions. |
| Failed cleanup or process death leaves encrypted residue; dropping cipher references is not guaranteed erasure of every key schedule or memory copy. | Application lifecycle and key-management owners; crashes bypass cleanup and the runtime does not promise secure memory erasure. | Close every non-nil store including failed construction, retry cleanup failures, protect process memory, and use the ownership-aware janitor in the operations guide. Review on shutdown, recovery, storage retention, or memory requirements. |
| Compromised tooling, actions, source, or release credentials can undermine integrity despite no runtime dependency modules. | External-sort maintainers/release owner; automation and maintainers remain supply-chain boundaries. | Review immutable pins and run selected security gates; verify actual public releases through clean consumers. Review on tool/action changes and each release. |

There is no implicit network, process, or environment access. Filesystem and
cryptographic entropy access occur only after explicit caller construction.
SSRF, SQL, archives, compression, URL parsing, plugin loading, authorization,
and application retry queues are not owned boundaries. Adding such an adapter
requires a new assessment. The process-wide nonce allocator is synchronized
cryptographic uniqueness state, not a public registry or background service.

## Per-module verdict

No confirmed runtime vulnerability was identified in this bounded manual
source audit. This model/navigation update changes no API, cryptographic scheme,
private chunk format, or runtime behavior and requires no module release.
Known caller-owned limitations are listed above. Scanner success, existing test
names, or this model alone do not establish ecosystem security completion;
selected scanner results and current required-check state must remain explicit
in the coordinator's ledger.

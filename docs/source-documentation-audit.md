# Source documentation audit

## Boundary and plan

Owner: repository documentation worker. Baseline:
`3d4f91134baec3aa66e357a2bca75071ce01c8b1`.
Tier A: source comments and directly related explanatory prose only, preserving
all executable tokens and build constraints. Outcome: truthful public contracts
and adjacent storage/cryptography/ownership rationale.
Direct consumers: the root externalsort package; catalog has no owned reverse
consumers. Named risks: borrowed plaintext lifetime, partial output/finalization,
retryable cleanup versus finalized close, cancellation checkpoints, nonce reuse,
authenticated chunk position/count, rooted containment and allocation bounds.

Smallest checks: tracked-file/package/export/member inventory; markers and build
variants; AST comment structure and executable-token equivalence; affected
gofmt; rendered native Go documentation; local links and changed-content secret
scan; one ordinary complete Tier A diff review. Existing examples and runtime
remain unchanged, so tests, fuzz, race, benchmarks, process-signal scenarios,
scanner campaigns, release rehearsal and consumers are not selected.
API configuration uses `apidiff` at `api/baseline.txt`, not `go-doc`: no native
API snapshot regeneration is applicable. Development pin is `.go-version`
1.27.2; `go.mod` and module catalog declare the minimum 1.27.0.

## Reproduce the inventory

Run from the repository root:

```sh
git ls-files '*go.mod' '*go.sum' '*.go'
rg -n '^package |^func |^type |^const |^var ' --glob '*.go'
rg -n 'TODO|FIXME|HACK|NOTE|WARNING|Deprecated:|Code generated|go:generate|go:build' --glob '*.go'
go doc -all .
```

One dependency-free module `github.com/faustbrian/go-external-sort`, one public
root package `externalsort`, and its external test variant `externalsort_test`.
No nested modules, adapters, cgo, vendor/generated Go files or generators.
No TODO/FIXME/HACK/NOTE/WARNING/deprecation markers. Existing exact-site Gosec
conversion annotations retain their admission-bound rationale. No specification
conformance or external fixture provenance claim is introduced.

## Complete tracked Go-file inventory

All 17 tracked Go files were audited, including private helpers and variants.

| File | Contract and finding |
| --- | --- |
| `store.go` | Root package comment lacks entry point, non-goals and lifetime/trust boundaries. Sentinel errors and Config fields lack comments. Factory/Open, Store/Add/ForEachSorted/Close require exact nil, aliasing, partial-output, retry, key and cancellation semantics. Add adjacent admission, nonce/AAD, lifecycle, spill-commit/rollback, framing and merge-buffer invariants. |
| `root_supported.go` | `!js && !plan9` rooted filesystem adapter. Retain behavior and describe containment purpose. |
| `root_unsupported.go` | `js || plan9` adapter refuses Open through ErrUnsafeParent. Explain refusal instead of path-based fallback; canonical package comment must cover it. |
| `store_test.go` | Public ordering, duplicates, encrypted owner-only files and eventual cleanup. Existing clear assertions/helper names need no narration. |
| `store_contract_test.go` | Validation/nil/zero, independent stores, cancellation, spill rollback, nonce and corruption boundaries, representations, callback panic and reader cleanup. Add rationale only for injected IO and repeated-entropy helpers. |
| `store_boundaries_test.go` | Exact limits, dependency validity, EOF, AAD field order, domain exhaustion and strict sorting/heap ordering. Document fake AEAD as intentionally non-cryptographic framing aid. |
| `store_concurrency_test.go` | Active-operation exclusion, reentrant callbacks and corruption-read overlap. Explain blocking-reader coordination rather than narrating tests. |
| `resource_bounds_test.go` | Fixed buffer, committed ciphertext bytes, opened-reader counts, cleanup and later-open failure. Assertions clear; existing evidence identifies logical owned handles, not whole-process OS usage. |
| `fault_campaign_test.go` | Hostile IO boundaries, partial IO, nonce consumption/exhaustion, parent identity/ancestor changes, secure-name retry limits and close finalization. Explain partial-write fixture's escaped nonce and stale-path containment case. |
| `fault_errors_test.go` | `!plan9` native syscall errors for injected failures, not public wrapping. |
| `fault_errors_plan9_test.go` | `plan9` synthetic error equivalents; production Open still refuses unsupported containment. |
| `process_lifecycle_test.go` | Caller-owned subprocess signal/umask fixtures, graceful cleanup versus abrupt-death residue and helper lifetime. Explain controlled helper-only signal ownership; no process tests run in this pass. |
| `umask_unix_test.go` | `unix` umask mutation used in isolated helper process and restored. Explain isolation requirement. |
| `umask_other_test.go` | `!unix` no-op helper; tests do not promise Unix umask behavior. |
| `fuzz_test.go` | Four bounded in-memory targets: reference record sort, config arithmetic, AES-GCM framing and merge histories. Add input bounds/oracles; no real-filesystem or crash proof claimed. |
| `benchmark_test.go` | Complete 4,096 x 32-byte record workload in 512-record chunks. Explain included per-iteration factory/store/IO/cleanup and excluded parent/key/input setup; avoid performance claims without execution. |
| `example_test.go` | Existing runnable root example owns all non-nil Stores and parent cleanup. Unchanged executable example. |

## Export and rationale coverage

All five public constants, eleven sentinel errors, Config and its four fields,
Factory/NewFactory/Open/formatting methods, Store and its formatting/Add/
ForEachSorted/Close methods are in scope. There are no public interfaces.
Private chunkFile/rootDirectory methods are IO seams exercised by white-box
fixtures; their ownership/containment semantics are documented on the interface.
Private buffer/heap sort.Interface methods are not separate consumer APIs.

Sorting is full-record bytes.Compare, with no injected comparator. Identical
records retain multiplicity; heap reader tie-breaking is deterministic, not a
promise of original-input identities or a configurable stable-key sort.
The retry and framing comments are supported by the contract, boundary, fault,
concurrency and resource tests named above; those tests were read, not executed.

Missing documentation can be repaired from source and existing tests. The
architecture nonce/AAD diagram contradicts current source and its adjacent
prose; repair that directly related diagram without redesigning the format.
No rationale is invented for version history or unmeasured micro-optimizations.
Known caller-owned limits (cooperative cancellation, untrusted shared parent,
process-memory key erasure, partial publication and crash residue) are explicit
limitations, not unresolved claims of protection.
Shared objective checker/CI rollout is coordinator-owned; no duplicated
repository-local executable policy is introduced.

## Evidence and completion

Go 1.27.2 gofmt passed for affected source. The disposable Go AST/scanner check
parsed all 17 tracked Go files, including every platform/test variant, and proved
identical executable tokens and Go directives against the baseline. It checked
name-prefixed comments on all 36 public declarations/members and exactly one
canonical package comment. Native `go doc -all .` was rendered and inspected.
`git diff --check`, changed local-link existence and changed-content secret
checks passed. The complete Tier A final diff was reviewed against source/tests;
no runtime change or unresolved content finding remains.
The configured apidiff baseline is unchanged; a go-doc snapshot is inapplicable.
Unit/example, vet/lint, race/fuzz/mutation, filesystem/process campaigns,
benchmarks, release and consumer execution were not run for this prose-only pass.
All task-owned Go caches and generated execution output were removed.
Shared objective checker/CI adoption, remote-main delivery and applicable hosted
CI remain coordinator-owned and are not claimed by this local audit.

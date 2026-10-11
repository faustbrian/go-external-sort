# Contributing

## Before Editing

1. Read [`AGENTS.md`](AGENTS.md) and the affected package's documentation.
2. Run ordinary Go tests for the affected package before editing.
3. Identify owned dependencies and reverse dependants in `modules.json`.
4. Preserve unrelated work and generated/corpus provenance.

## Changes

Keep commits focused and conventional. Update every affected changelog with
the behavior and migration impact. Public API changes require compatibility
evidence and documentation. Specification behavior requires a decision record,
fixture coverage, and interoperability evidence.

New direct dependencies and dependency updates must follow the
[dependency governance policy](AGENTS.md#dependencies-and-supply-chain). Package-local
update bots are forbidden; the root policy owns every module and action update.

Specification-backed changes must follow the
[specification governance contract](AGENTS.md#design), update
the affected stable decision entries, and complete the Specification Decisions
section of the pull request template. An unresolved interpretation or stale
source pin is release-blocking; peer behavior cannot silently select policy.

Selected mutation gates must finish with zero surviving viable mutants.

Do not add package-local workflows, permanent replacements, machine-specific
paths, bypass flags, broad mutation exclusions, or aggregate quality metrics
that hide a failing package.

## Verification

Use Go 1.27.2 for development and ordinary CI; the public minimum remains
Go 1.27.0. An ordinary checkout is sufficient for tests and a focused active
fuzz smoke, without sibling repositories or goal ledgers:

```bash
go test ./...
go test . -run '^$' -fuzz '^FuzzRecordBufferSort$' -fuzztime=2s
```

Select the native target affected by your change; ordinary Go tests already
exercise deterministic fuzz seeds. For shared checks, install the public
`golib` v2.1.0 release declared in `.golib.yaml` and verify its checksums:

```bash
make check
make ci
```

Both Make routes use bounded local checks, including one count-1, two-minute
race selection for `TestStoreRejectsCloseWhileIterationOwnsTheStore` and
`TestStoreClosesOpenedReadersWhenALaterOpenFails`. These tests assert lifecycle
exclusion, canceled iteration, eventual work-directory deletion and zero retained
readers after a later-open failure. The module declares no services or test tags.
Optional mutation analysis can select the single root package:

```bash
golib mutation --module . --package .
```

Use `make check-full` for explicit full assurance, including online specification
checks, the original four native fuzz budgets, full lifecycle/fault scenarios,
API compatibility and benchmarks. Select relevant storage, cryptographic framing,
nonce, callback, cancellation and cleanup assertions for affected changes.
The process signal/grace-expiry tests remain existing scenarios, not a new
automatic race selection; retain their declared crash-residue distinction in
[the operations guide](docs/operations.md). Scheduled and release qualification
retain their full selected gates and release rehearsal. The existing source-built
native-security diagnostic is separate from ordinary public verification and
release qualification. Report unavailable or failing checks without claiming
release readiness. CI/configuration-only changes do not require a library release.

## Adding A Module

Follow [repository structure policy](AGENTS.md#repository-structure). New modules
require an explicit purpose, ownership boundary, dependency review, package
catalog entry, full quality gates, documentation, changelog, license, security
policy, compatibility plan, and release dry-run.

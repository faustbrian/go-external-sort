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

Do not fork shared verification workflows, add permanent replacements, machine-specific
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
[the operations guide](docs/operations.md). Explicit full and release qualification
retain their selected gates and release rehearsal. The existing source-built
native-security diagnostic is separate from ordinary public verification and
release qualification. Report unavailable or failing checks without claiming
release readiness. CI/configuration-only changes do not require a library release.

## Selected Monthly Assurance

The owner-level [selected-assurance caller](.github/workflows/assurance.yml) is
configured for main only, on day 27 of each month at 06:47 UTC. It selects one
of the 4 current declared root-module fuzz targets using the shared UTC-week
rotation, with 30 seconds of active exploration. The selected risks are
record ordering against a reference sort, configuration allocation bounds,
malformed authenticated chunk framing and merged-output equivalence. These
bounded fuzz models use memory buffers and fake chunk readers; they do not
certify real filesystem durability, crash cleanup or process-signal behavior.
The existing lifecycle/fault and native-security diagnostics remain separate
full/manual contracts, including their crash-residue distinctions.

Base-free ordinary checks execute deterministic seeds, not active exploration;
committed-range CI selects affected active smoke. Manual assurance requires an
exact anchored selector from the root fuzz operation in `.golib.yaml`, for
example `^FuzzMergeHistories$`.
Empty, unknown and malformed selections refuse. The shared job bounds setup
and execution to ten minutes, active workers and CPU to two, owned memory to
2 GiB, native output to 1 MiB and private-cache lifetime. Failure, cancellation,
exhausted budgets and cleanup refusal remain attributable failures. This lane
does not run mutation or replace Required, original full fuzz budgets,
explicit `make check-full`, or relevant release qualification.

## Adding A Module

Follow [repository structure policy](AGENTS.md#repository-structure). New modules
require an explicit purpose, ownership boundary, dependency review, package
catalog entry, full quality gates, documentation, changelog, license, security
policy, compatibility plan, and release dry-run.

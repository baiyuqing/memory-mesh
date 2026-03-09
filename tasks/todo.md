# Task Plan

## Previous Task

- [x] Review benchmark connection lifecycle and identify insertion points for connection mode.
- [x] Add configurable benchmark modes: long-running shared connection pool mode and single-new-connection-per-transaction mode.
- [x] Add/adjust unit tests for mode parsing and mode behavior guards.
- [x] Update README docs to describe the new mode flag and examples.
- [x] Run test suite and record results.

## Current Task: Reorganize `main.go`

- [x] Confirm refactor scope and keep behavior unchanged.
- [x] Split `main.go` into focused files by concern (config, metrics, runner, reporting, entrypoint).
- [x] Run gofmt and test suite to verify no regressions.
- [x] Review git diff for minimal-impact structure-only changes.
- [x] Record review notes and verification results.

## Review

### Previous Task

- Added a new `-connection-mode` flag with `long-running` (default) and `per-transaction` modes.
- Refactored execution to use a query runner abstraction so worker logic is unchanged while connection lifecycle is mode-dependent.
- Added unit coverage for parsing/validating the connection mode option.
- Updated English and Chinese README files to describe both modes.
- Validation: `go test ./...` passed.

### Current Task

- Reorganized the single-file implementation into focused files while keeping package scope and behavior unchanged.
- Kept the same runtime flow in `main()` and moved parsing, metrics, query execution, and reporting into dedicated files.
- Verified formatting and tests after refactor.
- Validation: `go test ./...` passed.


## Current Task: Robust DSN Flag Parsing

- [x] Reproduce DSN parse failure for `"-dsn '...@tcp(...)'"` input shape.
- [x] Normalize DSN CLI args to support combined token and quoted literal values.
- [x] Add tests covering DSN normalization and account-segment preservation without hard-coded credentials.
- [x] Run go test to verify behavior.
- [x] Record review notes and verification in this task section.

### Review

- Reproduced the failing invocation shape where the entire `-dsn 'value'` arrives as one CLI argument.
- Updated flag parsing to normalize combined DSN arguments and strip matching wrapping quotes from DSN values.
- Added regression tests for both combined-argument and quoted-value DSN forms.
- Verified account segment extraction remains stable for DSN inputs, without embedding concrete credentials.
- Validation: `go test ./...` passed.

## Current Task: Count Per-Connection Latency Spikes

- [x] Review the current execution model and decide how to make "per-connection" reporting meaningful.
- [x] Add metrics to count latency samples above `200ms` per connection and in total.
- [x] Surface the new counts in interval logs and Prometheus output.
- [x] Add/adjust tests for spike counting and reporting output.
- [x] Run `gofmt` and `go test ./...`, then record review notes.

### Review

- Changed `long-running` mode to allocate one dedicated `sql.Conn` per worker so interval spike counts can be attributed to a stable connection ID.
- Added `-slow-threshold` with a default of `200ms`, and tracked both interval and cumulative counts for successful queries above that threshold.
- Extended console reporting with `interval_slow_over_<threshold>`, `interval_slow_by_conn`, and `total_slow_over_<threshold>`, plus a final summary total.
- Exported Prometheus metrics for the configured threshold, total spike count, and per-connection spike counters.
- Added unit coverage for spike counting/reset behavior, Prometheus output, and slow-threshold flag parsing.
- Validation: `gofmt -w config.go main.go main_test.go metrics.go reporting.go runner.go` and `go test ./...` passed.

## Current Task: Add Slow Query Ratio Output

- [x] Define the slow-query ratio denominator and align the slow-count semantics with it.
- [x] Add interval and total slow-query ratio output alongside raw counts.
- [x] Export a Prometheus ratio metric for slow queries.
- [x] Update tests and docs for the new ratio fields.
- [x] Run `gofmt` and `go test ./...`, then record review notes.

### Review

- Defined slow-query ratio as `slow_queries / total_queries`, where total queries include both successes and failures.
- Updated slow-query counting to include failed queries when their end-to-end duration exceeds the configured threshold.
- Added `interval_slow_ratio` and `total_slow_ratio` to console output, and `slow_ratio` to the final summary.
- Exported `mysqlbench_latency_spike_ratio` in Prometheus alongside the existing raw slow-query counters.
- Updated tests to cover slow failed queries and ratio calculations, and refreshed README examples in both languages.
- Validation: `gofmt -w main.go main_test.go metrics.go reporting.go` and `go test ./...` passed.

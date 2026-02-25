# GitHub Issue Seed Backlog for `hlgo`

> These are ready-to-copy issue drafts for bootstrapping repository work tracking.
> I could not publish directly to GitHub from this environment because no git remote is configured.

## Issue 1 — Bootstrap CLI Skeleton and Project Layout
**Labels:** `phase-1`, `cli`, `infra`

### Description
Create initial Go module and Cobra CLI skeleton matching the package layout in the v0.1.0 spec.

### Acceptance Criteria
- `go mod init` completed with module path chosen for repo.
- `main.go` + `cmd/root.go` compile.
- Global flags implemented: `--format`, `--testnet`, `--config`, `--quiet`, `--dry-run`, `--dex`.
- Domain command groups scaffolded: `info`, `order`, `position`, `account`, `ws`, `config`, `agent`, `hip3`.
- `go test ./...` passes.

## Issue 2 — Implement Config Loader + `config init/show/test`
**Labels:** `phase-1`, `config`, `security`

### Description
Implement Viper-backed config resolution + env override support and interactive initialization.

### Acceptance Criteria
- Supports `~/.hlgo/config.yaml` and `HL_CONFIG` override.
- Env var based key references (`HL_AGENT_KEY`, `HL_MASTER_KEY`) only.
- `config show` redacts secrets.
- `config test` validates API connectivity and agent approval status.
- Unit tests for missing/invalid config + redaction behavior.

## Issue 3 — Signer Wrapper + Python SDK Signature Vectors
**Labels:** `phase-1`, `signing`, `critical`

### Description
Wrap `sonirico/go-hyperliquid` signing APIs with explicit wallet selection and deterministic unit tests from Python SDK vectors.

### Acceptance Criteria
- L1 phantom signing path implemented (agent wallet).
- User-signed path implemented (master wallet).
- All required deterministic signature vectors ported and passing.
- Nonce generation helper uses Unix ms.
- CI job runs signer vector tests on every PR.

## Issue 4 — Decimal-Safe Wire Formatting (`PriceToWire`, `SizeToWire`)
**Labels:** `phase-1`, `precision`, `critical`

### Description
Implement `pkg/wire` using `shopspring/decimal` only, with strict rounding/tick-size rules.

### Acceptance Criteria
- Implements 5 sigfig + max decimals rule by market type.
- Integer pass-through behavior implemented.
- Size rounding to `szDecimals` implemented.
- Exhaustive table-driven tests added from spec examples.
- Randomized differential test against sonirico float64 behavior (report divergences).

## Issue 5 — Asset Resolver + Metadata Cache
**Labels:** `phase-1`, `resolver`, `performance`

### Description
Build resolver for perp/spot/HIP-3 assets with disk-backed metadata cache and TTL.

### Acceptance Criteria
- Resolves coins to asset IDs for all market types.
- Handles `dex:COIN` HIP-3 format.
- Fetches and caches `meta`, `spotMeta`, and `perpDexs`.
- TTL configurable via config (`metadata_ttl`).
- Unit tests for formula correctness and edge cases.

## Issue 6 — M1 Info Commands (Core Read Surface)
**Labels:** `phase-2`, `info`, `mvp`

### Description
Implement MVP read commands: state, mids, book, trades, open-orders, fills, meta, funding, perp-dexs.

### Acceptance Criteria
- All listed commands return JSON by default and support `--format table`.
- Address optional args default to configured agent wallet.
- `--dex` and `--spot` behavior implemented where applicable.
- Errors follow standard JSON schema on stderr.
- Unit tests for request payload shaping.

## Issue 7 — M1 Order Lifecycle Commands
**Labels:** `phase-3`, `exchange`, `mvp`

### Description
Implement order place/market/cancel/cancel-all/modify/batch with agent wallet signing.

### Acceptance Criteria
- `order place` maps all required wire fields correctly.
- Rounding uses `pkg/wire` before sign/send.
- `order market` computes IOC price from mids + slippage.
- `cancel` supports OID and CLOID.
- Integration test: place → verify open → modify → cancel on testnet.

## Issue 8 — Position + Account Commands (M1 Signed Actions)
**Labels:** `phase-4`, `account`, `exchange`

### Description
Implement leverage/margin and master-wallet account actions (transfer, withdraw, class-transfer, send-asset, approve-agent, dex-abstraction).

### Acceptance Criteria
- Agent vs master wallet enforcement implemented.
- Dangerous commands require `--confirm` unless `--dry-run`.
- Successful action responses normalized to JSON output schema.
- Integration tests for at least one user-signed flow.

## Issue 9 — Agent Compound Commands + E2E Agent Simulation
**Labels:** `phase-5`, `agent`, `testing`

### Description
Implement `agent snapshot`, `agent pnl`, `agent bracket` and add subprocess simulation script.

### Acceptance Criteria
- `agent snapshot` combines state + spot + open orders + fills + mids.
- `agent bracket` submits entry + TP + SL batch.
- E2E script runs full lifecycle against testnet.
- Script validates JSON parseability using `jq`.

## Issue 10 — CI Pipeline (Unit + Integration)
**Labels:** `phase-6`, `ci`, `quality`

### Description
Set up GitHub Actions with fast unit test gates and protected integration lane.

### Acceptance Criteria
- PR workflow runs unit tests for signer/wire/resolver.
- Main-only workflow runs integration tests with testnet secret key.
- `golangci-lint` (or equivalent static checks) integrated.
- Build artifacts generated for Linux amd64 + Darwin arm64.


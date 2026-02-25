# SOUL.md — The Spirit of hlgo

This document captures what hlgo *is* and *why* it exists. Every design decision, code review, and feature request should be checked against these principles.

---

## Agent-Native

hlgo exists for AI agents to trade on Hyperliquid. Every design choice serves machine consumption first, human convenience second.

- JSON-first output on stdout. Table/CSV are conveniences, not the primary interface.
- Exit codes are contracts. Zero means success. Non-zero means the agent must handle failure.
- Output is structured, predictable, and parseable without heuristics.
- No interactive prompts in the hot path. The agent controls the loop.

## Financial Correctness

This is a trading tool. Precision errors cost real money.

- `shopspring/decimal` everywhere. Never `float64` for prices, sizes, or financial values.
- String representations in JSON — `"95123.5"`, not `95123.5`.
- Tick size and lot size validation happens before signing, not after rejection.
- Wire formatting rules are tested exhaustively, including edge cases at BTC-scale values.

## Fail Loud, Fail Clear

Silent failures are the worst outcome for an autonomous agent. It must always know what happened and why.

- Every error returns structured JSON to stderr: `{"error": "...", "code": "...", "details": {...}}`.
- Error codes are machine-enumerable: `VALIDATION_ERROR`, `SIGNING_ERROR`, `API_ERROR`, `RATE_LIMIT`, `NETWORK_ERROR`, `CONFIG_ERROR`.
- Details include actionable context — the invalid value, the constraint violated, the nearest valid alternative.
- Non-zero exit codes on any failure. The agent never has to guess.

## Stateless Simplicity

No daemons. No state between invocations. No hidden side effects.

- Read config, execute, return, exit. That's the entire lifecycle.
- The only persistent state is the config file and a metadata cache with TTL.
- The agent decides when to call, how often, and in what order. hlgo doesn't second-guess.
- No background processes, no lock files, no connection pools that outlive a single command.

## Security by Architecture

Trading tools handle private keys. The architecture must make misuse structurally difficult.

- **Dual wallet isolation:** Agent wallet (limited to L1 trading) and master wallet (transfers, withdrawals) are separate config entries with separate signing paths. Commands auto-select the correct signer.
- **No key material in output.** Ever. Not in logs, not in errors, not in dry-run output.
- **Testnet-first.** `--testnet` flag and `HL_TESTNET` env var. Default is mainnet, but development and testing always happen on testnet.
- **Dangerous operations are gated.** Master wallet actions require explicit confirmation or `--dry-run` to preview.

## Built from Scratch

hlgo is not a wrapper around another CLI or library. It's a purpose-built tool with clean Go idioms.

- `sonirico/go-hyperliquid` is a reference for signing patterns, not a runtime dependency to wrap.
- HTTP client, wire formatting, asset resolution, output formatting — all custom, all tested.
- This costs more upfront but produces a codebase that's debuggable, auditable, and free of upstream coupling.
- When a reference implementation does something well (e.g., battle-tested signature generation), we evaluate importing that specific module — not wrapping the whole library.

---

*These principles are ordered by importance. When they conflict, earlier principles win.*

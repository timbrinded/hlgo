# hlgo

`hlgo` is a Go CLI for Hyperliquid built for autonomous and semi-autonomous agents.

It is designed so agents can:
- query market/account state,
- place/cancel orders,
- parse deterministic JSON output,
- branch on machine-readable error codes.

Project principles are in [SOUL.md](./SOUL.md). Repository-level engineering rules are in [AGENTS.md](./AGENTS.md).

## Why Use `hlgo`

Use `hlgo` when you want a stateless process-per-call interface to Hyperliquid that is easy for agents to orchestrate.

What it optimizes for:
- JSON-first output on `stdout`.
- Structured errors on `stderr` with stable error codes.
- Decimal-safe financial handling (`shopspring/decimal`, no `float64` in trading paths).
- Explicit signing architecture for Hyperliquid action types.
- No background daemon or hidden process state.

## Where `hlgo` Fits

Typical agent loop:
1. Read market/account state with `hlgo info ...`.
2. Decide action in your agent logic.
3. Preview with `--dry-run`.
4. Execute with `hlgo order ...`.
5. Verify with `hlgo info open-orders` / `hlgo info fills` / `hlgo info order-status`.

`hlgo` is the execution/data edge. Portfolio strategy and decision logic stay in your agent layer.

## Current Implementation Status

Implemented and production-relevant today:
- `version` command
- `config` command group (`init`, `show`, `test`)
- `info` command group (mids, metadata, book, trades, candles, funding, user state/open orders/fills/order status/rate limits, perp dex list)
- `agent` command group (`snapshot`, `pnl`, `bracket`)
- `order` command group (`place`, `market`, `cancel`, `cancel-all`, `modify`, `batch`, `schedule-cancel`)
- `position` command group (`leverage`, `margin`)
- `account` command group (`transfer`, `class-transfer`, `withdraw`, `send-asset`, `approve-agent`, `set-abstraction`)

## Install and Build

Prerequisites:
- Go `1.26+`

Build local binary:

```bash
make build
```

Output binary:
- `bin/hlgo`

Install to your Go bin path:

```bash
make install
```

## Networks, Endpoints, and Storage Locations

Network selection:
- Mainnet default
- Testnet with `--testnet` or `HL_TESTNET=true`

Base endpoints:
- Mainnet: `https://api.hyperliquid.xyz`
- Testnet: `https://api.hyperliquid-testnet.xyz`

Internal override (useful for local testing only):
- `HLGO_API_URL`

File locations:
- Config default: `~/.hlgo/config.yaml`
- Metadata cache: `~/.hlgo/cache/mainnet/` or `~/.hlgo/cache/testnet/`

## Configuration

Initialize config:

```bash
hlgo config init
```

Minimal default config file (created by `config init`):

```yaml
agent_key_env: HL_AGENT_KEY
master_key_env: HL_MASTER_KEY
default_dex: ""
metadata_ttl: 300
```

Field meanings:
- `agent_key_env`: env var name for agent private key (used by `order`, `position`, and `agent bracket`).
- `master_key_env`: env var name for master key (used by `account` commands).
- `default_dex`: default HIP-3 dex for commands that support dex selection.
- `metadata_ttl`: metadata cache TTL (seconds).

Set key env var:

```bash
export HL_AGENT_KEY=0xYOUR_PRIVATE_KEY
```

Inspect resolved config:

```bash
hlgo config show
```

Connectivity test:

```bash
hlgo config test --testnet
```

## Global Flags and Env Vars

Global flags:
- `--format` (`json|table|csv`, default `json`)
- `--testnet`
- `--config`
- `--quiet`
- `--dry-run`
- `--dex`

Supported env var bindings for global config:
- `HL_FORMAT`
- `HL_TESTNET`
- `HL_CONFIG`
- `HL_DEX`

Note:
- `--quiet` and `--dry-run` are flag-only by design (no env var binding).

## Machine Contract for Agents

### Standard Output

- Default output is JSON on `stdout`.
- `--format table` and `--format csv` are for human inspection.

`hlgo version` also emits JSON metadata:

```json
{"version":"v0.1.0","commit":"abc1234","date":"2026-02-25T08:00:00Z"}
```

### Error Output

All errors are structured JSON on `stderr`:

```json
{
  "error": "human-readable message",
  "code": "MACHINE_CODE",
  "details": {
    "optional": "context"
  }
}
```

Error codes:
- `VALIDATION_ERROR`
- `SIGNING_ERROR`
- `API_ERROR`
- `RATE_LIMIT`
- `NETWORK_ERROR`
- `CONFIG_ERROR`

Exit code mapping:
- `VALIDATION_ERROR` -> `1`
- `CONFIG_ERROR` -> `2`
- `NETWORK_ERROR` -> `3`
- `API_ERROR` -> `4`
- `SIGNING_ERROR` -> `5`
- `RATE_LIMIT` -> `6`

### Dry-Run Behavior

- `info ... --dry-run`: prints request payload JSON that would be sent to `/info`.
- Mutating commands (`order`, `position`, `account`, `agent bracket`) print action payloads without signing/sending.
- `agent snapshot --dry-run` and `agent pnl --dry-run`: print composed request sets.

## Symbol and Asset Resolution

Resolver supports:
- Perps by coin symbol: `BTC`, `ETH`
- Spot by base coin or pair: `PURR`, `PURR/USDC`
- HIP-3 perps with dex prefix: `dex:COIN` (example `xyz:XYZ100`)
- Numeric asset IDs (passthrough)

When symbol formats are unclear, use lookup first:

```bash
hlgo info lookup charizardusd --dex tngs --testnet
hlgo info lookup 110000 --testnet
```

`info lookup` returns canonical `coin` values to use with trading commands and the matching `asset_id`.

Important market-order note:
- `order market` needs mids lookup, so use named symbols (not numeric asset IDs).
- Spot aliases are internally mapped to canonical pair symbols before mids lookup.

## Command Reference (Implemented)

Top-level command groups:
- `hlgo version`
- `hlgo config ...`
- `hlgo info ...`
- `hlgo agent ...`
- `hlgo order ...`
- `hlgo position ...`
- `hlgo account ...`

Detailed command surface (flags + examples for every implemented command):
- [`skill/hlgo/references/command-reference.md`](./skill/hlgo/references/command-reference.md)

Notes:
- User-scoped info commands derive address from `HL_AGENT_KEY` if `--address` is omitted.
- `info funding` currently requires a `<coin>` argument even with `--predicted`.

## Trading Recipes (Testnet)

Preflight:

```bash
export HL_AGENT_KEY=0xYOUR_TESTNET_AGENT_PRIVATE_KEY
hlgo config test --testnet
```

Inspect mids:

```bash
hlgo info mids --testnet
```

Inspect available spot pairs:

```bash
hlgo info meta --spot --testnet
```

Perp limit buy:

```bash
hlgo order place --testnet --coin BTC --side buy --price 30000 --size 0.001 --dry-run
hlgo order place --testnet --coin BTC --side buy --price 30000 --size 0.001
```

Spot limit buy:

```bash
hlgo order place --testnet --coin BTC/USDC --side buy --price 30000 --size 0.001 --dry-run
hlgo order place --testnet --coin BTC/USDC --side buy --price 30000 --size 0.001
```

Perp market buys:

```bash
hlgo order market --testnet --coin BTC --side buy --size 0.001 --slippage 1 --dry-run
hlgo order market --testnet --coin BTC --side buy --size 0.001 --slippage 1

hlgo order market --testnet --coin ETH --side buy --size 0.01 --slippage 1 --dry-run
hlgo order market --testnet --coin ETH --side buy --size 0.01 --slippage 1
```

Spot market buys:

```bash
hlgo order market --testnet --coin BTC/USDC --side buy --size 0.001 --slippage 1 --dry-run
hlgo order market --testnet --coin BTC/USDC --side buy --size 0.001 --slippage 1

hlgo order market --testnet --coin ETH/USDC --side buy --size 0.01 --slippage 1 --dry-run
hlgo order market --testnet --coin ETH/USDC --side buy --size 0.01 --slippage 1
```

If your testnet spot symbols are unit-prefixed, use the market names returned by:

```bash
hlgo info meta --spot --testnet
```

Cancel by OID:

```bash
hlgo order cancel --testnet --coin BTC --oid 12345 --dry-run
hlgo order cancel --testnet --coin BTC --oid 12345
```

Cancel all BTC orders:

```bash
hlgo order cancel-all --testnet --coin BTC --dry-run
hlgo order cancel-all --testnet --coin BTC
```

## Agent Integration Example

Example shell flow for an agent worker:

```bash
set -euo pipefail

# 1) read mids
mids_json="$(hlgo info mids --testnet)"
btc_mid="$(echo "$mids_json" | jq -r '.BTC')"

# 2) basic decision stub
if [ "$(echo "$btc_mid > 0" | bc -l)" -eq 1 ]; then
  # 3) dry-run first
  hlgo order market --testnet --coin BTC --side buy --size 0.001 --slippage 0.5 --dry-run >/dev/null

  # 4) execute
  hlgo order market --testnet --coin BTC --side buy --size 0.001 --slippage 0.5
fi
```

## Security and Operational Guidance

- Never store private keys in repo files.
- Use env vars for key material (`HL_AGENT_KEY`, etc.).
- Prefer testnet during strategy and integration development.
- Run with `--dry-run` before all new flows.
- Treat `stderr` as structured machine data, not free text.

## Development

Common commands:

```bash
make build
make test
make lint
make check
```

Release instructions:
- [`RELEASE.md`](./RELEASE.md) documents the end-to-end release process (preflight, snapshot validation, tagging, and verification).

Versioned build:

```bash
go build -ldflags "-X main.version=x.y.z -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o hlgo .
```

Release tag flow:

```bash
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

## Troubleshooting

Common failure patterns:
- `CONFIG_ERROR` with missing key env var:
  - set `HL_AGENT_KEY` for `order`/`position`/`agent bracket`.
  - set `HL_MASTER_KEY` for `account` commands.
  - or update `agent_key_env` / `master_key_env` in config.
- `VALIDATION_ERROR` on order price/size:
  - check tick/lot constraints, side/tif values, and symbol format.
- `VALIDATION_ERROR` on market order mids:
  - verify coin exists in `hlgo info mids --testnet` (or selected network/dex).
- `NETWORK_ERROR` / `RATE_LIMIT`:
  - retry with backoff and inspect network/testnet status.

For command-specific options, run:

```bash
hlgo --help
hlgo info --help
hlgo agent --help
hlgo order --help
hlgo position --help
hlgo account --help
hlgo config --help
```

## License

This project is licensed under the MIT License. See [`LICENSE`](./LICENSE).

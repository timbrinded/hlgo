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
- `info` command group (mids, metadata, book, trades, candles, funding, user state/open orders/fills/order status/rate limits, perp dex list)
- `order` command group (`place`, `market`, `cancel`, `cancel-all`)
- `config` command group (`init`, `show`, `test`)

Scaffolded but not yet implemented with subcommands:
- `position`
- `account`

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
- `agent_key_env`: env var name for agent private key (used by implemented order commands).
- `master_key_env`: env var name for master key (reserved for future account commands).
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
- `order place --dry-run` and `order market --dry-run`: prints resolved action payload (and resolved order fields) without signing/sending.
- `order cancel ... --dry-run`: prints cancel action payload without signing/sending.

## Symbol and Asset Resolution

Resolver supports:
- Perps by coin symbol: `BTC`, `ETH`
- Spot by base coin or pair: `PURR`, `PURR/USDC`
- HIP-3 perps with dex prefix: `dex:COIN` (example `xyz:XYZ100`)
- Numeric asset IDs (passthrough)

Important market-order note:
- `order market` needs mids lookup, so use named symbols (not numeric asset IDs).
- Spot aliases are internally mapped to canonical pair symbols before mids lookup.

## Command Reference (Implemented)

### `info`

Read-only market and account data.

Commands:
- `hlgo info mids`
- `hlgo info meta [--spot] [--dex <name>]`
- `hlgo info meta-and-ctxs [--spot] [--dex <name>]`
- `hlgo info book <coin> [--sigfigs N] [--mantissa 1|2|5] [--depth N]`
- `hlgo info trades <coin>`
- `hlgo info candles <coin> <interval> [--start ...] [--end ...]`
- `hlgo info funding <coin> [--start ...] [--end ...] [--predicted]`
- `hlgo info state [--address ...] [--dex <name>]`
- `hlgo info spot-state [--address ...]`
- `hlgo info open-orders [--address ...] [--dex <name>]`
- `hlgo info fills [--address ...] [--start ...] [--end ...] [--aggregate-by-time]`
- `hlgo info order-status <oid-or-cloid> [--address ...]`
- `hlgo info rate-limit [--address ...]`
- `hlgo info perp-dexs`

Notes:
- User-scoped info commands derive address from `HL_AGENT_KEY` if `--address` is omitted.
- `info funding` currently requires a `<coin>` argument even with `--predicted`.

### `order`

Signed trading actions using agent wallet (L1 phantom agent signing path).

Commands:
- `hlgo order place`
- `hlgo order market`
- `hlgo order cancel`
- `hlgo order cancel-all`

### `order place`

Required:
- `--coin`
- `--side buy|sell`
- `--price`
- `--size`

Common optional flags:
- `--tif gtc|ioc|alo`
- `--reduce`
- `--cloid`
- `--builder` with `--builder-fee-tenths-bp`
- `--vault`
- `--expires-after <unix_ms|iso8601>`

### `order market`

Required:
- `--coin`
- `--side buy|sell`
- `--size`

Optional:
- `--slippage` (percent, default `0.5`)
- `--builder` with `--builder-fee-tenths-bp`
- `--vault`
- `--expires-after`

Semantics:
- Market order is implemented as aggressive IOC limit.
- Price source: current mids.
- Slippage is percent (`1` means 1%).
- Price is snapped to nearest wire-valid price before signing.

### `order cancel`

Required:
- `--coin`
- Exactly one of `--oid` or `--cloid`

Optional:
- `--vault`
- `--expires-after`

### `order cancel-all`

Optional:
- `--coin` (filter)
- `--vault`
- `--expires-after`

Behavior:
- Fetches open orders first, then submits cancel action for matching orders.

### `config`

Commands:
- `hlgo config init`
- `hlgo config show`
- `hlgo config test`

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

Versioned build:

```bash
go build -ldflags "-X main.version=x.y.z" -o hlgo .
```

## Troubleshooting

Common failure patterns:
- `CONFIG_ERROR` with missing key env var:
  - set `HL_AGENT_KEY`, or update `agent_key_env` in config.
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
hlgo order --help
hlgo config --help
```

# Hyperliquid Go CLI — Technical Specification

**Project:** `hlgo` — A Go CLI for Hyperliquid, designed as a Claude Code skill tool  
**Version:** 0.1.0 (Draft)  
**Date:** 2026-02-24  
**Author:** Tim / Moonsong Labs

-----

## 1. Purpose & Design Philosophy

`hlgo` is a Go command-line tool that wraps the full Hyperliquid API (perps, spot, HIP-3) for use as a **Claude Code skill**. The agent calls `hlgo` subcommands, receives structured JSON, and reasons about trading decisions.

### Design Principles

1. **JSON-first output.** Every command returns valid JSON to stdout by default. Human-readable table format via `--format table`. Errors return JSON to stderr with a consistent schema: `{"error": "<message>", "code": "<type>"}`.
2. **Stateless per invocation.** No daemon, no long-running process. Read config, execute, return, exit. The agent controls the loop.
3. **Dual wallet architecture.** Agent wallet (limited permissions) for all trading. Master wallet for transfers, withdrawals, and agent approval. Config specifies both; commands auto-select the correct signer.
4. **Decimal-everywhere.** All prices, sizes, and financial values use `shopspring/decimal` internally and string representations in JSON. Never `float64`.
5. **Fail loud.** Non-zero exit codes on any error. The agent must be able to distinguish success from failure programmatically.
6. **Testnet-first development.** `--testnet` flag (or `HL_TESTNET=true`) switches all endpoints. Default is mainnet.

-----

## 2. Target API Surface

The Hyperliquid API has three transport layers, all targeting `https://api.hyperliquid.xyz` (mainnet) or `https://api.hyperliquid-testnet.xyz` (testnet):

|Transport   |Endpoint                      |Auth Required                       |
|------------|------------------------------|------------------------------------|
|Info API    |`POST /info`                  |No                                  |
|Exchange API|`POST /exchange`              |Yes (EIP-712 signatures)            |
|WebSocket   |`wss://api.hyperliquid.xyz/ws`|No (user-specific subs need address)|

### 2.1 Asset ID Schema

The CLI must resolve human-readable coin names to integer asset IDs transparently:

|Market Type    |Asset ID Formula                                   |Example                |
|---------------|---------------------------------------------------|-----------------------|
|Validator perps|Index in `meta.universe`                           |ETH → `1`              |
|Spot           |`10000 + spotMeta.universe[i].index`               |PURR/USDC → `10000`    |
|HIP-3 perps    |`100000 + (perp_dex_index × 10000) + index_in_meta`|`xyz:XYZ100` → `110000`|

HIP-3 coins always use `{dex}:{coin}` format (e.g. `xyz:XYZ100`, `trove:AAPL`). The CLI accepts either the full name or the computed asset ID.

### 2.2 Signing Architecture

Two distinct signing paths, both using EIP-712 typed data:

**L1 Actions (Phantom Agent Signing)** — used for all trading operations:

- Chain ID: `1337`
- Domain name: `"Exchange"`
- Flow: serialize action → msgpack encode → keccak256 hash → construct phantom agent `{source, connectionId}` → EIP-712 sign the phantom agent
- **Agent wallet CAN sign these** (this is the primary path for the agent)

**User-Signed Actions** — used for transfers, withdrawals, agent management:

- Chain ID: `0xa4b1` (Arbitrum, 42161)
- Domain name: `"HyperliquidSignTransaction"`
- Flow: construct EIP-712 typed data directly → sign
- **Only master wallet can sign these**

### 2.3 Nonces

Set-based, not sequential. Use current Unix timestamp in milliseconds. Each nonce can only be used once per signer address. The CLI generates nonces automatically.

### 2.4 Rate Limits

|Axis         |Limit                                       |Notes                                        |
|-------------|--------------------------------------------|---------------------------------------------|
|IP-based     |1,200 weight/minute                         |Info queries: 2–20 weight; Exchange: 1 weight|
|Address-based|1 req per $1 lifetime volume + 10,000 buffer|Stale cancels cost 5×                        |
|WebSocket    |100 connections, 1,000 subs, 10 users per IP|—                                            |

The CLI should track weight consumption and warn (not block) when approaching limits. The agent can then decide to back off.

-----

## 3. Command Structure

All commands follow the pattern: `hlgo <domain> <action> [flags]`

Global flags available on every command:

|Flag       |Env Var     |Default               |Description                                  |
|-----------|------------|----------------------|---------------------------------------------|
|`--format` |`HL_FORMAT` |`json`                |Output format: `json`, `table`, `csv`        |
|`--testnet`|`HL_TESTNET`|`false`               |Use testnet endpoints                        |
|`--config` |`HL_CONFIG` |`~/.hlgo/config.yaml`|Config file path                             |
|`--quiet`  |—           |`false`               |Suppress non-essential output                |
|`--dry-run`|—           |`false`               |Show what would be signed/sent, don’t execute|
|`--dex`    |`HL_DEX`    |`""`                  |HIP-3 perp dex name (empty = validator perps)|

-----

## 4. Milestone 1 — Core Agent Loop (MVP)

**Goal:** Agent can check state, read markets, place/cancel orders, and manage positions across perps, spot, and HIP-3.

### 4.1 Info Commands (Read-Only, No Auth)

#### `hlgo info mids`
Get all mid-market prices.

```json
POST /info {"type": "allMids"}
```

Optional: `--dex <name>` for HIP-3 dex mids.
Returns: `{"BTC": "95123.5", "ETH": "3412.1", ...}`

#### `hlgo info meta`
Get universe metadata.

```json
POST /info {"type": "meta"}
POST /info {"type": "meta", "dex": "xyz"}
POST /info {"type": "spotMeta"}
```

Flags: `--spot`, `--dex <name>`

#### `hlgo info meta-and-ctxs`
Combined metadata + market context.

```json
POST /info {"type": "metaAndAssetCtxs"}
POST /info {"type": "metaAndAssetCtxs", "dex": "xyz"}
POST /info {"type": "spotMetaAndAssetCtxs"}
```

Flags: `--spot`, `--dex <name>`

#### `hlgo info book <coin>`
L2 order book.

```json
POST /info {"type": "l2Book", "coin": "ETH"}
POST /info {"type": "l2Book", "coin": "ETH", "nSigFigs": 5}
POST /info {"type": "l2Book", "coin": "xyz:XYZ100"}
```

Flags: `--depth <n>`, `--sigfigs <n>`

#### `hlgo info trades <coin>`
Recent trades.

```json
POST /info {"type": "recentTrades", "coin": "ETH"}
```

Flags: `--limit <n>` (default 50)

#### `hlgo info candles <coin> <interval>`
OHLCV candlestick data.

```json
POST /info {"type": "candleSnapshot", "coin": "ETH", "interval": "1h", "startTime": ..., "endTime": ...}
```

Intervals: `1m`, `3m`, `5m`, `15m`, `30m`, `1h`, `2h`, `4h`, `8h`, `12h`, `1d`, `3d`, `1w`, `1M`  
Flags: `--start <ISO>`, `--end <ISO>`, `--limit <n>`

#### `hlgo info funding <coin>`
Current and predicted funding rates.

```json
POST /info {"type": "fundingHistory", "coin": "ETH", "startTime": ..., "endTime": ...}
POST /info {"type": "predictedFundings"}
```

Flags: `--predicted`

#### `hlgo info state [address]`
Full clearinghouse state.

```json
POST /info {"type": "clearinghouseState", "user": "0x..."}
POST /info {"type": "clearinghouseState", "user": "0x...", "dex": "xyz"}
```

Defaults to agent wallet address if no address provided.

#### `hlgo info spot-state [address]`
Spot balances and holds.

```json
POST /info {"type": "spotClearinghouseState", "user": "0x..."}
```

#### `hlgo info open-orders [address]`
All open orders for user.

```json
POST /info {"type": "frontendOpenOrders", "user": "0x..."}
POST /info {"type": "frontendOpenOrders", "user": "0x...", "dex": "xyz"}
```

#### `hlgo info fills [address]`
Recent fills.

```json
POST /info {"type": "userFills", "user": "0x..."}
POST /info {"type": "userFillsByTime", "user": "0x...", "startTime": ..., "endTime": ...}
```

#### `hlgo info order-status <oid>`
Order status by OID or CLOID.

```json
POST /info {"type": "orderStatus", "user": "0x...", "oid": 12345}
```

#### `hlgo info rate-limit [address]`
Rate limit status.

```json
POST /info {"type": "userRateLimit", "user": "0x..."}
```

#### `hlgo info perp-dexs`
List all HIP-3 perp dexes.

```json
POST /info {"type": "perpDexs"}
```

### 4.2 Exchange Commands (Signed, Agent Wallet)

All exchange commands use the **agent wallet** for signing (L1 phantom agent path).

#### `hlgo order place`
Place a limit order.

Key flags:
- `--coin`, `--side`, `--price`, `--size`
- `--tif <gtc|ioc|alo>`
- `--reduce`
- `--cloid`
- `--tp`, `--sl`
- `--builder-fee`
- `--vault`

The CLI resolves asset IDs, tick size, and lot size from metadata, then rounds and signs correctly.

#### `hlgo order market`
IOC convenience wrapper with slippage-adjusted price.

#### `hlgo order cancel` / `cancel-all`
Cancel by OID, CLOID, per coin, or globally.

#### `hlgo order modify`
Modify existing order price/size.

#### `hlgo order batch`
Batch place orders from `--file orders.json`.

#### `hlgo position leverage`
Set leverage and margin mode.

#### `hlgo position margin`
Update isolated margin.

#### `hlgo order schedule-cancel`
Dead man’s switch.

### 4.3 Account Commands (Signed, Master Wallet)

- `hlgo account transfer`
- `hlgo account withdraw`
- `hlgo account class-transfer`
- `hlgo account send-asset`
- `hlgo account approve-agent`
- `hlgo account dex-abstraction`

### 4.4 Configuration & Setup

#### `hlgo config init`
Interactive setup creating `~/.hlgo/config.yaml`.

#### `hlgo config show`
Resolved config with key redaction.

#### `hlgo config test`
Wallet connectivity + agent approval checks.

-----

## 5. Milestone 2 — Extended Coverage

- Additional Info commands (`portfolio`, `user-funding`, `ledger`, `user-fees`, `token-details`, `sub-accounts`, `historical-orders`, `borrow-lend`, `vault`, `oi-cap`, `exchange-status`, `max-order`)
- WebSocket streaming commands (`ws trades`, `ws book`, `ws bbo`, `ws fills`, `ws orders`, `ws funding`, `ws candles`) with reconnect, heartbeat, and timeout support
- Advanced order types (`twap`, `twap-cancel`)
- Additional account commands (`spot-send`, `stake`, `vault-transfer`)

-----

## 6. Milestone 3 — Stretch Goals

- HIP-3 deployer/admin commands
- Compound agent workflow commands (`agent snapshot`, `agent pnl`, `agent bracket`)
- Data export commands (`export fills`, `export funding`, `export ledger`)
- Multi-dex aggregated state commands

-----

## 7. Technical Architecture

### 7.1 Package Layout

```text
hlgo/
├── cmd/
├── pkg/
│   ├── client/
│   ├── info/
│   ├── exchange/
│   ├── signer/
│   ├── resolver/
│   ├── wire/
│   ├── config/
│   └── output/
├── main.go
├── go.mod
└── go.sum
```

### 7.2 Core Dependencies

- `github.com/sonirico/go-hyperliquid`
- `github.com/spf13/cobra`
- `github.com/spf13/viper`
- `github.com/shopspring/decimal`
- `github.com/coder/websocket`
- `github.com/olekukonko/tablewriter`

### 7.3 Metadata Cache

Cache `meta`, `spotMeta`, `perpDexs` for 5 minutes by default under `~/.hlgo/cache/`.

### 7.4 Tick & Lot Size Precision Rules

- Perps `MAX_DECIMALS=6`; spot `MAX_DECIMALS=8`
- Price: max 5 significant figures, max `MAX_DECIMALS - szDecimals`, integer always valid
- Size: round to `szDecimals`, strip trailing zeros
- All operations with `shopspring/decimal`; no `float64`

### 7.5 Error Schema

```json
{
  "error": "Order rejected: price does not align to tick size 0.1",
  "code": "VALIDATION_ERROR",
  "details": {
    "field": "price",
    "value": "3412.15",
    "tick_size": "0.1",
    "nearest_valid": "3412.1"
  }
}
```

Error codes: `VALIDATION_ERROR`, `SIGNING_ERROR`, `API_ERROR`, `RATE_LIMIT`, `NETWORK_ERROR`, `CONFIG_ERROR`.

-----

## 8. Claude Code Skill Definition

The eventual `SKILL.md` should document:
- read-state commands
- trade commands
- HIP-3 notes
- account commands
- precision + dry-run + testnet guidance

-----

## 9. Testing Strategy

### 9.1 Signing test vectors
Port deterministic vectors from `hyperliquid-python-sdk/tests/signing_test.py` into `pkg/signer/signer_test.go`.

### 9.2 Price/size wire tests
Comprehensive table-driven tests for `pkg/wire/wire.go`, including BTC-scale edge cases.

### 9.3 Asset ID resolver tests
Unit tests for perp/spot/HIP-3 resolution and formula correctness.

### 9.4 Integration tests (`-tags=integration`)
Testnet flow: info round-trip, order lifecycle, spot lifecycle, HIP-3 lifecycle, user-signed action, agent approval, websocket connectivity.

### 9.5 Agent simulation e2e
Scripted CLI subprocess flow validating end-to-end JSON output and lifecycle behavior.

### 9.6 CI
- PRs: offline unit tests
- Main branch: integration tests with testnet secret wallet

-----

## 10. Build & Release

```bash
go build -o hlgo .
GOOS=linux GOARCH=amd64 go build -o hlgo-linux-amd64 .
GOOS=darwin GOARCH=arm64 go build -o hlgo-darwin-arm64 .
go install .
```

-----

## 11. Security Considerations

1. Never log private keys.
2. Agent wallet isolated to L1 trading actions.
3. Dangerous operations gated with explicit confirmation and dry-run support.
4. No key material in output.
5. Testnet-first workflow for development.

-----

## 12. Development Sequence

1. Skeleton + signer + precision + resolver tests
2. Core info commands
3. Core order lifecycle
4. Position/account commands
5. Agent compound + HIP-3 specifics
6. Agent simulation + CI
7. WebSocket
8. Extended info coverage
9. Stretch goals

Estimated MVP: **~8–13 focused days**.

-----

## Appendix A — API Quick Reference

Includes reference lists for:
- `POST /info` `type` values by milestone
- `POST /exchange` action types with signing path + wallet
- WebSocket subscription types

(See source specification for exhaustive endpoint/action matrix.)

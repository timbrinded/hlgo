# hlgo Claude Code Skill

## Overview

`hlgo` is a Hyperliquid CLI designed for agent-native workflows.

- Primary contract: JSON to `stdout`
- Errors: structured JSON to `stderr`
- Success: exit code `0`
- Failure: non-zero exit code with machine-readable `code`

Use pattern:

```bash
hlgo <domain> <action> [flags]
```

## Global Flags

| Flag | Env | Default | Notes |
|---|---|---|---|
| `--format` | `HL_FORMAT` | `json` | `json`, `table`, `csv` |
| `--testnet` | `HL_TESTNET` | `false` | Use testnet API endpoints |
| `--config` | `HL_CONFIG` | `~/.hlgo/config.yaml` | Config file path |
| `--quiet` | n/a | `false` | Suppress non-essential output |
| `--dry-run` | n/a | `false` | Show request/action payloads only |
| `--dex` | `HL_DEX` | `""` | HIP-3 perp dex selection |

## Read State Commands

### `info mids`
- Purpose: fetch all mid prices.
- Key flags: `--dex`.
- Example:
```bash
hlgo info mids --testnet --format json
```
- Example output:
```json
{"BTC":"95123.5","ETH":"3412.1"}
```

### `info meta`
- Purpose: fetch perp or spot metadata.
- Key flags: `--spot`, `--dex`.
- Example:
```bash
hlgo info meta --spot --testnet --format json
```
- Output: metadata object with `universe` (and `tokens` for spot).

### `info meta-and-ctxs`
- Purpose: fetch metadata + context (mark/funding/open-interest context).
- Key flags: `--spot`, `--dex`.
- Example:
```bash
hlgo info meta-and-ctxs --testnet --format json
```
- Output: API passthrough JSON array/object.

### `info book <coin>`
- Purpose: L2 order book snapshot.
- Key flags: `--sigfigs`, `--mantissa`.
- Example:
```bash
hlgo info book ETH --sigfigs 5 --testnet --format json
```
- Output: object with `coin`, `time`, `levels`.

### `info trades <coin>`
- Purpose: recent trades for a coin.
- Key flags: none.
- Example:
```bash
hlgo info trades ETH --testnet --format json
```
- Output: array of trade objects (`px`, `sz`, `side`, `time`, ...).

### `info candles <coin> <interval>`
- Purpose: OHLCV history.
- Key flags: `--start`, `--end`.
- Example:
```bash
hlgo info candles ETH 1h --start 2026-03-01T00:00:00Z --testnet --format json
```
- Output: array of candle objects (`o`, `h`, `l`, `c`, `v`, `t`, `T`).

### `info funding <coin>`
- Purpose: historical funding or predicted funding.
- Key flags: `--predicted`, `--start`, `--end`.
- Example:
```bash
hlgo info funding ETH --predicted --testnet --format json
```
- Output: predicted funding nested structure or funding history list.

### `info state [--address]`
- Purpose: perp clearinghouse state.
- Key flags: `--address`, `--dex`.
- Example:
```bash
hlgo info state --testnet --format json
```
- Output: object with `assetPositions`, `marginSummary`, `withdrawable`, `time`.

### `info spot-state [--address]`
- Purpose: spot balances/holds state.
- Key flags: `--address`.
- Example:
```bash
hlgo info spot-state --testnet --format json
```
- Output: spot clearinghouse state object.

### `info open-orders [--address]`
- Purpose: open order list.
- Key flags: `--address`, `--dex`.
- Example:
```bash
hlgo info open-orders --testnet --format json
```
- Output: array of open order objects.

### `info fills [--address]`
- Purpose: user fills.
- Key flags: `--address`, `--start`, `--end`, `--aggregate-by-time`.
- Example:
```bash
hlgo info fills --start 1700000000000 --end 1700003600000 --testnet --format json
```
- Output: array of fill objects.

### `info order-status <oid-or-cloid>`
- Purpose: query specific order status.
- Key flags: `--address`.
- Example:
```bash
hlgo info order-status 12345 --testnet --format json
```
- Output: status payload from Info API.

### `info rate-limit [--address]`
- Purpose: user rate limit status.
- Key flags: `--address`.
- Example:
```bash
hlgo info rate-limit --testnet --format json
```
- Output: rate-limit info object.

### `info perp-dexs`
- Purpose: discover HIP-3 perp dex names/indexes.
- Key flags: none.
- Example:
```bash
hlgo info perp-dexs --testnet --format json
```
- Output: array of `{name,index,numMarkets}`.

### `agent snapshot`
- Purpose: aggregate `state`, `spot-state`, `open-orders`, `fills`, `mids` into one response.
- Key flags: `--address`.
- Example:
```bash
hlgo agent snapshot --testnet --format json
```
- Output:
```json
{
  "account_value":"50000",
  "perp_positions":[],
  "spot_balances":[],
  "open_orders":[],
  "recent_fills":[],
  "mid_prices":{"ETH":"3000"},
  "timestamp":"2026-03-02T14:00:00Z",
  "partial":false
}
```

### `agent pnl`
- Purpose: compute unrealized/realized/funding PnL from `state`, `mids`, fills, and user funding.
- Key flags: `--address`, `--lookback-hours`, `--aggregate-fills`.
- Example:
```bash
hlgo agent pnl --lookback-hours 24 --testnet --format json
```
- Output:
```json
{
  "address":"0x...",
  "positions":[{"coin":"ETH","size":"0.1","entry_price":"3000","mid_price":"3100","unrealized_pnl":"10","funding_pnl":"-0.02"}],
  "total_unrealized_pnl":"10",
  "realized_pnl":"1.5",
  "total_funding_pnl":"-0.02",
  "total_pnl":"11.48",
  "funding_unavailable":false,
  "timestamp":"2026-03-02T14:00:00Z"
}
```

## Trade Commands

### `order place`
- Purpose: place limit order.
- Key flags: `--coin`, `--side`, `--price`, `--size`, `--tif`, `--reduce`, `--cloid`, `--builder`, `--builder-fee-tenths-bp`, `--vault`, `--expires-after`.
- Example:
```bash
hlgo order place --coin ETH --side buy --price 3000 --size 0.1 --tif gtc --testnet --format json
```
- Output: exchange response JSON, or dry-run action/resolved payload.

### `order market`
- Purpose: IOC wrapper using slippage-adjusted mid price.
- Key flags: `--coin`, `--side`, `--size`, `--slippage`.
- Example:
```bash
hlgo order market --coin ETH --side buy --size 0.1 --slippage 0.5 --testnet --format json
```

### `order cancel`
- Purpose: cancel by OID or CLOID.
- Key flags: `--coin`, one of `--oid` or `--cloid`, optional `--vault`, `--expires-after`.
- Example:
```bash
hlgo order cancel --coin ETH --oid 12345 --testnet --format json
```

### `order cancel-all`
- Purpose: cancel all open orders (optionally filtered by `--coin`).
- Key flags: `--coin`, `--vault`, `--expires-after`.
- Example:
```bash
hlgo order cancel-all --testnet --format json
```

### `order modify`
- Purpose: modify an existing order by OID.
- Key flags: `--coin`, `--oid`, `--side`, optional `--price`, `--size`, `--tif`, `--reduce`, `--cloid`.
- Example:
```bash
hlgo order modify --coin ETH --oid 12345 --side buy --price 2990 --size 0.1 --testnet --format json
```

### `order batch`
- Purpose: place multiple orders from a JSON file in one action.
- Key flags: `--file`, optional `--vault`, `--expires-after`.
- Example:
```bash
hlgo order batch --file ./orders.json --testnet --format json
```

### `order schedule-cancel`
- Purpose: dead-man switch (`scheduleCancel`).
- Key flags: `--time` (Unix ms / ISO8601) or `--clear`.
- Example:
```bash
hlgo order schedule-cancel --time 2026-03-02T15:00:00Z --testnet --format json
```

### `agent bracket`
- Purpose: submit entry + TP + SL in one grouped order action.
- Key flags: `--coin`, `--side`, `--price`, `--size`, `--tp`, `--sl`, plus standard order routing flags.
- Example:
```bash
hlgo agent bracket --coin ETH --side buy --price 3000 --size 0.1 --tp 3100 --sl 2950 --testnet --format json
```
- Output: exchange response or dry-run action with `grouping: "normalTpsl"` and 3 order wires.

### `position leverage`
- Purpose: set leverage and margin mode.
- Key flags: `--coin`, `--leverage`, `--mode` (`cross|isolated`), `--vault`.
- Example:
```bash
hlgo position leverage --coin ETH --leverage 5 --mode cross --testnet --format json
```

### `position margin`
- Purpose: adjust isolated margin.
- Key flags: `--coin`, `--side`, `--amount`, `--vault`.
- Example:
```bash
hlgo position margin --coin ETH --side buy --amount 25 --testnet --format json
```

## Account Commands (Master Wallet)

These commands use the user-signed path and the configured master key env var.

### `account transfer`
- Purpose: transfer USDC between spot/perp classes.
- Key flags: `--amount`, exactly one of `--to-perp` or `--to-spot`.
- Example:
```bash
hlgo account transfer --amount 100 --to-perp --testnet --format json
```

### `account class-transfer`
- Purpose: alias for transfer with `usdClassTransfer` semantics.
- Key flags: same as `account transfer`.
- Example:
```bash
hlgo account class-transfer --amount 100 --to-spot --testnet --format json
```

### `account withdraw`
- Purpose: withdraw USDC to EVM address.
- Key flags: `--destination`, `--amount`, `--confirm` (or `--yes`) unless `--dry-run`.
- Example:
```bash
hlgo account withdraw --destination 0xabc... --amount 50 --confirm --testnet --format json
```

### `account send-asset`
- Purpose: send spot token to another address.
- Key flags: `--destination`, `--token`, `--amount`, `--confirm` (or `--yes`) unless `--dry-run`.
- Example:
```bash
hlgo account send-asset --destination 0xabc... --token PURR:0x1 --amount 10 --confirm --testnet --format json
```

### `account approve-agent`
- Purpose: approve/revoke agent wallet.
- Key flags: `--agent`, `--name` for approve, `--revoke` + `--confirm` for revoke.
- Example:
```bash
hlgo account approve-agent --agent 0xabc... --name trader01 --testnet --format json
```

### `account set-abstraction`
- Purpose: set account abstraction mode.
- Key flags: `--user`, `--abstraction` (`unifiedAccount|portfolioMargin|disabled`).
- Example:
```bash
hlgo account set-abstraction --user 0xabc... --abstraction disabled --testnet --format json
```

## Precision and Safety

- Use decimal strings for all prices/sizes/amounts.
- Never rely on float math in agent logic.
- Always prefer `--dry-run` before live mutations.
- Use `--testnet` during development and simulation.
- Run `agent snapshot` before placing new orders to avoid stale context.

## HIP-3 Notes

- Use `--dex <name>` for HIP-3 perp contexts.
- HIP-3 coin naming: `dex:COIN` (example: `xyz:XYZ100`).
- Discover dexes via:
```bash
hlgo info perp-dexs --format json
```

## Error Handling Contract

On failure, stderr is JSON:

```json
{"error":"...","code":"VALIDATION_ERROR","details":{...}}
```

Machine codes:
- `VALIDATION_ERROR`
- `SIGNING_ERROR`
- `API_ERROR`
- `RATE_LIMIT`
- `NETWORK_ERROR`
- `CONFIG_ERROR`

Do not parse human-readable strings when branching logic; branch on `code` and exit status.

## Agent Workflow Example

```bash
hlgo agent snapshot --testnet --format json
hlgo info mids --testnet --format json
hlgo agent pnl --lookback-hours 24 --testnet --format json
hlgo position leverage --coin ETH --leverage 5 --testnet --format json
hlgo agent bracket --coin ETH --side buy --price 3000 --size 0.1 --tp 3100 --sl 2950 --testnet --format json
hlgo info open-orders --testnet --format json
hlgo order cancel-all --testnet --format json
```

# hlgo Command Reference

## Table of Contents

- [Global Invocation](#global-invocation)
- [Version](#version)
- [Config Commands](#config-commands)
- [Info Commands](#info-commands)
- [Agent Commands](#agent-commands)
- [Order Commands](#order-commands)
- [Position Commands](#position-commands)
- [Account Commands](#account-commands)
- [Order Batch File Shape](#order-batch-file-shape)

## Global Invocation

Use pattern:

```bash
hlgo <group> <command> [flags]
```

Global flags:

| Flag | Env | Default | Notes |
|---|---|---|---|
| `--format` | `HL_FORMAT` | `json` | `json`, `table`, `csv` |
| `--testnet` | `HL_TESTNET` | `false` | Use testnet API endpoints |
| `--config` | `HL_CONFIG` | `~/.hlgo/config.yaml` | Config file path |
| `--quiet` | n/a | `false` | Suppress non-essential output |
| `--dry-run` | n/a | `false` | Print request/action payloads only |
| `--dex` | `HL_DEX` | `""` | HIP-3 perp dex context |

Example:

```bash
hlgo info mids --testnet --format json
```

## Version

| Command | Purpose | Example |
|---|---|---|
| `hlgo version` | Print CLI version string | `hlgo version` |

## Config Commands

| Command | Purpose | Key Flags | Example |
|---|---|---|---|
| `hlgo config init` | Create config file | `--private-key-env`, `--default-dex`, `--metadata-ttl`, `--force` | `hlgo config init --private-key-env HL_PRIVATE_KEY` |
| `hlgo config show` | Show resolved config with key redaction | `--config`, `--testnet`, `--format` | `hlgo config show --testnet` |
| `hlgo config test` | Validate config readability, key envs, and API connectivity | `--config`, `--testnet` | `hlgo config test --testnet` |

## Info Commands

| Command | Purpose | Key Flags | Example |
|---|---|---|---|
| `hlgo info lookup <query>` | Resolve coin identifiers by name fragment or numeric asset ID across perp/spot/HIP-3 | `--all-dexes`, `--limit`, global `--dex` | `hlgo info lookup charizardusd --dex tngs --testnet --format json` |
| `hlgo info mids` | Fetch all mid prices | `--dex` | `hlgo info mids --testnet --format json` |
| `hlgo info meta` | Fetch perp or spot metadata | `--spot`, `--dex` | `hlgo info meta --spot --testnet --format json` |
| `hlgo info meta-and-ctxs` | Fetch metadata plus asset contexts | `--spot`, `--dex` | `hlgo info meta-and-ctxs --testnet --format json` |
| `hlgo info book <coin>` | Fetch L2 book snapshot | `--sigfigs`, `--mantissa`, `--depth` | `hlgo info book ETH --sigfigs 5 --mantissa 2 --depth 20 --testnet` |
| `hlgo info trades <coin>` | Fetch recent trades | none | `hlgo info trades ETH --testnet --format json` |
| `hlgo info candles <coin> <interval>` | Fetch OHLCV snapshots | `--start`, `--end` | `hlgo info candles ETH 1h --start 2026-03-01T00:00:00Z --end 2026-03-02T00:00:00Z --testnet` |
| `hlgo info funding <coin>` | Fetch funding history or predicted funding | `--predicted`, `--start`, `--end` | `hlgo info funding ETH --predicted --testnet --format json` |
| `hlgo info perp-dexs` | List HIP-3 perp dexes | none | `hlgo info perp-dexs --testnet --format json` |
| `hlgo info state` | Fetch perp clearinghouse state | `--address`, `--dex` | `hlgo info state --address 0xabc... --testnet --format json` |
| `hlgo info spot-state` | Fetch spot clearinghouse state | `--address` | `hlgo info spot-state --testnet --format json` |
| `hlgo info open-orders` | Fetch open orders | `--address`, `--dex` | `hlgo info open-orders --testnet --format json` |
| `hlgo info fills` | Fetch fills (latest or time-bounded) | `--address`, `--start`, `--end`, `--aggregate-by-time` | `hlgo info fills --start 1700000000000 --end 1700003600000 --aggregate-by-time --testnet` |
| `hlgo info order-status <oid-or-cloid>` | Fetch single-order status | `--address` | `hlgo info order-status 12345 --testnet --format json` |
| `hlgo info rate-limit` | Fetch user rate-limit info | `--address` | `hlgo info rate-limit --testnet --format json` |

## Agent Commands

| Command | Purpose | Key Flags | Example |
|---|---|---|---|
| `hlgo agent snapshot` | Aggregate state, spot-state, open-orders, fills, and mids | `--address` | `hlgo agent snapshot --testnet --format json` |
| `hlgo agent pnl` | Compute unrealized, realized, and funding PnL | `--address`, `--lookback-hours`, `--aggregate-fills` | `hlgo agent pnl --lookback-hours 24 --aggregate-fills --testnet --format json` |
| `hlgo agent bracket` | Place entry + TP + SL in one grouped action | `--coin`, `--side`, `--price`, `--size`, `--tp`, `--sl`, optional `--tif`, `--cloid`, `--on-behalf-of`, `--builder`, `--builder-fee-tenths-bp`, `--expires-after` | `hlgo agent bracket --coin ETH --side buy --price 3000 --size 0.1 --tp 3100 --sl 2950 --testnet --dry-run` |

## Order Commands

| Command | Purpose | Key Flags | Example |
|---|---|---|---|
| `hlgo order place` | Place limit order | Required: `--coin`, `--side`, `--price`, `--size`; Optional: `--tif`, `--reduce`, `--cloid`, `--on-behalf-of`, `--builder`, `--builder-fee-tenths-bp`, `--expires-after` | `hlgo order place --coin ETH --side buy --price 3000 --size 0.1 --tif gtc --testnet --dry-run` |
| `hlgo order market` | Place market IOC via slippage-adjusted mid | Required: `--coin`, `--side`, `--size`; Optional: `--slippage`, `--on-behalf-of`, `--builder`, `--builder-fee-tenths-bp`, `--expires-after` | `hlgo order market --coin ETH --side buy --size 0.1 --slippage 0.5 --testnet --dry-run` |
| `hlgo order cancel` | Cancel by OID or CLOID | Required: `--coin` and exactly one of `--oid` or `--cloid`; Optional: `--on-behalf-of`, `--expires-after` | `hlgo order cancel --coin ETH --oid 12345 --testnet --format json` |
| `hlgo order cancel-all` | Cancel all open orders (optional coin filter) | Optional: `--coin`, `--on-behalf-of`, `--expires-after` | `hlgo order cancel-all --coin ETH --testnet --format json` |
| `hlgo order modify` | Modify existing order by OID | Required: `--coin`, `--oid`, `--side`, plus at least one of `--price`/`--size`; Optional: `--tif`, `--reduce`, `--on-behalf-of`, `--expires-after` | `hlgo order modify --coin ETH --oid 12345 --side buy --price 2990 --size 0.1 --testnet --dry-run` |
| `hlgo order batch` | Place multiple orders from JSON file | Required: `--file`; Optional: `--on-behalf-of`, `--builder`, `--builder-fee-tenths-bp`, `--expires-after` | `hlgo order batch --file ./orders.json --testnet --dry-run` |
| `hlgo order schedule-cancel` | Set or clear dead-man switch | Exactly one of `--timeout` or `--clear` | `hlgo order schedule-cancel --timeout 15m --testnet --format json` |

## Position Commands

| Command | Purpose | Key Flags | Example |
|---|---|---|---|
| `hlgo position leverage` | Set leverage + margin mode | Required: `--coin`, `--leverage`; Optional: `--mode`, `--on-behalf-of` | `hlgo position leverage --coin ETH --leverage 5 --mode cross --testnet --format json` |
| `hlgo position margin` | Adjust isolated margin | Required: `--coin`, `--side`, `--amount`; Optional: `--on-behalf-of` | `hlgo position margin --coin ETH --side buy --amount 25 --testnet --format json` |

## Account Commands

| Command | Purpose | Key Flags | Example |
|---|---|---|---|
| `hlgo account transfer` | Transfer USDC between spot and perp | Required: `--amount`, exactly one of `--to-perp`/`--to-spot` | `hlgo account transfer --amount 100 --to-perp --testnet --dry-run` |
| `hlgo account class-transfer` | Alias transfer using `usdClassTransfer` semantics | Required: `--amount`, exactly one of `--to-perp`/`--to-spot` | `hlgo account class-transfer --amount 100 --to-spot --testnet --dry-run` |
| `hlgo account withdraw` | Withdraw USDC to destination address | Required: `--destination`, `--amount`; Requires `--confirm` or `--yes` unless `--dry-run` | `hlgo account withdraw --destination 0xabc... --amount 50 --confirm --testnet --format json` |
| `hlgo account send-asset` | Send spot token to destination address | Required: `--destination`, `--token`, `--amount`; Requires `--confirm` or `--yes` unless `--dry-run` | `hlgo account send-asset --destination 0xabc... --token PURR:0x1 --amount 10 --confirm --testnet --format json` |
| `hlgo account approve-agent` | Approve/revoke agent wallet | Required: `--agent`; Use `--name` to approve or `--revoke --confirm` to revoke | `hlgo account approve-agent --agent 0xabc... --name trader01 --testnet --format json` |
| `hlgo account set-abstraction` | Set abstraction mode | Required: `--user`, `--abstraction` (`unifiedAccount`, `portfolioMargin`, `disabled`) | `hlgo account set-abstraction --user 0xabc... --abstraction disabled --testnet --format json` |

## Order Batch File Shape

Use decimal-safe strings for all numeric fields:

```json
[
  {
    "coin": "ETH",
    "side": "buy",
    "price": "3000",
    "size": "0.10",
    "tif": "gtc",
    "reduce_only": false,
    "cloid": "entry-eth-001"
  },
  {
    "coin": "ETH",
    "side": "sell",
    "price": "3100",
    "size": "0.10",
    "tif": "gtc",
    "reduce_only": true
  }
]
```

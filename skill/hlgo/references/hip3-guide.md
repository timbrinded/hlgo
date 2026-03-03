# HIP-3 Perp Dex Guide

How to trade on builder-deployed perpetual dexes (HIP-3 markets) via hlgo.
Load this when a task involves `--dex`, `dex:COIN` format, or non-standard perps.

## Coin Naming

- **Standard perps:** bare coin names — `ETH`, `BTC`, `SOL`.
- **HIP-3 perps:** `{dex}:{COIN}` format — `xyz:XYZ100`, `trove:AAPL`.
- Case-insensitive. The resolver normalizes internally.

### UI Symbol vs API Coin Name (Important)

UI chart/watchlist symbols are display labels and can differ from order `--coin`.
`hlgo` expects the API market identifier from:

```bash
hlgo info lookup <symbol-or-id> --dex <dex> --testnet --format json
```

Or directly from:

```bash
hlgo info meta --dex <dex> --testnet --format json | jq -r '.universe[].name'
```

Examples:

```bash
# Negative: UI-style ticker (fails)
hlgo order place --coin tngs:CHARIZARDUSD --side buy --price 517.5 --size 0.01 --testnet --dry-run --format json
# -> VALIDATION_ERROR unknown coin

# Positive: API market name from lookup/meta (works)
hlgo info lookup charizardusd --dex tngs --testnet --format json
hlgo order place --coin tngs:CHARIZARD-TGUSD --side buy --price 517.5 --size 0.01 --testnet --dry-run --format json
```

## Discovery

List all available HIP-3 dexes and their coins:

```bash
hlgo info perp-dexs --testnet --format json
```

Fetch metadata for a specific dex:

```bash
hlgo info meta --dex xyz --testnet --format json
```

## The `--dex` Flag

Sets a default dex context so you can omit the `dex:` prefix in coin names:

```bash
# These two are equivalent:
hlgo order place --coin xyz:XYZ100 --side buy --price 10 --size 1 --testnet
hlgo order place --coin XYZ100    --side buy --price 10 --size 1 --dex xyz --testnet
```

`--dex` overrides the `default_dex` config value. Set in config for persistent dex context:

```yaml
# ~/.hlgo/config.yaml
default_dex: xyz
```

Also accepted via `HL_DEX` env var.

## Resolution Behaviour

The resolver automatically maps friendly names to integer asset IDs:

- Standard perps: index in `meta.universe[]` (e.g., ETH = 1).
- Spot markets: `10000 + market_index`.
- HIP-3 perps: `110000 + (dex_position - 1) * 10000 + index`.

If a coin name is ambiguous (exists in multiple dexes without `--dex` context), the error includes candidate hints:

```json
{
  "error": "unknown coin: XYZ100",
  "code": "VALIDATION_ERROR",
  "details": {
    "coin": "XYZ100",
    "hint": "use a valid coin name or specify --dex"
  }
}
```

## Spot Pair Aliases

Unit tokens (e.g., `UETH` with fullName `Unit ETH`) can be referenced by their alias `ETH` in spot contexts. If the alias is ambiguous across multiple markets, the error lists candidates with market indices for disambiguation:

```bash
# Unambiguous — resolves directly
hlgo order place --coin PURR --side buy --price 0.001 --size 100 --testnet

# Ambiguous — use explicit market name
hlgo order place --coin UETH/USDC --side buy --price 3000 --size 0.01 --testnet
```

## Commands That Accept `--dex`

`info lookup`, `info mids`, `info meta`, `info meta-and-ctxs`, `info state`, `info open-orders`, `order place`, `order market`, `order cancel`, `order cancel-all`, `order modify`, `order batch`, `agent snapshot`, `agent pnl`, `agent bracket`.

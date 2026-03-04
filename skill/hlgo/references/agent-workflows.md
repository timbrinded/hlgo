# Agent Workflows

Composed multi-step recipes for common agent trading patterns.
Load this when building bracket orders, snapshot-to-trade flows, or PnL monitoring loops.

## Bracket Order Workflow

Full 9-step flow: read state, compute prices, set leverage, dry-run, place bracket, verify, monitor, clean up.

```bash
set -euo pipefail

# 1. Baseline snapshot
snapshot=$(hlgo agent snapshot --testnet --format json)
account_value=$(echo "$snapshot" | jq -r '.account_value')

# 2. Read mid prices
mids=$(hlgo info mids --testnet --format json)
eth_mid=$(echo "$mids" | jq -r '.ETH')

# 3. Set leverage before placing orders
hlgo position leverage --coin ETH --leverage 5 --testnet --format json

# 4. Compute bracket prices from mid
#    Entry: resting below market (buy) to avoid immediate fill
#    TP: 5% above entry, SL: 5% below entry
#    Use 1 decimal at ETH ~2k levels to stay within max-5-significant-figure wire limits.
entry=$(awk -v m="$eth_mid" 'BEGIN{printf "%.1f", m*0.80}')
tp=$(awk -v e="$entry" 'BEGIN{printf "%.1f", e*1.05}')
sl=$(awk -v e="$entry" 'BEGIN{printf "%.1f", e*0.95}')

# 5. Generate unique CLOID for tracking
cloid="0x$(openssl rand -hex 16)"

# 6. Dry-run first to catch precision/signing issues before live submission
hlgo agent bracket \
  --coin ETH --side buy \
  --price "$entry" --size 0.01 \
  --tp "$tp" --sl "$sl" \
  --cloid "$cloid" \
  --dry-run --testnet --format json

# 7. Place bracket (entry + TP + SL in one grouped action)
hlgo agent bracket \
  --coin ETH --side buy \
  --price "$entry" --size 0.01 \
  --tp "$tp" --sl "$sl" \
  --cloid "$cloid" \
  --testnet --format json

# 8. Verify bracket appears in open orders
orders=$(hlgo info open-orders --testnet --format json)
echo "$orders" | jq --arg c "$cloid" '.[] | select(.cloid == $c)'

# 9. Monitor with snapshot + PnL
hlgo agent snapshot --testnet --format json
hlgo agent pnl --testnet --format json

# Cleanup: cancel all when done
hlgo order cancel-all --testnet --format json
```

### Price Calculation Patterns

- **Resting entry (buy):** `mid * 0.8` keeps the order below market to avoid immediate fill during testing.
- **Take-profit:** `entry * 1.05` (5% above entry for buys).
- **Stop-loss:** `entry * 0.95` (5% below entry for buys).
- For sell brackets, invert: entry above market, TP below entry, SL above entry.
- `%.2f` is not always safe on ETH-scale prices; default to one decimal or validate via `--dry-run`.
- Round computed prices to wire-valid precision. See `precision-rules.md` for sig-fig and decimal constraints.

## PnL Monitoring Workflow

```bash
# Snapshot + PnL with 24h lookback
hlgo agent pnl --lookback-hours 24 --aggregate-fills --testnet --format json
```

PnL formula: `total_pnl = total_unrealized_pnl + realized_pnl + total_funding_pnl`

Output fields:
- `positions[]` — per-coin unrealized PnL computed as `(mid - entry) * size`.
- `realized_pnl` — sum of `closedPnl` from fills in the lookback window.
- `total_funding_pnl` — sum of funding payments in the lookback window.
- `partial: true` — set when fills or funding data was unavailable. Check `errors[]` for details.
- `funding_unavailable` / `realized_unavailable` — flags indicating which component failed.

## Snapshot Composition

`agent snapshot` aggregates 5 substeps in one call:

| Substep | API call | Data |
|---|---|---|
| `state` | `clearinghouseState` | Account value, perp positions |
| `spot-state` | `spotClearinghouseState` | Spot balances |
| `open-orders` | `frontendOpenOrders` | All open orders |
| `fills` | `userFills` | Last 10 fills |
| `mids` | `allMids` | All mid prices |

Partial success: if some substeps fail, returns `partial: true` with an `errors[]` array listing each failed step, its error code, and message. Branch on the `partial` field to decide whether to proceed or abort.

## Delegated Trading (`--on-behalf-of`)

When using an approved agent/API wallet, set the target account context once (config `account_address`) and use explicit overrides only for read-dependent operations:

```bash
ACCOUNT_ADDRESS="0x1234567890abcdef1234567890abcdef12345678"

# 0. Persist default account context for reads/lookups
hlgo config init --account-address "$ACCOUNT_ADDRESS" --force

# 1. Check the delegated account's positions
hlgo info state --address "$ACCOUNT_ADDRESS" --testnet --format json

# 2. Place/adjust orders normally (agent authorization determines execution identity)
hlgo order place --coin ETH --side buy --price 3000 --size 0.1 \
  --dry-run --testnet --format json

# 3. Place live
hlgo order place --coin ETH --side buy --price 3000 --size 0.1 \
  --testnet --format json

# 4. Verify — open-orders uses account context reads
hlgo info open-orders --address "$ACCOUNT_ADDRESS" --testnet --format json

# 5. Cancel all using an explicit account-context override
hlgo order cancel-all --on-behalf-of "$ACCOUNT_ADDRESS" --testnet --format json
```

Key points:
- Read commands (`info`) use `--address` (or configured `account_address`) to query account state.
- `cancel-all` and `modify` use `--on-behalf-of` only for account-context open-order lookups.
- `--on-behalf-of` is rejected by `account` commands and by write paths where it has no direct effect.

## CLOID Generation

Client Order IDs (CLOIDs) are 16-byte random hex strings with `0x` prefix. Each CLOID is single-use per order.

```bash
cloid="0x$(openssl rand -hex 16)"
# Example: 0xa1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6
```

Use CLOIDs to track specific orders through the lifecycle: place, verify in open-orders, cancel by CLOID.

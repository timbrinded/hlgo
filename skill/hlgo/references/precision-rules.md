# Precision Rules

What the CLI validates for prices and sizes, what it auto-corrects, and what the API will reject.
Load this when placing or modifying orders, computing bracket prices, or debugging rejection errors.

## Price Rules — What the CLI Enforces

Two constraints apply simultaneously to every price:

### 1. Max 5 Significant Figures (non-integer prices)

Non-integer prices may not exceed 5 significant figures. Integer prices are exempt.

| Price | Sig Figs | Valid? | Why |
|---|---|---|---|
| `1234.5` | 5 | YES | Exactly 5 sig figs |
| `1234.56` | 6 | NO | `nearest_valid: 1234.6` |
| `123456` | 6 | YES | Integer exemption |
| `0.001234` | 4 | YES | Leading zeros don't count |
| `10.00` | 2 | YES | Trailing zeros normalized away |

### 2. Max Decimal Places = MAX_DECIMALS - szDecimals

- Perp markets: MAX_DECIMALS = 6
- Spot markets: MAX_DECIMALS = 8
- Allowed decimal places = MAX_DECIMALS - szDecimals for the asset

| Asset | szDecimals | Max Price Decimals | Example |
|---|---|---|---|
| ETH perp | 4 | 2 | `2567.9` OK, `2567.891` REJECTED |
| BTC perp | 5 | 1 | `95123.5` OK, `95123.56` REJECTED |
| SOL perp | 2 | 4 | `145.1234` OK, `145.12345` REJECTED |
| PURR spot | 0 | 8 | `0.00012345` OK (5 sig figs) |

### Validation Error Shape

When price validation fails, the error includes `nearest_valid`:

```json
{
  "error": "price has 6 significant figures, maximum is 5",
  "code": "VALIDATION_ERROR",
  "details": {
    "value": "1234.56",
    "sig_figs": 6,
    "max_sig_figs": 5,
    "nearest_valid": "1234.6"
  }
}
```

## Price Rules — Market Orders Auto-Snap

`order market` computes `mid * (1 +/- slippage%)` then calls `NearestValidPrice()` internally:

- Agents don't need to worry about price precision for market orders.
- Example: mid = `12345.6789` with 0.5% slippage (buy) -> `12407.418` -> snapped to `12407` (5 sig figs).

## Size Rules — What the CLI Does

Sizes are auto-rounded to `szDecimals` decimal places using **banker's rounding** (round half to even):

| Asset | szDecimals | Input | Output | Note |
|---|---|---|---|---|
| BTC perp | 5 | `0.00023` | `0.00023` | Already valid |
| BTC perp | 5 | `0.000231` | `0.00023` | Truncated (6th decimal) |
| SOL perp | 2 | `1.005` | `1.01` | Banker's rounding — rounds UP |
| SOL perp | 2 | `1.015` | `1.02` | Banker's rounding — rounds UP |
| PURR spot | 0 | `100` | `100` | Already valid |
| PURR spot | 0 | `100.7` | `101` | Auto-rounded to integer |

**Gotcha:** Banker's rounding can round UP, which might exceed your available balance. For sizes near balance limits, truncate (floor) rather than relying on auto-rounding.

## What the CLI Does NOT Validate

These constraints are enforced by the Hyperliquid API. The CLI passes them through — if the API rejects, you get an `API_ERROR`:

- **Minimum order value:** $10 USDC for perps, 10 quote token for spot. Reduce-only orders may bypass.
- **Lot size multiples:** size must be a multiple of `10^(-szDecimals)`.
- **Maximum order sizes.**
- **Margin/balance sufficiency.**

## Defensive Patterns for Agents

### Always dry-run first
```bash
# Preview exact wire-format values before signing
result=$(hlgo order place --coin ETH --side buy --price 3000 --size 0.1 --testnet --dry-run --format json)
echo "$result" | jq '.resolved'
# Shows: {"coin":"ETH","asset_id":1,"side":"buy","price":"3000","size":"0.1",...}
```

### Truncate near balance limits
```bash
# Floor truncation for size (avoids banker's rounding UP)
size=$(echo "scale=5; $raw_size / 1" | bc)  # truncate to 5 decimals for BTC
```

### Check account value before placing
```bash
snapshot=$(hlgo agent snapshot --testnet --format json)
account_value=$(echo "$snapshot" | jq -r '.account_value')
# Compare notional (price * size) against account_value before placing
```

### Debug opaque API rejections
If the API rejects with an unclear error after the CLI accepted the order:
1. Re-check price sig figs and decimal places against the asset's szDecimals.
2. Verify size is a valid lot multiple.
3. Confirm order notional exceeds minimum ($10 USDC for perps).
4. Use `--dry-run` to inspect the exact wire payload being sent.

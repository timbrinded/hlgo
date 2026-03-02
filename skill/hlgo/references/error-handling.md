# Error Handling

How to parse, branch on, and recover from hlgo errors.
Load this when implementing retry logic, handling partial success, or debugging rejection errors.

## Error Structure

All errors are structured JSON on stderr:

```json
{
  "error": "human-readable message",
  "code": "MACHINE_CODE",
  "details": {
    "field": "price",
    "value": "1234.56",
    "nearest_valid": "1234.6"
  }
}
```

- `error` — human-readable, not stable for parsing. Branch on `code` instead.
- `code` — machine-readable classification. Always present.
- `details` — optional context. Omitted entirely when empty (never `null`, never `{}`).

## Error Code Table

| Code | Exit | Retryable | Agent Action |
|---|---|---|---|
| `VALIDATION_ERROR` | 1 | No | Fix input. Check `details` for `field`, `value`, `nearest_valid`. |
| `CONFIG_ERROR` | 2 | No | Check config file and env vars. Missing key? Wrong env var name? |
| `NETWORK_ERROR` | 3 | Yes | Retry with exponential backoff. Check connectivity. |
| `API_ERROR` | 4 | Maybe | Inspect `details`. May be transient (retry) or permanent (bad request). |
| `SIGNING_ERROR` | 5 | No | Check key availability. Is the env var set? Is the key valid hex? |
| `RATE_LIMIT` | 6 | Yes | Use `retry_after_ms` from `details` if present, else back off 1s. |

## Detail Field Patterns

Different error codes include different detail fields:

**VALIDATION_ERROR:**
- `field` — which input failed validation.
- `value` — the rejected value.
- `nearest_valid` — the closest wire-valid value (for price precision errors).
- `hint` — disambiguation guidance (for unknown coin errors).

**RATE_LIMIT:**
- `retry_after_ms` — how long to wait before retrying (milliseconds).

**NETWORK_ERROR:**
- `stage` — which metadata/API call timed out (e.g., `perp_meta`, `spot_meta`).
- `timeout_ms` — the timeout that was exceeded.
- `cause` — underlying error string.

**API_ERROR:**
- `cause` — upstream error message from the Hyperliquid API.

## Retry Decision Tree

```bash
result=$(hlgo order place --coin ETH --side buy --price 3000 --size 0.1 --testnet --format json 2>/tmp/hlgo_err)
exit_code=$?

if [ $exit_code -eq 0 ]; then
  echo "Success"
elif [ $exit_code -eq 3 ] || [ $exit_code -eq 6 ]; then
  # NETWORK_ERROR or RATE_LIMIT — retry
  code=$(jq -r '.code' /tmp/hlgo_err)
  if [ "$code" = "RATE_LIMIT" ]; then
    wait_ms=$(jq -r '.details.retry_after_ms // 1000' /tmp/hlgo_err)
    sleep "$(echo "$wait_ms / 1000" | bc -l)"
  else
    sleep 2
  fi
  # Retry the command...
elif [ $exit_code -eq 1 ]; then
  # VALIDATION_ERROR — fix input, don't retry
  echo "Bad input:" >&2
  jq '.details' /tmp/hlgo_err >&2
elif [ $exit_code -eq 4 ]; then
  # API_ERROR — inspect before deciding
  echo "API error:" >&2
  cat /tmp/hlgo_err >&2
else
  # CONFIG_ERROR (2) or SIGNING_ERROR (5) — abort
  echo "Fatal error (exit $exit_code):" >&2
  cat /tmp/hlgo_err >&2
  exit 1
fi
```

## Partial Success in Composed Commands

`agent snapshot` and `agent pnl` aggregate multiple API calls. When some substeps fail:

```json
{
  "account_value": "10523.45",
  "perp_positions": [...],
  "open_orders": [],
  "partial": true,
  "errors": [
    {
      "step": "fills",
      "code": "NETWORK_ERROR",
      "error": "metadata request timed out"
    }
  ]
}
```

- **Branch on `partial`**, not just exit code. Exit code is 0 even for partial success.
- The `errors[]` array lists each failed substep with its own `code`.
- If all substeps fail, the command returns a non-zero exit code with a single error.
- Use `partial` to decide: proceed with available data, or abort and retry.

## Common Recovery Patterns

**Price rejection → use nearest_valid:**
```bash
# Parse the nearest valid price from the error and retry
nearest=$(jq -r '.details.nearest_valid' /tmp/hlgo_err)
hlgo order place --coin ETH --side buy --price "$nearest" --size 0.1 --testnet
```

**Unknown coin → check spelling and dex context:**
```bash
# Error includes hint field
jq -r '.details.hint' /tmp/hlgo_err
# Typically: "use a valid coin name (e.g. BTC, ETH), spot pair (e.g. ETH/USDC), or a numeric asset ID"
```

**Rate limit → respect retry_after_ms:**
```bash
wait_ms=$(jq -r '.details.retry_after_ms // 1000' /tmp/hlgo_err)
sleep "$(echo "$wait_ms / 1000" | bc -l)"
```

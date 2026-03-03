# hlgo Contracts and Safety

## Output Contract

- Print success payloads to `stdout` (JSON-first).
- Print failures to `stderr` as structured JSON:

```json
{"error":"...","code":"VALIDATION_ERROR","details":{"field":"value"}}
```

- Treat exit status as part of the contract.

## Error Codes and Exit Codes

| Code | Exit |
|---|---|
| `VALIDATION_ERROR` | `1` |
| `CONFIG_ERROR` | `2` |
| `NETWORK_ERROR` | `3` |
| `API_ERROR` | `4` |
| `SIGNING_ERROR` | `5` |
| `RATE_LIMIT` | `6` |

## Signing-Path Matrix

| Command Group | Wallet | Path |
|---|---|---|
| `order`, `position`, `agent bracket` | Agent wallet (`private_key_env`) | L1 phantom-agent path |
| `account` commands | Master wallet (`private_key_env`) | User-signed path |
| `info`, `agent snapshot`, `agent pnl`, `config`, `version` | No signing | Read/config path |

## Precision and Serialization Rules

- Use decimal-safe strings for all prices, sizes, and amounts.
- Do not use floating-point assumptions in agent logic.
- Keep financial values as strings in JSON payloads.
- Validate tick/lot constraints before submitting live orders.

## Time Parsing Rules

Flags that accept time support both Unix ms and ISO-8601 forms:

- `2026-03-02T14:00:00Z`
- `2026-03-02T14:00:00`
- `2026-03-02`
- `1700000000000`

## Confirmation Gates

Require explicit confirmation unless `--dry-run` is set:

- `account withdraw` (`--confirm` or `--yes`)
- `account send-asset` (`--confirm` or `--yes`)
- `account approve-agent --revoke` (`--confirm` or `--yes`)

## Dry-Run Expectations

- `info ... --dry-run`: print request payload that would be sent.
- `order/position/account ... --dry-run`: print action payload instead of signing/sending.
- `agent snapshot/pnl --dry-run`: print composed request set.

## HIP-3 and Asset Resolution

- Use `--dex <name>` for HIP-3 perp contexts.
- Use coin form `<dex>:<COIN>` for HIP-3 markets.
- Discover available HIP-3 dexes with `hlgo info perp-dexs --format json`.

## Safe Mutation Workflow

1. Read latest context (`info` or `agent snapshot`).
2. Generate candidate action.
3. Run mutation command with `--dry-run`.
4. Execute live mutation.
5. Verify with read commands (`info open-orders`, `info fills`, `agent pnl`).

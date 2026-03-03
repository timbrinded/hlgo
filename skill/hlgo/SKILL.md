---
name: hlgo
description: >-
  Operate and troubleshoot the hlgo Hyperliquid CLI for agent-native trading
  workflows. Trigger this skill for ANY task involving: hlgo command selection,
  flag lookup, JSON output or error contract parsing, dry-run planning,
  signing-path safety, debugging hlgo exit codes, composing multi-step trading
  workflows, integrating hlgo into pipelines, HIP-3 perp dex resolution,
  price/size precision questions, or generating scripts that call hlgo
  info/order/position/agent/account/config/version commands.
---

# hlgo CLI Skill

`hlgo` is a stateless, JSON-first Go CLI for Hyperliquid. All financial values are decimal strings — never floats. Every command exits, returns JSON on stdout, and structured errors on stderr.

## Start Here

1. **Validate environment:** `hlgo config test --testnet` — confirms config file is readable, key env vars are set, and API is reachable. Run this first because a bad config wastes every subsequent step.
2. **Pull baseline state:** `hlgo agent snapshot --testnet --format json` — aggregates account value, positions, open orders, recent fills, and mid prices in one call. Gives you the full picture before acting.
3. **Preview every mutation:** add `--dry-run` to any mutating command. This prints the exact wire payload without signing or sending. Catches precision errors, wrong signing path, and bad inputs before they cost money.
4. **Execute mutation.**
5. **Verify state:** `hlgo info open-orders`, `hlgo info fills`, or `hlgo agent pnl` — always confirm the mutation landed. Never assume success from exit code alone.

## Mutation Workflow Template

Every mutation follows read-preview-execute-verify:

```bash
# 1. Read current state
snapshot=$(hlgo agent snapshot --testnet --format json)

# 2. Dry-run the mutation
hlgo order place --coin ETH --side buy --price 3000 --size 0.1 \
  --testnet --dry-run --format json

# 3. Execute live
hlgo order place --coin ETH --side buy --price 3000 --size 0.1 \
  --testnet --format json

# 4. Verify
hlgo info open-orders --testnet --format json
```

## Command Selection

| Group | Purpose | Wallet | Signing Path |
|---|---|---|---|
| `info` | Market and account reads | None | No signing |
| `agent snapshot/pnl` | Composed read workflows | None | No signing |
| `agent bracket` | Entry + TP + SL grouped order | Agent (`agent_key_env`) | L1 phantom-agent |
| `order` | Order lifecycle (place, cancel, modify, batch) | Agent (`agent_key_env`) | L1 phantom-agent |
| `position` | Leverage and margin changes | Agent (`agent_key_env`) | L1 phantom-agent |
| `account` | Transfers, withdrawals, agent approval | Master (`master_key_env`) | User-signed |
| `config` / `version` | Setup and environment checks | None | No signing |

Pick the wrong wallet and signing fails silently or with `SIGNING_ERROR`. When in doubt, check the table.

## Operating Rules

- **Keep machine workflows on `--format json`.** Agents parse JSON; table/csv formats are for human eyes only.
- **Use decimal-safe strings for all prices, sizes, and amounts.** hlgo uses `shopspring/decimal` internally — never pass floats.
- **For HIP-3, use API coin names from `info meta --dex <dex>` `universe[].name` (not UI tickers).**
  - Negative example: `--coin tngs:CHARIZARDUSD` -> `VALIDATION_ERROR unknown coin`
  - Positive example: `--coin tngs:CHARIZARD` -> resolves and places correctly
- **Treat non-zero exit codes as failures; branch on JSON `code`.** Exit codes map to error categories (1=validation, 3=network, 6=rate-limit). The `code` field in stderr JSON is the stable contract.
- **Keep `--testnet` enabled during development and simulation.** Testnet is free. Mainnet costs real money. No flag = mainnet.
- **Never output private key material.** hlgo redacts keys in `config show`. Your scripts must too.

## Progressive Disclosure Map

Load only the reference needed for the current task:

| Reference | Load When... |
|---|---|
| [command-reference.md](references/command-reference.md) | Looking up exact syntax, flags, or examples for a specific command |
| [contracts-and-safety.md](references/contracts-and-safety.md) | Validating inputs, checking signing paths, or reviewing safety constraints |
| [agent-workflows.md](references/agent-workflows.md) | Building bracket orders, snapshot-to-trade flows, or PnL monitoring loops |
| [hip3-guide.md](references/hip3-guide.md) | Any task involving `--dex`, `dex:COIN` format, or non-standard perps |
| [error-handling.md](references/error-handling.md) | Parsing errors, implementing retry logic, or handling partial success |
| [precision-rules.md](references/precision-rules.md) | Placing or modifying orders, computing bracket prices, or debugging rejections |

## Completion Checklist

- Document command + required flags + example.
- Confirm whether the command uses agent-wallet or master-wallet signing (check table above).
- Include a `--dry-run` step for mutating actions unless explicitly told to execute live.
- Include one or more post-mutation verification commands.

# hlgo

Go CLI for Hyperliquid, built for agent-first automation and JSON-first output.

Core principles are defined in [SOUL.md](./SOUL.md).

## Build

```bash
make build
```

## Config

Initialize config:

```bash
hlgo config init
```

Set your agent key env var (default is `HL_AGENT_KEY`):

```bash
export HL_AGENT_KEY=0xYOUR_PRIVATE_KEY
```

Validate config + connectivity:

```bash
hlgo config test --testnet
```

## Market Orders on Testnet

Important: `order market --slippage` is a percentage value.

- `--slippage 1` means 1%.
- `--slippage 0.5` means 0.5%.

Run dry-run first:

```bash
# Perp buys
hlgo order market --testnet --coin BTC --side buy --size 0.001 --slippage 1 --dry-run
hlgo order market --testnet --coin ETH --side buy --size 0.01  --slippage 1 --dry-run

# Spot buys
hlgo order market --testnet --coin BTC/USDC --side buy --size 0.001 --slippage 1 --dry-run
hlgo order market --testnet --coin ETH/USDC --side buy --size 0.01  --slippage 1 --dry-run
```

Submit live testnet orders (remove `--dry-run`):

```bash
# Perp buys
hlgo order market --testnet --coin BTC --side buy --size 0.001 --slippage 1
hlgo order market --testnet --coin ETH --side buy --size 0.01  --slippage 1

# Spot buys
hlgo order market --testnet --coin BTC/USDC --side buy --size 0.001 --slippage 1
hlgo order market --testnet --coin ETH/USDC --side buy --size 0.01  --slippage 1
```

If testnet uses unit-prefixed spot symbols, use `UBTC/USDC` and `UETH/USDC`.

## Development

Run full local checks before pushing:

```bash
make check
```

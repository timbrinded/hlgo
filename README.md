# hlgo

A Go CLI for [Hyperliquid](https://hyperliquid.xyz) built for autonomous and semi-autonomous agents.
JSON-first output, structured errors with stable codes, decimal-safe financials, and no background daemon — designed so agents can query state, place orders, and parse results deterministically.

Project principles: [`SOUL.md`](./SOUL.md)

## Install

```bash
curl -sSfL https://raw.githubusercontent.com/timbrinded/hlgo/main/install.sh | sh
```

This installs the latest release to `~/.local/bin`. Override with `HLGO_INSTALL_DIR`:

```bash
curl -sSfL https://raw.githubusercontent.com/timbrinded/hlgo/main/install.sh | HLGO_INSTALL_DIR=/usr/local/bin sh
```

From source (requires Go 1.26+):

```bash
go install github.com/timbrinded/hlgo@latest
```

Or download a prebuilt binary from [Releases](https://github.com/timbrinded/hlgo/releases).

## Quick Start

```bash
# 1. Create config file (~/.hlgo/config.yaml)
hlgo config init

# 2. Export your agent private key
export HL_AGENT_KEY=0xYOUR_PRIVATE_KEY

# 3. Verify connectivity
hlgo config test --testnet

# 4. Fetch mid prices
hlgo info mids --testnet
```

All commands emit JSON to stdout and structured errors to stderr. Use `--dry-run` to preview any mutating command before execution.

## For Agents

The full agent manual lives in the skill system at [`skill/hlgo/SKILL.md`](./skill/hlgo/SKILL.md). It includes:

- Complete command reference with flags and examples
- Trading workflows and recipes
- HIP-3 / multi-dex guide
- Error codes, exit codes, and retry patterns
- Safety contracts and operational guardrails

Point your agent at the skill rather than this README.

## Development

```bash
make build    # fmt → vet → compile
make test     # race detector enabled
make lint     # golangci-lint
make check    # fmt → tidy → vet → lint → test (pre-push)
```

Versioned build:

```bash
go build -ldflags "-X main.version=x.y.z -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o hlgo .
```

Release process: [`RELEASE.md`](./RELEASE.md)

## License

MIT — see [`LICENSE`](./LICENSE).

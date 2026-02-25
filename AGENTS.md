# AGENTS.md — Repository Guidelines

- Repo: https://github.com/timbrinded/hlgo
- **Read `SOUL.md` first. It is the supreme authority for this project.** Every code change, design decision, and PR must align with its principles. If a rule in this file conflicts with SOUL.md, SOUL.md wins.
- Technical spec: `plans/hlgo-technical-spec-v0.1.0.md`

## Project Structure

- CLI entrypoint: `main.go` → `cmd/root.go`
- Commands: `cmd/<domain>.go` (one file per command group: info, order, position, account, config, version)
- Internal packages: `pkg/<domain>/` — each package is a self-contained unit with its own types and tests
- Package layout: `client/`, `info/`, `exchange/`, `signer/`, `resolver/`, `wire/`, `config/`, `output/`
- Tests: colocated `*_test.go` files within each package
- Plans and specs: `plans/`

## Build, Test, and Development

- Go 1.22+
- Build: `go build -o hlgo .`
- Test: `go test ./...`
- Vet: `go vet ./...`
- Version injection: `go build -ldflags "-X main.version=x.y.z" -o hlgo .`
- Dependencies: `github.com/spf13/cobra`, `github.com/spf13/viper`, `github.com/shopspring/decimal`

## Coding Conventions

- When in doubt about any decision, re-read `SOUL.md`. It takes precedence over convenience, convention, or expediency.
- Formatting: `gofmt` / `goimports`. No exceptions.
- Error handling: return `error`, never panic in library code. Commands return structured JSON errors to stderr.
- Naming: standard Go conventions — `MixedCaps`, not `snake_case`. Package names are single lowercase words.
- No `init()` functions. Explicit initialization only.
- No global mutable state. Pass dependencies explicitly.
- Keep files under ~500 LOC; split when it improves clarity.

## Domain Gotchas (Hyperliquid)

- **Never `float64` for financial values.** All prices, sizes, and amounts use `shopspring/decimal`. This is non-negotiable.
- **Decimal string serialization:** JSON output uses string representations — `"95123.5"`, not `95123.5`. No numeric JSON fields for financial values.
- **Nonces are millisecond timestamps**, not sequential counters. Use `time.Now().UnixMilli()`. Each nonce is single-use per signer address.
- **Two distinct signing paths:**
  - L1 (phantom agent): chain ID `1337`, domain `"Exchange"`, msgpack → keccak256 → EIP-712. Agent wallet signs these.
  - User-signed: chain ID `42161` (Arbitrum), domain `"HyperliquidSignTransaction"`, EIP-712 direct. Master wallet only.
  - Never mix these up. Commands must auto-select the correct signer based on action type.
- **Asset ID resolution:** Perp IDs are index-based, spot IDs are `10000 + index`, HIP-3 IDs are `100000 + (dex_index × 10000) + index`. HIP-3 coins use `{dex}:{coin}` format (e.g. `xyz:XYZ100`).
- **Tick and lot size validation happens before signing.** Perps: max 6 decimals. Spot: max 8 decimals. Price: max 5 significant figures. Validate early, fail loud.
- **`sonirico/go-hyperliquid` is a reference, not a dependency.** Read it for signing patterns. Don't import it as a runtime dependency. See `SOUL.md` §Built from Scratch.

## Git & Workflow

- Before committing, verify your changes align with `SOUL.md` principles. If they don't, fix them first.
- Commit messages: concise, action-oriented (e.g. `cmd: scaffold info command group`)
- Scope commits to the domain they touch (e.g. `signer:`, `wire:`, `cmd:`)
- Do not force-push to `main`.
- Run `go vet ./...` and `go test ./...` before committing.

## Security

- No private key material in output. Ever. Not in logs, errors, dry-run, or debug output.
- No secrets in test fixtures. Use deterministic test keys that are clearly marked as test-only.
- Config file (`~/.hlgo/config.yaml`) contains wallet keys. Never commit example configs with real keys.

# Hyperliquid Research Sources

Collected references to support implementation and issue execution.

## Official Documentation
- Hyperliquid API docs: https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api
- Info endpoint docs: https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/info-endpoint
- Exchange endpoint docs: https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/exchange-endpoint
- WebSocket docs: https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/websocket
- Signing docs: https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/signing

## SDK/Reference Implementations
- Official Python SDK: https://github.com/hyperliquid-dex/hyperliquid-python-sdk
- Sonirico Go library: https://github.com/sonirico/go-hyperliquid
- Hyperliquid Rust SDK examples: https://github.com/hyperliquid-dex/hyperliquid-rust-sdk

## Notes
- Use Python SDK signing vectors as canonical test fixtures.
- Prefer string-based decimal wire formatting in our own code, even when using existing Go signing internals.

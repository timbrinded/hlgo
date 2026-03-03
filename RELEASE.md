# Release Process

This runbook describes how to cut and verify a `hlgo` release.

## 1. Prerequisites

- Write access to `timbrinded/hlgo`
- Permission to push tags
- `gh` authenticated to GitHub
- Go toolchain from `go.mod`

Optional local tool install:

```bash
go install github.com/goreleaser/goreleaser/v2@latest
```

If you do not install it globally, use `go run github.com/goreleaser/goreleaser/v2@latest ...` in commands below.

## 2. Prepare the Branch

```bash
git checkout main
git pull --ff-only origin main
```

Choose the next SemVer tag (for example `v0.1.0`).

## 3. Pre-Release Validation

Run the full project checks:

```bash
make check
```

Validate release config:

```bash
goreleaser check
```

Run a local snapshot release (builds archives + checksums in `dist/` without publishing):

```bash
goreleaser release --snapshot --clean
```

Sanity-check version metadata in a built binary:

```bash
make build
./bin/hlgo version
```

Expected shape:

```json
{"version":"...","commit":"...","date":"..."}
```

## 4. Create the Release

Create and push the tag:

```bash
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

Pushing `v*` triggers `.github/workflows/release.yml`.

## 5. Verify GitHub Release Artifacts

After the workflow completes, verify the GitHub Release contains:

- `hlgo_<version>_linux_amd64.tar.gz`
- `hlgo_<version>_linux_arm64.tar.gz`
- `hlgo_<version>_darwin_amd64.tar.gz`
- `hlgo_<version>_darwin_arm64.tar.gz`
- `checksums.txt`

Each archive should include:

- `hlgo` binary
- `README.md`
- `SOUL.md`
- `LICENSE`

## 6. Post-Release Checklist

- Confirm release notes/changelog look correct.
- Spot-check `checksums.txt`.
- Announce release and update downstream automation if needed.

## 7. Rollback (If Needed)

If a bad release tag was pushed:

1. Delete the GitHub Release for that tag.
2. Delete the tag remotely and locally:

```bash
git push --delete origin v0.1.0
git tag -d v0.1.0
```

3. Apply fixes and cut a new patch tag (for example `v0.1.1`).

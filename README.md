# loopctl

A CLI tool for managing the LoopControl platform.

## Installation

### Homebrew

```bash
brew install dotdevlabs/tap/loopctl
```

### From source

```bash
go install github.com/dotdevlabs/loopctl/cmd/loopctl@latest
```

## Usage

```bash
loopctl [command]
```

### Commands

| Command | Description |
|---------|-------------|
| `version` | Print version information |
| `help` | Show help for any command |

### JSON output

Agent-facing commands support `--json` for machine-stable JSON output:

```bash
loopctl version --json
```

## Development

```bash
# Run CI checks (format, vet, lint, test, build)
./bin/ci

# Build
go build ./cmd/loopctl

# Test
go test ./... -race
```

## Release

Releases are automated via [goreleaser](https://goreleaser.com/) on git tags:

```bash
git tag v0.1.0
git push origin v0.1.0
```

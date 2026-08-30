# Stompy CLI

A portable command-line interface for [Stompy](https://stompy.ai) — manage projects, contexts, and tickets from your terminal.

Stompy is a persistent memory and knowledge management platform for AI-assisted development. The CLI gives you full access to Stompy's API without needing a browser or MCP client.

## Why a CLI?

- **Zero dependencies** — single static binary, no Python/Node/runtime needed
- **Cross-platform** — macOS, Linux, Windows (arm64 + amd64)
- **Self-updating** — built-in `stompy update` keeps you current
- **Scriptable** — pipe-friendly with JSON/YAML output modes
- **Fast** — direct REST API calls, no browser overhead
- **Works everywhere** — SSH sessions, CI/CD pipelines, containers

## Install

### Quick Install (macOS / Linux)

```bash
# macOS (Apple Silicon)
curl -sL https://github.com/stompy-ai/stompy-cli/releases/latest/download/stompy_$(curl -sI https://github.com/stompy-ai/stompy-cli/releases/latest | grep -i location | sed 's/.*tag\/v//' | tr -d '\r')_darwin_arm64.tar.gz | tar xz
sudo mv stompy /usr/local/bin/

# macOS (Intel)
curl -sL https://github.com/stompy-ai/stompy-cli/releases/latest/download/stompy_$(curl -sI https://github.com/stompy-ai/stompy-cli/releases/latest | grep -i location | sed 's/.*tag\/v//' | tr -d '\r')_darwin_amd64.tar.gz | tar xz
sudo mv stompy /usr/local/bin/

# Linux (amd64)
curl -sL https://github.com/stompy-ai/stompy-cli/releases/latest/download/stompy_$(curl -sI https://github.com/stompy-ai/stompy-cli/releases/latest | grep -i location | sed 's/.*tag\/v//' | tr -d '\r')_linux_amd64.tar.gz | tar xz
sudo mv stompy /usr/local/bin/
```

Or download manually from the [Releases](https://github.com/stompy-ai/stompy-cli/releases) page.

### Build from Source

Requires Go 1.21+:

```bash
git clone https://github.com/stompy-ai/stompy-cli.git
cd stompy-cli
make build      # → bin/stompy
make install    # → $GOPATH/bin/stompy
```

### Update

Stompy checks for updates in the background and notifies you when a new version is available. To upgrade:

```bash
stompy update
```

## Quick Start

```bash
# Authenticate (opens browser for OAuth login)
stompy login

# Set your default project
stompy project use my-project

# Lock a context (persistent memory)
stompy context lock deployment-notes --content "Production uses PostgreSQL 16 on Neon"

# Recall it later
stompy context recall deployment-notes

# Create a ticket
stompy ticket create --title "Add rate limiting" --type feature --priority high

# View the board
stompy ticket board

# Pipe content from files
cat architecture.md | stompy context lock architecture

# JSON output for scripting
stompy project list -o json | jq '.[].name'
```

## Authentication

### Browser Login (Primary)

```bash
stompy login     # Opens browser for OAuth 2.0 PKCE flow
stompy whoami    # Check current auth status
stompy logout    # Clear stored tokens
```

Tokens are stored in `~/.stompy/config.yaml` and auto-refresh when expired.

### API Key (CI/CD)

```bash
# Environment variable
export STOMPY_API_KEY=sk-your-api-key
stompy project list

# Or flag
stompy --api-key sk-your-api-key project list

# Or config
stompy config set api_key sk-your-api-key
```

## Commands

```
stompy
├── login                          # OAuth 2.0 browser login (PKCE)
├── logout                         # Clear stored tokens
├── whoami                         # Show auth status
├── project
│   ├── create <name>              # Create project
│   ├── list [--stats]             # List all projects
│   ├── info <name> [--stats]      # Project details
│   ├── delete <name> --confirm    # Delete project
│   └── use <name>                 # Set default project
├── context
│   ├── lock <topic> --content     # Create/update context
│   ├── recall <topic>             # Read context
│   ├── unlock <topic>             # Delete context
│   ├── list                       # List contexts
│   ├── search <query>             # Search contexts
│   ├── update <topic> --content   # Update context
│   └── move <topic> --to <proj>   # Move to another project
├── ticket
│   ├── create --title T           # Create ticket
│   ├── get <id>                   # Show ticket
│   ├── update <id>                # Update ticket
│   ├── move <id> --status S       # Transition status
│   ├── close <id>                 # Smart close (infers status)
│   ├── list                       # List tickets
│   ├── board                      # Kanban board view
│   ├── search <query>             # Search tickets
│   └── link add|list|remove       # Manage ticket links
├── config
│   ├── set <key> <value>          # Set config value
│   ├── get <key>                  # Get config value
│   └── show                       # Show all config
├── update                         # Self-update to latest version
├── version                        # Print version
└── completion [bash|zsh|fish|ps]  # Shell completions
```

### Global Flags

| Flag | Description |
|------|-------------|
| `--api-url` | Override API base URL |
| `--api-key` | Override API key |
| `-p, --project` | Override default project |
| `-o, --output` | Output format: `table` (default), `json`, `yaml` |
| `--verbose` | Debug HTTP logging |

### Content Input

Context commands accept content in three ways:

```bash
# Inline text
stompy context lock my-topic --content "Hello world"

# From file (@ prefix)
stompy context lock my-topic --content @notes.md

# From stdin (pipe)
cat README.md | stompy context lock readme-context
```

### Smart Ticket Close

`stompy ticket close` fetches the ticket type and transitions to the correct terminal status:

| Type | Terminal Status |
|------|----------------|
| task | done |
| bug | resolved |
| feature | shipped |
| decision | decided |

### Output Formats

All list/detail commands support structured output for scripting:

```bash
# Default table output (with colors)
stompy ticket list

# JSON (pipe to jq, python, etc.)
stompy ticket list -o json | jq '.[] | select(.priority == "high")'

# YAML
stompy project list -o yaml
```

## Configuration

Config is stored at `~/.stompy/config.yaml`:

```yaml
api_url: https://api.stompy.ai/api/v1
default_project: my-project
output_format: table

# OAuth tokens (managed by stompy login)
auth:
  access_token: eyJ...
  refresh_token: dGVz...
  token_expiry: "2026-02-22T12:00:00Z"
  email: user@example.com
```

## Shell Completions

```bash
# Bash
stompy completion bash > /etc/bash_completion.d/stompy

# Zsh
stompy completion zsh > "${fpath[1]}/_stompy"

# Fish
stompy completion fish > ~/.config/fish/completions/stompy.fish
```

## Links

- **Stompy Platform**: [stompy.ai](https://stompy.ai)
- **Web Dashboard**: [app.stompy.ai](https://app.stompy.ai)
- **Join the Waitlist**: [stompy.ai/waitlist](https://stompy.ai/waitlist)
- **API Documentation**: [docs.stompy.ai](https://docs.stompy.ai)

## Contributing

We welcome contributions! Stompy CLI is built with:

- **Go 1.21+** with [Cobra](https://github.com/spf13/cobra) (CLI framework) and [Viper](https://github.com/spf13/viper) (config)
- **[go-pretty](https://github.com/jedib0t/go-pretty)** for terminal-aware table rendering
- **TDD** — tests first, implementation second

```bash
# Run tests
make test

# Run tests with coverage
make test-coverage

# Lint
make lint

# Build all platforms
make build-all
```

### Development Setup

1. Fork and clone the repo
2. Run `make test` to verify everything passes
3. Create a feature branch: `git checkout -b feature/your-feature`
4. Write tests first, then implementation
5. Submit a PR

See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed guidelines.

## License

MIT License — see [LICENSE](LICENSE) for details.

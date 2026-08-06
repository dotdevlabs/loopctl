# loopctl

CLI for managing the [LoopControl](https://loopcontrol.ai) platform.

## Installation

### Homebrew

```bash
brew install dotdevlabs/tap/loopctl
```

### curl | sh

```bash
curl -sSL https://github.com/dotdevlabs/loopctl/releases/latest/download/install.sh | sh
```

Set `LOOPCTL_INSTALL_DIR` to override the install location (default: `/usr/local/bin`).

### go install

```bash
go install github.com/dotdevlabs/loopctl/cmd/loopctl@latest
```

## Authentication

loopctl authenticates via bearer token. Provide credentials using a named context or environment variables:

```bash
# Environment variables
export LOOPCTL_TOKEN=your-api-token
export LOOPCTL_CONTEXT=production   # optional: named context

# Or manage named contexts
loopctl context set production --token your-api-token
loopctl context use production
```

Config is stored at `~/.config/atmt/loopcontrol.yaml`.

## Global Flags

These flags apply to every command:

| Flag | Description |
|------|-------------|
| `--context <name>` | Named context to use |
| `--json` | Output machine-stable JSON |
| `--format <template>` | Go template for custom output |
| `--dry-run` | Print what would happen without making API calls |
| `--verbose` | Show HTTP request and response details on stderr (method, URL, status, body) |

When `--verbose` is set, each API call writes diagnostic lines to stderr:

```
> POST https://api.example.com/api/tasks
< 422 Unprocessable Entity
{"errors":[{"status":"422","detail":"title can't be blank"}]}
```

This output goes to stderr and does not affect `--json` stdout.

When an API request fails (any command), the CLI prints the human-readable error reason from the API response alongside the HTTP status code.

## Commands

### projects

Manage LoopControl projects.

#### projects list

List all projects.

```bash
loopctl projects list
loopctl projects list --json
```

**API:** `GET /api/projects`

#### projects get

Get a project by ID.

```bash
loopctl projects get <id>
loopctl projects get <id> --json
```

**API:** `GET /api/projects/:id`

#### projects create

Create a new project. By default, bootstraps a brand-new GitHub repository under the configured organization. Pass `--repo` to link an existing repository instead.

```bash
# Bootstrap a new repo (default path)
loopctl projects create --name "Daybreak" --platform-id <platform-id>

# Bootstrap with explicit pipeline and slug override
loopctl projects create --name "Daybreak" --platform-id <platform-id> \
  --pipeline-id <pipeline-id> --slug daybreak-v2

# Link to an existing repository
loopctl projects create --name "Daybreak" --platform-id <platform-id> \
  --repo https://github.com/org/repo
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--name` | yes | | Human/display name for the project |
| `--platform-id` | yes | | Platform ID |
| `--pipeline-id` | no | | Pipeline ID (sets the project's default pipeline) |
| `--slug` | no | derived from `--name` | Repo slug override (lowercase letters/digits/hyphens, must start with a letter) |
| `--organization` | no | `dotdevlabs` | GitHub organization for the new repo |
| `--organization-type` | no | `Organization` | Organization type (`Organization` or `User`) |
| `--repo` | no | | Existing repository URL; triggers existing-repo path instead of bootstrap |

The slug is automatically derived from `--name`: lowercased, spaces/underscores converted to hyphens, non-alphanumeric characters stripped, leading digits/hyphens removed. Use `--slug` to override.

**API:** `POST /api/projects`

---

### tasks

Manage LoopControl tasks.

#### tasks list

List tasks for a project. Output columns: `ID`, `KIND`, `TITLE`, `STAGE`, `STATUS`.

```bash
loopctl tasks list --project-id <project-id>
loopctl tasks list --project-id <project-id> --json
```

**Flags:**

| Flag | Required | Description |
|------|----------|-------------|
| `--project-id` | yes | Project ID to filter tasks |

**API:** `GET /api/tasks?project_id=<project-id>`

#### tasks get

Get a task by ID. Output columns: `ID`, `KIND`, `TITLE`, `STAGE`, `STATUS`.

```bash
loopctl tasks get <id>
loopctl tasks get <id> --json
```

**API:** `GET /api/tasks/:id`

#### tasks create

Create a new task. Output columns: `ID`, `KIND`, `TITLE`, `STAGE`, `STATUS`.

```bash
loopctl tasks create \
  --project-id <project-id> \
  --kind <kind> \
  --title "My task" \
  --description "Task details"
```

**Flags:**

| Flag | Required | Description |
|------|----------|-------------|
| `--project-id` | yes | Project ID |
| `--kind` | yes | Task kind |
| `--title` | yes | Task title |
| `--description` | yes | Task description |

**API:** `POST /api/tasks`

#### tasks update

Update an existing task. Only the flags you provide are changed. Output columns: `ID`, `KIND`, `TITLE`, `STAGE`, `STATUS`.

```bash
loopctl tasks update <id> --title "New title"
loopctl tasks update <id> --kind feature --description "Updated details"
```

**Flags:**

| Flag | Required | Description |
|------|----------|-------------|
| `--kind` | no | Task kind |
| `--title` | no | Task title |
| `--description` | no | Task description |

At least one flag must be provided.

**API:** `PATCH /api/tasks/:id`

#### tasks comments

List comments on a task.

```bash
loopctl tasks comments <id>
loopctl tasks comments <id> --json
```

**API:** `GET /api/tasks/:id/comments`

#### tasks watch

Follow a task to a terminal state, streaming stage and activity changes as they occur. Exits with code 0 when the task reaches `completed` or `reviewed`; exits non-zero on `rejected`, container error, or timeout.

```bash
loopctl tasks watch <id>
loopctl tasks watch <id> --interval 30s --timeout 60m
loopctl tasks watch <id> --json
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--interval` | `15s` | Poll interval (e.g. `15s`, `1m`) |
| `--timeout` | _(none)_ | Give up after this duration (e.g. `30m`); empty means no timeout |
| `--json` | — | Emit final task state as a JSON:API resource on exit (`{"id":"...","type":"tasks","attributes":{...}}`) |

**Exit codes:**
- `0` — task reached `completed` or `reviewed`
- non-zero — task `rejected`, `container.error` activity received, timeout exceeded, or network error

**APIs:** `GET /api/tasks/:id`, `GET /api/tasks/:id/activities`

---

### ai

Print the full AI-ingestible command reference.

```bash
loopctl ai           # markdown output
loopctl ai --json    # structured JSON for agent ingestion
```

### auth

Manage API authentication. Provided by ctlkit.

```bash
loopctl auth login
loopctl auth logout
loopctl auth status
```

### context

Manage named contexts (base URL + token pairs). Provided by ctlkit.

```bash
loopctl context list
loopctl context set <name> --token <token>
loopctl context use <name>
loopctl context delete <name>
```

### version

Print version information.

```bash
loopctl version
loopctl version --json
```

---

## Common Workflows

### Create a task and follow it to completion

```bash
# Create the task
loopctl tasks create \
  --project-id <project-id> \
  --kind feature \
  --title "My task" \
  --description "Details"

# Stream stage transitions until the task completes (or fails)
loopctl tasks watch <task-id>

# Same with a 60-minute safety timeout and 30-second poll interval
loopctl tasks watch <task-id> --interval 30s --timeout 60m

# Emit final state as JSON (for scripting or agents)
loopctl tasks watch <task-id> --json
```

### List a project's tasks as JSON (for scripting or agents)

```bash
loopctl tasks list --project-id <project-id> --json
```

### Preview a write without making API calls

```bash
loopctl projects create --name "Daybreak" --platform-id "pf1" --dry-run
loopctl tasks create --project-id p1 --kind bug --title "Fix it" --description "..." --dry-run
```

---

## Development

```bash
# Run full CI checks (format, vet, lint, test, build)
./bin/ci

# Build
go build ./cmd/loopctl

# Test with race detector
go test ./... -race

# Coverage report
go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out
```

**Module proxy**: `ctlkit` (`github.com/dotdevlabs/ctlkit`) is a public GitHub repo but is not indexed by the public Go module proxy. Set these env vars when building outside CI so Go fetches it directly and skips the checksum database:

```bash
export GONOSUMDB='github.com/dotdevlabs/*'
export GOPRIVATE='github.com/dotdevlabs/*'
```

## Release

Releases are automated via [goreleaser](https://goreleaser.com/) on git tags. Static binaries are published for darwin/linux/windows × amd64/arm64.

```bash
git tag v0.1.0
git push origin v0.1.0
```

Goreleaser publishes:
- GitHub Release with archives and `checksums.txt`
- Homebrew formula to `dotdevlabs/homebrew-tap`
- `install.sh` attached to the release

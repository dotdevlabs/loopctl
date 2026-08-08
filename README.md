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

Create a new project. By default, bootstraps a brand-new GitHub repository. Pass `--repo` to link an existing repository instead.

Select the platform and pipeline by **name** (recommended) or by numeric **ID**. Use `loopctl platforms list` and `loopctl pipelines list` to discover available values.

```bash
# Bootstrap a new repo by platform name
loopctl projects create --name "Daybreak" --platform rails

# Bootstrap with explicit pipeline name and slug override
loopctl projects create --name "Daybreak" --platform rails \
  --pipeline "Autonomous Feature" --slug daybreak-v2

# Use numeric IDs directly (backward-compatible)
loopctl projects create --name "Daybreak" --platform-id <platform-id> \
  --pipeline-id <pipeline-id>

# Link to an existing repository
loopctl projects create --name "Daybreak" --platform rails \
  --repo https://github.com/org/repo
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--name` | yes | | Human/display name for the project |
| `--platform` | yes* | | Platform name or slug (resolved to ID automatically) |
| `--platform-id` | yes* | | Platform numeric ID (alternative to `--platform`) |
| `--pipeline` | no | | Pipeline name (resolved to ID automatically; alternative to `--pipeline-id`) |
| `--pipeline-id` | no | | Pipeline numeric ID (alternative to `--pipeline`) |
| `--slug` | no | derived from `--name` | Repo slug override (lowercase letters/digits/hyphens, must start with a letter) |
| `--repo` | no | | Existing repository URL; triggers existing-repo path instead of bootstrap |

\* Exactly one of `--platform` or `--platform-id` is required. Specifying both is an error.

The slug is automatically derived from `--name`: lowercased, spaces/underscores converted to hyphens, non-alphanumeric characters stripped, leading digits/hyphens removed. Use `--slug` to override.

If a named platform or pipeline is not found, the command fails with a clear error and creates nothing.

**API:** `POST /api/projects` (preceded by `GET /api/platforms` and/or `GET /api/pipelines` when resolving by name)

---

### platforms

List available platforms. Use this to discover the platform names and IDs accepted by `projects create`.

#### platforms list

```bash
loopctl platforms list
loopctl platforms list --json
```

Output columns: `ID`, `NAME`, `DISPLAY_NAME`.

**API:** `GET /api/platforms`

---

### pipelines

Manage LoopControl pipelines. Use `pipelines list` to discover names and IDs accepted by `projects create`.

#### pipelines list

```bash
loopctl pipelines list
loopctl pipelines list --json
```

Output columns: `ID`, `NAME`, `KIND`.

**API:** `GET /api/pipelines`

#### pipelines create

Create a new pipeline for your account, optionally linked to a task kind.

```bash
loopctl pipelines create --name "My Pipeline"
loopctl pipelines create --name "My Pipeline" --kind my-task-kind
loopctl pipelines create --name "My Pipeline" --kind my-task-kind --description "Optional description"
loopctl pipelines create --name "My Pipeline" --dry-run
```

**Flags:**

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | yes | Name for the new pipeline |
| `--kind` | no | Task-kind name the pipeline belongs to |
| `--description` | no | Optional description for the pipeline |

Output columns on success: `ID`, `NAME`, `KIND`. Use `--json` for the full resource.

**API:** `POST /api/pipelines`

---

### task-kinds

Manage LoopControl task kinds. Built-in kinds are shared and read-only; custom kinds are scoped to your account.

#### task-kinds list

```bash
loopctl task-kinds list
loopctl task-kinds list --json
```

Output columns: `ID`, `NAME`, `BUILT_IN`.

**API:** `GET /api/task_kinds`

#### task-kinds create

Create a new custom task kind for your account.

```bash
loopctl task-kinds create --name my-kind
loopctl task-kinds create --name my-kind --dry-run
```

**Flags:**

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | yes | Slug/identifier for the task kind |

Output columns on success: `ID`, `NAME`.

Attempting to create a built-in task kind surfaces the API's human-readable error.

**API:** `POST /api/task_kinds`

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

Update an existing task's criteria fields. Only the flags you provide are changed. Output columns: `ID`, `KIND`, `TITLE`, `STAGE`, `STATUS`.

```bash
loopctl tasks update <id> --implementation-criteria "Step-by-step plan..."
loopctl tasks update <id> --verification-criteria "Run these checks..."
loopctl tasks update <id> \
  --implementation-criteria "Step-by-step plan..." \
  --verification-criteria "Run these checks..."
```

**Flags:**

| Flag | Required | Description |
|------|----------|-------------|
| `--implementation-criteria` | no | Implementation plan written by the planning agent |
| `--verification-criteria` | no | Verification steps for the implementing agent |

At least one flag must be provided.

**API:** `PATCH /api/tasks/:id`

#### tasks cancel

Cancel an in-progress task by ID. On success, prints a confirmation. With `--json`, emits `{"id":"...","status":"cancelled","stage":"..."}`. Attempting to cancel an already-finished task surfaces the API's human-readable error.

```bash
loopctl tasks cancel <id>
loopctl tasks cancel <id> --json
loopctl tasks cancel <id> --dry-run
```

**API:** `POST /api/tasks/:task_id/cancellation`

---

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

### schema

Inspect the API's published contract.

#### schema fetch

Fetch the full API contract from the schema endpoint and display each endpoint's method, path, and description.

```bash
loopctl schema fetch
loopctl schema fetch --json
```

Output columns: `METHOD`, `PATH`, `DESCRIPTION`.

**API:** `GET /api/schema`

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

### Discover available platforms, pipelines, and task kinds

```bash
loopctl platforms list
loopctl pipelines list
loopctl task-kinds list
```

### Create a custom pipeline and use it for a project

```bash
# Create a pipeline (kind is optional)
loopctl pipelines create --name "My Workflow"

# Or link it to a specific task kind
loopctl task-kinds create --name my-workflow-kind
loopctl pipelines create --name "My Workflow" --kind my-workflow-kind

# Use the new pipeline when creating a project
loopctl projects create --name "Daybreak" --platform rails --pipeline "My Workflow"
```

### Create a custom task kind

```bash
loopctl task-kinds create --name my-kind
```

### Preview a write without making API calls

```bash
loopctl projects create --name "Daybreak" --platform rails --dry-run
loopctl projects create --name "Daybreak" --platform-id "pf1" --dry-run
loopctl tasks create --project-id p1 --kind bug --title "Fix it" --description "..." --dry-run
loopctl pipelines create --name "My Workflow" --dry-run
loopctl task-kinds create --name my-kind --dry-run
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

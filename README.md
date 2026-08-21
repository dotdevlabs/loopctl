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
# Bootstrap a brand-new machine in one step (no prior config required)
loopctl onboard --email you@example.com

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

#### projects update

Update an existing project's attributes. Only the flags you provide are changed — unset flags are not sent to the API and will not clobber server state.

```bash
# Bind a pipeline to a project
loopctl projects update <id> --pipeline-id <pipeline-id>

# Rename a project
loopctl projects update <id> --display-name "New Name"

# Change the tracked branch and pipeline together
loopctl projects update <id> --git-branch main --pipeline-id 9

# Preview without making API calls
loopctl projects update <id> --pipeline-id 9 --dry-run
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--display-name` | string | Human-readable display name |
| `--git-branch` | string | Git branch to track |
| `--environment-id` | int | Environment numeric ID |
| `--container-image` | string | Container image reference |
| `--platform-id` | int | Platform numeric ID |
| `--pipeline-id` | int | Pipeline numeric ID |
| `--failure-policy` | string | Failure policy for tasks |
| `--fallback-agent-id` | int | Fallback agent numeric ID |

At least one flag must be provided. Output columns on success: `ID`, `NAME`, `PLATFORM`, `REPO`. Use `--json` for the full resource.

**API:** `PATCH /api/projects/:id`

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

Create a new pipeline for your account, optionally linked to a task kind. Pass `--stages` to author the pipeline's ordered stages at creation time.

```bash
loopctl pipelines create --name "My Pipeline"
loopctl pipelines create --name "My Pipeline" --kind my-task-kind
loopctl pipelines create --name "My Pipeline" --kind my-task-kind --description "Optional description"
loopctl pipelines create --name "My Pipeline" --dry-run

# Create a pipeline with stages
loopctl pipelines create --name "My Pipeline" \
  --stages '[{"name":"plan","role":"planning","instructions":"Plan the work."},{"name":"implement","role":"implementing","instructions":"Implement the plan."}]'
```

**Flags:**

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | yes | Name for the new pipeline |
| `--kind` | no | Task-kind name the pipeline belongs to |
| `--description` | no | Optional description for the pipeline |
| `--stages` | no | JSON array of ordered stages (see stage shape below) |

**Stage shape** (each element of the `--stages` array):

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Unique stage identifier within the pipeline |
| `role` | string | Lifecycle role (e.g. `"planning"`, `"implementing"`) |
| `instructions` | string | Full agent prompt text for this stage |

Output columns on success: `ID`, `NAME`, `KIND`. Use `--json` for the full resource.

**API:** `POST /api/pipelines`

#### pipelines update

Update an existing pipeline's attributes and stages. Only the flags you provide are changed.

```bash
loopctl pipelines update <id> --name "Renamed Pipeline"
loopctl pipelines update <id> --description "New description"
loopctl pipelines update <id> --kind my-task-kind

# Replace all stages
loopctl pipelines update <id> \
  --stages '[{"name":"plan","role":"planning","instructions":"Plan the work."},{"name":"implement","role":"implementing","instructions":"Implement the plan."}]'

# Preview without making API calls
loopctl pipelines update <id> --name "Renamed" --dry-run
```

**Flags:**

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | no | New name for the pipeline |
| `--description` | no | Human-readable description |
| `--kind` | no | Task kind name the pipeline belongs to |
| `--stages` | no | JSON array of ordered stages (replaces all existing stages; see stage shape above) |

At least one flag must be provided. Output columns on success: `ID`, `NAME`, `KIND`. Use `--json` for the full resource.

**API:** `PATCH /api/pipelines/:id`

#### pipelines stages

Manage the individual stages of a pipeline. These commands expose the full stage attribute set per the published API contract. Because there is no dedicated stages endpoint, each write command performs a read-modify-write: it GETs the current pipeline, applies your changes, and PATCHes the full updated stages array atomically.

> **Note:** Two concurrent `stages add` (or `update`/`remove`) calls can overwrite each other — this is inherent to the API design (no conditional PATCH). For safe concurrent edits, serialize your calls.

##### pipelines stages list

List the stages of a pipeline.

```bash
loopctl pipelines stages list <pipeline-id>
loopctl pipelines stages list <pipeline-id> --json
```

Output columns: `POSITION`, `NAME`, `ROLE`, `STAGE_TYPE`, `GATE`, `AGENT`.

**API:** `GET /api/pipelines/:id`

##### pipelines stages add

Append a new stage to a pipeline. Only the flags you provide are included — unset optional flags are not sent.

```bash
# Minimal — just a name
loopctl pipelines stages add <pipeline-id> --name implement

# With role and instructions
loopctl pipelines stages add <pipeline-id> \
  --name implement \
  --role implementing \
  --instructions "Implement the approved plan."

# Full attribute set
loopctl pipelines stages add <pipeline-id> \
  --name review \
  --role reviewing \
  --instructions "Review the implementation." \
  --gate manual \
  --agent my-reviewer \
  --advance-notice "30m" \
  --position 3 \
  --runs-in-container \
  --on-failure '{"max_rework_count":2,"rework_to_position":1}' \
  --prompt-sections '[{"key":"context","content":"..."}]' \
  --stage-triggers '["on_push"]' \
  --advance-requirements '["tests_pass"]' \
  --branch-conditions '["main"]' \
  --environment '{"CI":"true"}'

# Preview without making API calls
loopctl pipelines stages add <pipeline-id> --name review --dry-run
```

**Flags:**

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | yes | Stage name (unique identifier within the pipeline) |
| `--role` | no | Lifecycle role (e.g. `planning`, `implementing`, `reviewing`) |
| `--stage-type` | no | `stage_type` for custom stage types |
| `--custom-stage-name` | no | `custom_stage_name` (custom type only) |
| `--template` | no | Template (custom type only) |
| `--instructions` | no | Full agent prompt text for this stage |
| `--gate` | no | Advance gate (e.g. `manual`, `ci_pass`) |
| `--agent` | no | Agent slug override |
| `--advance-notice` | no | Advance notice value |
| `--position` | no | Explicit position override |
| `--runs-in-container` | no | Override `runs_in_container` for this stage |
| `--on-failure` | no | JSON object: `{"max_rework_count":3,"rework_to_position":0}` |
| `--prompt-sections` | no | JSON array of prompt section objects |
| `--stage-triggers` | no | JSON array of trigger strings |
| `--advance-requirements` | no | JSON array of requirement strings |
| `--branch-conditions` | no | JSON array of branch condition strings |
| `--environment` | no | JSON object of environment key/value pairs |

Output columns on success: `ID`, `NAME`, `KIND`. Use `--json` for the full pipeline resource (stages included).

**APIs:** `GET /api/pipelines/:id` then `PATCH /api/pipelines/:id`

##### pipelines stages update

Update an existing stage by name. Only the flags you provide are changed — other fields are preserved, including any server-generated fields.

```bash
# Change instructions
loopctl pipelines stages update <pipeline-id> <stage-name> \
  --instructions "Updated prompt text."

# Change gate and add failure policy
loopctl pipelines stages update <pipeline-id> review \
  --gate ci_pass \
  --on-failure '{"max_rework_count":3}'

# Preview without making API calls
loopctl pipelines stages update <pipeline-id> review --gate manual --dry-run
```

Accepts the same flags as `stages add` except `--name` (the stage is identified by the `<stage-name>` positional argument). At least one flag must be provided. If two stages share the same name, the first match is updated.

Output columns on success: `ID`, `NAME`, `KIND`. Use `--json` for the full pipeline resource.

**APIs:** `GET /api/pipelines/:id` then `PATCH /api/pipelines/:id`

##### pipelines stages remove

Remove a stage by name. Sends the remaining stages (or an empty array if removing the last stage).

```bash
loopctl pipelines stages remove <pipeline-id> <stage-name>
loopctl pipelines stages remove <pipeline-id> review --dry-run
```

Output columns on success: `ID`, `NAME`, `KIND`. Use `--json` for the full pipeline resource.

**APIs:** `GET /api/pipelines/:id` then `PATCH /api/pipelines/:id`

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

#### task-kinds list-default-pipelines

List all account-level default pipelines, one row per task kind that has a default configured.

```bash
loopctl task-kinds list-default-pipelines
loopctl task-kinds list-default-pipelines --json
```

Output columns: `ID`, `KIND`, `PIPELINE_ID`.

**API:** `GET /api/account_pipeline_defaults`

#### task-kinds set-default-pipeline

Set the default pipeline for a task kind. Tasks of this kind will use the specified pipeline unless overridden at task creation time.

The first argument is the task kind **name** (e.g. `feature`), not a numeric ID. Use `loopctl task-kinds list` to see available kind names.

```bash
loopctl task-kinds set-default-pipeline <kind-name> --pipeline-id <integer-pipeline-id>
loopctl task-kinds set-default-pipeline <kind-name> --pipeline-id <integer-pipeline-id> --json
loopctl task-kinds set-default-pipeline <kind-name> --pipeline-id <integer-pipeline-id> --dry-run
```

**Flags:**

| Flag | Required | Description |
|------|----------|-------------|
| `--pipeline-id` | yes | Integer ID of the pipeline to set as default for this kind |

Output columns on success: `ID`, `KIND`, `PIPELINE_ID`.

**API:** `PATCH /api/account_pipeline_defaults/:kind`

#### task-kinds clear-default-pipeline

Clear the default pipeline for a task kind, returning it to unset.

The first argument is the task kind **name** (e.g. `feature`), not a numeric ID.

```bash
loopctl task-kinds clear-default-pipeline <kind-name>
loopctl task-kinds clear-default-pipeline <kind-name> --json
loopctl task-kinds clear-default-pipeline <kind-name> --dry-run
```

On success, prints a confirmation message. With `--json`, emits `{"kind":"...","status":"cleared"}`.

**API:** `DELETE /api/account_pipeline_defaults/:kind`

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

Get a task by ID. Output columns: `ID`, `KIND`, `TITLE`, `STAGE`, `STATUS`, `DEPENDENCIES_MET`.

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

# Create and follow to completion in one command
loopctl tasks create \
  --project-id <project-id> \
  --kind <kind> \
  --title "My task" \
  --description "Task details" \
  --watch

# With a timeout and custom poll interval
loopctl tasks create \
  --project-id <project-id> \
  --kind <kind> \
  --title "My task" \
  --description "Task details" \
  --watch --interval 30s --timeout 60m

# Create a task that depends on one or more existing tasks
loopctl tasks create \
  --project-id <project-id> \
  --kind <kind> \
  --title "Dependent task" \
  --description "Runs after t1 and t2 complete" \
  --depends-on <task-id-1> \
  --depends-on <task-id-2>
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--project-id` | yes | | Project ID |
| `--kind` | yes | | Task kind |
| `--title` | yes | | Task title |
| `--description` | yes | | Task description |
| `--depends-on` | no | | Task ID this task depends on; repeat to add multiple |
| `--watch` | no | `false` | Follow the task to a terminal state after creating it |
| `--interval` | no | `15s` | Poll interval when `--watch` is set (e.g. `15s`, `1m`) |
| `--timeout` | no | _(none)_ | Give up after this duration when `--watch` is set (e.g. `30m`); empty means no timeout |

When `--watch` is set, the command first prints the created task row, then streams stage and activity changes until the task reaches a terminal state — identical behavior to `tasks watch`. Exit codes match those of `tasks watch`.

**API:** `POST /api/tasks` (followed by `GET /api/tasks/:id` and `GET /api/tasks/:id/activities` when `--watch` is set)

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

#### tasks unblock

Unblock a task that is blocked (e.g. due to `max_reworks_exceeded`). On success, prints a confirmation. With `--json`, emits `{"id":"...","status":"...","stage":"..."}`. Attempting to unblock a task that is not blocked surfaces the API's human-readable error.

```bash
loopctl tasks unblock <id>
loopctl tasks unblock <id> --json
loopctl tasks unblock <id> --dry-run
```

**API:** `POST /api/tasks/:task_id/unblock`

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

#### tasks comments create

Post a comment on a task. Token counts are optional billing metadata.

```bash
loopctl tasks comments create <task-id> --body "Implementation complete."
loopctl tasks comments create <task-id> --body "..." --input-tokens 1234 --output-tokens 567
loopctl tasks comments create <task-id> --body "..." --json
loopctl tasks comments create <task-id> --body "..." --dry-run
```

**Flags:**

| Flag | Required | Description |
|------|----------|-------------|
| `--body` | yes | Comment body text |
| `--input-tokens` | no | Input token count for billing |
| `--output-tokens` | no | Output token count for billing |

**API:** `POST /api/tasks/:id/comments`

---

#### tasks todos list

List todos for a task, optionally filtered by stage name. Follows pagination automatically.

```bash
loopctl tasks todos list <task-id>
loopctl tasks todos list <task-id> --stage-name implementing
loopctl tasks todos list <task-id> --json
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--stage-name` | Filter todos by stage name |

**API:** `GET /api/tasks/:id/todos`

---

#### tasks todos create

Create one or more todos for a task. Use `--content` to create a single todo or `--bulk-json` to create multiple todos in one request.

```bash
# Single todo
loopctl tasks todos create <task-id> --content "Write unit tests" --status pending --stage-name implementing

# With active form text
loopctl tasks todos create <task-id> --content "Deploy" --active-form "Deploying to staging..." --position 0

# Bulk creation
loopctl tasks todos create <task-id> --bulk-json '[{"content":"Step A","status":"pending"},{"content":"Step B","status":"pending"}]'
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--content` | Todo content (required unless `--bulk-json`) |
| `--status` | Todo status: `pending`, `in_progress`, or `completed` (default: `pending`) |
| `--stage-name` | Stage name this todo belongs to |
| `--active-form` | Active form description shown while working |
| `--position` | Display position (0-indexed) |
| `--bulk-json` | JSON array of todo objects for bulk creation |

**API:** `POST /api/tasks/:id/todos`

---

#### tasks todos update

Update a todo's status, content, or active-form text.

```bash
loopctl tasks todos update <task-id> <todo-id> --status in_progress
loopctl tasks todos update <task-id> <todo-id> --status completed
loopctl tasks todos update <task-id> <todo-id> --content "Revised description"
loopctl tasks todos update <task-id> <todo-id> --active-form "Now deploying..."
loopctl tasks todos update <task-id> <todo-id> --dry-run
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--status` | New status: `pending`, `in_progress`, or `completed` |
| `--content` | New content text |
| `--active-form` | New active form description |

**API:** `PATCH /api/tasks/:id/todos/:todo_id`

---

### topup

Fund your LoopControl account. When the account balance is insufficient, the API returns HTTP 402 with the available payment products and rails. `loopctl topup` settles the payment automatically when a wallet is configured, or prints a Stripe hosted-checkout link for manual payment in a browser.

```bash
# Print the checkout link for the default top-up package (no wallet configured)
loopctl topup

# Choose a specific product
loopctl topup --product trial
loopctl topup --product subscription

# Machine-stable JSON output (for scripting or agents)
loopctl topup --json
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--product` | `topup` | Product to fund: `trial` (5-hour trial), `topup` (top-up package), or `subscription` (subscription prepay) |

#### Payment rails

`loopctl topup` supports three payment rails, selected by what the 402 response advertises intersected with what the local wallet can satisfy:

| Rail | Requires | Behavior |
|------|----------|----------|
| `x402` | `LOOPCTL_ARBITRUM_PRIVATE_KEY` | Signs and submits a USDC payment on Arbitrum (EIP-3009), then retries the request automatically |
| `l402` | `LOOPCTL_LN_HOST` + `LOOPCTL_LN_MACAROON_HEX` | Pays the Lightning invoice via the configured LND node, then retries the request automatically |
| `human_link` | _(none)_ | Prints the Stripe checkout URL for manual payment in a browser (default fallback) |

Rails are tried in preference order: `x402` → `l402` → `human_link`. The first configured rail that the 402 response also advertises wins.

#### Wallet configuration

Wallet credentials are supplied via environment variables — never stored in the config file.

**x402 (Arbitrum USDC):**

| Variable | Description |
|----------|-------------|
| `LOOPCTL_ARBITRUM_PRIVATE_KEY` | 64-char hex secp256k1 private key (no `0x` prefix) |

**L402 (Lightning via LND REST):**

| Variable | Description |
|----------|-------------|
| `LOOPCTL_LN_HOST` | LND REST base URL, e.g. `https://localhost:8080` |
| `LOOPCTL_LN_MACAROON_HEX` | Hex-encoded admin or invoice macaroon |
| `LOOPCTL_LN_TLS_SKIP_VERIFY` | Set to `"true"` to skip TLS verification (dev only) |

**Example — auto-pay with Arbitrum USDC:**

```bash
export LOOPCTL_ARBITRUM_PRIVATE_KEY=<64-char-hex-key>
loopctl topup   # settles automatically; no browser required
```

**Example — auto-pay with Lightning:**

```bash
export LOOPCTL_LN_HOST=https://localhost:8080
export LOOPCTL_LN_MACAROON_HEX=<hex-macaroon>
loopctl topup   # pays the Lightning invoice and retries automatically
```

#### JSON output

With `--json`, the output depends on which rail settled the payment:

```json
{"product": "topup", "rail": "human_link", "url": "https://checkout.stripe.com/..."}
```

```json
{"rail": "x402", "status": "paid", "from": "0x...", "product": "topup"}
```

```json
{"rail": "l402", "status": "paid", "product": "topup"}
```

**API:** `POST /api/topup`

---

### schema

Inspect the API's published contract.

#### schema show

Display the API's published contract. Reads from the committed contract document in the API's repository.

```bash
loopctl schema show
loopctl schema show --json
```

Output columns: `METHOD`, `PATH`, `DESCRIPTION`.

---

### ai

Print the full AI-ingestible command reference.

```bash
loopctl ai           # markdown output
loopctl ai --json    # structured JSON for agent ingestion
```

### onboard

Register a new machine and persist API credentials in a single step. Does not require any prior configuration. Calls `POST /api/registrations` (unauthenticated), then writes the returned token into `~/.config/atmt/loopcontrol.yaml` under the specified context name.

```bash
# Minimal — register with just an email
loopctl onboard --email you@example.com

# Custom context name (default: "default")
loopctl onboard --email you@example.com --name production

# Custom token label and account name
loopctl onboard --email you@example.com --token-name "ci-runner" --account-name "Acme Corp"

# Point at a non-production instance
loopctl onboard --email you@example.com --url https://staging.loopcontrol.ai

# Machine-stable JSON output
loopctl onboard --email you@example.com --json
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--email` | yes | | Email address for the new account |
| `--name` | no | `"default"` | Context name to store credentials under |
| `--url` | no | `"https://app.loopcontrol.ai"` | LoopControl API base URL |
| `--token-name` | no | | Label for the generated API token |
| `--account-name` | no | | Account name |

On success, prints `Registered. Context "<name>" saved.` With `--json`, emits:

```json
{"context":"default","token":"<token>","token_label":"<label>"}
```

If the config already has a current context, it is preserved — only the new named context is added. If no current context existed, the new context becomes the current context.

**API:** `POST /api/registrations` (no authentication required)

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

### Bootstrap a new machine

```bash
# Register and configure credentials in one step
loopctl onboard --email you@example.com

# Verify the stored token works
loopctl projects list

# Bootstrap a second named context (e.g. for a staging environment)
loopctl onboard --email you@example.com \
  --url https://staging.loopcontrol.ai \
  --name staging
loopctl context use staging
```

### Create a task and follow it to completion

```bash
# Create and follow in one command (--watch)
loopctl tasks create \
  --project-id <project-id> \
  --kind feature \
  --title "My task" \
  --description "Details" \
  --watch

# With a 60-minute safety timeout and 30-second poll interval
loopctl tasks create \
  --project-id <project-id> \
  --kind feature \
  --title "My task" \
  --description "Details" \
  --watch --interval 30s --timeout 60m

# Two-step: create then follow separately
loopctl tasks create \
  --project-id <project-id> \
  --kind feature \
  --title "My task" \
  --description "Details"
loopctl tasks watch <task-id>

# Follow an existing task; emit final state as JSON (for scripting or agents)
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

# Create a pipeline with stages authored at creation time (basic shape)
loopctl pipelines create --name "My Workflow" \
  --stages '[{"name":"plan","role":"planning","instructions":"Plan the work."},{"name":"implement","role":"implementing","instructions":"Implement the plan."}]'

# Use the new pipeline when creating a project
loopctl projects create --name "Daybreak" --platform rails --pipeline "My Workflow"
```

### Manage pipeline stages with the full attribute set

```bash
# List stages for a pipeline
loopctl pipelines stages list <pipeline-id>

# Add a minimal stage (name only required)
loopctl pipelines stages add <pipeline-id> --name review --role reviewing

# Add a stage with a failure policy and manual gate
loopctl pipelines stages add <pipeline-id> \
  --name review \
  --role reviewing \
  --instructions "Review the implementation carefully." \
  --gate manual \
  --on-failure '{"max_rework_count":2,"rework_to_position":1}'

# Update a stage's instructions and gate in place
loopctl pipelines stages update <pipeline-id> review \
  --instructions "Updated review instructions." \
  --gate ci_pass

# Remove a stage
loopctl pipelines stages remove <pipeline-id> review

# Preview any write without making API calls
loopctl pipelines stages add <pipeline-id> --name review --dry-run
loopctl pipelines stages update <pipeline-id> review --gate manual --dry-run
loopctl pipelines stages remove <pipeline-id> review --dry-run
```

### Create a custom task kind and set its default pipeline

```bash
loopctl task-kinds create --name my-kind

# Set a pipeline as the default for the kind (use kind name, not ID)
loopctl task-kinds set-default-pipeline my-kind --pipeline-id <integer-pipeline-id>

# Inspect all account-level default pipelines
loopctl task-kinds list-default-pipelines

# Clear the default pipeline for the kind (use kind name, not ID)
loopctl task-kinds clear-default-pipeline my-kind
```

### Dispatch a dependency graph at once

```bash
# Create the first task and capture its ID
TASK_A=$(loopctl tasks create \
  --project-id <project-id> --kind feature \
  --title "Task A" --description "..." --json | jq -r '.data.id')

# Create a second task that only starts after Task A completes
TASK_B=$(loopctl tasks create \
  --project-id <project-id> --kind feature \
  --title "Task B" --description "..." \
  --depends-on "$TASK_A" --json | jq -r '.data.id')

# Create a third task that depends on both A and B
loopctl tasks create \
  --project-id <project-id> --kind feature \
  --title "Task C" --description "..." \
  --depends-on "$TASK_A" \
  --depends-on "$TASK_B"
```

A dependent task is held until every dependency reaches a completed state. Repeat `--depends-on` for each dependency; the flag cannot be comma-separated.

### Preview a write without making API calls

```bash
loopctl projects create --name "Daybreak" --platform rails --dry-run
loopctl projects create --name "Daybreak" --platform-id "pf1" --dry-run
loopctl tasks create --project-id p1 --kind bug --title "Fix it" --description "..." --dry-run
loopctl pipelines create --name "My Workflow" --dry-run
loopctl pipelines update <id> --name "Renamed" --dry-run
loopctl pipelines stages add <pipeline-id> --name review --dry-run
loopctl pipelines stages update <pipeline-id> review --gate manual --dry-run
loopctl pipelines stages remove <pipeline-id> review --dry-run
loopctl task-kinds create --name my-kind --dry-run
loopctl task-kinds set-default-pipeline <kind-name> --pipeline-id <integer-pipeline-id> --dry-run
loopctl task-kinds clear-default-pipeline <kind-name> --dry-run
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

### API spec sync and contract gate

The authoritative OpenAPI spec is committed at `internal/schema/testdata/api_spec.yaml` and kept in sync with the `dotdevlabs/loopcontrol` repository. The bidirectional contract gate (`TestBidirectionalCoverage`) runs on every `go test ./...` and fails when:

- Any spec operation lacks a loopctl command **and** is not in the `Excluded` map with a reason
- Any `Covered` or `Excluded` entry references a path no longer in the spec
- A `Covered` entry claims a query parameter the spec does not document
- A paginated spec operation is not declared `Paginated: true` in the manifest

To update the local spec copy:

```bash
GITHUB_TOKEN=<token> ./scripts/sync_spec.sh          # update
GITHUB_TOKEN=<token> ./scripts/sync_spec.sh --check  # verify in sync (CI uses this)
```

CI runs `--check` automatically when `GITHUB_TOKEN` is present. Without the token the check is skipped silently so local builds never fail due to network access.

When adding a new loopctl command, update `internal/schema/coverage.go`:
- Add a `Covered` entry for each API operation the command calls
- Declare any non-pagination query parameters in `QueryParams`
- Set `Paginated: true` for commands that follow `links.next` pagination

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

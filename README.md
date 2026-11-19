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

Project JSON output includes `relationships.platform` and `relationships.agent_size`. The agent size relationship contains `data: null` when the project has no size assignment.

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

The output includes `webhook_url` (null when disabled) and
`webhook_secret_configured`. The signing secret is never returned.

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
| `--agent-size-id` | string | Agent size ID from `agent-sizes list`. Use `--agent-size-id=` to clear the assignment. |
| `--failure-policy` | string | Failure policy for tasks |
| `--fallback-agent-id` | int | Fallback agent numeric ID |
| `--webhook-url` | string | Stage-change webhook destination URL |
| `--webhook-signing-secret` | string | Signing secret; prefer stdin to keep the value out of shell history |
| `--webhook-signing-secret-stdin` | bool | Read the signing secret from standard input |
| `--disable-webhook` | bool | Disable the webhook |

At least one flag must be provided. Output columns on success: `ID`, `NAME`, `PLATFORM`, `REPO`. Use `--json` for the full resource.

**API:** `PATCH /api/projects/:id`

Configure the initial URL and secret together, reading the secret from a file:

```bash
loopctl projects update <id> --webhook-url https://receiver.example/hook \
  --webhook-signing-secret-stdin < /path/to/signing-secret
loopctl projects update <id> --disable-webhook
```

Standard input removes one trailing LF or CRLF and preserves other whitespace.
Once configured, the URL can change independently; a nonblank secret rotates
the signing key, while a blank secret preserves it. The two secret flags are
mutually exclusive. Disabling cannot be combined with webhook configuration.
`--dry-run` previews the URL, disable flag, and a redacted secret marker.

---

### agents

List and inspect LoopControl agents.

#### agents list

```bash
loopctl agents list
loopctl agents list --json
```

Output columns: `ID`, `NAME`, `SLUG`.

**API:** `GET /api/agents`

#### agents get

Get an agent by ID. The JSON output includes install and launch commands and the credential schema's environment variable, file path, and encoding.

```bash
loopctl agents get <id>
loopctl agents get <id> --json
```

Output columns: `ID`, `NAME`, `SLUG`, `INSTALL_COMMAND`, `LAUNCH_COMMAND`.

**API:** `GET /api/agents/:id`

#### agents update

Update an agent. Only the flags you pass are sent; at least one is required.

```bash
loopctl agents update <id> --launch-command "claude --run"
loopctl agents update <id> --name "Claude" --rate-limit-rpm 50 --rate-limit-tpm 40000
```

| Flag | Type | Description |
|------|------|-------------|
| `--name` | string | Agent name |
| `--description` | string | Agent description |
| `--launch-command` | string | Command that launches the agent |
| `--install-command` | string | Command that installs the agent |
| `--rate-limit-rpm` | int | Requests per minute limit |
| `--rate-limit-tpm` | int | Tokens per minute limit |

Output columns: `ID`, `NAME`, `SLUG`, `INSTALL_COMMAND`, `LAUNCH_COMMAND`. Supports `--dry-run`.

**API:** `PATCH /api/agents/:id`

---

### account

#### account credit

Credit compute minutes to the current account. Platform-admin tokens only (otherwise 403). Idempotent by `--description`: repeating a description returns the original credit unchanged (HTTP 200 instead of 201).

```bash
loopctl account credit --minutes 60
loopctl account credit --minutes 120 --bucket monthly --description "2026-11 grant"
```

| Flag | Type | Description |
|------|------|-------------|
| `--minutes` | int | Required; at least 1 |
| `--bucket` | `monthly` or `topup` | Balance bucket (server default `topup`) |
| `--description` | string | Idempotency key |

Output columns: `ID`, `MINUTES`, `BUCKET`, `DESCRIPTION`, `MONTHLY_BALANCE_AFTER`, `TOPUP_BALANCE_AFTER`, `CREATED_AT`. Supports `--dry-run`.

**API:** `POST /api/balance_credits`

---

### credentials

List and inspect credentials available to your account. Credential values are never printed.

#### credentials list

```bash
loopctl credentials list
loopctl credentials list --json
```

Output columns: `ID`, `NAME`, `SHARED`.

**API:** `GET /api/credential_sets`

#### credentials get

Get a credential by ID.

```bash
loopctl credentials get <id>
loopctl credentials get <id> --json
```

Output columns: `ID`, `NAME`, `SHARED`.

Use `--json` to retrieve the capacity status and the identifiers of affected blocked tasks under `relationships.tasks.data`. List output contains a single `data` collection.

**API:** `GET /api/credential_sets/:id`

---

### environments

Manage LoopControl environments and their entries. Environments group environment variables and credential references that are injected into project runs. Assign an environment to a project with `projects update --environment-id`.

#### environments list

List all environments.

```bash
loopctl environments list
loopctl environments list --json
```

Output columns: `ID`, `NAME`, `SHARED`, `DEFAULT`.

**API:** `GET /api/environments`

#### environments get

Get an environment by ID.

```bash
loopctl environments get <id>
loopctl environments get <id> --json
```

Output columns: `ID`, `NAME`, `SHARED`, `DEFAULT`, `DESCRIPTION`.

**API:** `GET /api/environments/:id`

#### environments create

Create a new environment.

```bash
loopctl environments create --name production
loopctl environments create --name staging --description "Staging environment"
loopctl environments create --name shared-creds --shared
loopctl environments create --name default-env --default
loopctl environments create --name production --dry-run
```

**Flags:**

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | yes | Name of the environment |
| `--description` | no | Human-readable description |
| `--shared` | no | Mark the environment as shared (accessible to all projects) |
| `--default` | no | Mark this environment as the account default |

Output columns on success: `ID`, `NAME`, `SHARED`, `DEFAULT`. Use `--json` for the full resource.

**API:** `POST /api/environments`

#### environments update

Update an existing environment. Only the flags you provide are changed — unset flags are not sent to the API.

```bash
loopctl environments update <id> --name "Renamed"
loopctl environments update <id> --description "Updated description"
loopctl environments update <id> --shared
loopctl environments update <id> --no-shared
loopctl environments update <id> --default
loopctl environments update <id> --no-default
loopctl environments update <id> --name "Renamed" --dry-run
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--name` | New name for the environment |
| `--description` | New description |
| `--shared` | Set shared to true |
| `--no-shared` | Set shared to false (`--shared` and `--no-shared` are mutually exclusive) |
| `--default` | Set default to true |
| `--no-default` | Set default to false (`--default` and `--no-default` are mutually exclusive) |

At least one flag must be provided. Output columns on success: `ID`, `NAME`, `SHARED`, `DEFAULT`. Use `--json` for the full resource.

**API:** `PATCH /api/environments/:id`

#### environments entries

Manage entries within an environment. Each entry binds either an environment variable name or a file path (with an optional `raw` or `base64` encoding) to a custom string value or a credential reference. Credential values are never printed.

##### environments entries list

List all entries for an environment.

```bash
loopctl environments entries list <environment-id>
loopctl environments entries list <environment-id> --json
```

Output columns: `ID`, `ENV_VAR`, `FILE_PATH`, `ENCODING`, `TYPE`, `CREDENTIAL_ID`, `CREDENTIAL_NAME`, `CREATED_AT`. Custom values are intentionally omitted from table output to avoid leaking secrets.

**API:** `GET /api/environments/:environment_id/entries`

##### environments entries add

Add a new entry to an environment.

```bash
# Custom entry — value via flag
loopctl environments entries add <environment-id> \
  --env-var DATABASE_URL \
  --type custom \
  --value "postgres://localhost/mydb"

# Custom entry — value read from stdin (useful for multi-line or secret values)
echo "postgres://localhost/mydb" | loopctl environments entries add <environment-id> \
  --env-var DATABASE_URL \
  --type custom

# Credential entry — references a credential set
loopctl environments entries add <environment-id> \
  --env-var GITHUB_TOKEN \
  --type credential \
  --credential-set-id <credential-set-id>

# File entry — written to a file path instead of an env var
loopctl environments entries add <environment-id> \
  --file-path /etc/app/key.pem \
  --encoding base64 \
  --type custom \
  --value "$KEY_B64"

# Preview without making API calls
loopctl environments entries add <environment-id> --env-var FOO --type custom --value bar --dry-run
```

**Flags:**

| Flag | Required | Description |
|------|----------|-------------|
| `--env-var` | one of | Environment variable name. An entry targets exactly one of `--env-var` or `--file-path`; the server rejects both (422) and its error is shown |
| `--file-path` | one of | File path target (use instead of `--env-var`) |
| `--encoding` | no | Encoding for a file target: `raw` or `base64` |
| `--type` | yes | Entry type: `custom` or `credential` |
| `--value` | no | Custom value (for `--type custom`; reads from stdin when omitted) |
| `--credential-set-id` | no | Credential set ID (for `--type credential`) |

Output columns on success: `ID`, `ENV_VAR`, `FILE_PATH`, `ENCODING`, `TYPE`, `CREDENTIAL_ID`, `CREDENTIAL_NAME`, `CREATED_AT`.

**API:** `POST /api/environments/:environment_id/entries`

##### environments entries change

Update an existing entry. Only the flags you provide are changed.

```bash
loopctl environments entries change <environment-id> <entry-id> --value "new-value"
loopctl environments entries change <environment-id> <entry-id> --env-var NEW_NAME
loopctl environments entries change <environment-id> <entry-id> --file-path /new/path --encoding raw
loopctl environments entries change <environment-id> <entry-id> --type credential --credential-set-id <id>
loopctl environments entries change <environment-id> <entry-id> --value "new-value" --dry-run
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--env-var` | New environment variable name |
| `--file-path` | New file path target |
| `--encoding` | New encoding: `raw` or `base64` |
| `--type` | New entry type: `custom` or `credential` |
| `--value` | New custom value |
| `--credential-set-id` | New credential set ID |

At least one flag must be provided. Output columns on success: `ID`, `ENV_VAR`, `FILE_PATH`, `ENCODING`, `TYPE`, `CREDENTIAL_ID`, `CREDENTIAL_NAME`, `CREATED_AT`.

**API:** `PATCH /api/environments/:environment_id/entries/:id`

##### environments entries remove

Remove an entry from an environment.

```bash
loopctl environments entries remove <environment-id> <entry-id>
loopctl environments entries remove <environment-id> <entry-id> --dry-run
```

**API:** `DELETE /api/environments/:environment_id/entries/:id`

---

### agent-sizes

`loopctl agent-sizes list` lists all available agent sizes and follows pagination automatically. Use `--json` to retrieve their IDs, multipliers, and CPU and memory limits. A null limit uses the limit derived from the multiplier.

```bash
loopctl agent-sizes list --json
loopctl projects update <project-id> --agent-size-id <size-id>
```

**API:** `GET /api/agent_sizes`

### platforms

List and inspect available platforms. Use this to discover the platform names and IDs accepted by `projects create`.

#### platforms list

```bash
loopctl platforms list
loopctl platforms list --json
```

Output columns: `ID`, `NAME`, `DISPLAY_NAME`, `CONTAINER_IMAGE`, `NEEDS_DATABASE`, `PORT`, `BUILT_IN`.

`NEEDS_DATABASE` and `PORT` are blank when the platform does not specify them. Use `--json` to retrieve the full resource including non-secret environment variables and required secret descriptors (env var name, secret name, key, optional flag) — never secret values.

**API:** `GET /api/platforms`

#### platforms get

Get a platform by ID.

```bash
loopctl platforms get <id>
loopctl platforms get <id> --json
```

Output columns: `ID`, `NAME`, `DISPLAY_NAME`, `CONTAINER_IMAGE`, `NEEDS_DATABASE`, `PORT`, `BUILT_IN`.

Use `--json` to retrieve the full resource including non-secret environment variables and required secret descriptors — never secret values.

**API:** `GET /api/platforms/:id`

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

Manage the individual stages of a pipeline. `stages add` and `stages remove` read the pipeline's stage relationship and included resources before submitting the revised workflow with a PATCH. `stages update` addresses a stage directly by its `id` via the single-stage PATCH endpoint.

> **Note:** Two concurrent `stages add` (or `stages remove`) calls can overwrite each other — this is inherent to the read-modify-write design (no conditional PATCH). For safe concurrent edits, serialize your calls.

##### pipelines stages list

List the stages of a pipeline.

```bash
loopctl pipelines stages list <pipeline-id>
loopctl pipelines stages list <pipeline-id> --json
```

The table shows `ID`, `POSITION`, `NAME`, `ROLE`, `STAGE_TYPE`, `GATE`, `AGENT`, and `APPROVAL_BUTTON_TEXT`. JSON output includes `attributes.rework_allowed_positions`.

**API:** `GET /api/pipelines/:id`

##### pipelines stages add

Append a new stage to a pipeline. Only the flags you provide are included — unset optional flags are not sent.

```bash
# Minimal — just a name
loopctl pipelines stages add <pipeline-id> --name implement

# With role and instructions
loopctl pipelines stages add <pipeline-id> \
  --custom-stage-name implement \
  --instructions "Implement the approved plan."

# Full attribute set
loopctl pipelines stages add <pipeline-id> \
  --custom-stage-name review \
  --instructions "Review the implementation." \
  --gate human \
  --advance-notice "30m" \
  --position 3 \
  --runs-in-container \
  --on-failure rework \
  --max-rework-count 2 \
  --rework-to-position 1 \
  --rework-allowed-positions 0,1

# Preview without making API calls
loopctl pipelines stages add <pipeline-id> --name review --dry-run
```

**Flags:**

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | no | Legacy stage name. Prefer `--custom-stage-name`. |
| `--role` | no | Legacy stage role. Prefer `--stage-type`. |
| `--stage-type` | no | `stage_type` for custom stage types |
| `--custom-stage-name` | no | `custom_stage_name` (custom type only) |
| `--instructions` | no | Full agent prompt text for this stage |
| `--gate` | no | Advance gate, `automated` or `human` |
| `--advance-notice` | no | Advance notice value |
| `--approval-button-text` | no | Human approval button label. Clear with `--approval-button-text=`. |
| `--position` | no | Explicit position override |
| `--runs-in-container` | no | Override `runs_in_container` for this stage |
| `--on-failure` | no | Failure mode, `rework` or `block` |
| `--max-rework-count` | no | Maximum number of rework iterations |
| `--rework-to-position` | no | Default position for rework |
| `--rework-allowed-positions` | no | Comma-separated earlier positions available for rework. Pass an empty value to allow only the default position. |
| `--prompt-sections` | no | JSON object of prompt section overrides |
| `--stage-triggers` | no | JSON array of stage trigger objects |
| `--advance-requirements` | no | JSON array of requirement strings |

Output columns on success are `ID`, `NAME`, and `KIND`. Use `pipelines stages list --json` with the pipeline ID to retrieve the resulting stages.

**APIs:** `GET /api/pipelines/:id` then `PATCH /api/pipelines/:id`

##### pipelines stages update

Update an existing stage by its `id`. Only the flags you provide are changed — other fields are preserved on the server. Use `pipelines stages list <pipeline-id>` to discover stage IDs.

```bash
# Change instructions
loopctl pipelines stages update <pipeline-id> <stage-id> \
  --instructions "Updated prompt text."

# Change gate and add failure policy
loopctl pipelines stages update <pipeline-id> <stage-id> \
  --gate automated \
  --on-failure rework \
  --max-rework-count 3 \
  --rework-allowed-positions 0,1

# Preview without making API calls
loopctl pipelines stages update <pipeline-id> <stage-id> --gate human --dry-run
```

Accepts the same flags as `stages add` except `--name` (the stage is identified by the `<stage-id>` positional argument). At least one flag must be provided.

Use `--approval-button-text "Approve release"` to set the button label and
`--approval-button-text=` to restore the default. The stages list shows the label
under `APPROVAL_BUTTON_TEXT` or as `attributes.approval_button_text` in JSON.
When creating or updating a pipeline with `--stages`, include
`"approval_button_text":"Approve release"` in the stage object or use an empty
string to clear it. Add `--dry-run` to preview the encoded value.

The update response uses the same columns as the stages list. Use `--json` for the full stage resource.

**API:** `PATCH /api/pipelines/:pipeline_id/stages/:id`

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

Output columns: `ID`, `NAME`, `BUILT_IN`, `CREATE_PR`.

**API:** `GET /api/task_kinds`

#### task-kinds create

Create a new custom task kind for your account.

```bash
loopctl task-kinds create --name my-kind
loopctl task-kinds create --name my-kind --create-pull-request
loopctl task-kinds create --name my-kind --dry-run
```

**Flags:**

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | yes | Slug/identifier for the task kind |
| `--create-pull-request` | no | Whether tasks of this kind open a branch and pull request (default: false) |

Output columns on success: `ID`, `NAME`.

Attempting to create a built-in task kind surfaces the API's human-readable error.

**API:** `POST /api/task_kinds`

#### task-kinds update

Update an existing task kind's name or pull request setting. Only the flags you provide are changed — unset flags are not sent to the API.

```bash
loopctl task-kinds update <id> --name "new-name"
loopctl task-kinds update <id> --create-pull-request
loopctl task-kinds update <id> --no-create-pull-request
loopctl task-kinds update <id> --name "new-name" --dry-run
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--name` | New name for the task kind |
| `--create-pull-request` | Set create_pull_request to true |
| `--no-create-pull-request` | Set create_pull_request to false (`--create-pull-request` and `--no-create-pull-request` are mutually exclusive) |

At least one flag must be provided. Output columns on success: `ID`, `NAME`, `CREATE_PR`. Use `--json` for the full resource.

**API:** `PATCH /api/task_kinds/:id`

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

List tasks across the entire account, or filter to a single project. Output columns: `ID`, `KIND`, `TITLE`, `STAGE`, `STATUS`, `BLOCKED_REASON`, `BILLABLE_MINS`, `MINS_FINAL`, `COMPUTE_COST`, `COST_FINAL`. Pagination is followed automatically; when `--project-id` is set, the filter is preserved across all pages. `--json` and template output include the full index representation (all `TaskListAttributes` fields).

`BILLABLE_MINS` is the cumulative billable compute minutes accrued by the task (integer; 0 while no billable time has accrued). `MINS_FINAL` is `true` once the task is in a terminal state and all billing periods are closed.

`COMPUTE_COST` shows the task's compute cost as `<amount_cents> <currency>` (e.g. `250 usd`). `COST_FINAL` is `true` once the cost will not change (task is terminal and all billing periods are closed). Both columns are blank when no cost has been reported yet.

```bash
# List all account tasks (global index)
loopctl tasks list --json

# Filter to one project
loopctl tasks list --project-id <project-id>
loopctl tasks list --project-id <project-id> --json
```

**Flags:**

| Flag | Required | Description |
|------|----------|-------------|
| `--project-id` | no | Project ID to filter tasks (omit for account-wide listing) |

**API:** `GET /api/tasks` (global) or `GET /api/tasks?project_id=<project-id>` (filtered)

#### tasks get

Get a task by ID. Output columns: `ID`, `KIND`, `TITLE`, `STAGE`, `STATUS`, `BLOCKED_REASON`, `DEPENDENCIES_MET`, `BILLABLE_MINS`, `MINS_FINAL`, `COMPUTE_COST`, `COST_FINAL`.

`BILLABLE_MINS` is the cumulative billable compute minutes accrued by the task (integer; 0 while no billable time has accrued). `MINS_FINAL` is `true` once the task is in a terminal state and all billing periods are closed.

`COMPUTE_COST` shows the task's compute cost as `<amount_cents> <currency>` (e.g. `250 usd`). `COST_FINAL` is `true` once the cost will not change (task is terminal and all billing periods are closed). Both columns are blank when no cost has been reported yet.

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

When `--watch` is set, the command first prints the created task row, then streams stage and activity changes until the task reaches a terminal state — identical behavior to `tasks watch`. Exit codes match those of `tasks watch`. The same retry and timeout rules described below apply to watching; retries do not create the task again.

**API:** `POST /api/tasks` (followed by `GET /api/tasks/:id` and `GET /api/tasks/:id/activities` when `--watch` is set)

#### tasks update

Update an existing task's criteria fields or dependency list. Only the flags you provide are changed. Output columns: `ID`, `KIND`, `TITLE`, `STAGE`, `STATUS`.

```bash
loopctl tasks update <id> --implementation-criteria "Step-by-step plan..."
loopctl tasks update <id> --verification-criteria "Run these checks..."
loopctl tasks update <id> \
  --implementation-criteria "Step-by-step plan..." \
  --verification-criteria "Run these checks..."
loopctl tasks update <id> --depends-on <dep-id>
loopctl tasks update <id> --depends-on <dep1> --depends-on <dep2>
loopctl tasks update <id> --remove-depends-on <dep-id>
loopctl tasks update <id> --clear-depends-on
loopctl tasks update <id> --depends-on <dep-id> --dry-run
```

**Flags:**

| Flag | Required | Description |
|------|----------|-------------|
| `--implementation-criteria` | no | Implementation plan written by the planning agent |
| `--verification-criteria` | no | Verification steps for the implementing agent |
| `--depends-on` | no | Complete list of dependency task IDs; repeat to set multiple (replaces existing list) |
| `--remove-depends-on` | no | Task ID to remove from dependencies; repeat to remove multiple |
| `--clear-depends-on` | no | Remove all dependencies |

`--depends-on` is mutually exclusive with `--remove-depends-on` and `--clear-depends-on`.
`--remove-depends-on` and `--clear-depends-on` are mutually exclusive with each other.

At least one flag must be provided.

**API:** `PATCH /api/tasks/:id` (preceded by `GET /api/tasks/:id` when `--remove-depends-on` is used)

#### tasks cancel

Cancel an in-progress task by ID. On success, prints `task <id>: status <status>, stage <stage>`. With `--json`, emits `{"id":"...","status":"rejected","stage":"..."}` for a task the API reports as rejected. Both modes use the status and stage returned by the API. Attempting to cancel an already-finished task exits with the API's error.

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

Follow server task state, streaming stage and activity changes. Exits successfully when `terminal` is true and `status` is `completed`; exits non-zero for terminal rejection, unexpected terminal status, blocked status (including its reason), or timeout. Stage names never determine completion. Human approval waits are reported once per waiting episode, with the stage and PR URL when available; watching continues through approval and later stages. With `--json`, progress goes to stderr and stdout contains one final JSON document. This also applies to `tasks create --watch`. Activity that already exists when the watch starts is not reported. A new `container.error` activity is printed with its detail but does not end the watch, because LoopControl retries the container.

```bash
loopctl tasks watch <id>
loopctl tasks watch <id> --interval 30s --timeout 60m
loopctl tasks watch <id> --json
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--interval` | `15s` | Positive poll interval (e.g. `15s`, `1m`); also caps retry delays |
| `--timeout` | _(none)_ | Positive duration limiting the entire watch (e.g. `30m`), including requests and retry waits; empty means no timeout |
| `--json` | — | Emit final task state as a JSON:API resource on exit (`{"id":"...","type":"tasks","attributes":{...}}`) |

Temporary failures while fetching the task or its activities (including activity pagination) are retried: connection errors, request timeouts, EOF or unexpected EOF, and HTTP 408, 429, and 500–599. Consecutive failures wait 1 second initially (or the poll interval if shorter), then double the delay up to `--interval`. A fully successful poll resets the backoff. There is no retry count limit; `--timeout` starts before the first watch request and bounds requests and waits. Interrupting the command also stops the watch.

Each scheduled retry prints its error and delay to stderr, including with `--json`; retry diagnostics never appear on JSON stdout. A watch timeout emits `{"error":"timeout"}` on JSON stdout and exits non-zero. Other HTTP 4xx responses end the watch immediately with the API error body/detail. Permanent local errors, such as invalid configuration or malformed complete JSON responses, also end the watch.

**Exit codes:**

- `0` — server reports `terminal: true` and `status: completed`
- non-zero — task `rejected`, task `blocked` (with its reason), timeout exceeded, cancellation, or a non-retryable error

`container.error` activities never change the exit code on their own.

**APIs:** `GET /api/tasks/:id`, `GET /api/tasks/:id/activities`

---

#### tasks comments list

List comments on a task. Pagination is followed automatically.

```bash
loopctl tasks comments list <task-id>
loopctl tasks comments list <task-id> --json
```

Output columns: `ID`, `BODY` (truncated to 60 characters), `COMMENT_TYPE`, `CREATED_AT`.

**API:** `GET /api/tasks/:task_id/comments`

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
| `--input-tokens` | no | Input token count for billing, sent alongside the comment object |
| `--output-tokens` | no | Output token count for billing, sent alongside the comment object |

**API:** `POST /api/tasks/:id/comments`

---

#### tasks transitions list

List the stage transitions for a task. Pagination is followed automatically.

```bash
loopctl tasks transitions list <task-id>
loopctl tasks transitions list <task-id> --json
```

Output columns: `ID`, `FROM_STAGE`, `TO_STAGE`, `CUSTOM_STAGE_NAME`, `CREATED_AT`.

**API:** `GET /api/tasks/:task_id/transitions`

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

#### tasks containers

List the containers that ran a task. Pagination is followed automatically.

```bash
loopctl tasks containers <task-id>
loopctl tasks containers <task-id> --json
```

Output columns: `ID`, `UID`, `STATUS`, `AGENT_STATUS`, `STATUS_MESSAGE`, `UPDATED_AT`.

**API:** `GET /api/tasks/:task_id/containers`

#### tasks recordings list

List all recordings attached to a task. Pagination is followed automatically.

```bash
loopctl tasks recordings list <task-id>
loopctl tasks recordings list <task-id> --json
```

**API:** `GET /api/tasks/:task_id/recordings`

---

#### tasks recordings get

Inspect the metadata of a single recording.

```bash
loopctl tasks recordings get <task-id> <recording-id>
loopctl tasks recordings get <task-id> <recording-id> --json
```

**API:** `GET /api/tasks/:task_id/recordings/:id`

---

#### tasks recordings content

Retrieve the raw binary content of a recording (JSONL transcript or asciinema cast). Writes exact bytes to stdout or to an explicit output file; no truncation, re-encoding, or status text is mixed into stdout. Diagnostics go to stderr.

```bash
# Stream raw content to stdout
loopctl tasks recordings content <task-id> <recording-id>

# Save to a file (errors on existing file — no silent overwrite)
loopctl tasks recordings content <task-id> <recording-id> --output session.cast
loopctl tasks recordings content <task-id> <recording-id> --output transcript.jsonl

# Preview the URL without downloading
loopctl tasks recordings content <task-id> <recording-id> --dry-run
```

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--output` | `-o` | Write content to this file path instead of stdout. Errors if the file already exists. |

**API:** `GET /api/tasks/:task_id/recordings/:recording_id/content`

---

### topup

Fund your LoopControl account. `loopctl topup` reads the billing API's HTTP 402 funding menu. It uses the first available package that matches the requested product kind or the exact package ID you supply. The command settles payment with a configured wallet or prints a hosted checkout URL.

```bash
# Print the checkout link for the default top-up package (no wallet configured)
loopctl topup

# Choose a specific product
loopctl topup --product trial
loopctl topup --product subscription
loopctl topup --product 50h

# Machine-stable JSON output (for scripting or agents)
loopctl topup --json
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--product` | `topup` | Package ID or a product kind such as `trial`, `topup`, or `subscription` |

#### Payment rails

`loopctl topup` supports three payment rails, selected by what the 402 response advertises intersected with what the local wallet can satisfy:

| Rail | Requires | Behavior |
|------|----------|----------|
| `x402` | `LOOPCTL_ARBITRUM_PRIVATE_KEY` | Signs and submits an exact USDC payment on Arbitrum (EIP-3009), then waits for settlement |
| `l402` | `LOOPCTL_LN_HOST` + `LOOPCTL_LN_MACAROON_HEX` | Pays the Lightning invoice via the configured LND node, submits the payment proof, and waits for settlement |
| `human_link` | _(none)_ | Prints the Stripe checkout URL for manual payment in a browser (default fallback) |

Rails are tried in preference order: `x402` → `l402` → `human_link`. The first configured rail that the 402 response also advertises wins.

#### Wallet configuration

Wallet credentials are supplied via environment variables — never stored in the config file.

**x402 (Arbitrum USDC):**

| Variable | Description |
|----------|-------------|
| `LOOPCTL_ARBITRUM_PRIVATE_KEY` | 64-char hex secp256k1 private key (no `0x` prefix) |

**L402 (Lightning via LND REST):**

Loopctl submits the Lightning proof in `Authorization` and authenticates the payment submission with the active account token in `X-Api-Token`.

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
loopctl topup   # pays the Lightning invoice and waits for settlement
```

#### JSON output

With `--json`, the command obtains a hosted checkout URL and returns a stable object:

```json
{"product": "topup", "rail": "human_link", "url": "https://checkout.stripe.com/..."}
```

**API:** `GET /api/billing/funding_envelope`, the selected `POST /api/billing/topups`, `POST /api/billing/x402_payments`, or `POST /api/billing/l402_payments`, and `GET /api/billing/pending_purchases/:id` for checkout or settlement.

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

In the JSON reference each flag may carry `enum` (the fixed set of accepted values, e.g. `--status`, `--gate`, `--on-failure`, entry `--type`, `--product`) and `schema` (a JSON Schema for flags whose value is JSON, e.g. `--stages`, `--prompt-sections`, `--stage-triggers`, `--advance-requirements`, `--bulk-json`), plus `annotations`. Open-ended flags such as `tasks create --kind` declare no enum. Enums and schemas are read from the embedded API spec copy (`internal/schema/testdata/api_spec.yaml`), never typed by hand; tests fail on any difference.

The built-in server address is `https://loopcontrol.ai`, so with no context selected `LOOPCTL_TOKEN` alone is enough to run commands. Set `LOOPCTL_BASE_URL` to override the address (a context's own `base_url` takes precedence).

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
| `--url` | no | `LOOPCTL_BASE_URL`, else `"https://loopcontrol.ai"` | LoopControl API base URL |
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

### List tasks as JSON (for scripting or agents)

```bash
# All account tasks — one call, all pages aggregated automatically
loopctl tasks list --json

# Filter to a single project
loopctl tasks list --project-id <project-id> --json
```

### Discover available agents, platforms, pipelines, task kinds, and credentials

```bash
loopctl agents list
loopctl credentials list
loopctl platforms list
loopctl pipelines list
loopctl task-kinds list
```

### Create an environment and assign it to a project

```bash
# Create an environment with a custom entry
loopctl environments create --name production

# Add a custom value entry
loopctl environments entries add <environment-id> \
  --env-var DATABASE_URL \
  --type custom \
  --value "postgres://localhost/prod"

# List available credentials, then add a credential entry (value never printed)
loopctl credentials list
loopctl environments entries add <environment-id> \
  --env-var GITHUB_TOKEN \
  --type credential \
  --credential-set-id <credential-set-id>

# Assign the environment to a project
loopctl projects update <project-id> --environment-id <environment-id>

# Inspect the environment's entries
loopctl environments entries list <environment-id>
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

# Update a stage's instructions and gate in place (use stage id, not name)
loopctl pipelines stages update <pipeline-id> <stage-id> \
  --instructions "Updated review instructions." \
  --gate ci_pass

# Remove a stage
loopctl pipelines stages remove <pipeline-id> review

# Preview any write without making API calls
loopctl pipelines stages add <pipeline-id> --name review --dry-run
loopctl pipelines stages update <pipeline-id> <stage-id> --gate manual --dry-run
loopctl pipelines stages remove <pipeline-id> review --dry-run
```

### Create a custom task kind and set its default pipeline

```bash
loopctl task-kinds create --name my-kind

# Create a kind that opens a branch and pull request for each task
loopctl task-kinds create --name my-kind --create-pull-request

# Update whether an existing kind opens a pull request
loopctl task-kinds update <id> --create-pull-request
loopctl task-kinds update <id> --no-create-pull-request

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
loopctl pipelines stages update <pipeline-id> <stage-id> --gate manual --dry-run
loopctl pipelines stages remove <pipeline-id> review --dry-run
loopctl task-kinds create --name my-kind --dry-run
loopctl task-kinds update <id> --name "new-name" --dry-run
loopctl task-kinds set-default-pipeline <kind-name> --pipeline-id <integer-pipeline-id> --dry-run
loopctl task-kinds clear-default-pipeline <kind-name> --dry-run
loopctl environments create --name production --dry-run
loopctl environments update <id> --name "Renamed" --dry-run
loopctl environments entries add <environment-id> --env-var FOO --type custom --value bar --dry-run
loopctl environments entries change <environment-id> <entry-id> --value "new-value" --dry-run
loopctl environments entries remove <environment-id> <entry-id> --dry-run
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

`TestCommandResponseContracts` executes covered commands against responses built from the embedded OpenAPI specification. It checks actual decoder projections, followed links, and command output. Add an invocation and its HTTP steps to the matrix when introducing or changing a response reader.

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

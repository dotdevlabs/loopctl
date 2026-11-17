# Pipeline stages

## Create and update stages

Use `custom_stage_name` and `prompt_sections.template` when authoring stages:

```bash
loopctl pipelines create --name "Delivery" \
  --stages '[{"custom_stage_name":"plan","prompt_sections":{"template":"Plan the work."}}]'
loopctl pipelines update <pipeline-id> \
  --stages '[{"custom_stage_name":"plan","prompt_sections":{"template":"Plan the work."}}]'
```

`--stages` accepts a JSON array of objects and preserves supplied fields. On
update, it replaces the pipeline's stages. Omitted stage types and defaults are
left to the server; set `stage_type` explicitly when needed using a server stage
type name.

To append a stage without replacing existing stages, use `stages add`. A legacy
`--name` is not required:

```bash
loopctl pipelines stages add <pipeline-id> \
  --custom-stage-name review \
  --prompt-sections '{"template":"Review the changes."}'
loopctl pipelines stages update <pipeline-id> <stage-id> \
  --prompt-sections '{"template":"Review the changes and verification evidence."}'
```

Use `--stage-type` to specify a server stage type name. Stage updates send only
the fields whose flags you supply. The server validates stage requirements.

The legacy fields `name`, `role`, and `instructions` are deprecated in favor of
`custom_stage_name`, `stage_type`, and `prompt_sections.template`. The retained
`--role` flag warns: `Deprecated: use stage_type: 'custom'.` Use
`--custom-stage-name` and `--prompt-sections` for new stage definitions.

Set the task page's human approval button label with `--approval-button-text
"Approve release"` on `stages add` or `stages update`. Labels can be up to 64
characters. Pass `--approval-button-text=` to restore the stage default.

When creating or updating a pipeline with `--stages`, set
`"approval_button_text":"Approve release"` in the stage object. An empty string
or `null` uses the stage default. Use `--dry-run` to see the request JSON. For
`stages add`, the preview shows only the stage being appended.

## Inspect stages

List a pipeline's workflow stages:

```bash
loopctl pipelines stages list <pipeline-id>
```

Stages appear once each, in ascending numeric `position` order. Stages with the
same position retain their order from the pipeline's stages relationship.

The table columns are `ID`, `POSITION`, `NAME`, `ROLE`, `STAGE_TYPE`, `GATE`,
`AGENT`, and `APPROVAL_BUTTON_TEXT`. `AGENT` contains the stage's `agent_id`.
Nullable names, agent IDs, and approval button labels appear as blank cells.
Use the stage's `ID` when updating it:

```bash
loopctl pipelines stages update <pipeline-id> <stage-id> --gate human
```

The update response uses the same table columns, including the stage ID and
agent ID.

## JSON output

```bash
loopctl pipelines stages list <pipeline-id> --json
```

JSON output is an ordered array of stage resources. Each element contains a
string `id`, `type` set to `pipeline_stages`, and an `attributes` object with the
published stage attributes:

- `stage_type`, `custom_stage_name`, `name`, `role`, `instructions`, `position`
- `gate`, `approval_button_text`, `on_failure`, `max_rework_count`, `rework_to_position`
- `agent_id`, `environment_id`, `runs_in_container_override`
- `advance_requirements`, `advance_notice`
- `prompt_sections`, `prompt_configuration`, `stage_triggers`

`attributes.approval_button_text` is `null` when the API omits the label or
returns `null`.

Nested prompt values are preserved in full, as are nullable values, zero,
`false`, and empty collections. Scripts consuming this command should read the
stage array directly; the output no longer contains the parent pipeline resource.
A pipeline with an empty stages relationship produces no table rows and JSON
`[]`.

## API and errors

Listing makes one `GET /api/pipelines/{id}` request. It selects the
`pipeline_stages` resources identified by the pipeline's `stages` relationship
from the response's top-level `included` resources. Unrelated included resources
are excluded.

A missing or malformed stages relationship, a missing referenced stage, or stage
attributes that cannot be decoded causes the command to fail with an error.
API failures, including a missing pipeline, are also returned as errors.

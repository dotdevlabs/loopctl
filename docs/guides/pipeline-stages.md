# Inspect pipeline stages

List a pipeline's workflow stages:

```bash
loopctl pipelines stages list <pipeline-id>
```

Stages appear once each, in ascending numeric `position` order. Stages with the
same position retain their order from the pipeline's stages relationship.

The table columns are `ID`, `POSITION`, `NAME`, `ROLE`, `STAGE_TYPE`, `GATE`, and
`AGENT`. `AGENT` contains the stage's `agent_id`. Nullable names and agent IDs
appear as blank cells. Use the stage's `ID` when updating it:

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
- `gate`, `on_failure`, `max_rework_count`, `rework_to_position`
- `agent_id`, `environment_id`, `runs_in_container_override`
- `advance_requirements`, `advance_notice`
- `prompt_sections`, `prompt_configuration`, `stage_triggers`

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

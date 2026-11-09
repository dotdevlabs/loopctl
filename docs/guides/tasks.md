# Task cancellation and todos

## Cancel a task

```bash
loopctl tasks cancel <task-id>
loopctl tasks cancel <task-id> --json
```

Cancellation reports the status and stage returned by the server. Human output
has the form `task <task-id>: status <status>, stage <stage>`. JSON output contains
`id`, `status`, and `stage`; scripts should use the returned values rather than
assume a particular cancellation status.

If the response is empty or either `status` or `stage` is missing, null, or not
a string, the command fails with a cancellation response contract error instead
of reporting success. This error describes the response; it does not establish
whether the server applied the cancellation.

## Create and update todos

```bash
loopctl tasks todos create <task-id> --content "Review the changes"
loopctl tasks todos create <task-id> --content "Review the changes" --status pending
loopctl tasks todos update <task-id> <todo-id> --status completed
```

When `--status` is omitted from single-todo creation, the request omits status
and the server applies its default. An explicit `--status` is sent as supplied.
Bulk creation with `--bulk-json` preserves the statuses supplied in each todo
object.

Status flag metadata comes from the embedded API specification. Use
`loopctl tasks todos create --help` or `loopctl tasks todos update --help` to
inspect the command interface; the CLI does not maintain a separate status list
or creation default in its flag help.

# Working with tasks

## Watch a task

```bash
loopctl tasks watch <task-id> --interval 15s --timeout 60m
loopctl tasks watch <task-id> --json
```

Watching follows the server's task state, including custom pipeline stages and
automated merging. It exits successfully only when `terminal` is `true` and
`status` is `completed`. A terminal rejection, unexpected terminal status, or
`blocked` status exits non-zero; blocked tasks include the server's blocked
reason when available. Stage names such as `reviewed` do not end the watch.

When `awaiting_human_gate` becomes true, the watcher reports that the task is
waiting for approval at `awaiting_human_gate_stage`, including the PR URL when
available. It reports each waiting episode once and continues through approval
and later stages. Use `--timeout` to limit the watch; without it, there is no
timeout. A timeout exits non-zero.

With `--json`, approval notices go to stderr and stdout contains one final JSON
document for a terminal or blocked task. A timeout produces
`{"error":"timeout"}` on stdout. These rules also apply when adding `--watch`
to `loopctl tasks create`; JSON mode emits only the final result rather than a
separate creation document.

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

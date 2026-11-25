# Working with tasks

## Stage versions

`loopctl tasks get <task-id>` and `loopctl tasks list` display `STAGE_VERSION`.
With `--json`, task attributes include numeric `stage_version`, including zero.
This is the same stage-change version carried by webhook messages, allowing
consumers to compare a task read with a received event.
The version increases with each stage change. See
[Project stage-change webhooks](projects.md) to configure delivery.

## API error messages

When an API failure returns JSON:API error objects, loopctl displays each
object's `detail`, falling back to its `title` when no detail is supplied.
Multiple messages appear in server order, separated by `; `.

## Download recording content

```bash
loopctl tasks recordings content <task-id> <recording-id>
loopctl tasks recordings content <task-id> <recording-id> --output recording.txt
```

Content is written as raw bytes to stdout, or to the file named by `--output`.
An existing output file is refused. If the download fails, the command exits
non-zero and removes its temporary output file.

Recording failures display the server's error detail. A 404 can report either
`Recording content not found` or
`Recording content has changed in storage since it was uploaded`. A 410 reports
`Recording file has been deleted`; a 422 reports
`Recording type does not support content`.

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

Run the displayed `loopctl tasks approve <task-id>` command from another terminal
using a user-owned API token. Successful approval prints `task <task-id> approved`.
Add `--json` to get the task resource with `awaiting_human_gate` set to false and
`awaiting_human_gate_stage` set to null. Add `--dry-run` to preview the request
without sending it. The pipeline may advance after the command exits.

With `--json`, approval notices go to stderr and stdout contains one final JSON
document for a terminal or blocked task. A timeout produces
`{"error":"timeout"}` on stdout. These rules also apply when adding `--watch`
to `loopctl tasks create`; JSON mode emits only the final result rather than a
separate creation document.

## Request rework with a comment

Start the comment body with the case-sensitive `/rework` token, followed by a
space and a description of the requested change.

```bash
loopctl tasks comments create <task-id> --body "/rework <description of the change>"
```

- At a human gate, the task goes back to an allowed rework stage. If there is
  no permitted destination, it stays paused.
- While an agent is working, the server stops the session and sends the task
  back for rework. If there is no rework destination, the current stage restarts.
- For pending, blocked, completed, or rejected tasks, the server stores the
  comment and leaves the task in its current state.
- A bare `/rework` returns a validation error and creates no comment.

Post an ordinary comment to give the working agent guidance without starting
rework. See `loopctl tasks comments create --help` for the command reference.

## Cancel a task

```bash
loopctl tasks cancel <task-id>
loopctl tasks cancel <task-id> --json
```

Cancellation reports the status and stage returned by the server. Human output
has the form `task <task-id>: status <status>, stage <stage>`. JSON output contains
`id`, `status`, and `stage`; scripts should use the returned values rather than
assume a particular cancellation status.

The command reads `status` and `stage` from `data.attributes` in the JSON:API
response. Both fields must be strings. An invalid response causes the command
to fail. Use `loopctl tasks get <task-id>` to verify the task's state afterward.

If the API refuses a cancellation, for example because the task is already
rejected, the command exits with an error and displays the server's message.

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

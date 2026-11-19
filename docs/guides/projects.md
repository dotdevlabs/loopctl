# Projects

## Assign an agent size

List available sizes to find the ID, multiplier, and CPU and memory limits.
Then assign a size to the project or clear its assignment.

```bash
loopctl agent-sizes list
loopctl projects update <project-id> --agent-size-id <size-id>
loopctl projects update <project-id> --agent-size-id=
```

Run `loopctl projects get <project-id> --json` to check the assignment under
`relationships.agent_size.data`. A value of `null` means the project has no
size assigned. The platform ID is under `relationships.platform.data.id`.

## Configure a webhook

Set the destination URL and signing secret together when first enabling a
project's stage-change webhook. Read the secret from a file through standard
input to keep its value out of shell history:

```bash
loopctl projects update <project-id> --webhook-url https://receiver.example/hook \
  --webhook-signing-secret-stdin < /path/to/signing-secret
```

The URL must use HTTP(S), contain no credentials or fragment, and resolve only
to public addresses.

The CLI removes one trailing LF or CRLF from standard input and preserves all
other whitespace. Alternatively, `--webhook-signing-secret` accepts the secret
directly; it cannot be combined with `--webhook-signing-secret-stdin`.

Once configured, change the URL independently with `--webhook-url`, or rotate
the secret with `--webhook-signing-secret-stdin`. A blank secret update preserves
the existing secret. Add `--dry-run` to preview the update without sending it;
the preview replaces the secret with `[REDACTED]`. The CLI also redacts the secret
from verbose response output and API errors.

## Inspect or disable a webhook

```bash
loopctl projects get <project-id>
loopctl projects get <project-id> --json
loopctl projects update <project-id> --disable-webhook
```

Project reads show `WEBHOOK_URL` and `WEBHOOK_SECRET_CONFIGURED` in the table,
or `webhook_url` and `webhook_secret_configured` in JSON attributes. They do not
return the signing secret. When disabled, the URL is null in JSON and the
configured flag is false.

Disabling clears the URL and secret and stops future events. It cannot be
combined with URL or secret configuration flags. To enable the webhook again,
set both URL and secret together.

Already queued events retain their destination and signature after configuration
changes. Keep previous verification secrets until pending events finish after
rotation.

Webhook messages carry the task's [stage version](tasks.md#stage-versions),
which is also available through task reads.

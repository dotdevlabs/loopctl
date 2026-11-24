# View account compute usage

Run `loopctl usage` to see billable compute for the account selected by your active context and API token. The default report shows the effective period, total hours and minutes, compute cost, overage minutes and charge, current balances, and minutes by project. The monthly and top-up balances reflect the current account even when you request a past period.

```bash
loopctl usage
```

The default period starts at the current billing cycle start and runs through now. If the account has no billing cycle, the default covers the trailing 30 days. The report labels the period `billing_cycle`, `trailing_30_days`, or `requested_period` and shows the billing cycle end when available. The project table labels usage without a project as `Unattributed`.

## Choose a period

`--from` includes the start and `--to` excludes the end. Supply `--from` whenever you use `--to`. With `--from` alone, the period runs through now. Both flags accept dates or ISO 8601 date-times. Dates start at midnight UTC, and date-times without an offset use UTC.

```bash
loopctl usage --from 2026-11-01 --to 2026-12-01
loopctl usage --from 2026-11-01T09:00:00+02:00
```

Add `--daily` to see minutes for each UTC day in the window, including days with zero usage.

```bash
loopctl usage --from 2026-11-01 --to 2026-12-01 --daily
```

If the API rejects a period, `loopctl usage` prints the API's error message to standard error and exits with a nonzero status.

## Use the report in a script

`--json` prints the API document, including the project and daily breakdowns. `--daily` controls the table display. Use `--format` to select fields with a Go template. If you supply both `--json` and `--format`, `--json` takes precedence.

```bash
loopctl usage --json
loopctl usage --format '{{.Data.Attributes.BillableComputeMinutes}}'
```

# View account compute usage

Run `loopctl usage` to see billable compute for the account selected by your active context and API token. The default report shows the effective period, total hours and minutes, compute cost, overage minutes and charge, current balances, and minutes by project. The monthly and top-up balances reflect the current account even when you request a past period.

```bash
loopctl usage
```

The default period starts at the current billing cycle start and runs through now. If the account has no billing cycle, the default covers the trailing 30 days.

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

## Use the report in a script

`--json` prints the API document, including the project and daily breakdowns. Use `--format` to select fields with a Go template.

```bash
loopctl usage --json
loopctl usage --format '{{.Data.Attributes.BillableComputeMinutes}}'
```

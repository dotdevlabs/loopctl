# Lightning top-ups

Configure `LOOPCTL_LN_HOST` with your LND REST URL and
`LOOPCTL_LN_MACAROON_HEX` with your hex-encoded LND macaroon. Select the
LoopControl account you want to fund by passing its named context.

```sh
loopctl --context <account-context> topup
```

When the Lightning rail is selected, the CLI pays the invoice, submits the
proof to `POST /api/billing/l402_payments`, and waits for settlement. The
submission uses the selected account's API token in `X-Api-Token` and the
Lightning proof in `Authorization`. The API token comes from the usual
LoopControl authentication configuration. The LND macaroon configures access
to the wallet.

If the payment submission returns HTTP 401, check the active LoopControl API
token and confirm that the selected context points to the intended account.
An HTTP 402 from the payment submission means the Lightning proof is missing or
malformed.

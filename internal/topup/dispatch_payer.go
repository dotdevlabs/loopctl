package topup

import (
	"context"
	"io"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

// RailPayer is a Payer for a specific payment rail.
type RailPayer interface {
	Payer
	RailName() string // e.g. "x402" or "l402"
	Configured() bool // true when wallet/credentials are present
}

// SelectingPayer picks the first configured RailPayer whose name appears in
// the 402 advertisement, and falls back to Fallback when none matches.
type SelectingPayer struct {
	Rails    []RailPayer
	Fallback Payer
}

func (s SelectingPayer) Pay(ctx context.Context, product string, info apiclient.FundingInfo, out io.Writer) error {
	advertised := make(map[string]bool, len(info.Rails))
	for _, r := range info.Rails {
		advertised[r] = true
	}
	for _, rp := range s.Rails {
		if advertised[rp.RailName()] && rp.Configured() {
			return rp.Pay(ctx, product, info, out)
		}
	}
	return s.Fallback.Pay(ctx, product, info, out)
}

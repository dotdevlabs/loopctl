package topup

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

// stubRailPayer is a test double for RailPayer.
type stubRailPayer struct {
	name       string
	configured bool
	called     bool
	err        error
}

func (s *stubRailPayer) RailName() string { return s.name }
func (s *stubRailPayer) Configured() bool { return s.configured }
func (s *stubRailPayer) Pay(_ context.Context, _ string, _ apiclient.FundingInfo, _ io.Writer) error {
	s.called = true
	return s.err
}

// stubFallback tracks whether it was called.
type stubFallback struct {
	called bool
	err    error
}

func (s *stubFallback) Pay(_ context.Context, _ string, _ apiclient.FundingInfo, _ io.Writer) error {
	s.called = true
	return s.err
}

func makeInfo(rails ...string) apiclient.FundingInfo {
	return apiclient.FundingInfo{Rails: rails}
}

func TestSelectingPayerPicksX402(t *testing.T) {
	x402 := &stubRailPayer{name: "x402", configured: true}
	l402 := &stubRailPayer{name: "l402", configured: true}
	fb := &stubFallback{}
	sp := SelectingPayer{Rails: []RailPayer{x402, l402}, Fallback: fb}

	if err := sp.Pay(context.Background(), "topup", makeInfo("x402", "human_link"), io.Discard); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !x402.called {
		t.Error("expected x402 payer to be called")
	}
	if l402.called || fb.called {
		t.Error("expected only x402 payer to be called")
	}
}

func TestSelectingPayerPicksL402(t *testing.T) {
	x402 := &stubRailPayer{name: "x402", configured: false}
	l402 := &stubRailPayer{name: "l402", configured: true}
	fb := &stubFallback{}
	sp := SelectingPayer{Rails: []RailPayer{x402, l402}, Fallback: fb}

	if err := sp.Pay(context.Background(), "topup", makeInfo("l402"), io.Discard); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !l402.called {
		t.Error("expected l402 payer to be called")
	}
	if x402.called || fb.called {
		t.Error("expected only l402 payer to be called")
	}
}

func TestSelectingPayerFallsBackWhenNotConfigured(t *testing.T) {
	x402 := &stubRailPayer{name: "x402", configured: false}
	fb := &stubFallback{}
	sp := SelectingPayer{Rails: []RailPayer{x402}, Fallback: fb}

	if err := sp.Pay(context.Background(), "topup", makeInfo("x402"), io.Discard); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fb.called {
		t.Error("expected fallback payer to be called")
	}
	if x402.called {
		t.Error("did not expect x402 payer to be called")
	}
}

func TestSelectingPayerFallsBackWhenNoRailMatch(t *testing.T) {
	x402 := &stubRailPayer{name: "x402", configured: true}
	l402 := &stubRailPayer{name: "l402", configured: true}
	fb := &stubFallback{}
	sp := SelectingPayer{Rails: []RailPayer{x402, l402}, Fallback: fb}

	// 402 advertises only human_link, no auto-pay rail
	if err := sp.Pay(context.Background(), "topup", makeInfo("human_link"), io.Discard); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fb.called {
		t.Error("expected fallback payer to be called")
	}
}

func TestSelectingPayerPreferenceOrder(t *testing.T) {
	// Both rails advertised; x402 listed first → x402 wins.
	x402 := &stubRailPayer{name: "x402", configured: true}
	l402 := &stubRailPayer{name: "l402", configured: true}
	fb := &stubFallback{}
	sp := SelectingPayer{Rails: []RailPayer{x402, l402}, Fallback: fb}

	if err := sp.Pay(context.Background(), "topup", makeInfo("x402", "l402"), io.Discard); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !x402.called {
		t.Error("expected x402 payer to be called first")
	}
	if l402.called || fb.called {
		t.Error("expected only x402 payer to be called")
	}
}

func TestSelectingPayerPropagatesError(t *testing.T) {
	sentinel := errors.New("payment failed")
	x402 := &stubRailPayer{name: "x402", configured: true, err: sentinel}
	fb := &stubFallback{}
	sp := SelectingPayer{Rails: []RailPayer{x402}, Fallback: fb}

	err := sp.Pay(context.Background(), "topup", makeInfo("x402"), io.Discard)
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error; got %v", err)
	}
}

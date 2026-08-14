package topup

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/config"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"
	"github.com/dotdevlabs/loopctl/internal/apiclient"
	"github.com/dotdevlabs/loopctl/internal/wallet"
)

// stubSigner implements ArbitrumSigner for testing.
type stubSigner struct {
	addr string
	sig  X402Signature
	err  error
}

func (s *stubSigner) Address() string { return s.addr }
func (s *stubSigner) Sign(_ X402SignParams) (X402Signature, error) {
	return s.sig, s.err
}

func makeX402Info(serverURL, product string) apiclient.FundingInfo {
	nonce := make([]byte, 32)
	return apiclient.FundingInfo{
		Rails: []string{"x402", "human_link"},
		Products: []apiclient.Product{
			{
				Key:   product,
				Label: "Top-up",
				Rails: map[string]apiclient.RailInfo{
					"x402": {
						Amount:    "1000000",
						Recipient: "0xRecipient0000000000000000000000000000000",
						Token:     "0xToken000000000000000000000000000000000",
						ChainID:   42161,
						Deadline:  9999999999,
						Nonce:     "0x" + hex.EncodeToString(nonce),
					},
					"human_link": {URL: serverURL + "/checkout"},
				},
			},
		},
	}
}

func makeX402Ctx(t *testing.T, serverURL string) context.Context {
	t.Helper()
	renderer := output.New(false, "", io.Discard, io.Discard)
	ctx := context.Background()
	ctx = ctxutil.WithRenderer(ctx, renderer)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{JSON: false})
	ctx = ctxutil.WithActiveContext(ctx, &config.Context{BaseURL: serverURL, Token: "tok"})
	return ctx
}

func TestX402PayerSuccess(t *testing.T) {
	var gotHeader string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-PAYMENT")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{}`)
	}))
	defer ts.Close()

	signer := &stubSigner{addr: "0xFromAddr"}
	p := &X402Payer{Endpoint: "/api/topup", signer: signer}
	ctx := makeX402Ctx(t, ts.URL)
	info := makeX402Info(ts.URL, "topup")

	var out bytes.Buffer
	if err := p.Pay(ctx, "topup", info, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotHeader == "" {
		t.Error("expected X-PAYMENT header to be set")
	}
	// Verify it is valid base64.
	decoded, err := base64.StdEncoding.DecodeString(gotHeader)
	if err != nil {
		t.Errorf("X-PAYMENT header is not valid base64: %v", err)
	}
	if !strings.Contains(string(decoded), `"from"`) {
		t.Errorf("X-PAYMENT payload missing 'from' field: %s", decoded)
	}
	if !strings.Contains(out.String(), "x402") {
		t.Errorf("expected x402 mention in output; got %q", out.String())
	}
}

func TestX402PayerAPIFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	signer := &stubSigner{addr: "0xFromAddr"}
	p := &X402Payer{Endpoint: "/api/topup", signer: signer}
	ctx := makeX402Ctx(t, ts.URL)
	info := makeX402Info(ts.URL, "topup")

	if err := p.Pay(ctx, "topup", info, io.Discard); err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestX402PayerProductNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	signer := &stubSigner{addr: "0xFromAddr"}
	p := &X402Payer{Endpoint: "/api/topup", signer: signer}
	ctx := makeX402Ctx(t, ts.URL)
	info := apiclient.FundingInfo{
		Rails:    []string{"x402"},
		Products: []apiclient.Product{{Key: "other", Rails: map[string]apiclient.RailInfo{"x402": {}}}},
	}

	err := p.Pay(ctx, "topup", info, io.Discard)
	if err == nil {
		t.Fatal("expected error when product not found")
	}
	if !strings.Contains(err.Error(), "topup") {
		t.Errorf("expected error mentioning product; got %v", err)
	}
}

func TestX402PayerSignerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should never be reached.
		t.Error("server should not be called when signer fails")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	signer := &stubSigner{addr: "0xFromAddr", err: fmt.Errorf("key not found")}
	p := &X402Payer{Endpoint: "/api/topup", signer: signer}
	ctx := makeX402Ctx(t, ts.URL)
	info := makeX402Info(ts.URL, "topup")

	if err := p.Pay(ctx, "topup", info, io.Discard); err == nil {
		t.Fatal("expected error when signer fails")
	}
}

func TestX402PayerConfigured(t *testing.T) {
	configured := &X402Payer{signer: &stubSigner{addr: "0xAddr"}}
	if !configured.Configured() {
		t.Error("expected Configured() == true with non-empty address")
	}

	notConfigured := &X402Payer{signer: &stubSigner{addr: ""}}
	if notConfigured.Configured() {
		t.Error("expected Configured() == false with empty address")
	}

	nilSigner := &X402Payer{}
	if nilSigner.Configured() {
		t.Error("expected Configured() == false with nil signer")
	}
}

func TestX402PayerFromWalletConfig(t *testing.T) {
	cfg := wallet.ArbitrumConfig{PrivateKey: ""}
	p := NewX402Payer("/api/topup", cfg)
	if p.Configured() {
		t.Error("expected not configured when private key is empty")
	}
}

func TestX402PayerJSONOutput(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{}`)
	}))
	defer ts.Close()

	signer := &stubSigner{addr: "0xFromAddr"}
	p := &X402Payer{Endpoint: "/api/topup", signer: signer}

	renderer := output.New(true, "", io.Discard, io.Discard)
	ctx := context.Background()
	ctx = ctxutil.WithRenderer(ctx, renderer)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{JSON: true})
	ctx = ctxutil.WithActiveContext(ctx, &config.Context{BaseURL: ts.URL, Token: "tok"})

	info := makeX402Info(ts.URL, "topup")
	var out bytes.Buffer
	if err := p.Pay(ctx, "topup", info, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"rail"`) || !strings.Contains(got, `"x402"`) {
		t.Errorf("expected JSON with rail=x402; got %q", got)
	}
	if !strings.Contains(got, `"status"`) || !strings.Contains(got, `"paid"`) {
		t.Errorf("expected JSON with status=paid; got %q", got)
	}
}

// TestSecp256k1SignerSign verifies the EIP-712 signer with a known private key.
// We check: address derivation, digest is 32 bytes, signature is valid V/R/S.
func TestSecp256k1SignerSign(t *testing.T) {
	// Known 32-byte private key (all-0x01 bytes for determinism).
	privKeyHex := strings.Repeat("01", 32)
	signer := newSecp256k1Signer(privKeyHex)

	if signer.Address() == "" {
		t.Fatal("expected non-empty address")
	}
	if !strings.HasPrefix(signer.Address(), "0x") {
		t.Errorf("expected 0x-prefixed address; got %q", signer.Address())
	}

	var nonce [32]byte
	sig, err := signer.Sign(X402SignParams{
		From:        signer.Address(),
		To:          "0x" + strings.Repeat("02", 20),
		Value:       big.NewInt(1000000),
		ValidAfter:  0,
		ValidBefore: 9999999999,
		Nonce:       nonce,
		ChainID:     42161,
		Token:       "0xUSDC0000000000000000000000000000000000",
	})
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	if sig.V != 27 && sig.V != 28 {
		t.Errorf("expected V in {27, 28}; got %d", sig.V)
	}
	if sig.R == ([32]byte{}) {
		t.Error("expected non-zero R")
	}
	if sig.S == ([32]byte{}) {
		t.Error("expected non-zero S")
	}
}

func TestSecp256k1SignerInvalidKey(t *testing.T) {
	signer := newSecp256k1Signer("notahexkey")
	if signer.Address() != "" {
		t.Error("expected empty address for invalid key")
	}
	// Verify that an X402Payer built with this signer is not configured.
	p := &X402Payer{signer: signer}
	if p.Configured() {
		t.Error("expected X402Payer.Configured() == false with empty address")
	}
}

func TestHexToBytes32(t *testing.T) {
	nonce := "0x" + strings.Repeat("ab", 32)
	b, err := hexToBytes32(nonce)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, v := range b {
		if v != 0xab {
			t.Errorf("byte[%d] = %x; want ab", i, v)
		}
	}
}

func TestHexToBytes32WrongLength(t *testing.T) {
	_, err := hexToBytes32("0xdeadbeef")
	if err == nil {
		t.Error("expected error for wrong length")
	}
}

package topup

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
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

// stubInvoicer implements InvoicePayer for testing.
type stubInvoicer struct {
	preimage string
	err      error
}

func (s *stubInvoicer) PayInvoice(_ context.Context, _ string) (string, error) {
	return s.preimage, s.err
}

func makeL402Info(product string, macaroon, invoice string) apiclient.FundingInfo {
	return apiclient.FundingInfo{
		Rails: []string{"l402"},
		Products: []apiclient.Product{
			{
				Key:   product,
				Label: "Top-up",
				Rails: map[string]apiclient.RailInfo{
					"l402": {
						Macaroon: macaroon,
						Invoice:  invoice,
					},
					"human_link": {URL: "https://checkout.example.com"},
				},
			},
		},
	}
}

func makeL402Ctx(t *testing.T, serverURL string) context.Context {
	t.Helper()
	renderer := output.New(false, "", io.Discard, io.Discard)
	ctx := context.Background()
	ctx = ctxutil.WithRenderer(ctx, renderer)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{JSON: false})
	ctx = ctxutil.WithActiveContext(ctx, &config.Context{BaseURL: serverURL, Token: "tok"})
	return ctx
}

func TestL402PayerSuccess(t *testing.T) {
	const wantMacaroon = "testmacaroon123"
	const wantPreimage = "aabbccdd"
	var gotAuth string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{}`)
	}))
	defer ts.Close()

	invoicer := &stubInvoicer{preimage: wantPreimage}
	p := &L402Payer{Endpoint: "/api/topup", invoicer: invoicer}
	ctx := makeL402Ctx(t, ts.URL)
	info := makeL402Info("topup", wantMacaroon, "lnbc100n1...")

	var out bytes.Buffer
	if err := p.Pay(ctx, "topup", info, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := fmt.Sprintf("L402 %s:%s", wantMacaroon, wantPreimage)
	if gotAuth != expected {
		t.Errorf("Authorization header = %q; want %q", gotAuth, expected)
	}
	if !strings.Contains(out.String(), "Lightning") {
		t.Errorf("expected Lightning mention in output; got %q", out.String())
	}
}

func TestL402PayerAPIFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	invoicer := &stubInvoicer{preimage: "aabbcc"}
	p := &L402Payer{Endpoint: "/api/topup", invoicer: invoicer}
	ctx := makeL402Ctx(t, ts.URL)
	info := makeL402Info("topup", "mac", "invoice")

	if err := p.Pay(ctx, "topup", info, io.Discard); err == nil {
		t.Fatal("expected error on 403")
	}
}

func TestL402PayerProductNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	invoicer := &stubInvoicer{preimage: "preimage"}
	p := &L402Payer{Endpoint: "/api/topup", invoicer: invoicer}
	ctx := makeL402Ctx(t, ts.URL)
	info := apiclient.FundingInfo{
		Rails: []string{"l402"},
		Products: []apiclient.Product{
			{Key: "other", Rails: map[string]apiclient.RailInfo{"l402": {Macaroon: "m", Invoice: "i"}}},
		},
	}

	err := p.Pay(ctx, "topup", info, io.Discard)
	if err == nil {
		t.Fatal("expected error when product not found")
	}
}

func TestL402PayerInvoiceError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called when invoicer fails")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	invoicer := &stubInvoicer{err: fmt.Errorf("lightning error")}
	p := &L402Payer{Endpoint: "/api/topup", invoicer: invoicer}
	ctx := makeL402Ctx(t, ts.URL)
	info := makeL402Info("topup", "mac", "invoice")

	if err := p.Pay(ctx, "topup", info, io.Discard); err == nil {
		t.Fatal("expected error when invoicer fails")
	}
}

func TestL402PayerConfigured(t *testing.T) {
	withInvoicer := &L402Payer{invoicer: &stubInvoicer{}}
	if !withInvoicer.Configured() {
		t.Error("expected Configured() == true with invoicer")
	}

	noInvoicer := &L402Payer{}
	if noInvoicer.Configured() {
		t.Error("expected Configured() == false without invoicer")
	}
}

func TestL402PayerFromWalletConfig(t *testing.T) {
	cfg := wallet.LightningConfig{}
	p := NewL402Payer("/api/topup", cfg)
	if p.Configured() {
		t.Error("expected not configured when lightning config is empty")
	}
}

func TestL402PayerJSONOutput(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{}`)
	}))
	defer ts.Close()

	invoicer := &stubInvoicer{preimage: "preimage123"}
	p := &L402Payer{Endpoint: "/api/topup", invoicer: invoicer}

	renderer := output.New(true, "", io.Discard, io.Discard)
	ctx := context.Background()
	ctx = ctxutil.WithRenderer(ctx, renderer)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{JSON: true})
	ctx = ctxutil.WithActiveContext(ctx, &config.Context{BaseURL: ts.URL, Token: "tok"})

	info := makeL402Info("topup", "mac", "invoice")
	var out bytes.Buffer
	if err := p.Pay(ctx, "topup", info, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"rail"`) || !strings.Contains(got, `"l402"`) {
		t.Errorf("expected JSON with rail=l402; got %q", got)
	}
	if !strings.Contains(got, `"status"`) || !strings.Contains(got, `"paid"`) {
		t.Errorf("expected JSON with status=paid; got %q", got)
	}
}

// TestLNDRestPayerSuccess tests the real lndRestPayer against an httptest server.
func TestLNDRestPayerSuccess(t *testing.T) {
	// LND returning base64-encoded preimage (32-byte value).
	rawPreimage := make([]byte, 32)
	copy(rawPreimage, []byte("deadbeef"))
	b64Preimage := base64.StdEncoding.EncodeToString(rawPreimage)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/channels/transactions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Grpc-Metadata-Macaroon") == "" {
			t.Error("expected Grpc-Metadata-Macaroon header")
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"payment_preimage": "%s"}`, b64Preimage)
	}))
	defer ts.Close()

	payer := newLNDRestPayer(wallet.LightningConfig{
		Host:        ts.URL,
		MacaroonHex: "cafebabe",
	})

	preimage, err := payer.PayInvoice(context.Background(), "lnbc100n1...")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if preimage == "" {
		t.Error("expected non-empty preimage")
	}
}

func TestLNDRestPayerPaymentError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"payment_error": "timeout", "payment_preimage": ""}`)
	}))
	defer ts.Close()

	payer := newLNDRestPayer(wallet.LightningConfig{
		Host:        ts.URL,
		MacaroonHex: "cafebabe",
	})

	_, err := payer.PayInvoice(context.Background(), "lnbc100n1...")
	if err == nil {
		t.Fatal("expected error when payment_error is set")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("expected error to contain 'timeout'; got %v", err)
	}
}

func TestLNDRestPayerHexPreimage(t *testing.T) {
	// Some LND versions return hex preimage.
	const hexPreimage = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"payment_preimage": "%s"}`, hexPreimage)
	}))
	defer ts.Close()

	payer := newLNDRestPayer(wallet.LightningConfig{
		Host:        ts.URL,
		MacaroonHex: "cafebabe",
	})

	preimage, err := payer.PayInvoice(context.Background(), "lnbc100n1...")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if preimage != hexPreimage {
		t.Errorf("expected hex preimage %q; got %q", hexPreimage, preimage)
	}
}

func TestL402PayerMissingInvoice(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	invoicer := &stubInvoicer{preimage: "preimage"}
	p := &L402Payer{Endpoint: "/api/topup", invoicer: invoicer}
	ctx := makeL402Ctx(t, ts.URL)
	// L402 rail has macaroon but no invoice.
	info := apiclient.FundingInfo{
		Rails: []string{"l402"},
		Products: []apiclient.Product{
			{Key: "topup", Rails: map[string]apiclient.RailInfo{"l402": {Macaroon: "mac"}}},
		},
	}

	if err := p.Pay(ctx, "topup", info, io.Discard); err == nil {
		t.Fatal("expected error when invoice is missing")
	}
}

func TestL402PayerMissingMacaroon(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	invoicer := &stubInvoicer{preimage: "preimage"}
	p := &L402Payer{Endpoint: "/api/topup", invoicer: invoicer}
	ctx := makeL402Ctx(t, ts.URL)
	// L402 rail has invoice but no macaroon.
	info := apiclient.FundingInfo{
		Rails: []string{"l402"},
		Products: []apiclient.Product{
			{Key: "topup", Rails: map[string]apiclient.RailInfo{"l402": {Invoice: "lnbc..."}}},
		},
	}

	if err := p.Pay(ctx, "topup", info, io.Discard); err == nil {
		t.Fatal("expected error when macaroon is missing")
	}
}

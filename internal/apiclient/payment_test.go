package apiclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/config"
)

const fullBody402 = `{
	"rails": ["human_link"],
	"products": [
		{"key": "trial", "label": "5-hour trial", "rails": {"human_link": {"url": "https://checkout.stripe.com/trial"}}},
		{"key": "topup", "label": "Top-up package", "rails": {"human_link": {"url": "https://checkout.stripe.com/topup"}}},
		{"key": "subscription", "label": "Subscription prepay", "rails": {"human_link": {"url": "https://checkout.stripe.com/sub"}}}
	]
}`

func testActiveCtx(serverURL string) *config.Context {
	return &config.Context{BaseURL: serverURL, Token: "tok"}
}

func TestParse402Full(t *testing.T) {
	e := parse402([]byte(fullBody402))
	if e == nil {
		t.Fatal("expected non-nil PaymentRequiredError")
	}
	if len(e.Info.Rails) != 1 || e.Info.Rails[0] != "human_link" {
		t.Errorf("unexpected rails: %v", e.Info.Rails)
	}
	if len(e.Info.Products) != 3 {
		t.Fatalf("expected 3 products; got %d", len(e.Info.Products))
	}
	if e.Info.Products[0].Key != "trial" {
		t.Errorf("expected first product key=trial; got %q", e.Info.Products[0].Key)
	}
	if e.Info.Products[1].Key != "topup" {
		t.Errorf("expected second product key=topup; got %q", e.Info.Products[1].Key)
	}
	if e.Info.Products[2].Key != "subscription" {
		t.Errorf("expected third product key=subscription; got %q", e.Info.Products[2].Key)
	}
}

func TestParse402EmptyBody(t *testing.T) {
	e := parse402([]byte{})
	if e == nil {
		t.Fatal("expected non-nil PaymentRequiredError")
	}
	if len(e.Info.Rails) != 0 {
		t.Errorf("expected zero rails; got %v", e.Info.Rails)
	}
	if len(e.Info.Products) != 0 {
		t.Errorf("expected zero products; got %v", e.Info.Products)
	}
}

func TestParse402MalformedBody(t *testing.T) {
	e := parse402([]byte("not json at all"))
	if e == nil {
		t.Fatal("expected non-nil PaymentRequiredError (no panic)")
	}
}

func TestParse402ProductURL(t *testing.T) {
	e := parse402([]byte(fullBody402))
	if e == nil {
		t.Fatal("expected non-nil PaymentRequiredError")
	}
	railInfo, ok := e.Info.Products[0].Rails["human_link"]
	if !ok {
		t.Fatal("expected human_link rail on first product")
	}
	if railInfo.URL != "https://checkout.stripe.com/trial" {
		t.Errorf("expected trial URL; got %q", railInfo.URL)
	}
}

func TestPaymentRequiredErrorMessage(t *testing.T) {
	e := &PaymentRequiredError{}
	if e.Error() != "payment required" {
		t.Errorf("expected 'payment required'; got %q", e.Error())
	}
}

func TestPostJSON402(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = fmt.Fprint(w, fullBody402)
	}))
	defer ts.Close()

	type empty struct{}
	_, err := PostJSON[empty](context.Background(), testActiveCtx(ts.URL), "/api/topup", nil)
	if err == nil {
		t.Fatal("expected error on 402")
	}
	var payErr *PaymentRequiredError
	if !errors.As(err, &payErr) {
		t.Fatalf("expected *PaymentRequiredError; got %T: %v", err, err)
	}
	if len(payErr.Info.Products) != 3 {
		t.Errorf("expected 3 products; got %d", len(payErr.Info.Products))
	}
}

func TestGetJSON402(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = fmt.Fprint(w, fullBody402)
	}))
	defer ts.Close()

	type empty struct{}
	_, err := GetJSON[empty](context.Background(), testActiveCtx(ts.URL), "/api/topup")
	if err == nil {
		t.Fatal("expected error on 402")
	}
	var payErr *PaymentRequiredError
	if !errors.As(err, &payErr) {
		t.Fatalf("expected *PaymentRequiredError; got %T: %v", err, err)
	}
}

func TestPostEnvelope402(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = fmt.Fprint(w, fullBody402)
	}))
	defer ts.Close()

	type empty struct{}
	_, err := PostEnvelope[empty](context.Background(), testActiveCtx(ts.URL), "/api/topup", nil)
	if err == nil {
		t.Fatal("expected error on 402")
	}
	var payErr *PaymentRequiredError
	if !errors.As(err, &payErr) {
		t.Fatalf("expected *PaymentRequiredError; got %T: %v", err, err)
	}
	if payErr.Info.Products[1].Key != "topup" {
		t.Errorf("expected topup product; got %q", payErr.Info.Products[1].Key)
	}
}

func TestGetJSONAPISingle402(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = fmt.Fprint(w, fullBody402)
	}))
	defer ts.Close()

	type empty struct{}
	_, err := GetJSONAPISingle[empty](context.Background(), testActiveCtx(ts.URL), "/api/test")
	if err == nil {
		t.Fatal("expected error on 402")
	}
	var payErr *PaymentRequiredError
	if !errors.As(err, &payErr) {
		t.Fatalf("expected *PaymentRequiredError; got %T: %v", err, err)
	}
}

func TestDeleteJSONAPI402(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = fmt.Fprint(w, fullBody402)
	}))
	defer ts.Close()

	err := DeleteJSONAPI(context.Background(), testActiveCtx(ts.URL), "/api/test/1")
	if err == nil {
		t.Fatal("expected error on 402")
	}
	var payErr *PaymentRequiredError
	if !errors.As(err, &payErr) {
		t.Fatalf("expected *PaymentRequiredError; got %T: %v", err, err)
	}
}

const x402Body402 = `{
	"rails": ["x402", "human_link"],
	"products": [
		{"key": "topup", "label": "Top-up package", "rails": {
			"x402": {
				"url": "",
				"amount": "1000000",
				"recipient": "0xRecipient",
				"token": "0xToken",
				"chain_id": 42161,
				"deadline": 9999999999,
				"nonce": "0xdeadbeef"
			},
			"human_link": {"url": "https://checkout.stripe.com/topup"}
		}}
	]
}`

const l402Body402 = `{
	"rails": ["l402"],
	"products": [
		{"key": "topup", "label": "Top-up package", "rails": {
			"l402": {
				"url": "",
				"macaroon": "dGVzdG1hY2Fyb29u",
				"invoice": "lnbc100n1..."
			}
		}}
	]
}`

func TestParse402X402Rail(t *testing.T) {
	e := parse402([]byte(x402Body402))
	if e == nil {
		t.Fatal("expected non-nil PaymentRequiredError")
	}
	if len(e.Info.Rails) != 2 {
		t.Fatalf("expected 2 rails; got %d", len(e.Info.Rails))
	}
	if e.Info.Rails[0] != "x402" {
		t.Errorf("expected first rail=x402; got %q", e.Info.Rails[0])
	}
	ri, ok := e.Info.Products[0].Rails["x402"]
	if !ok {
		t.Fatal("expected x402 rail on product")
	}
	if ri.Amount != "1000000" {
		t.Errorf("expected Amount=1000000; got %q", ri.Amount)
	}
	if ri.Recipient != "0xRecipient" {
		t.Errorf("expected Recipient=0xRecipient; got %q", ri.Recipient)
	}
	if ri.Token != "0xToken" {
		t.Errorf("expected Token=0xToken; got %q", ri.Token)
	}
	if ri.ChainID != 42161 {
		t.Errorf("expected ChainID=42161; got %d", ri.ChainID)
	}
	if ri.Deadline != 9999999999 {
		t.Errorf("expected Deadline=9999999999; got %d", ri.Deadline)
	}
	if ri.Nonce != "0xdeadbeef" {
		t.Errorf("expected Nonce=0xdeadbeef; got %q", ri.Nonce)
	}
}

func TestParse402L402Rail(t *testing.T) {
	e := parse402([]byte(l402Body402))
	if e == nil {
		t.Fatal("expected non-nil PaymentRequiredError")
	}
	ri, ok := e.Info.Products[0].Rails["l402"]
	if !ok {
		t.Fatal("expected l402 rail on product")
	}
	if ri.Macaroon != "dGVzdG1hY2Fyb29u" {
		t.Errorf("expected Macaroon=dGVzdG1hY2Fyb29u; got %q", ri.Macaroon)
	}
	if ri.Invoice != "lnbc100n1..." {
		t.Errorf("expected Invoice=lnbc100n1...; got %q", ri.Invoice)
	}
}

func TestRailInfoBackwardCompatible(t *testing.T) {
	// Original human_link-only body should still decode correctly with new struct.
	e := parse402([]byte(fullBody402))
	if e == nil {
		t.Fatal("expected non-nil PaymentRequiredError")
	}
	ri := e.Info.Products[0].Rails["human_link"]
	if ri.URL != "https://checkout.stripe.com/trial" {
		t.Errorf("expected trial URL; got %q", ri.URL)
	}
	// New fields should be zero-valued.
	if ri.Amount != "" || ri.Macaroon != "" || ri.ChainID != 0 {
		t.Error("expected new fields to be zero-valued on human_link rail")
	}
}

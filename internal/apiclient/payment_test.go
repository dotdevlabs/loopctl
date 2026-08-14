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

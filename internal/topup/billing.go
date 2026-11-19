package topup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

const (
	fundingEndpoint  = "/api/billing/funding_envelope"
	stripeEndpoint   = "/api/billing/topups"
	x402Endpoint     = "/api/billing/x402_payments"
	l402Endpoint     = "/api/billing/l402_payments"
	purchaseEndpoint = "/api/billing/pending_purchases/"
	billingMedia     = "application/vnd.api+json"
)

type packageParams struct {
	PackageID string `json:"package_id"`
}

type fundingRoute struct {
	Method string        `json:"method"`
	Href   string        `json:"href"`
	Params packageParams `json:"params"`
}

type fundingProduct struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Available bool   `json:"available"`
	Rails     struct {
		Human *fundingRoute `json:"human_link"`
		X402  *struct {
			Params    packageParams `json:"params"`
			Scheme    string        `json:"scheme"`
			Network   string        `json:"network"`
			Version   int           `json:"x402Version"`
			Amount    string        `json:"maxAmountRequired"`
			Recipient string        `json:"payTo"`
			Asset     string        `json:"asset"`
		} `json:"x402"`
		Lightning *struct {
			Params   packageParams `json:"params"`
			Invoice  string        `json:"invoice"`
			Macaroon string        `json:"macaroon"`
		} `json:"lightning"`
	} `json:"payment_rails"`
}

type fundingDocument struct {
	Meta struct {
		Products []fundingProduct `json:"fundable_products"`
	} `json:"meta"`
}

func (d fundingDocument) product(selection string) (apiclient.FundingInfo, error) {
	kind := selection
	if kind == "subscription" {
		kind = "subscription_prepay"
	}
	for _, p := range d.Meta.Products {
		if !p.Available || (p.ID != selection && p.Kind != kind) {
			continue
		}
		product := apiclient.Product{Key: selection, Rails: map[string]apiclient.RailInfo{}}
		if rail := p.Rails.Human; rail != nil {
			product.Rails["human_link"] = apiclient.RailInfo{URL: rail.Href, Method: rail.Method, PackageID: rail.Params.PackageID}
		}
		if rail := p.Rails.X402; rail != nil {
			product.Rails["x402"] = apiclient.RailInfo{PackageID: rail.Params.PackageID, Scheme: rail.Scheme, Network: rail.Network,
				Version: rail.Version, Amount: rail.Amount, Recipient: rail.Recipient, Token: rail.Asset}
		}
		if rail := p.Rails.Lightning; rail != nil {
			product.Rails["l402"] = apiclient.RailInfo{PackageID: rail.Params.PackageID, Invoice: rail.Invoice, Macaroon: rail.Macaroon}
		}
		var available []string
		for _, name := range []string{"human_link", "x402", "l402"} {
			if _, ok := product.Rails[name]; ok {
				available = append(available, name)
			}
		}
		return apiclient.FundingInfo{Products: []apiclient.Product{product}, Rails: available}, nil
	}
	return apiclient.FundingInfo{}, fmt.Errorf("product %q not found among available funding products", selection)
}

type purchaseDocument struct {
	Data struct {
		ID         string `json:"id"`
		Attributes struct {
			Status        string  `json:"status"`
			CheckoutURL   *string `json:"checkout_url"`
			FailureReason *string `json:"failure_reason"`
		} `json:"attributes"`
	} `json:"data"`
}

func billingRequest[T any](ctx context.Context, method, path string, body []byte, headers http.Header, status int) (T, error) {
	var result T
	active := ctxutil.ActiveContextFrom(ctx)
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(active.BaseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return result, err
	}
	request.Header.Set("Authorization", "Bearer "+active.Token)
	request.Header.Set("Accept", billingMedia)
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, values := range headers {
		request.Header[key] = values
	}
	client := *http.DefaultClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(request)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}
	if resp.StatusCode != status {
		return result, fmt.Errorf("billing response (%d): %w", resp.StatusCode, apiclient.DecodeAPIError(resp, raw))
	}
	if err := apiclient.DecodeResponse(resp, "", raw, &result); err != nil {
		return result, fmt.Errorf("decoding billing response: %w", err)
	}
	return result, nil
}

func awaitPurchase(ctx context.Context, purchase purchaseDocument, checkout bool) (string, error) {
	id := purchase.Data.ID
	if id == "" {
		return "", fmt.Errorf("billing response is missing the purchase ID")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		a := purchase.Data.Attributes
		switch a.Status {
		case "failed":
			if a.FailureReason != nil {
				return "", fmt.Errorf("purchase %s failed: %s", id, *a.FailureReason)
			}
			return "", fmt.Errorf("purchase %s failed", id)
		case "settled", "pending":
			if !checkout && a.Status == "settled" {
				return "", nil
			}
			if checkout && a.CheckoutURL != nil && *a.CheckoutURL != "" {
				return *a.CheckoutURL, nil
			}
		default:
			return "", fmt.Errorf("purchase %s has unexpected status %q", id, a.Status)
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("waiting for purchase %s: %w", id, ctx.Err())
		case <-time.After(time.Second):
		}
		var err error
		purchase, err = billingRequest[purchaseDocument](ctx, http.MethodGet, purchaseEndpoint+url.PathEscape(id), nil, nil, http.StatusOK)
		if err != nil {
			return "", err
		}
	}
}

func submitPayment(ctx context.Context, endpoint, packageID string, headers http.Header) error {
	query := url.Values{"package_id": {packageID}}
	purchase, err := billingRequest[purchaseDocument](ctx, http.MethodPost, endpoint+"?"+query.Encode(), nil, headers, http.StatusAccepted)
	if err != nil {
		return err
	}
	_, err = awaitPurchase(ctx, purchase, false)
	return err
}

func checkoutURL(ctx context.Context, rail apiclient.RailInfo) (string, error) {
	switch rail.Method {
	case http.MethodGet:
		href, err := url.Parse(rail.URL)
		if err != nil || rail.URL == "" {
			return "", fmt.Errorf("invalid checkout URL")
		}
		base, err := url.Parse(ctxutil.ActiveContextFrom(ctx).BaseURL)
		if err != nil {
			return "", err
		}
		href = base.ResolveReference(href)
		if (href.Scheme != "https" && href.Scheme != "http") || href.Host == "" {
			return "", fmt.Errorf("invalid checkout URL")
		}
		return href.String(), nil
	case http.MethodPost:
		if rail.PackageID == "" {
			return "", fmt.Errorf("checkout route is missing the package ID")
		}
		body, err := json.Marshal(packageParams{PackageID: rail.PackageID})
		if err != nil {
			return "", err
		}
		purchase, err := billingRequest[purchaseDocument](ctx, http.MethodPost, stripeEndpoint, body, nil, http.StatusAccepted)
		if err != nil {
			return "", err
		}
		return awaitPurchase(ctx, purchase, true)
	default:
		return "", fmt.Errorf("unsupported checkout method %q", rail.Method)
	}
}

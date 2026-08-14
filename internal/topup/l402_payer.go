package topup

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/loopctl/internal/apiclient"
	"github.com/dotdevlabs/loopctl/internal/wallet"
)

// InvoicePayer pays a BOLT-11 Lightning invoice and returns the preimage.
type InvoicePayer interface {
	PayInvoice(ctx context.Context, invoice string) (preimage string, err error)
}

// L402Payer pays a 402 that advertises the "l402" rail via Lightning.
type L402Payer struct {
	Endpoint string
	invoicer InvoicePayer
}

// NewL402Payer constructs an L402Payer from a LightningConfig.
func NewL402Payer(endpoint string, cfg wallet.LightningConfig) *L402Payer {
	p := &L402Payer{Endpoint: endpoint}
	if cfg.Configured() {
		p.invoicer = newLNDRestPayer(cfg)
	}
	return p
}

func (p *L402Payer) RailName() string { return "l402" }
func (p *L402Payer) Configured() bool { return p.invoicer != nil }

func (p *L402Payer) Pay(ctx context.Context, product string, info apiclient.FundingInfo, out io.Writer) error {
	railInfo, err := findRail(info, product, "l402")
	if err != nil {
		return err
	}
	if railInfo.Invoice == "" {
		return fmt.Errorf("l402: no invoice in rail info for product %q", product)
	}
	if railInfo.Macaroon == "" {
		return fmt.Errorf("l402: no macaroon in rail info for product %q", product)
	}

	preimage, err := p.invoicer.PayInvoice(ctx, railInfo.Invoice)
	if err != nil {
		return fmt.Errorf("l402: paying invoice: %w", err)
	}

	activeCtx := ctxutil.ActiveContextFrom(ctx)
	fullURL := strings.TrimRight(activeCtx.BaseURL, "/") + p.Endpoint //nolint:gosec // G107: URL from user config
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader([]byte("{}")))
	if err != nil {
		return fmt.Errorf("l402: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("L402 %s:%s", railInfo.Macaroon, preimage))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("l402: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("l402: payment request returned %d", resp.StatusCode)
	}

	jsonMode := ctxutil.GlobalFlagsFrom(ctx).JSON
	if jsonMode {
		return json.NewEncoder(out).Encode(struct {
			Rail    string `json:"rail"`
			Status  string `json:"status"`
			Product string `json:"product"`
		}{Rail: "l402", Status: "paid", Product: product})
	}
	_, err = fmt.Fprintln(out, "Payment sent via L402 (Lightning)")
	return err
}

// lndRestPayer implements InvoicePayer using the LND REST API.
type lndRestPayer struct {
	host        string
	macaroonHex string
	client      *http.Client
}

func newLNDRestPayer(cfg wallet.LightningConfig) *lndRestPayer {
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	if cfg.TLSSkipVerify {
		tlsCfg.InsecureSkipVerify = true //nolint:gosec // G402: dev-only opt-in via explicit env var
	}
	return &lndRestPayer{
		host:        strings.TrimRight(cfg.Host, "/"),
		macaroonHex: cfg.MacaroonHex,
		client: &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsCfg},
		},
	}
}

func (l *lndRestPayer) PayInvoice(ctx context.Context, invoice string) (string, error) {
	body, err := json.Marshal(map[string]string{"payment_request": invoice})
	if err != nil {
		return "", fmt.Errorf("lnd: encoding request: %w", err)
	}

	url := l.host + "/v1/channels/transactions" //nolint:gosec // G107: URL from user config
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("lnd: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Grpc-Metadata-Macaroon", l.macaroonHex)

	resp, err := l.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("lnd: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("lnd: reading response: %w", err)
	}

	var result struct {
		PaymentPreimage string `json:"payment_preimage"`
		PaymentError    string `json:"payment_error"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("lnd: decoding response: %w", err)
	}

	if result.PaymentError != "" {
		return "", fmt.Errorf("lnd: payment error: %s", result.PaymentError)
	}
	if result.PaymentPreimage == "" {
		return "", fmt.Errorf("lnd: no preimage in response")
	}

	// A Lightning preimage is always 32 bytes.
	// Hex encoding: exactly 64 hex characters.
	// Base64 encoding: 44 characters (with padding) or 43 (without).
	// Check hex first to avoid misidentifying a hex string as valid base64.
	if len(result.PaymentPreimage) == 64 {
		if _, err := hex.DecodeString(result.PaymentPreimage); err == nil {
			return result.PaymentPreimage, nil
		}
	}
	// Fall back to base64 (standard LND REST v2 encoding).
	if decoded, err := base64.StdEncoding.DecodeString(result.PaymentPreimage); err == nil {
		return hex.EncodeToString(decoded), nil
	}
	return result.PaymentPreimage, nil
}

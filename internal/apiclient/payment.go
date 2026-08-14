package apiclient

import (
	"encoding/json"
	"net/http"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
)

// RailInfo holds per-rail payment info for a fundable product.
type RailInfo struct {
	URL string `json:"url"`
}

// Product is a single fundable product advertised in a 402 body.
type Product struct {
	Key   string              `json:"key"`
	Label string              `json:"label"`
	Rails map[string]RailInfo `json:"rails"`
}

// FundingInfo is the parsed body of an HTTP 402 Payment Required response.
type FundingInfo struct {
	Rails    []string  `json:"rails"`
	Products []Product `json:"products"`
}

// PaymentRequiredError is returned when the API responds with HTTP 402.
// Callers type-assert to access FundingInfo.
type PaymentRequiredError struct {
	Info FundingInfo
}

func (e *PaymentRequiredError) Error() string {
	return "payment required"
}

// parse402 best-effort parses body into a PaymentRequiredError.
func parse402(body []byte) *PaymentRequiredError {
	e := &PaymentRequiredError{}
	_ = json.Unmarshal(body, &e.Info) // best-effort; empty Info is valid
	return e
}

// errorRespJSONAPI converts a non-2xx response, returning PaymentRequiredError on 402
// and a CLIError (with JSON:API or flat error extraction) otherwise.
func errorRespJSONAPI(body []byte, status int) error {
	if status == http.StatusPaymentRequired {
		return parse402(body)
	}
	return clierror.New(statusToCode(status), extractJSONAPIOrFlatError(body, status), "")
}

// errorRespFlat converts a non-2xx response, returning PaymentRequiredError on 402
// and a CLIError (with flat error extraction) otherwise.
func errorRespFlat(body []byte, status int) error {
	if status == http.StatusPaymentRequired {
		return parse402(body)
	}
	return clierror.New(statusToCode(status), extractAPIError(body, status), "")
}

// Package apiclient provides an HTTP POST helper that surfaces the full API
// error body (including the errors[] array) on non-2xx responses.
package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
	"github.com/dotdevlabs/ctlkit/pkg/config"
	"github.com/dotdevlabs/ctlkit/pkg/httpclient"
)

// browserUserAgent matches ctlkit's UA.
const browserUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// PostEnvelope POSTs a JSON body to path and decodes a successful response into
// an Envelope[T]. On non-2xx it extracts errors[]/message/error from the body
// and returns a CLIError with a human-readable message.
func PostEnvelope[T any](ctx context.Context, activeCtx *config.Context, path string, body any) (httpclient.Envelope[T], error) {
	b, err := json.Marshal(body)
	if err != nil {
		return httpclient.Envelope[T]{}, fmt.Errorf("encoding request body: %w", err)
	}

	url := strings.TrimRight(activeCtx.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return httpclient.Envelope[T]{}, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+activeCtx.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", browserUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return httpclient.Envelope[T]{}, ctx.Err()
		}
		return httpclient.Envelope[T]{}, fmt.Errorf("request failed: %w", err)
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if closeErr := resp.Body.Close(); closeErr != nil {
		return httpclient.Envelope[T]{}, fmt.Errorf("closing response body: %w", closeErr)
	}
	if readErr != nil {
		return httpclient.Envelope[T]{}, fmt.Errorf("reading response: %w", readErr)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var env httpclient.Envelope[T]
		if len(respBody) > 0 {
			if err := json.Unmarshal(respBody, &env); err != nil {
				return httpclient.Envelope[T]{}, fmt.Errorf("decoding response: %w", err)
			}
		}
		return env, nil
	}

	return httpclient.Envelope[T]{}, clierror.New(
		statusToCode(resp.StatusCode),
		extractAPIError(respBody, resp.StatusCode),
		"",
	)
}

func extractAPIError(body []byte, status int) string {
	if len(body) > 0 {
		var errBody struct {
			Errors  []string `json:"errors"`
			Message string   `json:"message"`
			Error   string   `json:"error"`
		}
		if json.Unmarshal(body, &errBody) == nil {
			if len(errBody.Errors) > 0 {
				return strings.Join(errBody.Errors, "; ")
			}
			if errBody.Message != "" {
				return errBody.Message
			}
			if errBody.Error != "" {
				return errBody.Error
			}
		}
	}
	return fmt.Sprintf("HTTP %d", status)
}

func statusToCode(status int) clierror.ErrorCode {
	switch status {
	case http.StatusUnauthorized:
		return clierror.CodeUnauthorized
	case http.StatusForbidden:
		return clierror.CodeForbidden
	case http.StatusNotFound:
		return clierror.CodeNotFound
	case http.StatusConflict:
		return clierror.CodeConflict
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return clierror.CodeBadRequest
	case http.StatusServiceUnavailable:
		return clierror.CodeNotReady
	default:
		return clierror.CodeServerError
	}
}

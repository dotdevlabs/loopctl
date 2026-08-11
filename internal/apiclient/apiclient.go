// Package apiclient provides HTTP helpers that surface full API error bodies on non-2xx responses.
package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
	"github.com/dotdevlabs/ctlkit/pkg/config"
	"github.com/dotdevlabs/ctlkit/pkg/httpclient"
)

// browserUserAgent matches ctlkit's UA.
const browserUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

type contextKey int

const verboseKey contextKey = 1

// WithVerbose stores w in ctx so that subsequent requests log to it.
func WithVerbose(ctx context.Context, w io.Writer) context.Context {
	return context.WithValue(ctx, verboseKey, w)
}

func verboseFrom(ctx context.Context) io.Writer {
	w, _ := ctx.Value(verboseKey).(io.Writer)
	return w
}

func logVerbose(w io.Writer, method, url string, status int, body []byte) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, "> %s %s\n", method, url)
	fmt.Fprintf(w, "< %d %s\n", status, http.StatusText(status))
	if len(body) > 0 {
		fmt.Fprintf(w, "%s\n", body)
	}
}

// rawResourceLinks holds the links object on a JSON:API resource.
type rawResourceLinks struct {
	Self string `json:"self,omitempty"`
}

// rawResource is used to decode a JSON:API resource with deferred attribute parsing.
type rawResource struct {
	ID         string           `json:"id"`
	Type       string           `json:"type"`
	Attributes json.RawMessage  `json:"attributes"`
	Links      rawResourceLinks `json:"links,omitempty"`
}

// rawCollectionLinks holds the top-level links on a JSON:API collection response.
type rawCollectionLinks struct {
	First string `json:"first,omitempty"`
	Prev  string `json:"prev,omitempty"`
	Self  string `json:"self,omitempty"`
	Next  string `json:"next,omitempty"`
	Last  string `json:"last,omitempty"`
}

// SingleResult wraps a JSON:API resource with its links.self URL.
type SingleResult[T any] struct {
	httpclient.Resource[T]
	SelfLink string
}

// GetJSONAPISingle GETs path and decodes the JSON:API single-resource response.
// On non-2xx it extracts the human-readable error and returns a CLIError.
func GetJSONAPISingle[T any](ctx context.Context, activeCtx *config.Context, path string) (httpclient.Resource[T], error) {
	res, err := GetJSONAPISingleFull[T](ctx, activeCtx, path)
	if err != nil {
		return httpclient.Resource[T]{}, err
	}
	return res.Resource, nil
}

// GetJSONAPISingleFull GETs path and decodes the JSON:API single-resource response
// including data.links.self into SingleResult.SelfLink.
func GetJSONAPISingleFull[T any](ctx context.Context, activeCtx *config.Context, path string) (SingleResult[T], error) {
	fullURL := strings.TrimRight(activeCtx.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return SingleResult[T]{}, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("Authorization", "Bearer "+activeCtx.Token)
	req.Header.Set("User-Agent", browserUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return SingleResult[T]{}, ctx.Err()
		}
		return SingleResult[T]{}, fmt.Errorf("request failed: %w", err)
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if closeErr := resp.Body.Close(); closeErr != nil {
		return SingleResult[T]{}, fmt.Errorf("closing response body: %w", closeErr)
	}
	if readErr != nil {
		return SingleResult[T]{}, fmt.Errorf("reading response: %w", readErr)
	}

	logVerbose(verboseFrom(ctx), http.MethodGet, fullURL, resp.StatusCode, respBody)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		type rawDoc struct {
			Data   rawResource       `json:"data"`
			Errors []jsonAPIErrEntry `json:"errors"`
		}
		var doc rawDoc
		if len(respBody) > 0 {
			if err := json.Unmarshal(respBody, &doc); err != nil {
				return SingleResult[T]{}, fmt.Errorf("decoding response: %w", err)
			}
		}
		if len(doc.Errors) > 0 {
			return SingleResult[T]{}, extractJSONAPIError(doc.Errors)
		}
		res := httpclient.Resource[T]{ID: doc.Data.ID, Type: doc.Data.Type}
		if len(doc.Data.Attributes) > 0 {
			if err := json.Unmarshal(doc.Data.Attributes, &res.Attributes); err != nil {
				return SingleResult[T]{}, fmt.Errorf("decoding attributes: %w", err)
			}
		}
		return SingleResult[T]{Resource: res, SelfLink: doc.Data.Links.Self}, nil
	}

	return SingleResult[T]{}, clierror.New(
		statusToCode(resp.StatusCode),
		extractJSONAPIOrFlatError(respBody, resp.StatusCode),
		"",
	)
}

// rawCollectionDoc is the wire-format wrapper for a JSON:API collection response.
type rawCollectionDoc struct {
	Data   []rawResource      `json:"data"`
	Links  rawCollectionLinks `json:"links"`
	Meta   map[string]any     `json:"meta,omitempty"`
	Errors []jsonAPIErrEntry  `json:"errors"`
}

// fetchJSONAPICollectionPage fetches one page from an absolute URL and decodes it.
func fetchJSONAPICollectionPage[T any](ctx context.Context, token, fullURL string) (httpclient.Collection[T], error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return httpclient.Collection[T]{}, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", browserUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return httpclient.Collection[T]{}, ctx.Err()
		}
		return httpclient.Collection[T]{}, fmt.Errorf("request failed: %w", err)
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if closeErr := resp.Body.Close(); closeErr != nil {
		return httpclient.Collection[T]{}, fmt.Errorf("closing response body: %w", closeErr)
	}
	if readErr != nil {
		return httpclient.Collection[T]{}, fmt.Errorf("reading response: %w", readErr)
	}

	logVerbose(verboseFrom(ctx), http.MethodGet, fullURL, resp.StatusCode, respBody)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var doc rawCollectionDoc
		if len(respBody) > 0 {
			if err := json.Unmarshal(respBody, &doc); err != nil {
				return httpclient.Collection[T]{}, fmt.Errorf("decoding response: %w", err)
			}
		}
		if len(doc.Errors) > 0 {
			return httpclient.Collection[T]{}, extractJSONAPIError(doc.Errors)
		}
		col := httpclient.Collection[T]{
			Data: make([]httpclient.Resource[T], len(doc.Data)),
			Links: httpclient.Links{
				First: doc.Links.First,
				Prev:  doc.Links.Prev,
				Next:  doc.Links.Next,
				Last:  doc.Links.Last,
			},
			Meta: doc.Meta,
		}
		for i, item := range doc.Data {
			res := httpclient.Resource[T]{ID: item.ID, Type: item.Type}
			if len(item.Attributes) > 0 {
				if err := json.Unmarshal(item.Attributes, &res.Attributes); err != nil {
					return httpclient.Collection[T]{}, fmt.Errorf("decoding attributes[%d]: %w", i, err)
				}
			}
			col.Data[i] = res
		}
		return col, nil
	}

	return httpclient.Collection[T]{}, clierror.New(
		statusToCode(resp.StatusCode),
		extractJSONAPIOrFlatError(respBody, resp.StatusCode),
		"",
	)
}

// GetJSONAPICollection GETs path and decodes the JSON:API collection response.
// On non-2xx it extracts the human-readable error and returns a CLIError.
func GetJSONAPICollection[T any](ctx context.Context, activeCtx *config.Context, path string) (httpclient.Collection[T], error) {
	fullURL := strings.TrimRight(activeCtx.BaseURL, "/") + path
	return fetchJSONAPICollectionPage[T](ctx, activeCtx.Token, fullURL)
}

// resolveNextURL handles both absolute and relative next links.
func resolveNextURL(next, baseURL string) string {
	if strings.HasPrefix(next, "http://") || strings.HasPrefix(next, "https://") {
		return next
	}
	return strings.TrimRight(baseURL, "/") + next
}

// GetJSONAPICollectionAllPages fetches the first page at path, then follows the server-provided
// links.next URLs until exhausted, accumulating all items. Stops after 1000 pages to prevent loops.
func GetJSONAPICollectionAllPages[T any](ctx context.Context, activeCtx *config.Context, path string) (httpclient.Collection[T], error) {
	result, err := GetJSONAPICollection[T](ctx, activeCtx, path)
	if err != nil {
		return httpclient.Collection[T]{}, err
	}

	const maxPages = 1000
	for page := 1; result.Links.Next != "" && page < maxPages; page++ {
		nextURL := resolveNextURL(result.Links.Next, activeCtx.BaseURL)
		nextPage, err := fetchJSONAPICollectionPage[T](ctx, activeCtx.Token, nextURL)
		if err != nil {
			return httpclient.Collection[T]{}, err
		}
		result.Data = append(result.Data, nextPage.Data...)
		result.Links = nextPage.Links
		result.Meta = nextPage.Meta
	}

	return result, nil
}

// SelfLinkPath converts a links.self value (absolute or relative) to a URL path
// suitable for passing to GetJSONAPISingleFull. Returns "" if self is empty.
func SelfLinkPath(self, baseURL string) string {
	if self == "" {
		return ""
	}
	if !strings.HasPrefix(self, "http://") && !strings.HasPrefix(self, "https://") {
		return self
	}
	// Absolute URL: strip scheme+host, keep path+query.
	base := strings.TrimRight(baseURL, "/")
	if strings.HasPrefix(self, base) {
		return self[len(base):]
	}
	// Different host: parse and extract path.
	u, err := url.Parse(self)
	if err != nil {
		return self
	}
	if u.RawQuery != "" {
		return u.Path + "?" + u.RawQuery
	}
	return u.Path
}

// GetEnvelope GETs path and decodes a flat JSON envelope {"data": T}.
// On non-2xx it extracts the human-readable error and returns a CLIError.
func GetEnvelope[T any](ctx context.Context, activeCtx *config.Context, path string) (httpclient.Envelope[T], error) {
	url := strings.TrimRight(activeCtx.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return httpclient.Envelope[T]{}, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+activeCtx.Token)
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

	logVerbose(verboseFrom(ctx), http.MethodGet, url, resp.StatusCode, respBody)

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

// GetJSON GETs path and decodes the flat JSON response body directly into T.
// On non-2xx it extracts the human-readable error and returns a CLIError.
func GetJSON[T any](ctx context.Context, activeCtx *config.Context, path string) (T, error) {
	var zero T
	url := strings.TrimRight(activeCtx.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return zero, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+activeCtx.Token)
	req.Header.Set("User-Agent", browserUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return zero, ctx.Err()
		}
		return zero, fmt.Errorf("request failed: %w", err)
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if closeErr := resp.Body.Close(); closeErr != nil {
		return zero, fmt.Errorf("closing response body: %w", closeErr)
	}
	if readErr != nil {
		return zero, fmt.Errorf("reading response: %w", readErr)
	}

	logVerbose(verboseFrom(ctx), http.MethodGet, url, resp.StatusCode, respBody)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var result T
		if len(respBody) > 0 {
			if err := json.Unmarshal(respBody, &result); err != nil {
				return zero, fmt.Errorf("decoding response: %w", err)
			}
		}
		return result, nil
	}

	return zero, clierror.New(
		statusToCode(resp.StatusCode),
		extractAPIError(respBody, resp.StatusCode),
		"",
	)
}

// PostJSONAPISingle POSTs a JSON body to path and decodes the JSON:API single-resource response.
// On non-2xx it extracts JSON:API errors[].detail/title and returns a CLIError.
func PostJSONAPISingle[T any](ctx context.Context, activeCtx *config.Context, path string, body any) (httpclient.Resource[T], error) {
	b, err := json.Marshal(body)
	if err != nil {
		return httpclient.Resource[T]{}, fmt.Errorf("encoding request body: %w", err)
	}

	url := strings.TrimRight(activeCtx.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return httpclient.Resource[T]{}, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/vnd.api+json")
	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("Authorization", "Bearer "+activeCtx.Token)
	req.Header.Set("User-Agent", browserUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return httpclient.Resource[T]{}, ctx.Err()
		}
		return httpclient.Resource[T]{}, fmt.Errorf("request failed: %w", err)
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if closeErr := resp.Body.Close(); closeErr != nil {
		return httpclient.Resource[T]{}, fmt.Errorf("closing response body: %w", closeErr)
	}
	if readErr != nil {
		return httpclient.Resource[T]{}, fmt.Errorf("reading response: %w", readErr)
	}

	logVerbose(verboseFrom(ctx), http.MethodPost, url, resp.StatusCode, respBody)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		type rawDoc struct {
			Data   rawResource       `json:"data"`
			Errors []jsonAPIErrEntry `json:"errors"`
		}
		var doc rawDoc
		if len(respBody) > 0 {
			if err := json.Unmarshal(respBody, &doc); err != nil {
				return httpclient.Resource[T]{}, fmt.Errorf("decoding response: %w", err)
			}
		}
		if len(doc.Errors) > 0 {
			return httpclient.Resource[T]{}, extractJSONAPIError(doc.Errors)
		}
		res := httpclient.Resource[T]{ID: doc.Data.ID, Type: doc.Data.Type}
		if len(doc.Data.Attributes) > 0 {
			if err := json.Unmarshal(doc.Data.Attributes, &res.Attributes); err != nil {
				return httpclient.Resource[T]{}, fmt.Errorf("decoding attributes: %w", err)
			}
		}
		return res, nil
	}

	return httpclient.Resource[T]{}, clierror.New(
		statusToCode(resp.StatusCode),
		extractJSONAPIOrFlatError(respBody, resp.StatusCode),
		"",
	)
}

// PostJSON POSTs body to path and decodes the flat JSON response body directly into T.
// Pass nil body to send a request with no body.
// On non-2xx it extracts JSON:API or flat error info and returns a CLIError.
func PostJSON[T any](ctx context.Context, activeCtx *config.Context, path string, body any) (T, error) {
	var zero T
	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			return zero, fmt.Errorf("encoding request body: %w", err)
		}
	}

	url := strings.TrimRight(activeCtx.BaseURL, "/") + path
	var bodyReader io.Reader
	if reqBody != nil {
		bodyReader = bytes.NewReader(reqBody)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bodyReader)
	if err != nil {
		return zero, fmt.Errorf("building request: %w", err)
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+activeCtx.Token)
	req.Header.Set("User-Agent", browserUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return zero, ctx.Err()
		}
		return zero, fmt.Errorf("request failed: %w", err)
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if closeErr := resp.Body.Close(); closeErr != nil {
		return zero, fmt.Errorf("closing response body: %w", closeErr)
	}
	if readErr != nil {
		return zero, fmt.Errorf("reading response: %w", readErr)
	}

	logVerbose(verboseFrom(ctx), http.MethodPost, url, resp.StatusCode, respBody)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var result T
		if len(respBody) > 0 {
			if err := json.Unmarshal(respBody, &result); err != nil {
				return zero, fmt.Errorf("decoding response: %w", err)
			}
		}
		return result, nil
	}

	return zero, clierror.New(
		statusToCode(resp.StatusCode),
		extractJSONAPIOrFlatError(respBody, resp.StatusCode),
		"",
	)
}

// PostJSONBodyJSONAPIResponse POSTs a JSON body to path and decodes the JSON:API single-resource response.
// Unlike PostJSONAPISingle, it sets Content-Type: application/json (not application/vnd.api+json).
// On non-2xx it extracts JSON:API errors[].detail/title and returns a CLIError.
func PostJSONBodyJSONAPIResponse[T any](ctx context.Context, activeCtx *config.Context, path string, body any) (httpclient.Resource[T], error) {
	b, err := json.Marshal(body)
	if err != nil {
		return httpclient.Resource[T]{}, fmt.Errorf("encoding request body: %w", err)
	}

	url := strings.TrimRight(activeCtx.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return httpclient.Resource[T]{}, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("Authorization", "Bearer "+activeCtx.Token)
	req.Header.Set("User-Agent", browserUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return httpclient.Resource[T]{}, ctx.Err()
		}
		return httpclient.Resource[T]{}, fmt.Errorf("request failed: %w", err)
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if closeErr := resp.Body.Close(); closeErr != nil {
		return httpclient.Resource[T]{}, fmt.Errorf("closing response body: %w", closeErr)
	}
	if readErr != nil {
		return httpclient.Resource[T]{}, fmt.Errorf("reading response: %w", readErr)
	}

	logVerbose(verboseFrom(ctx), http.MethodPost, url, resp.StatusCode, respBody)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		type rawDoc struct {
			Data   rawResource       `json:"data"`
			Errors []jsonAPIErrEntry `json:"errors"`
		}
		var doc rawDoc
		if len(respBody) > 0 {
			if err := json.Unmarshal(respBody, &doc); err != nil {
				return httpclient.Resource[T]{}, fmt.Errorf("decoding response: %w", err)
			}
		}
		if len(doc.Errors) > 0 {
			return httpclient.Resource[T]{}, extractJSONAPIError(doc.Errors)
		}
		res := httpclient.Resource[T]{ID: doc.Data.ID, Type: doc.Data.Type}
		if len(doc.Data.Attributes) > 0 {
			if err := json.Unmarshal(doc.Data.Attributes, &res.Attributes); err != nil {
				return httpclient.Resource[T]{}, fmt.Errorf("decoding attributes: %w", err)
			}
		}
		return res, nil
	}

	return httpclient.Resource[T]{}, clierror.New(
		statusToCode(resp.StatusCode),
		extractJSONAPIOrFlatError(respBody, resp.StatusCode),
		"",
	)
}

// PatchJSONBodyJSONAPIResponse PATCHes a JSON body to path and decodes the JSON:API single-resource response.
// Unlike PatchJSONAPISingle, it sets Content-Type: application/json (not application/vnd.api+json).
// On non-2xx it extracts JSON:API errors[].detail/title and returns a CLIError.
func PatchJSONBodyJSONAPIResponse[T any](ctx context.Context, activeCtx *config.Context, path string, body any) (httpclient.Resource[T], error) {
	b, err := json.Marshal(body)
	if err != nil {
		return httpclient.Resource[T]{}, fmt.Errorf("encoding request body: %w", err)
	}

	url := strings.TrimRight(activeCtx.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(b))
	if err != nil {
		return httpclient.Resource[T]{}, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("Authorization", "Bearer "+activeCtx.Token)
	req.Header.Set("User-Agent", browserUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return httpclient.Resource[T]{}, ctx.Err()
		}
		return httpclient.Resource[T]{}, fmt.Errorf("request failed: %w", err)
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if closeErr := resp.Body.Close(); closeErr != nil {
		return httpclient.Resource[T]{}, fmt.Errorf("closing response body: %w", closeErr)
	}
	if readErr != nil {
		return httpclient.Resource[T]{}, fmt.Errorf("reading response: %w", readErr)
	}

	logVerbose(verboseFrom(ctx), http.MethodPatch, url, resp.StatusCode, respBody)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		type rawDoc struct {
			Data   rawResource       `json:"data"`
			Errors []jsonAPIErrEntry `json:"errors"`
		}
		var doc rawDoc
		if len(respBody) > 0 {
			if err := json.Unmarshal(respBody, &doc); err != nil {
				return httpclient.Resource[T]{}, fmt.Errorf("decoding response: %w", err)
			}
		}
		if len(doc.Errors) > 0 {
			return httpclient.Resource[T]{}, extractJSONAPIError(doc.Errors)
		}
		res := httpclient.Resource[T]{ID: doc.Data.ID, Type: doc.Data.Type}
		if len(doc.Data.Attributes) > 0 {
			if err := json.Unmarshal(doc.Data.Attributes, &res.Attributes); err != nil {
				return httpclient.Resource[T]{}, fmt.Errorf("decoding attributes: %w", err)
			}
		}
		return res, nil
	}

	return httpclient.Resource[T]{}, clierror.New(
		statusToCode(resp.StatusCode),
		extractJSONAPIOrFlatError(respBody, resp.StatusCode),
		"",
	)
}

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

	logVerbose(verboseFrom(ctx), http.MethodPost, url, resp.StatusCode, respBody)

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

// PatchJSONAPISingle PATCHes path with a JSON body and decodes the JSON:API single-resource response.
// On non-2xx it extracts JSON:API errors[].detail/title and returns a CLIError.
func PatchJSONAPISingle[T any](ctx context.Context, activeCtx *config.Context, path string, body any) (httpclient.Resource[T], error) {
	b, err := json.Marshal(body)
	if err != nil {
		return httpclient.Resource[T]{}, fmt.Errorf("encoding request body: %w", err)
	}

	url := strings.TrimRight(activeCtx.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(b))
	if err != nil {
		return httpclient.Resource[T]{}, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/vnd.api+json")
	req.Header.Set("Authorization", "Bearer "+activeCtx.Token)
	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("User-Agent", browserUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return httpclient.Resource[T]{}, ctx.Err()
		}
		return httpclient.Resource[T]{}, fmt.Errorf("request failed: %w", err)
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if closeErr := resp.Body.Close(); closeErr != nil {
		return httpclient.Resource[T]{}, fmt.Errorf("closing response body: %w", closeErr)
	}
	if readErr != nil {
		return httpclient.Resource[T]{}, fmt.Errorf("reading response: %w", readErr)
	}

	logVerbose(verboseFrom(ctx), http.MethodPatch, url, resp.StatusCode, respBody)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		type patchRawDoc struct {
			Data   rawResource       `json:"data"`
			Errors []jsonAPIErrEntry `json:"errors"`
		}
		var doc patchRawDoc
		if len(respBody) > 0 {
			if err := json.Unmarshal(respBody, &doc); err != nil {
				return httpclient.Resource[T]{}, fmt.Errorf("decoding response: %w", err)
			}
		}
		if len(doc.Errors) > 0 {
			return httpclient.Resource[T]{}, extractJSONAPIError(doc.Errors)
		}
		res := httpclient.Resource[T]{ID: doc.Data.ID, Type: doc.Data.Type}
		if len(doc.Data.Attributes) > 0 {
			if err := json.Unmarshal(doc.Data.Attributes, &res.Attributes); err != nil {
				return httpclient.Resource[T]{}, fmt.Errorf("decoding attributes: %w", err)
			}
		}
		return res, nil
	}

	return httpclient.Resource[T]{}, clierror.New(
		statusToCode(resp.StatusCode),
		extractJSONAPIOrFlatError(respBody, resp.StatusCode),
		"",
	)
}

type jsonAPIErrEntry struct {
	Status string `json:"status"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

func extractJSONAPIError(errs []jsonAPIErrEntry) error {
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		if e.Detail != "" {
			parts = append(parts, e.Detail)
		} else if e.Title != "" {
			parts = append(parts, e.Title)
		}
	}
	if len(parts) == 0 {
		return clierror.New(clierror.CodeServerError, "JSON:API error", "")
	}
	return clierror.New(clierror.CodeBadRequest, strings.Join(parts, "; "), "")
}

func extractJSONAPIOrFlatError(body []byte, status int) string {
	if len(body) > 0 {
		var errDoc struct {
			Errors []jsonAPIErrEntry `json:"errors"`
		}
		if json.Unmarshal(body, &errDoc) == nil && len(errDoc.Errors) > 0 {
			parts := make([]string, 0, len(errDoc.Errors))
			for _, e := range errDoc.Errors {
				if e.Detail != "" {
					parts = append(parts, e.Detail)
				} else if e.Title != "" {
					parts = append(parts, e.Title)
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, "; ")
			}
		}
	}
	return http.StatusText(status)
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
	return http.StatusText(status)
}

// DeleteJSONAPI issues a DELETE to path and returns nil on 2xx (including 204).
// On non-2xx it extracts JSON:API errors and returns a CLIError.
func DeleteJSONAPI(ctx context.Context, activeCtx *config.Context, path string) error {
	url := strings.TrimRight(activeCtx.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("Authorization", "Bearer "+activeCtx.Token)
	req.Header.Set("User-Agent", browserUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("request failed: %w", err)
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if closeErr := resp.Body.Close(); closeErr != nil {
		return fmt.Errorf("closing response body: %w", closeErr)
	}
	if readErr != nil {
		return fmt.Errorf("reading response: %w", readErr)
	}

	logVerbose(verboseFrom(ctx), http.MethodDelete, url, resp.StatusCode, respBody)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return clierror.New(
		statusToCode(resp.StatusCode),
		extractJSONAPIOrFlatError(respBody, resp.StatusCode),
		"",
	)
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

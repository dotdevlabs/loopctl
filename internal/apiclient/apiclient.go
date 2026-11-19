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
	resourceData
	Relationships map[string]json.RawMessage `json:"relationships,omitempty"`
	Links         rawResourceLinks           `json:"links,omitempty"`
}

type resourceData struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Attributes json.RawMessage `json:"attributes"`
}

// rawCollectionLinks holds the top-level links on a JSON:API collection response.
type rawCollectionLinks struct {
	First string `json:"first,omitempty"`
	Prev  string `json:"prev,omitempty"`
	Next  string `json:"next,omitempty"`
	Last  string `json:"last,omitempty"`
}

// ResourceIdentifier identifies a resource in JSON:API relationship linkage.
type ResourceIdentifier struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// IncludedResource retains attributes for deferred, lossless decoding.
type IncludedResource struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Attributes json.RawMessage `json:"attributes"`
}

// SingleResult wraps a JSON:API resource with its links.self URL.
type SingleResult[T any] struct {
	httpclient.Resource[T]
	SelfLink      string
	Relationships map[string]json.RawMessage
	Included      []IncludedResource
	response      *http.Response
}

func (r SingleResult[T]) SelectRelationships(names ...string) map[string]json.RawMessage {
	for _, name := range names {
		ObserveResponse(r.response, "/data/relationships/"+name, json.RawMessage(nil))
	}
	return SelectRelationships(r.Relationships, names...)
}

func (r SingleResult[T]) Decode(path string, data []byte, target any) error {
	return DecodeResponse(r.response, path, data, target)
}

func (r SingleResult[T]) Self() string {
	ObserveResponse(r.response, "/data/links", rawResourceLinks{})
	return r.SelfLink
}

func (r SingleResult[T]) IncludedResources() []IncludedResource {
	ObserveResponse(r.response, "/included", []IncludedResource{})
	return r.Included
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
	return requestJSONAPISingle[T](ctx, activeCtx, http.MethodGet, path, nil)
}

func PostJSONAPIAction[T any](ctx context.Context, activeCtx *config.Context, path string) (httpclient.Resource[T], error) {
	res, err := requestJSONAPISingle[T](ctx, activeCtx, http.MethodPost, path, nil)
	return res.Resource, err
}

func WriteJSONAPISingleFull[T any](ctx context.Context, activeCtx *config.Context, method, path string, body any) (SingleResult[T], error) {
	b, err := json.Marshal(body)
	if err != nil {
		return SingleResult[T]{}, fmt.Errorf("encoding request body: %w", err)
	}
	return requestJSONAPISingle[T](ctx, activeCtx, method, path, bytes.NewReader(b))
}

func requestJSONAPISingle[T any](ctx context.Context, activeCtx *config.Context, method, path string, body io.Reader) (SingleResult[T], error) {
	fullURL := strings.TrimRight(activeCtx.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return SingleResult[T]{}, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.api+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
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

	respBody = sanitizeProjectResponse(ctx, req.URL.Path, respBody)
	logVerbose(verboseFrom(ctx), method, fullURL, resp.StatusCode, respBody)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		type rawDoc struct {
			Included []IncludedResource `json:"included"`
			Data     rawResource        `json:"data"`
			Errors   []jsonAPIErrEntry  `json:"errors"`
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
		ObserveResponse(resp, "/data", &doc.Data.resourceData)
		res := httpclient.Resource[T]{ID: doc.Data.ID, Type: doc.Data.Type}
		if len(doc.Data.Attributes) > 0 {
			if err := DecodeResponse(resp, "/data/attributes", doc.Data.Attributes, &res.Attributes); err != nil {
				return SingleResult[T]{}, fmt.Errorf("decoding attributes: %w", err)
			}
		}
		return SingleResult[T]{Resource: res, SelfLink: doc.Data.Links.Self, Relationships: doc.Data.Relationships, Included: doc.Included, response: resp}, nil
	}

	return SingleResult[T]{}, &HTTPError{StatusCode: resp.StatusCode, Err: responseError(resp, respBody)}
}

// rawCollectionDoc is the wire-format wrapper for a JSON:API collection response.
type rawCollectionDoc struct {
	collectionPagination
	Data   []rawResource     `json:"data"`
	Errors []jsonAPIErrEntry `json:"errors"`
}

type collectionPagination struct {
	Links rawCollectionLinks `json:"links"`
	Meta  json.RawMessage    `json:"meta,omitempty"`
}

type CollectionResource[T any] struct {
	httpclient.Resource[T]
	Relationships map[string]json.RawMessage `json:"relationships,omitempty"`
	response      *http.Response
}

func (r CollectionResource[T]) DecodeRelationship(name string, target any) error {
	return decodeRelationship(r.response, "/data/*/relationships/"+name, r.Relationships[name], target)
}

func (r SingleResult[T]) DecodeRelationship(name string, target any) error {
	return decodeRelationship(r.response, "/data/relationships/"+name, r.Relationships[name], target)
}

func decodeRelationship(resp *http.Response, path string, data json.RawMessage, target any) error {
	if len(data) == 0 {
		ObserveResponse(resp, path, target)
		return nil
	}
	return DecodeResponse(resp, path, data, target)
}

type Collection[T any] struct {
	Data     []CollectionResource[T] `json:"data"`
	Links    httpclient.Links        `json:"links"`
	Meta     map[string]any          `json:"meta,omitempty"`
	PageMeta httpclient.PageMeta
}

// fetchJSONAPICollectionPage fetches one page from an absolute URL and decodes it.
func fetchJSONAPICollectionPage[T any](ctx context.Context, token, fullURL string, paginated bool, relationships ...string) (Collection[T], error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return Collection[T]{}, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", browserUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return Collection[T]{}, ctx.Err()
		}
		return Collection[T]{}, fmt.Errorf("request failed: %w", err)
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if closeErr := resp.Body.Close(); closeErr != nil {
		return Collection[T]{}, fmt.Errorf("closing response body: %w", closeErr)
	}
	if readErr != nil {
		return Collection[T]{}, fmt.Errorf("reading response: %w", readErr)
	}

	respBody = sanitizeProjectResponse(ctx, req.URL.Path, respBody)
	logVerbose(verboseFrom(ctx), http.MethodGet, fullURL, resp.StatusCode, respBody)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var doc rawCollectionDoc
		if len(respBody) > 0 {
			if err := json.Unmarshal(respBody, &doc); err != nil {
				return Collection[T]{}, fmt.Errorf("decoding response: %w", err)
			}
		}
		if len(doc.Errors) > 0 {
			return Collection[T]{}, extractJSONAPIError(doc.Errors)
		}
		col := Collection[T]{
			Data: make([]CollectionResource[T], len(doc.Data)),
		}
		if paginated {
			ObserveResponse(resp, "", &doc.collectionPagination)
			col.Links = httpclient.Links{
				First: doc.Links.First,
				Prev:  doc.Links.Prev,
				Next:  doc.Links.Next,
				Last:  doc.Links.Last,
			}
			if len(doc.Meta) != 0 {
				if err := json.Unmarshal(doc.Meta, &col.Meta); err != nil {
					return Collection[T]{}, fmt.Errorf("decoding pagination metadata: %w", err)
				}
			}
		}
		for i, item := range doc.Data {
			ObserveResponse(resp, "/data/*", &item.resourceData)
			res := httpclient.Resource[T]{ID: item.ID, Type: item.Type}
			if len(item.Attributes) > 0 {
				if err := DecodeResponse(resp, "/data/*/attributes", item.Attributes, &res.Attributes); err != nil {
					return Collection[T]{}, fmt.Errorf("decoding attributes[%d]: %w", i, err)
				}
			}
			col.Data[i] = CollectionResource[T]{Resource: res, Relationships: SelectRelationships(item.Relationships, relationships...), response: resp}
			for _, name := range relationships {
				ObserveResponse(resp, "/data/*/relationships/"+name, json.RawMessage(nil))
			}
		}
		return col, nil
	}

	return Collection[T]{}, &HTTPError{StatusCode: resp.StatusCode, Err: responseError(resp, respBody)}
}

func SelectRelationships(available map[string]json.RawMessage, names ...string) map[string]json.RawMessage {
	selected := make(map[string]json.RawMessage, len(names))
	for _, name := range names {
		if value, ok := available[name]; ok {
			selected[name] = value
		}
	}
	return selected
}

// GetJSONAPICollection GETs path and decodes the JSON:API collection response.
// On non-2xx it extracts the human-readable error and returns a CLIError.
func GetJSONAPICollection[T any](ctx context.Context, activeCtx *config.Context, path string, relationships ...string) (Collection[T], error) {
	fullURL := strings.TrimRight(activeCtx.BaseURL, "/") + path
	return fetchJSONAPICollectionPage[T](ctx, activeCtx.Token, fullURL, true, relationships...)
}

func GetJSONAPIUnpaginatedCollection[T any](ctx context.Context, activeCtx *config.Context, path string, relationships ...string) (Collection[T], error) {
	fullURL := strings.TrimRight(activeCtx.BaseURL, "/") + path
	return fetchJSONAPICollectionPage[T](ctx, activeCtx.Token, fullURL, false, relationships...)
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
func GetJSONAPICollectionAllPages[T any](ctx context.Context, activeCtx *config.Context, path string, relationships ...string) (Collection[T], error) {
	result, err := GetJSONAPICollection[T](ctx, activeCtx, path, relationships...)
	if err != nil {
		return Collection[T]{}, err
	}

	const maxPages = 1000
	for page := 1; result.Links.Next != "" && page < maxPages; page++ {
		nextURL := resolveNextURL(result.Links.Next, activeCtx.BaseURL)
		nextPage, err := fetchJSONAPICollectionPage[T](ctx, activeCtx.Token, nextURL, true, relationships...)
		if err != nil {
			return Collection[T]{}, err
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

	return httpclient.Envelope[T]{}, errorRespFlat(respBody, resp.StatusCode)
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

	return zero, errorRespFlat(respBody, resp.StatusCode)
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

	return httpclient.Resource[T]{}, errorRespJSONAPI(respBody, resp.StatusCode)
}

// PostJSON POSTs body to path and decodes the flat JSON response body directly into T.
// Pass nil body to send a request with no body.
// On non-2xx it extracts JSON:API or flat error info and returns a CLIError.
func PostJSON[T any](ctx context.Context, activeCtx *config.Context, path string, body any) (T, error) {
	return postJSON[T](ctx, activeCtx, path, "application/json", body)
}

func PostJSONAPIResponse[T any](ctx context.Context, activeCtx *config.Context, path string, body any) (T, error) {
	return postJSON[T](ctx, activeCtx, path, "application/vnd.api+json", body)
}

func postJSON[T any](ctx context.Context, activeCtx *config.Context, path, accept string, body any) (T, error) {
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
	req.Header.Set("Accept", accept)
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
			if err := DecodeResponse(resp, "", respBody, &result); err != nil {
				return zero, fmt.Errorf("decoding response: %w", err)
			}
		}
		return result, nil
	}

	return zero, responseError(resp, respBody)
}

// PostJSONFlatErr POSTs body to path and decodes the flat JSON response body directly into T.
// Pass nil body to send a request with no body.
// On non-2xx it extracts the top-level error/message/errors field from a flat JSON body.
func PostJSONFlatErr[T any](ctx context.Context, activeCtx *config.Context, path string, body any) (T, error) {
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

	resp, err := http.DefaultClient.Do(req) //#nosec G107 -- URL is constructed from trusted config
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

	return zero, errorRespFlat(respBody, resp.StatusCode)
}

// PostJSONBodyJSONAPIResponse POSTs a JSON body to path and decodes the JSON:API single-resource response.
// Unlike PostJSONAPISingle, it sets Content-Type: application/json (not application/vnd.api+json).
// On non-2xx it extracts JSON:API errors[].detail/title and returns a CLIError.
func PostJSONBodyJSONAPIResponse[T any](ctx context.Context, activeCtx *config.Context, path string, body any) (httpclient.Resource[T], error) {
	res, err := WriteJSONAPISingleFull[T](ctx, activeCtx, http.MethodPost, path, body)
	return res.Resource, err
}

// PatchJSONBodyJSONAPIResponse PATCHes a JSON body to path and decodes the JSON:API single-resource response.
// Unlike PatchJSONAPISingle, it sets Content-Type: application/json (not application/vnd.api+json).
// On non-2xx it extracts JSON:API errors[].detail/title and returns a CLIError.
func PatchJSONBodyJSONAPIResponse[T any](ctx context.Context, activeCtx *config.Context, path string, body any) (httpclient.Resource[T], error) {
	res, err := WriteJSONAPISingleFull[T](ctx, activeCtx, http.MethodPatch, path, body)
	return res.Resource, err
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

	return httpclient.Envelope[T]{}, errorRespFlat(respBody, resp.StatusCode)
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

	return httpclient.Resource[T]{}, errorRespJSONAPI(respBody, resp.StatusCode)
}

type jsonAPIErrEntry struct {
	Status string `json:"status"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

type jsonAPIErrorDocument struct {
	Errors []jsonAPIErrEntry `json:"errors"`
}

func responseError(resp *http.Response, body []byte) error {
	if resp.StatusCode == http.StatusPaymentRequired && !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/vnd.api+json") {
		return parse402(body)
	}
	return DecodeAPIError(resp, body)
}

func DecodeAPIError(resp *http.Response, body []byte) error {
	mediaType := resp.Header.Get("Content-Type")
	if len(body) != 0 && strings.HasPrefix(mediaType, "application/vnd.api+json") {
		ObserveResponse(resp, "", &jsonAPIErrorDocument{})
	}
	return clierror.New(statusToCode(resp.StatusCode), extractAPIError(body, resp.StatusCode), "")
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
	return extractAPIError(body, status)
}

func extractAPIError(body []byte, status int) string {
	if len(body) > 0 {
		var errDoc jsonAPIErrorDocument
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
	if raw := strings.TrimSpace(string(body)); raw != "" {
		return raw
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

	return errorRespJSONAPI(respBody, resp.StatusCode)
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

// GetRawContent GETs path and streams the raw response body to dst.
// It uses a redirect-safe client that drops the Authorization header when
// redirected to a different host (preventing credential forwarding to CDN/S3).
// On non-2xx it returns a CLIError; body bytes are never logged to verbose.
func GetRawContent(ctx context.Context, activeCtx *config.Context, path string, dst io.Writer) error {
	fullURL := strings.TrimRight(activeCtx.BaseURL, "/") + path
	originalHost := hostOf(fullURL)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			if req.URL.Host != originalHost {
				req.Header.Del("Authorization")
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+activeCtx.Token)
	req.Header.Set("User-Agent", browserUserAgent)

	resp, err := client.Do(req) //#nosec G107 -- URL is constructed from trusted config
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	vw := verboseFrom(ctx)
	if vw != nil {
		fmt.Fprintf(vw, "> %s %s\n", http.MethodGet, fullURL)
		fmt.Fprintf(vw, "< %d %s\n", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if _, err := io.Copy(dst, resp.Body); err != nil {
			return fmt.Errorf("reading content: %w", err)
		}
		return nil
	}

	errBody, _ := io.ReadAll(resp.Body)
	switch resp.StatusCode {
	case http.StatusNotFound:
		return clierror.New(clierror.CodeNotFound, extractJSONAPIOrFlatError(errBody, resp.StatusCode), "")
	case http.StatusGone:
		msg := extractJSONAPIOrFlatError(errBody, resp.StatusCode)
		if msg == http.StatusText(http.StatusGone) {
			msg = "recording file has been deleted"
		}
		return clierror.New(clierror.CodeNotFound, msg, "")
	case http.StatusUnprocessableEntity:
		return clierror.New(clierror.CodeBadRequest, extractJSONAPIOrFlatError(errBody, resp.StatusCode), "")
	default:
		return errorRespJSONAPI(errBody, resp.StatusCode)
	}
}

// hostOf extracts the host (host:port) from a URL string, or returns "" on parse error.
func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Host
}

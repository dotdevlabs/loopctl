// Package schema provides the API contract types, loader, and request conformance checker.
package schema

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// SourceURL is the canonical location of the API contract document.
const SourceURL = "https://raw.githubusercontent.com/dotdevlabs/loopcontrol/main/docs/api_contract.json"

//go:embed testdata/schema.json
var fixtureData []byte

// Field describes a single field in a request body.
type Field struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// RequestBody describes the expected request body for an endpoint.
type RequestBody struct {
	ContentType string  `json:"content_type"`
	Nesting     string  `json:"nesting"`
	Required    []Field `json:"required"`
	Optional    []Field `json:"optional"`
}

// EndpointAttrs holds the machine-readable attributes for one API endpoint.
type EndpointAttrs struct {
	Method      string       `json:"http_method"`
	Path        string       `json:"path"`
	Description string       `json:"description"`
	RequestBody *RequestBody `json:"-"`
}

type rawEndpointAttrs struct {
	Method      string          `json:"http_method"`
	Path        string          `json:"path"`
	Description string          `json:"description"`
	RequestBody json.RawMessage `json:"request_body"`
}

type rawEndpoint struct {
	ID    string           `json:"id"`
	Attrs rawEndpointAttrs `json:"attributes"`
}

type schemaDoc struct {
	Data []rawEndpoint `json:"data"`
}

// Load parses the embedded schema fixture and returns all endpoint definitions.
func Load() ([]EndpointAttrs, error) {
	return parseSchema(fixtureData)
}

func parseSchema(data []byte) ([]EndpointAttrs, error) {
	var doc schemaDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing schema: %w", err)
	}
	endpoints := make([]EndpointAttrs, 0, len(doc.Data))
	for _, raw := range doc.Data {
		ep := EndpointAttrs{
			Method:      raw.Attrs.Method,
			Path:        raw.Attrs.Path,
			Description: raw.Attrs.Description,
		}
		if len(raw.Attrs.RequestBody) > 0 && string(raw.Attrs.RequestBody) != "null" {
			var rb RequestBody
			if err := json.Unmarshal(raw.Attrs.RequestBody, &rb); err != nil {
				return nil, fmt.Errorf("parsing request_body for %s %s: %w", ep.Method, ep.Path, err)
			}
			ep.RequestBody = &rb
		}
		endpoints = append(endpoints, ep)
	}
	return endpoints, nil
}

// MatchPath returns true when actualPath matches the schema template path.
// Template segments starting with ":" are treated as wildcards.
func MatchPath(template, actual string) bool {
	tParts := strings.Split(template, "/")
	aParts := strings.Split(actual, "/")
	if len(tParts) != len(aParts) {
		return false
	}
	for i, tp := range tParts {
		if strings.HasPrefix(tp, ":") {
			continue
		}
		if tp != aParts[i] {
			return false
		}
	}
	return true
}

// FindEndpoint returns the first endpoint in endpoints whose method and path match.
func FindEndpoint(endpoints []EndpointAttrs, method, path string) *EndpointAttrs {
	for i := range endpoints {
		ep := &endpoints[i]
		if !strings.EqualFold(ep.Method, method) {
			continue
		}
		if MatchPath(ep.Path, path) {
			return ep
		}
	}
	return nil
}

// CheckRequest validates r against endpoints and returns a list of violation strings.
// It checks:
//   - The endpoint (method + path) is declared in the schema.
//   - For requests with a body: every field in the body is in the endpoint's allowed set.
//   - The nesting key is present if the schema requires it.
func CheckRequest(r *http.Request, endpoints []EndpointAttrs) []string {
	// Strip query string from path for matching.
	path := r.URL.Path

	ep := FindEndpoint(endpoints, r.Method, path)
	if ep == nil {
		return []string{fmt.Sprintf("endpoint not in schema: %s %s", r.Method, path)}
	}

	// Read body without consuming it.
	var bodyBytes []byte
	if r.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(r.Body)
		if err == nil {
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
	}

	if len(bodyBytes) == 0 {
		if ep.RequestBody != nil && len(ep.RequestBody.Required) > 0 {
			return []string{fmt.Sprintf("required body missing for %s %s", r.Method, path)}
		}
		return nil
	}

	// There is a body — check it against the schema.
	if ep.RequestBody == nil {
		return []string{fmt.Sprintf("body sent but schema declares request_body null for %s %s", r.Method, path)}
	}

	var topLevel map[string]json.RawMessage
	if err := json.Unmarshal(bodyBytes, &topLevel); err != nil {
		return []string{fmt.Sprintf("body is not valid JSON object: %v", err)}
	}

	var toCheck map[string]json.RawMessage

	if ep.RequestBody.Nesting != "" {
		raw, ok := topLevel[ep.RequestBody.Nesting]
		if !ok {
			return []string{fmt.Sprintf("nesting key %q missing from body", ep.RequestBody.Nesting)}
		}
		if err := json.Unmarshal(raw, &toCheck); err != nil {
			return []string{fmt.Sprintf("nesting key %q is not a JSON object: %v", ep.RequestBody.Nesting, err)}
		}
		// Also check that there are no unexpected top-level keys.
		var topViolations []string
		for key := range topLevel {
			if key != ep.RequestBody.Nesting {
				topViolations = append(topViolations, fmt.Sprintf("unexpected top-level key %q (only %q is allowed)", key, ep.RequestBody.Nesting))
			}
		}
		if len(topViolations) > 0 {
			return topViolations
		}
	} else {
		toCheck = topLevel
	}

	allowed := make(map[string]bool)
	for _, f := range ep.RequestBody.Required {
		allowed[f.Name] = true
	}
	for _, f := range ep.RequestBody.Optional {
		allowed[f.Name] = true
	}

	var violations []string
	for field := range toCheck {
		if !allowed[field] {
			violations = append(violations, fmt.Sprintf("field %q not in schema for %s %s", field, r.Method, path))
		}
	}
	return violations
}

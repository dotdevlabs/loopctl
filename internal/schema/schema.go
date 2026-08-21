// Package schema provides the API contract types, loader, and request conformance checker.
package schema

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// SourceURL is the canonical location of the published OpenAPI spec via the GitHub Contents API.
// Fetching with Accept: application/vnd.github.raw+json returns the raw file content without CDN caching.
const SourceURL = "https://api.github.com/repos/dotdevlabs/loopcontrol/contents/docs/api_spec.yaml"

//go:embed testdata/api_spec.yaml
var fixtureData []byte

// Field describes a single field in a request body.
type Field struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	ItemFields  []Field `json:"item_fields,omitempty"`
}

// RequestBody describes the expected request body for an endpoint.
type RequestBody struct {
	ContentType string  `json:"content_type"`
	Nesting     string  `json:"nesting"`
	Required    []Field `json:"required"`
	Optional    []Field `json:"optional"`
}

// QueryParam describes a query parameter declared in the spec for an operation.
type QueryParam struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Type     string `json:"type"` // "string", "integer", "boolean"
}

// EndpointAttrs holds the machine-readable attributes for one API endpoint.
type EndpointAttrs struct {
	Method      string       `json:"http_method"`
	Path        string       `json:"path"`
	OperationID string       `json:"operation_id"`
	Description string       `json:"description"`
	PathParams  []string     `json:"path_params,omitempty"`
	QueryParams []QueryParam `json:"query_params,omitempty"`
	RequestBody *RequestBody `json:"-"`
	IsPaginated bool         `json:"is_paginated"`
}

// OpenAPI YAML structure types (unexported).

type oaSpec struct {
	Paths map[string]oaPathItem `yaml:"paths"`
}

type oaPathItem struct {
	Get    *oaOperation `yaml:"get"`
	Post   *oaOperation `yaml:"post"`
	Patch  *oaOperation `yaml:"patch"`
	Put    *oaOperation `yaml:"put"`
	Delete *oaOperation `yaml:"delete"`
}

type oaOperation struct {
	OperationID string                     `yaml:"operationId"`
	Summary     string                     `yaml:"summary"`
	Parameters  []oaParameter              `yaml:"parameters"`
	RequestBody *oaRequestBody             `yaml:"requestBody"`
	Responses   map[string]oaResponseEntry `yaml:"responses"`
}

type oaParameter struct {
	Name     string   `yaml:"name"`
	In       string   `yaml:"in"` // "path", "query", "header"
	Required bool     `yaml:"required"`
	Schema   oaSchema `yaml:"schema"`
}

type oaResponseEntry struct {
	Content map[string]oaMediaType `yaml:"content"`
}

type oaRequestBody struct {
	Content map[string]oaMediaType `yaml:"content"`
}

type oaMediaType struct {
	Schema oaSchema `yaml:"schema"`
}

type oaSchema struct {
	Type       string              `yaml:"type"`
	Required   []string            `yaml:"required"`
	Properties map[string]oaSchema `yaml:"properties"`
	Items      *oaSchema           `yaml:"items"`
}

var oaParamRE = regexp.MustCompile(`\{([^}]+)\}`)

// oaPathToTemplate converts an OpenAPI path like /api/tasks/{id} to /api/tasks/:id.
func oaPathToTemplate(p string) string {
	return oaParamRE.ReplaceAllString(p, `:$1`)
}

// Load parses the embedded OpenAPI spec and returns all endpoint definitions.
func Load() ([]EndpointAttrs, error) {
	return parseOpenAPI(fixtureData)
}

func parseOpenAPI(data []byte) ([]EndpointAttrs, error) {
	var spec oaSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parsing api_spec.yaml: %w", err)
	}

	var endpoints []EndpointAttrs
	for path, pathItem := range spec.Paths {
		template := oaPathToTemplate(path)
		pathParams := extractPathParams(path)
		ops := []struct {
			method string
			op     *oaOperation
		}{
			{"GET", pathItem.Get},
			{"POST", pathItem.Post},
			{"PATCH", pathItem.Patch},
			{"PUT", pathItem.Put},
			{"DELETE", pathItem.Delete},
		}
		for _, o := range ops {
			if o.op == nil {
				continue
			}
			ep := EndpointAttrs{
				Method:      strings.ToUpper(o.method),
				Path:        template,
				OperationID: o.op.OperationID,
				Description: o.op.Summary,
				PathParams:  pathParams,
			}
			for _, p := range o.op.Parameters {
				if p.In == "query" {
					ep.QueryParams = append(ep.QueryParams, QueryParam{
						Name:     p.Name,
						Required: p.Required,
						Type:     p.Schema.Type,
					})
				}
			}
			if o.op.RequestBody != nil {
				ep.RequestBody = extractRequestBody(o.op.RequestBody)
			}
			ep.IsPaginated = responseIsPaginated(o.op.Responses)
			endpoints = append(endpoints, ep)
		}
	}
	return endpoints, nil
}

// extractPathParams returns the parameter names from OpenAPI path template segments like {task_id}.
func extractPathParams(path string) []string {
	matches := oaParamRE.FindAllStringSubmatch(path, -1)
	if len(matches) == 0 {
		return nil
	}
	params := make([]string, 0, len(matches))
	for _, m := range matches {
		params = append(params, m[1])
	}
	return params
}

// responseIsPaginated returns true when the 200 response schema requires both "links" and "meta".
func responseIsPaginated(responses map[string]oaResponseEntry) bool {
	entry, ok := responses["200"]
	if !ok {
		return false
	}
	for _, mt := range entry.Content {
		schema := mt.Schema
		hasLinks, hasMeta := false, false
		for _, req := range schema.Required {
			switch req {
			case "links":
				hasLinks = true
			case "meta":
				hasMeta = true
			}
		}
		if hasLinks && hasMeta {
			return true
		}
	}
	return false
}

// extractRequestBody parses an OpenAPI requestBody into a RequestBody.
// It only processes application/json content. If the top-level schema has exactly
// one property of type object, that property is the nesting key; otherwise no nesting.
// Array fields are included but item-level field checking is skipped when the spec
// defines array items as type: object with no properties (i.e. any item keys allowed).
func extractRequestBody(rb *oaRequestBody) *RequestBody {
	mt, ok := rb.Content["application/json"]
	if !ok {
		return nil
	}

	schema := mt.Schema
	if schema.Type != "object" || len(schema.Properties) == 0 {
		return nil
	}

	result := &RequestBody{ContentType: "application/json"}

	// Detect nesting: exactly one top-level property of type object.
	if len(schema.Properties) == 1 {
		for nestKey, nestSchema := range schema.Properties {
			if nestSchema.Type == "object" {
				result.Nesting = nestKey
				extractFields(nestSchema, result)
				return result
			}
		}
	}

	// No nesting — extract fields from top-level schema.
	extractFields(schema, result)
	return result
}

func extractFields(schema oaSchema, result *RequestBody) {
	requiredSet := make(map[string]bool, len(schema.Required))
	for _, r := range schema.Required {
		requiredSet[r] = true
	}

	for fieldName, fieldSchema := range schema.Properties {
		f := Field{
			Name: fieldName,
			Type: fieldSchema.Type,
		}
		// For array fields: only populate ItemFields when the spec defines item properties.
		if fieldSchema.Type == "array" && fieldSchema.Items != nil && len(fieldSchema.Items.Properties) > 0 {
			for itemName := range fieldSchema.Items.Properties {
				f.ItemFields = append(f.ItemFields, Field{Name: itemName})
			}
		}
		if requiredSet[fieldName] {
			result.Required = append(result.Required, f)
		} else {
			result.Optional = append(result.Optional, f)
		}
	}
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

// CheckQuery validates that every query parameter in r.URL.Query() is documented
// in the spec for this endpoint. Returns violation strings for any undocumented params.
func CheckQuery(r *http.Request, ep *EndpointAttrs) []string {
	if ep == nil {
		return nil
	}
	allowed := make(map[string]bool, len(ep.QueryParams))
	for _, qp := range ep.QueryParams {
		allowed[qp.Name] = true
	}
	var violations []string
	for key := range r.URL.Query() {
		if !allowed[key] {
			violations = append(violations, fmt.Sprintf("query param %q not documented in spec for %s %s", key, ep.Method, ep.Path))
		}
	}
	return violations
}

// CheckRequest validates r against endpoints and returns a list of violation strings.
// It checks:
//   - The endpoint (method + path) is declared in the schema.
//   - For requests with a body: every field in the body is in the endpoint's allowed set.
//   - The nesting key is present if the schema requires it.
func CheckRequest(r *http.Request, endpoints []EndpointAttrs) []string {
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

	fieldMeta := make(map[string]Field)
	for _, f := range ep.RequestBody.Required {
		fieldMeta[f.Name] = f
	}
	for _, f := range ep.RequestBody.Optional {
		fieldMeta[f.Name] = f
	}

	var violations []string
	for field, rawVal := range toCheck {
		meta, ok := fieldMeta[field]
		if !ok {
			violations = append(violations, fmt.Sprintf("field %q not in schema for %s %s", field, r.Method, path))
			continue
		}
		// For array fields with item_fields defined, validate each item's keys.
		if meta.Type == "array" && len(meta.ItemFields) > 0 {
			var items []json.RawMessage
			if err := json.Unmarshal(rawVal, &items); err != nil {
				violations = append(violations, fmt.Sprintf("field %q is not a valid JSON array: %v", field, err))
				continue
			}
			allowedItemFields := make(map[string]bool, len(meta.ItemFields))
			for _, itemField := range meta.ItemFields {
				allowedItemFields[itemField.Name] = true
			}
			for i, item := range items {
				var itemObj map[string]json.RawMessage
				if err := json.Unmarshal(item, &itemObj); err != nil {
					violations = append(violations, fmt.Sprintf("field %q item[%d] is not a JSON object: %v", field, i, err))
					continue
				}
				for key := range itemObj {
					if !allowedItemFields[key] {
						violations = append(violations, fmt.Sprintf("field %q item[%d] has unknown key %q", field, i, key))
					}
				}
			}
		}
	}
	return violations
}

package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
)

const sensitiveValueKey contextKey = 2

// WithSensitiveValue protects a submitted secret if the server echoes it in a
// project response, including an error or an otherwise public attribute.
func WithSensitiveValue(ctx context.Context, value string) context.Context {
	return context.WithValue(ctx, sensitiveValueKey, value)
}

// sanitizeProjectResponse runs before logging, error construction, or decoding.
// Decode first so JSON escaping cannot bypass redaction. Project responses must
// be JSON; withhold malformed bodies rather than expose an unparseable secret.
func sanitizeProjectResponse(ctx context.Context, path string, body []byte) []byte {
	if path != "/api/projects" && !strings.HasPrefix(path, "/api/projects/") && !strings.HasPrefix(path, "/api/projects?") {
		return body
	}
	if len(body) == 0 {
		return body
	}
	var doc any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if !json.Valid(body) || decoder.Decode(&doc) != nil {
		return []byte(`invalid project response (body withheld)`)
	}
	secrets := []string{}
	if value, _ := ctx.Value(sensitiveValueKey).(string); value != "" {
		secrets = append(secrets, value)
	}
	var collect func(any)
	collect = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			for key, child := range v {
				if key == "webhook_signing_secret" {
					if secret, ok := child.(string); ok && secret != "" {
						secrets = append(secrets, secret)
					}
					delete(v, key)
				} else {
					collect(child)
				}
			}
		case []any:
			for _, child := range v {
				collect(child)
			}
		}
	}
	collect(doc)
	// Validation errors may embed a JSON-encoded value inside their detail string.
	for _, secret := range append([]string(nil), secrets...) {
		encoded, _ := json.Marshal(secret)
		secrets = append(secrets, string(encoded[1:len(encoded)-1]))
	}
	var redact func(any) any
	redact = func(value any) any {
		switch v := value.(type) {
		case string:
			for _, secret := range secrets {
				v = strings.ReplaceAll(v, secret, "[REDACTED]")
			}
			return v
		case map[string]any:
			result := make(map[string]any, len(v))
			for key, child := range v {
				result[redact(key).(string)] = redact(child)
			}
			return result
		case []any:
			for i, child := range v {
				v[i] = redact(child)
			}
		}
		return value
	}
	// The decoded tree contains only JSON values, so marshaling cannot fail.
	result, _ := json.Marshal(redact(doc))
	return result
}

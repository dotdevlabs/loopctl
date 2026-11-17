package apiclient

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestSanitizeProjectResponse(t *testing.T) {
	secret := "private-\"key\\<&>"
	encoded, _ := json.Marshal(secret)
	body, _ := json.Marshal(map[string]any{
		"webhook_signing_secret": secret,
		"detail":                 "validation: " + secret + " encoded: " + string(encoded),
		"items":                  []any{map[string]any{"webhook_signing_secret": "other-secret", "name": "other-secret"}, secret},
		"number":                 json.Number("9007199254740993"),
	})
	for _, path := range []string{"/api/projects", "/api/projects/p1", "/api/projects?page=2"} {
		t.Run(path, func(t *testing.T) {
			got := sanitizeProjectResponse(context.Background(), path, body)
			for _, forbidden := range []string{"webhook_signing_secret", "private-", "other-secret"} {
				if strings.Contains(string(got), forbidden) {
					t.Fatalf("leak: %s", got)
				}
			}
			if !strings.Contains(string(got), "9007199254740993") {
				t.Fatalf("number changed: %s", got)
			}
		})
	}
	if got := sanitizeProjectResponse(context.Background(), "/api/tasks", body); string(got) != string(body) {
		t.Fatal("unrelated diagnostic changed")
	}
	if got := sanitizeProjectResponse(context.Background(), "/api/projects", nil); len(got) != 0 {
		t.Fatal("empty response changed")
	}
	ctx := WithSensitiveValue(context.Background(), secret)
	if got := sanitizeProjectResponse(ctx, "/api/projects/p1", []byte(`{"error":`+string(encoded)+`}`)); strings.Contains(string(got), "private-") {
		t.Fatalf("submitted value leaked: %s", got)
	}
	if got := sanitizeProjectResponse(ctx, "/api/projects/p1", []byte(secret)); strings.Contains(string(got), "private-") {
		t.Fatal("malformed body leaked")
	}
}
